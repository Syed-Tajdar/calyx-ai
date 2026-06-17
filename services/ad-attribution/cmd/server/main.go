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

	"github.com/Syed-Tajdar/call-loom-intelligence/services/ad-attribution/internal/config"
	"github.com/Syed-Tajdar/call-loom-intelligence/services/ad-attribution/internal/consumer"
	"github.com/Syed-Tajdar/call-loom-intelligence/services/ad-attribution/internal/dedup"
	"github.com/Syed-Tajdar/call-loom-intelligence/services/ad-attribution/internal/dispatcher"
	"github.com/Syed-Tajdar/call-loom-intelligence/services/ad-attribution/internal/mockbridge"
	"github.com/Syed-Tajdar/call-loom-intelligence/services/ad-attribution/internal/providerconfig"
	"github.com/Syed-Tajdar/call-loom-intelligence/services/ad-attribution/internal/publisher"
	"github.com/Syed-Tajdar/call-loom-intelligence/services/ad-attribution/pkg/clickhouse"
	"github.com/Syed-Tajdar/call-loom-intelligence/services/ad-attribution/pkg/kafka"
	redisclient "github.com/Syed-Tajdar/call-loom-intelligence/services/ad-attribution/pkg/redis"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	cfg, err := config.Load()
	if err != nil {
		logger.Error("config load failed", "service", "ad-attribution", "error", err)
		os.Exit(1)
	}

	redisClient, err := redisclient.NewClient(cfg.RedisURL)
	if err != nil {
		logger.Error("redis connect failed", "service", "ad-attribution", "error", err)
		os.Exit(1)
	}
	defer redisClient.Close()

	chClient, err := clickhouse.NewClient(cfg.ClickHouseAddr, cfg.ClickHouseDB, cfg.ClickHouseUser, cfg.ClickHousePass)
	if err != nil {
		logger.Error("clickhouse connect failed", "service", "ad-attribution", "error", err)
		os.Exit(1)
	}
	defer chClient.Close()

	convReader := kafka.NewReader(cfg.KafkaBrokers, cfg.TopicConversions, cfg.ConsumerGroup)
	defer convReader.Close()

	pub := publisher.NewService(cfg.KafkaBrokers, cfg.TopicAttributed, cfg.TopicDLQ)
	defer pub.Close()

	dedupChecker := dedup.NewChecker(redisClient, cfg.DedupTTL)
	dispatch := dispatcher.NewService(cfg.FireMaxRetries)
	resolver := providerconfig.NewClient(cfg.ProviderConfigURL)
	consumerSvc := consumer.NewService(cfg, convReader, chClient, dedupChecker, dispatch, pub, resolver, logger)

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
		if err := convReader.Ping(ctx); err != nil {
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
			logger.Error("consumer stopped", "service", "ad-attribution", "error", err)
		}
	}()

	if cfg.MockBridgeEnabled {
		pingReader := kafka.NewReader(cfg.KafkaBrokers, cfg.TopicPings, cfg.BridgeGroup)
		convWriter := kafka.NewWriter(cfg.KafkaBrokers, cfg.TopicConversions)
		bridge := mockbridge.NewService(cfg, pingReader, convWriter, logger)
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer pingReader.Close()
			defer convWriter.Close()
			if err := bridge.Run(ctx); err != nil {
				logger.Error("mock bridge stopped", "service", "ad-attribution", "error", err)
			}
		}()
	}

	go func() {
		logger.Info("ad-attribution listening",
			"service", "ad-attribution",
			"addr", addr,
			"mock_mode", cfg.MockMode,
			"mock_bridge", cfg.MockBridgeEnabled,
			"attribution_provider", cfg.AttributionProvider,
		)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server error", "service", "ad-attribution", "error", err)
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
