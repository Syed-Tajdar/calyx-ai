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

	"github.com/Syed-Tajdar/calyx-ai/services/scoring/internal/config"
	"github.com/Syed-Tajdar/calyx-ai/services/scoring/internal/consumer"
	"github.com/Syed-Tajdar/calyx-ai/services/scoring/internal/mockemitter"
	"github.com/Syed-Tajdar/calyx-ai/services/scoring/internal/scoring"
	"github.com/Syed-Tajdar/calyx-ai/services/scoring/pkg/clickhouse"
	"github.com/Syed-Tajdar/calyx-ai/services/scoring/pkg/kafka"
	redisclient "github.com/Syed-Tajdar/calyx-ai/services/scoring/pkg/redis"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	cfg, err := config.Load()
	if err != nil {
		logger.Error("config load failed", "service", "scoring", "error", err)
		os.Exit(1)
	}

	logger.Info("provider map resolved",
		"service", "scoring",
		"mock_mode", cfg.MockMode,
		"transcription", cfg.TranscriptionProvider,
		"ai", cfg.AIProvider,
		"recording", cfg.RecordingSource,
		"mock_emitter", cfg.MockEmitterEnabled,
	)

	redisClient, err := redisclient.NewClient(cfg.RedisURL)
	if err != nil {
		logger.Error("redis connect failed", "service", "scoring", "error", err)
		os.Exit(1)
	}
	defer redisClient.Close()

	chClient, err := clickhouse.NewClient(cfg.ClickHouseAddr, cfg.ClickHouseDB, cfg.ClickHouseUser, cfg.ClickHousePass)
	if err != nil {
		logger.Error("clickhouse connect failed", "service", "scoring", "error", err)
		os.Exit(1)
	}
	defer chClient.Close()

	ai := scoring.Registry(cfg.AIProvider, cfg.OpenAIAPIKey)
	engine := scoring.NewEngine(ai)
	idempotency := scoring.NewIdempotencyGuard(redisClient)

	transcriptReader := kafka.NewReader(cfg.KafkaBrokers, cfg.TopicTranscriptReady, cfg.ConsumerGroup)
	defer transcriptReader.Close()

	kafkaWriter := kafka.NewWriter(cfg.KafkaBrokers)
	defer kafkaWriter.Close()

	consumerSvc := consumer.NewService(cfg, transcriptReader, kafkaWriter, chClient, engine, idempotency, logger)

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
		if err := transcriptReader.Ping(ctx); err != nil {
			http.Error(w, "kafka unavailable", http.StatusServiceUnavailable)
			return
		}
		if err := redisClient.Ping(ctx).Err(); err != nil {
			http.Error(w, "redis unavailable", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	addr := fmt.Sprintf(":%s", cfg.Port)
	srv := &http.Server{Addr: addr, Handler: mux, ReadTimeout: 5 * time.Second, WriteTimeout: 10 * time.Second}

	runCtx, runCancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := consumerSvc.Run(runCtx); err != nil {
			logger.Error("scoring consumer stopped", "service", "scoring", "error", err)
		}
	}()

	if cfg.MockEmitterEnabled {
		pingReader := kafka.NewReader(cfg.KafkaBrokers, cfg.TopicPings, cfg.MockEmitterGroup)
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer pingReader.Close()
			emitter := mockemitter.NewServiceWithReader(cfg, pingReader, kafkaWriter, logger)
			if err := emitter.Run(runCtx); err != nil {
				logger.Error("mock emitter stopped", "service", "scoring", "error", err)
			}
		}()
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		logger.Info("scoring server starting", "service", "scoring", "addr", addr)
		if serveErr := srv.ListenAndServe(); serveErr != nil && serveErr != http.ErrServerClosed {
			logger.Error("server failed", "service", "scoring", "error", serveErr)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutdown signal received", "service", "scoring")
	runCancel()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
	wg.Wait()
	logger.Info("shutdown complete", "service", "scoring")
}
