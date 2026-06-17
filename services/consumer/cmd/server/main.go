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

	"github.com/Syed-Tajdar/calyx-ai/services/consumer/internal/config"
	"github.com/Syed-Tajdar/calyx-ai/services/consumer/internal/consumer"
	"github.com/Syed-Tajdar/calyx-ai/services/consumer/internal/writer"
	"github.com/Syed-Tajdar/calyx-ai/services/consumer/pkg/clickhouse"
	"github.com/Syed-Tajdar/calyx-ai/services/consumer/pkg/kafka"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	cfg, err := config.Load()
	if err != nil {
		logger.Error("config load failed", "service", "consumer", "error", err)
		os.Exit(1)
	}

	chClient, err := clickhouse.NewClient(cfg.ClickHouseAddr, cfg.ClickHouseDB, cfg.ClickHouseUser, cfg.ClickHousePass)
	if err != nil {
		logger.Error("clickhouse connect failed", "service", "consumer", "error", err)
		os.Exit(1)
	}
	defer chClient.Close()

	batchWriter := writer.NewBatchWriter(chClient)
	kafkaReader := kafka.NewReader(cfg.KafkaBrokers, cfg.KafkaTopic, cfg.ConsumerGroup)
	consumerSvc := consumer.NewService(cfg, kafkaReader, batchWriter, logger)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()
		if err := batchWriter.Ping(ctx); err != nil {
			http.Error(w, "clickhouse unavailable", http.StatusServiceUnavailable)
			return
		}
		if err := kafkaReader.Ping(ctx); err != nil {
			http.Error(w, "kafka unavailable", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	addr := fmt.Sprintf(":%s", cfg.Port)
	srv := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	runCtx, runCancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		if runErr := consumerSvc.Run(runCtx); runErr != nil {
			logger.Error("consumer stopped", "service", "consumer", "error", runErr)
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		logger.Info("consumer health server starting", "service", "consumer", "addr", addr)
		if serveErr := srv.ListenAndServe(); serveErr != nil && serveErr != http.ErrServerClosed {
			logger.Error("health server failed", "service", "consumer", "error", serveErr)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutdown signal received", "service", "consumer")
	runCancel()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("http shutdown failed", "service", "consumer", "error", err)
	}

	wg.Wait()
	logger.Info("shutdown complete", "service", "consumer")
}
