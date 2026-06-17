package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Syed-Tajdar/calyx-ai/services/providerconfig/internal/catalog"
	"github.com/Syed-Tajdar/calyx-ai/services/providerconfig/internal/config"
	"github.com/Syed-Tajdar/calyx-ai/services/providerconfig/internal/crypto"
	"github.com/Syed-Tajdar/calyx-ai/services/providerconfig/internal/handlers"
	"github.com/Syed-Tajdar/calyx-ai/services/providerconfig/internal/loader"
	"github.com/Syed-Tajdar/calyx-ai/services/providerconfig/internal/registry"
	redisclient "github.com/Syed-Tajdar/calyx-ai/services/providerconfig/pkg/redis"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	cfg, err := config.Load()
	if err != nil {
		logger.Error("config load failed", "service", "providerconfig", "error", err)
		os.Exit(1)
	}

	cat, err := catalog.Load(cfg.CatalogPath)
	if err != nil {
		logger.Error("catalog load failed", "service", "providerconfig", "error", err)
		os.Exit(1)
	}
	if err := catalog.ValidateAgainstRegistry(cat, registry.RegisteredAdapters()); err != nil {
		logger.Error("catalog registry sync failed", "service", "providerconfig", "error", err)
		os.Exit(1)
	}

	redisClient, err := redisclient.NewClient(cfg.RedisURL)
	if err != nil {
		logger.Error("redis connect failed", "service", "providerconfig", "error", err)
		os.Exit(1)
	}
	defer redisClient.Close()

	pgStore, err := loader.NewPostgresBindingStore(cfg.DatabaseURL)
	if err != nil {
		logger.Error("postgres connect failed", "service", "providerconfig", "error", err)
		os.Exit(1)
	}
	defer pgStore.Close()

	var decryptor *crypto.Decryptor
	if cfg.CredentialKeyHex != "" {
		decryptor, err = crypto.NewDecryptor(cfg.CredentialKeyHex)
		if err != nil {
			logger.Error("decryptor init failed", "service", "providerconfig", "error", err)
			os.Exit(1)
		}
	}

	secretStore := loader.NewPostgresSecretStore(pgStore.DB(), decryptor)
	resolver := loader.NewResolver(
		cfg,
		cat,
		pgStore,
		loader.NewRedisBindingCache(redisClient),
		secretStore,
		loader.NewLRUSecretCache(),
		decryptor,
	)

	mux := http.NewServeMux()
	mux.Handle("GET /healthz", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	mux.Handle("GET /readyz", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()
		if err := pgStore.Ping(ctx); err != nil {
			http.Error(w, "postgres unavailable", http.StatusServiceUnavailable)
			return
		}
		if err := redisClient.Ping(ctx).Err(); err != nil {
			http.Error(w, "redis unavailable", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	mux.Handle("POST /v1/resolve", handlers.NewResolveHandler(resolver, logger))

	addr := fmt.Sprintf(":%s", cfg.Port)
	srv := &http.Server{Addr: addr, Handler: mux}

	go func() {
		logger.Info("providerconfig listening", "service", "providerconfig", "addr", addr, "mock_mode", cfg.MockMode)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server error", "service", "providerconfig", "error", err)
			os.Exit(1)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
}
