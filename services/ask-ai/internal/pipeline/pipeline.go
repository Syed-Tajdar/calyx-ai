package pipeline

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/Syed-Tajdar/call-loom-intelligence/services/ask-ai/internal/config"
	"github.com/Syed-Tajdar/call-loom-intelligence/services/ask-ai/internal/cost"
	"github.com/Syed-Tajdar/call-loom-intelligence/services/ask-ai/internal/embeddings"
	"github.com/Syed-Tajdar/call-loom-intelligence/services/ask-ai/internal/models"
	"github.com/Syed-Tajdar/call-loom-intelligence/services/ask-ai/internal/nlsql"
	"github.com/Syed-Tajdar/call-loom-intelligence/services/ask-ai/internal/nlquery"
	"github.com/Syed-Tajdar/call-loom-intelligence/services/ask-ai/internal/providerconfig"
	pgstore "github.com/Syed-Tajdar/call-loom-intelligence/services/ask-ai/pkg/postgres"
	"github.com/google/uuid"
)

// Service orchestrates Ask AI question handling.
type Service struct {
	cfg        config.Config
	llm        nlquery.Provider
	executor   *nlsql.Executor
	cache      *embeddings.Cache
	store      *pgstore.Store
	budget     *cost.BudgetGuard
	resolver   *providerconfig.Client
	logger     *slog.Logger
}

// NewService creates the Ask AI pipeline.
func NewService(
	cfg config.Config,
	llm nlquery.Provider,
	executor *nlsql.Executor,
	cache *embeddings.Cache,
	store *pgstore.Store,
	budget *cost.BudgetGuard,
	resolver *providerconfig.Client,
	logger *slog.Logger,
) *Service {
	return &Service{
		cfg:      cfg,
		llm:      llm,
		executor: executor,
		cache:    cache,
		store:    store,
		budget:   budget,
		resolver: resolver,
		logger:   logger,
	}
}

// Ask processes a natural language question.
func (s *Service) Ask(ctx context.Context, req models.AskRequest) (models.AskResponse, error) {
	if req.TenantID == "" || req.Question == "" {
		return models.AskResponse{}, fmt.Errorf("pipeline.Ask: tenant_id and question required")
	}
	traceID := req.TraceID
	if traceID == "" {
		traceID = uuid.NewString()
	}

	if cached, hit, err := s.cache.Lookup(ctx, req.TenantID, req.Question); err != nil {
		return models.AskResponse{}, err
	} else if hit && cached != nil {
		s.logger.Info("ask cache hit", "service", "ask-ai", "tenant_id", req.TenantID, "trace_id", traceID)
		return *cached, nil
	}

	if s.resolver != nil {
		resolveCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		_, _ = s.resolver.Resolve(resolveCtx, req.TenantID, traceID)
		cancel()
	}

	intent := nlsql.ParseIntent(req.Question)
	sqlText, err := nlsql.BuildTemplateSQL(intent, req.TenantID, s.cfg.MaxRows)
	if err != nil {
		answer, tokens, fmtErr := s.llm.FormatAnswer(ctx, req.Question, string(intent), nil)
		if fmtErr != nil {
			return models.AskResponse{}, fmtErr
		}
		if budgetErr := s.budget.Allow(ctx, req.TenantID, tokens); budgetErr != nil {
			return models.AskResponse{}, budgetErr
		}
		return s.persist(ctx, req, string(intent), "", answer, 0, tokens, false)
	}
	if err := nlsql.Validate(sqlText, req.TenantID); err != nil {
		return models.AskResponse{}, err
	}

	rows, rowCount, err := s.executor.Execute(ctx, sqlText)
	if err != nil {
		return models.AskResponse{}, err
	}

	answer, tokens, err := s.llm.FormatAnswer(ctx, req.Question, string(intent), rows)
	if err != nil {
		return models.AskResponse{}, err
	}
	if err := s.budget.Allow(ctx, req.TenantID, tokens); err != nil {
		return models.AskResponse{}, err
	}

	return s.persist(ctx, req, string(intent), sqlText, answer, rowCount, tokens, false)
}

// ListHistory returns recent questions for a tenant.
func (s *Service) ListHistory(ctx context.Context, tenantID string, limit int) ([]models.HistoryRecord, error) {
	return s.store.ListHistory(ctx, tenantID, limit)
}

func (s *Service) persist(
	ctx context.Context,
	req models.AskRequest,
	intent, sqlText, answer string,
	rowCount, tokens int,
	cacheHit bool,
) (models.AskResponse, error) {
	id, err := s.store.InsertQueryLog(ctx, req.TenantID, req.Question, intent, answer, sqlText, rowCount, cacheHit, tokens)
	if err != nil {
		return models.AskResponse{}, err
	}
	resp := models.AskResponse{
		QueryID:    id,
		Question:   req.Question,
		Answer:     answer,
		Intent:     intent,
		CacheHit:   cacheHit,
		RowCount:   rowCount,
		TokensUsed: tokens,
	}
	s.logger.Info("ask answered",
		"service", "ask-ai",
		"tenant_id", req.TenantID,
		"query_id", id,
		"intent", intent,
		"row_count", rowCount,
		"cache_hit", cacheHit,
	)
	return resp, nil
}
