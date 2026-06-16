package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/Syed-Tajdar/call-loom-intelligence/services/scoring/internal/config"
	"github.com/Syed-Tajdar/call-loom-intelligence/services/scoring/internal/events"
	"github.com/Syed-Tajdar/call-loom-intelligence/services/scoring/internal/models"
	"github.com/Syed-Tajdar/call-loom-intelligence/services/scoring/internal/pii"
	"github.com/Syed-Tajdar/call-loom-intelligence/services/scoring/internal/scoring"
	"github.com/Syed-Tajdar/call-loom-intelligence/services/scoring/pkg/clickhouse"
	"github.com/Syed-Tajdar/call-loom-intelligence/services/scoring/pkg/kafka"
	"github.com/google/uuid"
	kafkago "github.com/segmentio/kafka-go"
)

// Service consumes call_transcript_ready events and scores calls.
type Service struct {
	cfg       config.Config
	reader    *kafka.Reader
	writer    *kafka.Writer
	ch        *clickhouse.Client
	engine    *scoring.Engine
	idempotency *scoring.IdempotencyGuard
	logger    *slog.Logger
}

// NewService creates the scoring consumer.
func NewService(
	cfg config.Config,
	reader *kafka.Reader,
	writer *kafka.Writer,
	ch *clickhouse.Client,
	engine *scoring.Engine,
	idempotency *scoring.IdempotencyGuard,
	logger *slog.Logger,
) *Service {
	return &Service{
		cfg:         cfg,
		reader:      reader,
		writer:      writer,
		ch:          ch,
		engine:      engine,
		idempotency: idempotency,
		logger:      logger,
	}
}

// Run blocks until context is cancelled.
func (s *Service) Run(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		readCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		msg, err := s.reader.Fetch(readCtx)
		cancel()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			continue
		}

		if err := s.handleMessage(ctx, msg); err != nil {
			s.logger.Error("scoring handle failed", "service", "scoring", "error", err)
		}
		if err := s.reader.Commit(ctx, msg); err != nil {
			return fmt.Errorf("consumer.Run commit: %w", err)
		}
	}
}

func (s *Service) handleMessage(ctx context.Context, msg kafkago.Message) error {
	if err := events.ValidateTranscriptReady(msg.Value); err != nil {
		_ = s.ch.InsertDLQ(ctx, "", "", "call_transcript_ready", err.Error(), scoring.PayloadHash(msg.Value))
		return nil
	}

	var evt models.TranscriptReadyEvent
	if err := json.Unmarshal(msg.Value, &evt); err != nil {
		return fmt.Errorf("consumer.handleMessage unmarshal: %w", err)
	}

	already, err := s.idempotency.AlreadyScored(ctx, evt.TenantID, evt.CallID)
	if err != nil {
		return err
	}
	if already {
		s.logger.Info("call already scored, skipping",
			"service", "scoring", "tenant_id", evt.TenantID, "call_id", evt.CallID)
		return nil
	}

	redacted := pii.Redact(evt.TranscriptText)
	result, err := s.engine.Score(ctx, evt.TenantID, evt.CallID, redacted)
	if err != nil {
		_ = s.ch.InsertDLQ(ctx, evt.TenantID, evt.CallID, "call_transcript_ready", err.Error(), scoring.PayloadHash(msg.Value))
		return err
	}

	if err := s.ch.InsertTranscript(ctx, evt.TenantID, evt.CallID, redacted, evt.DurationSeconds); err != nil {
		_ = s.ch.InsertDLQ(ctx, evt.TenantID, evt.CallID, "call_transcript_ready", err.Error(), scoring.PayloadHash(msg.Value))
		return err
	}
	if err := s.ch.InsertScore(ctx, evt.TenantID, evt.CallID, evt.Platform, result.OverallScore, result.Sentiment, result.RulesPassed, result.RulesFailed); err != nil {
		_ = s.ch.InsertDLQ(ctx, evt.TenantID, evt.CallID, "call_transcript_ready", err.Error(), scoring.PayloadHash(msg.Value))
		return err
	}

	if err := s.idempotency.MarkScored(ctx, evt.TenantID, evt.CallID); err != nil {
		return err
	}

	scored := models.CallScoredEvent{
		SchemaVersion:  "1",
		EventID:        uuid.NewString(),
		TenantID:       evt.TenantID,
		CallID:         evt.CallID,
		OverallScore:   result.OverallScore,
		Sentiment:      result.Sentiment,
		ScoredAtUnixMs: time.Now().UTC().UnixMilli(),
	}
	body, _ := json.Marshal(scored)
	if err := s.writer.Write(ctx, s.cfg.TopicCallScored, evt.TenantID, body); err != nil {
		s.logger.Error("call_scored publish failed", "service", "scoring", "error", err)
	}

	s.logger.Info("call scored",
		"service", "scoring",
		"tenant_id", evt.TenantID,
		"call_id", evt.CallID,
		"score", result.OverallScore,
	)
	return nil
}
