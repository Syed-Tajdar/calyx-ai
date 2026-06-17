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

	"github.com/Syed-Tajdar/call-loom-intelligence/services/ask-ai/internal/config"
	"github.com/Syed-Tajdar/call-loom-intelligence/services/ask-ai/internal/cost"
	"github.com/Syed-Tajdar/call-loom-intelligence/services/ask-ai/internal/embeddings"
	"github.com/Syed-Tajdar/call-loom-intelligence/services/ask-ai/internal/handlers"
	"github.com/Syed-Tajdar/call-loom-intelligence/services/ask-ai/internal/nlsql"
	"github.com/Syed-Tajdar/call-loom-intelligence/services/ask-ai/internal/nlquery"
	"github.com/Syed-Tajdar/call-loom-intelligence/services/ask-ai/internal/pipeline"
	"github.com/Syed-Tajdar/call-loom-intelligence/services/ask-ai/internal/providerconfig"
	chreader "github.com/Syed-Tajdar/call-loom-intelligence/services/ask-ai/pkg/clickhouse"
	pgstore "github.com/Syed-Tajdar/call-loom-intelligence/services/ask-ai/pkg/postgres"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	cfg, err := config.Load()
	if err != nil {
		logger.Error("config load failed", "service", "ask-ai", "error", err)
		os.Exit(1)
	}

	pgStore, err := pgstore.NewStore(cfg.DatabaseURL)
	if err != nil {
		logger.Error("postgres connect failed", "service", "ask-ai", "error", err)
		os.Exit(1)
	}
	defer pgStore.Close()

	chReader, err := chreader.NewReader(cfg.ClickHouseAddr, cfg.ClickHouseDB, cfg.ClickHouseUser, cfg.ClickHousePass)
	if err != nil {
		logger.Error("clickhouse connect failed", "service", "ask-ai", "error", err)
		os.Exit(1)
	}
	defer chReader.Close()

	llm := nlquery.Registry(cfg.LLMProvider, cfg.OpenAIAPIKey)
	executor := nlsql.NewExecutor(chReader, time.Duration(cfg.QueryTimeoutSec)*time.Second, cfg.MaxRows)
	cache := embeddings.NewCache(pgStore)
	budget := cost.NewBudgetGuard(cfg.TokenBudgetDaily)
	resolver := providerconfig.NewClient(cfg.ProviderConfigURL)
	pipe := pipeline.NewService(cfg, llm, executor, cache, pgStore, budget, resolver, logger)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()
		if err := pgStore.Ping(ctx); err != nil {
			http.Error(w, "postgres unavailable", http.StatusServiceUnavailable)
			return
		}
		if err := chReader.Ping(ctx); err != nil {
			http.Error(w, "clickhouse unavailable", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.Handle("/v1/ask", handlers.NewAskHandler(pipe, logger))
	mux.Handle("/v1/history", handlers.NewAskHandler(pipe, logger))

	addr := fmt.Sprintf(":%s", cfg.Port)
	srv := &http.Server{Addr: addr, Handler: mux}

	go func() {
		logger.Info("ask-ai listening",
			"service", "ask-ai",
			"addr", addr,
			"mock_mode", cfg.MockMode,
			"llm_provider", cfg.LLMProvider,
		)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server error", "service", "ask-ai", "error", err)
			os.Exit(1)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
}
