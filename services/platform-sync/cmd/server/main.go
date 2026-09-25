package main

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
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
	logger.Info("platform-sync init", "dedup_mode", "content_hash", "mock_mode", cfg.MockMode, "writeback_enabled", cfg.WritebackEnabled, "sync_lock", "redis_setnx")

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("POST /sync", func(w http.ResponseWriter, r *http.Request) {
		handleIntegrationAction(w, r, logger, svc.RunOne)
	})
	mux.HandleFunc("POST /test-auth", func(w http.ResponseWriter, r *http.Request) {
		handleTestAuth(w, r, logger, svc)
	})

	if cfg.WritebackAuthToken != "" {
		mux.HandleFunc("POST /sync/window", writeback.RequireBearer(cfg.WritebackAuthToken, func(w http.ResponseWriter, r *http.Request) {
			handleSyncWindow(w, r, logger, svc)
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
	defer ticker.Stop()
	go func() {
		for range ticker.C {
			runCtx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
			if err := svc.RunAll(runCtx); err != nil {
				logger.Error("scheduled sync failed", "error", redact.SanitizeErr(err))
			}
			cancel()
		}
	}()

	sweepTicker := time.NewTicker(adapter.RingbaSweepInterval())
	defer sweepTicker.Stop()
	go func() {
		for range sweepTicker.C {
			runCtx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
			if err := svc.RunAllSweeps(runCtx); err != nil {
				logger.Error("scheduled ringba sweep failed", "error", redact.SanitizeErr(err))
			}
			cancel()
		}
	}()

	deepSweepTicker := time.NewTicker(adapter.RingbaDeepSweepInterval())
	defer deepSweepTicker.Stop()
	go func() {
		for range deepSweepTicker.C {
			runCtx, cancel := context.WithTimeout(context.Background(), 40*time.Minute)
			if err := svc.RunAllDeepSweeps(runCtx); err != nil {
				logger.Error("scheduled ringba deep sweep failed", "error", redact.SanitizeErr(err))
			}
			cancel()
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = server.Shutdown(shutdownCtx)
}

type syncWindowRequest struct {
	IntegrationID string `json:"integration_id"`
	TenantID      string `json:"tenant_id"`
	Start         string `json:"start"`
	End           string `json:"end"`
	Reason        string `json:"reason"`
}

func handleSyncWindow(w http.ResponseWriter, r *http.Request, logger *slog.Logger, svc *syncer.Service) {
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
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Minute)
	defer cancel()
	if err := svc.RunWindow(ctx, req.TenantID, req.IntegrationID, start.UTC(), end.UTC(), reason); err != nil {
		if errors.Is(err, syncer.ErrAlreadyRunning) {
			logger.Info("window sync rejected — already running",
				"integration_id", req.IntegrationID, "tenant_id", req.TenantID)
			http.Error(w, `{"status":"already_running"}`, http.StatusConflict)
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
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Minute)
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