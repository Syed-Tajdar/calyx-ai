package main



import (

	"context"

	"encoding/json"

	"log/slog"

	"net/http"

	"os"

	"os/signal"

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

	svc := syncer.NewService(st, ad, prod, dedupChecker, decryptor, logger)



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

			runCtx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)

			if err := svc.RunAll(runCtx); err != nil {

				logger.Error("scheduled sync failed", "error", redact.SanitizeErr(err))

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

	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)

	defer cancel()

	if err := fn(ctx, req.TenantID, req.IntegrationID); err != nil {

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


