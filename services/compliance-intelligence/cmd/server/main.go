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

	"github.com/Syed-Tajdar/calyx-ai/services/compliance-intelligence/internal/config"
	"github.com/Syed-Tajdar/calyx-ai/services/compliance-intelligence/internal/consumer"
	"github.com/Syed-Tajdar/calyx-ai/services/compliance-intelligence/internal/dedup"
	"github.com/Syed-Tajdar/calyx-ai/services/compliance-intelligence/internal/dnc"
	"github.com/Syed-Tajdar/calyx-ai/services/compliance-intelligence/internal/providerconfig"
	"github.com/Syed-Tajdar/calyx-ai/services/compliance-intelligence/internal/publisher"
	"github.com/Syed-Tajdar/calyx-ai/services/compliance-intelligence/pkg/clickhouse"
	"github.com/Syed-Tajdar/calyx-ai/services/compliance-intelligence/pkg/kafka"
	redisclient "github.com/Syed-Tajdar/calyx-ai/services/compliance-intelligence/pkg/redis"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	cfg, err := config.Load()
	if err != nil {
		logger.Error("config load failed", "service", "compliance-intelligence", "error", err)
		os.Exit(1)
	}

	redisClient, err := redisclient.NewClient(cfg.RedisURL)
	if err != nil {
		logger.Error("redis connect failed", "service", "compliance-intelligence", "error", err)
		os.Exit(1)
	}
	defer redisClient.Close()

	resolver := providerconfig.NewClient(cfg.ProviderConfigURL)

	chClient, err := clickhouse.NewClient(cfg.ClickHouseAddr, cfg.ClickHouseDB, cfg.ClickHouseUser, cfg.ClickHousePass)
	if err != nil {
		logger.Error("clickhouse connect failed", "service", "compliance-intelligence", "error", err)
		os.Exit(1)
	}
	defer chClient.Close()

	reader := kafka.NewReader(cfg.KafkaBrokers, cfg.TopicPings, cfg.ConsumerGroup)
	defer reader.Close()

	pub := publisher.NewService(cfg.KafkaBrokers, cfg.KafkaBrokers, cfg.TopicRiskScored, cfg.TopicViolationDetected)
	defer pub.Close()

	dncProvider := dnc.Registry(cfg.DNCProvider)
	dedupChecker := dedup.NewChecker(redisClient, cfg.DedupTTL)
	consumerSvc := consumer.NewService(cfg, reader, chClient, dedupChecker, dncProvider, pub, resolver, logger)

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

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := consumerSvc.Run(ctx); err != nil {
			logger.Error("consumer stopped", "service", "compliance-intelligence", "error", err)
		}
	}()

	go func() {
		logger.Info("compliance-intelligence listening",
			"service", "compliance-intelligence",
			"addr", addr,
			"mock_mode", cfg.MockMode,
			"dnc_provider", cfg.DNCProvider,
			"providerconfig_url", cfg.ProviderConfigURL,
		)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server error", "service", "compliance-intelligence", "error", err)
			os.Exit(1)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	_ = srv.Shutdown(shutdownCtx)
	wg.Wait()
}
