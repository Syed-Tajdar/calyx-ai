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
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/Syed-Tajdar/calyx-ai/services/platform-sync/internal/adapter"
	"github.com/Syed-Tajdar/calyx-ai/services/platform-sync/internal/config"
	"github.com/Syed-Tajdar/calyx-ai/services/platform-sync/internal/crypto"
	"github.com/Syed-Tajdar/calyx-ai/services/platform-sync/internal/dedup"
	"github.com/Syed-Tajdar/calyx-ai/services/platform-sync/internal/producer"
	"github.com/Syed-Tajdar/calyx-ai/services/platform-sync/internal/redact"
	"github.com/Syed-Tajdar/calyx-ai/services/platform-sync/internal/store"
	"github.com/Syed-Tajdar/calyx-ai/services/platform-sync/internal/syncer"
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
	mux.HandleFunc("POST /sync", func(w http.ResponseWriter, r *http.Request) {
		inflight.Add(1)
		defer inflight.Done()
		handleIntegrationAction(w, r, logger, svc.RunOne, rootCtx)
	})
	mux.HandleFunc("POST /test-auth", func(w http.ResponseWriter, r *http.Request) {
		handleTestAuth(w, r, logger, svc)
	})

	if cfg.WritebackAuthToken != "" {
		mux.HandleFunc("POST /sync/window", writeback.RequireBearer(cfg.WritebackAuthToken, func(w http.ResponseWriter, r *http.Request) {
			inflight.Add(1)
			defer inflight.Done()
			handleSyncWindow(w, r, logger, svc, rootCtx)
		}))
	} else {
		logger.Error("WRITEBACK_INTERNAL_TOKEN empty — POST /sync/window not registered")
	}

	if cfg.WritebackEnabled {
		if cfg.WritebackAuthToken == "" {
			logger.Error("WRITEBACK_ENABLED is on but WRITEBACK_INTERNAL_TOKEN is empty — routes not registered")
		} else {
			wb := writeback.NewService(st, st, decryptor, adapter.NewRingbaAdapter(adapter.NewHTTPClient()), writeback.NewRedisLocker(redisClient))
			mux.HandleFunc("POST /writeback/convert", writeback.RequireBearer(cfg.WritebackAuthToken, func(w http.ResponseWriter, r *http.Request) {
				handleWriteback(w, r, logger, wb, "convert")
			}))
			mux.HandleFunc("POST /writeback/void", writeback.RequireBearer(cfg.WritebackAuthToken, func(w http.ResponseWriter, r *http.Request) {
				handleWriteback(w, r, logger, wb, "void")
			}))
		}
	}

	server := &http.Server{Addr: ":" + cfg.Port, Handler: mux}
	go func() {
		logger.Info("platform-sync starting", "addr", cfg.Port, "per_integration_sync_mode", true)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server failed", "error", err)
			os.Exit(1)
		}
	}()

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
				inflight.Add(1)
				runCtx, cancel := context.WithTimeout(rootCtx, 10*time.Minute)
				if err := svc.RunAll(runCtx); err != nil && !errors.Is(err, context.Canceled) {
					logger.Error("scheduled sync failed", "error", redact.SanitizeErr(err))
				}
				cancel()
				inflight.Done()
			}
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
