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

	"github.com/Syed-Tajdar/calyx-ai/services/ingestion/internal/config"
	"github.com/Syed-Tajdar/calyx-ai/services/ingestion/internal/dedup"
	"github.com/Syed-Tajdar/calyx-ai/services/ingestion/internal/handlers"
	"github.com/Syed-Tajdar/calyx-ai/services/ingestion/internal/producer"
	"github.com/Syed-Tajdar/calyx-ai/services/ingestion/internal/redishealth"
	"github.com/Syed-Tajdar/calyx-ai/services/ingestion/internal/store"
	redisclient "github.com/Syed-Tajdar/calyx-ai/services/ingestion/pkg/redis"
	"github.com/Syed-Tajdar/calyx-ai/services/ingestion/pkg/kafka"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	cfg, err := config.Load()
	if err != nil {
		logger.Error("config load failed", "service", "ingestion", "error", err)
		os.Exit(1)
	}

	// No ping-or-exit here (v1 sync health, item 5): on 2026-10-18 redis came up without a
	// network and ingestion crash-looped. The client dials lazily; the watch logs the outage once,
	// retries with backoff, and /readyz answers 503 until redis is back.
	redisClient, err := redisclient.NewLazyClient(cfg.RedisURL)
	if err != nil {
		logger.Error("redis url invalid", "service", "ingestion", "error", err)
		os.Exit(1)
	}
	defer redisClient.Close()
	rootCtx, rootCancel := context.WithCancel(context.Background())
	defer rootCancel()
	redishealth.New(func(ctx context.Context) error { return redisClient.Ping(ctx).Err() }, logger, "ingestion").Start(rootCtx)

	integrationStore, err := store.NewIntegrationStore(cfg.DatabaseURL)
	if err != nil {
		logger.Error("postgres connect failed", "service", "ingestion", "error", err)
		os.Exit(1)
	}
	defer integrationStore.Close()

	kafkaClient := kafka.NewClient(cfg.KafkaBrokers, cfg.KafkaTopic)
	defer kafkaClient.Close()

	dedupChecker := dedup.NewChecker(redisClient, cfg.DedupTTL)
	pingProducer := producer.NewProducer(kafkaClient, cfg.KafkaTopic, cfg.EncryptionKey)

	logger.Info("ingestion server init",
		"service", "ingestion",
		"dedup_mode", "content_hash",
	)

	health := handlers.NewHealthHandler(dedupChecker, pingProducer, integrationStore)
	webhook := handlers.NewWebhookHandler(cfg, integrationStore, dedupChecker, pingProducer, logger)
	callloomNative := handlers.NewCallLoomNativeHandler()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", health.Liveness)
	mux.HandleFunc("GET /readyz", health.Readiness)
	mux.Handle("GET /webhooks/", webhook)
	mux.Handle("POST /webhooks/", webhook)
	mux.HandleFunc("GET /sources/callloom-native/v1/pings", callloomNative.ListPings)

	addr := fmt.Sprintf(":%s", cfg.Port)
	srv := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		logger.Info("ingestion server starting", "service", "ingestion", "addr", addr)
		if serveErr := srv.ListenAndServe(); serveErr != nil && serveErr != http.ErrServerClosed {
			logger.Error("server failed", "service", "ingestion", "error", serveErr)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutdown signal received", "service", "ingestion")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("http shutdown failed", "service", "ingestion", "error", err)
	}

	if err := pingProducer.Flush(shutdownCtx); err != nil {
		logger.Error("kafka flush failed", "service", "ingestion", "error", err)
	}

	wg.Wait()
	logger.Info("shutdown complete", "service", "ingestion")
}
