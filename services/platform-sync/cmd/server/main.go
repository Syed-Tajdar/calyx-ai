package main

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/Syed-Tajdar/calyx-ai/services/platform-sync/internal/adapter"
	"github.com/Syed-Tajdar/calyx-ai/services/platform-sync/internal/config"
	"github.com/Syed-Tajdar/calyx-ai/services/platform-sync/internal/crypto"
	"github.com/Syed-Tajdar/calyx-ai/services/platform-sync/internal/dedup"
	"github.com/Syed-Tajdar/calyx-ai/services/platform-sync/internal/producer"
	"github.com/Syed-Tajdar/calyx-ai/services/platform-sync/internal/publisherpause"
	"github.com/Syed-Tajdar/calyx-ai/services/platform-sync/internal/redact"
	"github.com/Syed-Tajdar/calyx-ai/services/platform-sync/internal/redishealth"
	"github.com/Syed-Tajdar/calyx-ai/services/platform-sync/internal/store"
	"github.com/Syed-Tajdar/calyx-ai/services/platform-sync/internal/syncer"
	"github.com/Syed-Tajdar/calyx-ai/services/platform-sync/internal/transcribe"
	"github.com/Syed-Tajdar/calyx-ai/services/platform-sync/internal/writeback"
	"github.com/Syed-Tajdar/calyx-ai/services/platform-sync/pkg/kafka"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

const shutdownBudget = 25 * time.Second

