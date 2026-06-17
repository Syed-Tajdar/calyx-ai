package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/Syed-Tajdar/calyx-ai/services/enrichment/internal/cache"
	"github.com/Syed-Tajdar/calyx-ai/services/enrichment/internal/config"
	"github.com/Syed-Tajdar/calyx-ai/services/enrichment/internal/consumer"
	"github.com/Syed-Tajdar/calyx-ai/services/enrichment/internal/providers"
	"github.com/Syed-Tajdar/calyx-ai/services/enrichment/pkg/clickhouse"
	"github.com/Syed-Tajdar/calyx-ai/services/enrichment/pkg/kafka"
	redisclient "github.com/Syed-Tajdar/calyx-ai/services/enrichment/pkg/redis"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	cfg, err := config.Load()
	if err != nil {
		logger.Error("config load failed", "service", "enrichment", "error", err)
		os.Exit(1)
	}

	logger.Info("provider map resolved",
		"service", "enrichment",
		"mock_mode", cfg.MockMode,
		"enrichment", cfg.EnrichmentProvider,
	)

	redisClient, err := redisclient.NewClient(cfg.RedisURL)
	if err != nil {
		logger.Error("redis connect failed", "service", "enrichment", "error", err)
		os.Exit(1)
	}
	defer redisClient.Close()

	chClient, err := clickhouse.NewClient(cfg.ClickHouseAddr, cfg.ClickHouseDB, cfg.ClickHouseUser, cfg.ClickHousePass)
	if err != nil {
		logger.Error("clickhouse connect failed", "service", "enrichment", "error", err)
		os.Exit(1)
	}
	defer chClient.Close()

	provider := providers.Registry(cfg.EnrichmentProvider, cfg.EnrollHereAPIKey, cfg.WhitepagesAPIKey)
	cacheStore := cache.NewStore(redisClient, cfg.CacheTTL)
	reader := kafka.NewReader(cfg.KafkaBrokers, cfg.TopicPings, cfg.ConsumerGroup)
	defer reader.Close()

	consumerSvc := consumer.NewService(cfg, reader, chClient, cacheStore, provider, logger)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()
		if err := chClient.Ping(ctx); err != nil {
			http.Error(w, "clickhouse unavailable", http.StatusServiceUnavailable)
			return
		}
		if err := reader.Ping(ctx); err != nil {
			http.Error(w, "kafka unavailable", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	addr := fmt.Sprintf(":%s", cfg.Port)
	srv := &http.Server{Addr: addr, Handler: mux}

	runCtx, runCancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := consumerSvc.Run(runCtx); err != nil {
			logger.Error("enrichment consumer stopped", "service", "enrichment", "error", err)
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		logger.Info("enrichment server starting", "service", "enrichment", "addr", addr)
		if serveErr := srv.ListenAndServe(); serveErr != nil && serveErr != http.ErrServerClosed {
			logger.Error("server failed", "service", "enrichment", "error", serveErr)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	runCancel()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
	wg.Wait()
}