type integrationRequest struct {
	IntegrationID string `json:"integration_id"`
	TenantID      string `json:"tenant_id"`
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	cfg, err := config.Load()
	if err != nil {
		logger.Error("config failed", "error", err)
		os.Exit(1)
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Error("postgres failed", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	redisOpts, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		logger.Error("redis url failed", "error", err)
		os.Exit(1)
	}
	redisClient := redis.NewClient(redisOpts)

	readyPG, err := newReadyPostgres(cfg.DatabaseURL)
	if err != nil {
		logger.Error("readyz postgres pool failed", "error", err)
		os.Exit(1)
	}
	defer readyPG.Close()
	readyRedis, err := newReadyRedis(cfg.RedisURL)
	if err != nil {
		logger.Error("readyz redis client failed", "error", err)
		os.Exit(1)
	}
	defer readyRedis.Close()

	decryptor, err := crypto.NewCredentialDecryptor(cfg.CredentialEncryptionKey)
	if err != nil {
		logger.Error("decryptor failed", "error", err)
		os.Exit(1)
	}

	kafkaClient := kafka.NewClient(cfg.KafkaBrokers, cfg.KafkaTopic)
	defer kafkaClient.Close()

	st := store.NewIntegrationStore(pool)
	ad := adapter.NewRegistry()
	prod := producer.NewProducer(kafkaClient, cfg.KafkaTopic, cfg.ANIEncryptionKey)
	dedupChecker := dedup.NewChecker(redisClient, cfg.DedupTTL)
	syncLock := syncer.NewRedisSyncLocker(redisClient)
	svc := syncer.NewService(st, ad, prod, dedupChecker, decryptor, logger, syncLock)
	logger.Info("platform-sync init",
		"dedup_mode", "content_hash",
		"mock_mode", cfg.MockMode,
		"writeback_enabled", cfg.WritebackEnabled,
		"sync_lock", "redis_setnx_heartbeat",
		"lock_ttl", syncer.SyncLockTTL().String(),
		"lock_renew", syncer.SyncLockRenewInterval().String())

	// rootCtx cancelled on SIGTERM — tickers and HTTP sync/window share it (B6).
	rootCtx, rootCancel := context.WithCancel(context.Background())
	var inflight sync.WaitGroup

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		probeCtx, cancel := context.WithTimeout(r.Context(), 1500*time.Millisecond)
		defer cancel()
		if err := readyPG.Ping(probeCtx); err != nil {
			http.Error(w, "postgres unavailable", http.StatusServiceUnavailable)
			return
		}
		if err := readyRedis.Ping(probeCtx).Err(); err != nil {
			http.Error(w, "redis unavailable", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("GET /kafka-status", func(w http.ResponseWriter, r *http.Request) {
		probeCtx, cancel := context.WithTimeout(r.Context(), 1500*time.Millisecond)
		defer cancel()
		err := kafkaClient.Ping(probeCtx)
		w.Header().Set("Content-Type", "application/json")
		if err != nil {
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": false, "error": err.Error()})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
	})
	if cfg.WritebackAuthToken == "" {
		logger.Error("WRITEBACK_INTERNAL_TOKEN empty — POST /sync, /test-auth, /sync/window not registered")
	} else {
		mux.HandleFunc("POST /sync", writeback.RequireBearer(cfg.WritebackAuthToken, func(w http.ResponseWriter, r *http.Request) {
			inflight.Add(1)
			defer inflight.Done()
			handleIntegrationAction(w, r, logger, svc.RunOne, rootCtx)
		}))
		mux.HandleFunc("POST /test-auth", writeback.RequireBearer(cfg.WritebackAuthToken, func(w http.ResponseWriter, r *http.Request) {
			handleTestAuth(w, r, logger, svc)
		}))
		mux.HandleFunc("POST /sync/window", writeback.RequireBearer(cfg.WritebackAuthToken, func(w http.ResponseWriter, r *http.Request) {
			inflight.Add(1)
			defer inflight.Done()
			handleSyncWindow(w, r, logger, svc, rootCtx)
		}))
	}

	if cfg.WritebackAuthToken != "" {
		wb := writeback.NewService(st, st, decryptor, adapter.NewRingbaAdapter(adapter.NewHTTPClient()), writeback.NewRedisLocker(redisClient))
		wb.SetGlobalEnabled(cfg.WritebackEnabled)
		wb.SetSettleWindow(writeback.ParseSettleWindow(os.Getenv("WRITEBACK_SETTLE_SECONDS")))
		// Stage 4b: Retreaver joins the registry. It shares the sync per-company limiter and is
		// still behind WRITEBACK_ENABLED and each integration's writeback_enabled (both false
		// by default). Its settle window is separate until Phase 3 measures the real value.
		wb.Register(writeback.NewRetreaverWriter(
			adapter.NewRetreaverCallClient(),
			writeback.ParseSettleWindow(os.Getenv("WRITEBACK_SETTLE_SECONDS_RETREAVER")),
		))
		logger.Info("writeback registry", "platforms", strings.Join(wb.SupportedPlatforms(), ","))
		mux.HandleFunc("POST /writeback/convert", writeback.RequireBearer(cfg.WritebackAuthToken, func(w http.ResponseWriter, r *http.Request) {
			handleWriteback(w, r, logger, wb, "convert")
		}))
		mux.HandleFunc("POST /writeback/void", writeback.RequireBearer(cfg.WritebackAuthToken, func(w http.ResponseWriter, r *http.Request) {
			handleWriteback(w, r, logger, wb, "void")
		}))
	} else if cfg.WritebackEnabled {
		logger.Error("WRITEBACK_ENABLED is on but WRITEBACK_INTERNAL_TOKEN is empty — routes not registered")
	}

	// v2 Phase 2D: publisher pause / unpause. Same bearer token as write-back. The routes exist
	// whenever the token is set so the API always gets an audited answer; with
	// WORKFLOW_ACTIONS_ENABLED off (default) every request is refused (403) before any platform
	// call, and Ringba stays refused until WORKFLOW_RINGBA_PAUSE_VERIFIED is on.
	if cfg.WritebackAuthToken != "" {
		pp := publisherpause.NewService(
			publisherpause.Config{
				ActionsEnabled:      cfg.WorkflowActionsEnabled,
				RingbaPauseVerified: cfg.WorkflowRingbaPauseVerified,
			},
			st, st, decryptor,
			publisherpause.NewRedisLocker(redisClient),
			adapter.NewRingbaPublisherClient(),
			adapter.NewRetreaverAffiliateClient(),
		)
		logger.Info("publisher pause routes",
			"workflow_actions_enabled", cfg.WorkflowActionsEnabled,
			"workflow_ringba_pause_verified", cfg.WorkflowRingbaPauseVerified)
		mux.HandleFunc("POST /publisher/pause", writeback.RequireBearer(cfg.WritebackAuthToken,
			publisherpause.Handler(pp, publisherpause.ActionPause, logger)))
		mux.HandleFunc("POST /publisher/unpause", writeback.RequireBearer(cfg.WritebackAuthToken,
			publisherpause.Handler(pp, publisherpause.ActionUnpause, logger)))
	} else if cfg.WorkflowActionsEnabled {
		logger.Error("WORKFLOW_ACTIONS_ENABLED is on but WRITEBACK_INTERNAL_TOKEN is empty — publisher routes not registered")
	}

	// v2 Phase 2B: transcription worker, archive cleanup and the internal recording routes. Nothing
	// here starts unless FEATURE_TRANSCRIPTION=true; with it off no Deepgram, archive or recording
	// request is ever made. Separate goroutines: the Live sync and the write-back are not touched.
	tcfg, err := config.LoadTranscription()
	if err != nil {
		logger.Error("transcription config failed", "error", err)
		os.Exit(1)
	}
	if tcfg.Enabled {
		var archive transcribe.ObjectStore
		if tcfg.ArchiveConfigured() {
			archive = transcribe.NewS3Store(transcribe.S3Config{
				Endpoint: tcfg.ArchiveEndpoint, Region: tcfg.ArchiveRegion, Bucket: tcfg.ArchiveBucket,
				KeyID: tcfg.ArchiveKeyID, SecretKey: tcfg.ArchiveSecretKey,
			})
		}
		spanish, perr := regexp.Compile(tcfg.SpanishPattern)
		if perr != nil {
			logger.Error("TRANSCRIBE_SPANISH_CAMPAIGN_PATTERN is not a valid regexp", "error", perr)
			os.Exit(1)
		}
		tstore := transcribe.NewPGStore(pool)
		worker := &transcribe.Worker{
			Store:   tstore,
			Decrypt: decryptor.DecryptMap,
			Sources: map[string]transcribe.Source{
				"ringba":    transcribe.NewRingbaSource(adapter.NewRingbaAdapter(adapter.NewHTTPClient())),
				"retreaver": transcribe.NewRetreaverSource(adapter.NewRetreaverCallClient()),
			},
			Mock:    transcribe.MockSource{},
			DG:      transcribe.NewDeepgram(tcfg.DeepgramBaseURL, tcfg.DeepgramAPIKey),
			Archive: archive,
			Cfg: transcribe.WorkerConfig{
				DailyCapMinutes: tcfg.DailyCapMinutes, MaxAttempts: tcfg.MaxAttempts, SpanishPattern: spanish,
				AllowMock: os.Getenv("APP_ENV") != "production",
			},
			Logger: logger,
		}
		go worker.Run(rootCtx)
		go (&transcribe.Cleanup{Store: tstore, Archive: archive, RetentionMonths: tcfg.RetentionMonths, Logger: logger}).Run(rootCtx)
		if cfg.WritebackAuthToken != "" {
			rt := &transcribe.Routes{Store: tstore, Archive: archive, Logger: logger, Status: transcribe.StatusInfo{
				Enabled: true, DeepgramKeySet: tcfg.DeepgramAPIKey != "", ArchiveConfigured: tcfg.ArchiveConfigured(),
				DailyCapMinutes: tcfg.DailyCapMinutes, RetentionMonths: tcfg.RetentionMonths,
			}}
			mux.HandleFunc("GET /internal/transcription/status", writeback.RequireBearer(cfg.WritebackAuthToken, rt.StatusHandler))
			mux.HandleFunc("GET /internal/recordings/audio", writeback.RequireBearer(cfg.WritebackAuthToken, rt.Audio))
			mux.HandleFunc("GET /internal/recordings/evidence", writeback.RequireBearer(cfg.WritebackAuthToken, rt.Evidence))
		}
	}
	logger.Info("transcription", "enabled", tcfg.Enabled, "archive_configured", tcfg.ArchiveConfigured(),
		"deepgram_key_set", tcfg.DeepgramAPIKey != "", "daily_cap_minutes", tcfg.DailyCapMinutes)

	server := &http.Server{Addr: ":" + cfg.Port, Handler: mux}
	go func() {
		logger.Info("platform-sync starting", "addr", cfg.Port, "per_integration_sync_mode", true)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server failed", "error", err)
			os.Exit(1)
		}
	}()

	// Item 5: redis is watched, not assumed. While it is down no sync is attempted (that only
	// produced one "sync lock" error per integration per tick); each tick records the outage on
	// every integration instead, and the first tick after recovery runs at once.
	redisWatch := redishealth.New(func(ctx context.Context) error { return redisClient.Ping(ctx).Err() }, logger, "platform-sync")
	redisWatch.Start(rootCtx)

	ticker := time.NewTicker(cfg.SyncInterval)
	sweepTicker := time.NewTicker(adapter.RingbaSweepInterval())
	deepSweepTicker := time.NewTicker(adapter.RingbaDeepSweepInterval())
	var tickersWG sync.WaitGroup

	tickersWG.Add(1)
	go func() {
		defer tickersWG.Done()
		defer ticker.Stop()
		for {
			select {
			case <-rootCtx.Done():
				return
			case <-ticker.C:
			case <-redisWatch.Recovered():
			}
			inflight.Add(1)
			runCtx, cancel := context.WithTimeout(rootCtx, 10*time.Minute)
			if !redisWatch.Up() {
				svc.MarkAllUnavailable(runCtx, "redis unavailable: "+redisWatch.LastError())
			} else if err := svc.RunAll(runCtx); err != nil && !errors.Is(err, context.Canceled) {
				logger.Error("scheduled sync failed", "error", redact.SanitizeErr(err))
			}
			cancel()
			inflight.Done()
		}
	}()

	tickersWG.Add(1)
	go func() {
		defer tickersWG.Done()
		defer sweepTicker.Stop()
		for {
			select {
			case <-rootCtx.Done():
				return
			case <-sweepTicker.C:
				if !redisWatch.Up() {
					continue // the tick records the outage; the next hourly sweep covers the gap
				}
				inflight.Add(1)
				runCtx, cancel := context.WithTimeout(rootCtx, 30*time.Minute)
				if err := svc.RunAllSweeps(runCtx); err != nil && !errors.Is(err, context.Canceled) {
					logger.Error("scheduled ringba sweep failed", "error", redact.SanitizeErr(err))
				}
				cancel()
				inflight.Done()
			}
		}
	}()

	tickersWG.Add(1)
	go func() {
		defer tickersWG.Done()
		defer deepSweepTicker.Stop()
		for {
			select {
			case <-rootCtx.Done():
				return
			case <-deepSweepTicker.C:
				if !redisWatch.Up() {
					continue
				}
				inflight.Add(1)
				runCtx, cancel := context.WithTimeout(rootCtx, 40*time.Minute)
				if err := svc.RunAllDeepSweeps(runCtx); err != nil && !errors.Is(err, context.Canceled) {
					logger.Error("scheduled ringba deep sweep failed", "error", redact.SanitizeErr(err))
				}
				cancel()
				inflight.Done()
			}
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	started := time.Now()
	logger.Info("shutdown signal received — cancelling in-flight runs")
	rootCancel()

	done := make(chan struct{})
	go func() {
		inflight.Wait()
		close(done)
	}()
	select {
	case <-done:
		logger.Info("in-flight runs stopped; locks released via owner defer")
	case <-time.After(shutdownBudget - 2*time.Second):
		logger.Warn("shutdown wait timed out waiting for runs")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = server.Shutdown(shutdownCtx)
	logger.Info("shutdown complete", "shutdown_ms", time.Since(started).Milliseconds())
}

func newReadyPostgres(databaseURL string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, err
	}
	cfg.MaxConns = 2
	cfg.MinConns = 0
	cfg.MaxConnLifetime = 5 * time.Second
	cfg.HealthCheckPeriod = 30 * time.Second
	cfg.ConnConfig.ConnectTimeout = 1500 * time.Millisecond
	return pgxpool.NewWithConfig(context.Background(), cfg)
}

func newReadyRedis(redisURL string) (*redis.Client, error) {
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, err
	}
	opts.DialTimeout = 1500 * time.Millisecond
	opts.ReadTimeout = 1500 * time.Millisecond
	opts.WriteTimeout = 1500 * time.Millisecond
	opts.MaxRetries = 1
	if u, err := url.Parse(redisURL); err == nil && u.Hostname() != "" {
		host := u.Hostname()
		if net.ParseIP(host) == nil {
			opts.Addr = net.JoinHostPort(host, portOrDefault(u.Port(), "6379"))
		}
	}
	return redis.NewClient(opts), nil
}

func portOrDefault(p, d string) string {
	if p == "" {
		return d
	}
	return p
}

type syncWindowRequest struct {
	IntegrationID string `json:"integration_id"`
	TenantID      string `json:"tenant_id"`
	Start         string `json:"start"`
	End           string `json:"end"`
	Reason        string `json:"reason"`
}

func handleSyncWindow(w http.ResponseWriter, r *http.Request, logger *slog.Logger, svc *syncer.Service, shutdownCtx context.Context) {
	var req syncWindowRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if req.IntegrationID == "" || req.TenantID == "" || req.Start == "" || req.End == "" {
		http.Error(w, "integration_id, tenant_id, start, and end required", http.StatusBadRequest)
		return
	}
	start, err := time.Parse(time.RFC3339, req.Start)
	if err != nil {
		http.Error(w, "start must be RFC3339", http.StatusBadRequest)
		return
	}
	end, err := time.Parse(time.RFC3339, req.End)
	if err != nil {
		http.Error(w, "end must be RFC3339", http.StatusBadRequest)
		return
	}
	reason := req.Reason
	if reason == "" {
		reason = "manual"
	}
	ctx, cancel := context.WithTimeout(shutdownCtx, 30*time.Minute)
	defer cancel()
	if err := svc.RunWindow(ctx, req.TenantID, req.IntegrationID, start.UTC(), end.UTC(), reason); err != nil {
		if errors.Is(err, syncer.ErrAlreadyRunning) {
			logger.Info("window sync rejected — already running",
				"integration_id", req.IntegrationID, "tenant_id", req.TenantID)
			http.Error(w, `{"status":"already_running"}`, http.StatusConflict)
			return
		}
		if errors.Is(err, context.Canceled) {
			logger.Info("window sync cancelled by shutdown",
				"integration_id", req.IntegrationID)
			http.Error(w, `{"status":"cancelled"}`, http.StatusServiceUnavailable)
			return
		}
		logger.Error("window sync failed", "error", redact.SanitizeErr(err))
		http.Error(w, "request failed", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

func handleIntegrationAction(
	w http.ResponseWriter,
	r *http.Request,
	logger *slog.Logger,
	fn func(context.Context, string, string) error,
	shutdownCtx context.Context,
) {
	var req integrationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if req.IntegrationID == "" || req.TenantID == "" {
		http.Error(w, "integration_id and tenant_id required", http.StatusBadRequest)
		return
	}
	ctx, cancel := context.WithTimeout(shutdownCtx, 10*time.Minute)
	defer cancel()
	if err := fn(ctx, req.TenantID, req.IntegrationID); err != nil {
		if errors.Is(err, syncer.ErrAlreadyRunning) {
			logger.Info("api sync rejected — already running",
				"integration_id", req.IntegrationID, "tenant_id", req.TenantID)
			http.Error(w, `{"status":"already_running"}`, http.StatusConflict)
			return
		}
		logger.Error("request failed", "error", redact.SanitizeErr(err))
		http.Error(w, "request failed", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

func handleTestAuth(w http.ResponseWriter, r *http.Request, logger *slog.Logger, svc *syncer.Service) {
	var req integrationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if req.IntegrationID == "" || req.TenantID == "" {
		http.Error(w, "integration_id and tenant_id required", http.StatusBadRequest)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	if err := svc.TestAuth(ctx, req.TenantID, req.IntegrationID); err != nil {
		safeMsg := redact.SanitizeErr(err)
		logger.Error("test-auth failed", "error", safeMsg)
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"ok": "false", "message": safeMsg})
		return
	}
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"ok": "true", "message": "credentials valid"})
}

type convertHTTPRequest struct {
	IntegrationID string  `json:"integration_id"`
	TenantID      string  `json:"tenant_id"`
	UserID        string  `json:"user_id"`
	InboundCallID string  `json:"inbound_call_id"`
	Amount        float64 `json:"amount"`
}

func handleWriteback(w http.ResponseWriter, r *http.Request, logger *slog.Logger, wb *writeback.Service, action string) {
	var req convertHTTPRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if req.IntegrationID == "" || req.TenantID == "" || strings.TrimSpace(req.InboundCallID) == "" {
		http.Error(w, "integration_id, tenant_id, and inbound_call_id required", http.StatusBadRequest)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 45*time.Second)
	defer cancel()
	in := writeback.ConvertRequest{
		TenantID:      req.TenantID,
		UserID:        req.UserID,
		IntegrationID: req.IntegrationID,
		InboundCallID: req.InboundCallID,
		Amount:        req.Amount,
	}
	var result writeback.Result
	var err error
	if action == "void" {
		result, err = wb.Void(ctx, in)
	} else {
		result, err = wb.Convert(ctx, in)
	}
	w.Header().Set("Content-Type", "application/json")
	if err != nil {
		safeMsg := redact.SanitizeErr(err)
		logger.Error("writeback failed", "action", action, "error", safeMsg)
		status := writeback.HTTPStatus(err)
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(map[string]string{"message": safeMsg, "outcome": "failed"})
		return
	}
	if result.Outcome == "pending" {
		w.WriteHeader(http.StatusAccepted)
	} else {
		w.WriteHeader(http.StatusOK)
	}
	_ = json.NewEncoder(w).Encode(result)
}
