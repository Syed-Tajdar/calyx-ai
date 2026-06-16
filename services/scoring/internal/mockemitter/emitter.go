package mockemitter

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/Syed-Tajdar/call-loom-intelligence/services/scoring/internal/config"
	"github.com/Syed-Tajdar/call-loom-intelligence/services/scoring/internal/models"
	"github.com/Syed-Tajdar/call-loom-intelligence/services/scoring/pkg/kafka"
	"github.com/google/uuid"
)

// Service consumes pings and emits mock transcript_ready events when MOCK_MODE is on.
type Service struct {
	cfg    config.Config
	reader *kafka.Reader
	writer *kafka.Writer
	logger *slog.Logger
}

// NewServiceWithReader creates mock emitter with explicit reader.
func NewServiceWithReader(cfg config.Config, reader *kafka.Reader, writer *kafka.Writer, logger *slog.Logger) *Service {
	return &Service{cfg: cfg, reader: reader, writer: writer, logger: logger}
}

// Run blocks until context cancelled.
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

		var ping models.PingMessage
		if err := json.Unmarshal(msg.Value, &ping); err != nil {
			_ = s.reader.Commit(ctx, msg)
			continue
		}
		if ping.IsDuplicate || ping.ExternalCallID == "" {
			_ = s.reader.Commit(ctx, msg)
			continue
		}

		evt := models.TranscriptReadyEvent{
			SchemaVersion:    "1",
			EventID:          uuid.NewString(),
			TenantID:         ping.TenantID,
			CallID:           ping.ExternalCallID,
			Platform:         ping.Platform,
			TranscriptText:   fmt.Sprintf("Agent: Thank you for calling. Caller: I need help with my account. Call ID %s.", ping.ExternalCallID),
			DurationSeconds:  120,
			ReceivedAtUnixMs: time.Now().UTC().UnixMilli(),
		}
		body, _ := json.Marshal(evt)
		if err := s.writer.Write(ctx, s.cfg.TopicTranscriptReady, ping.TenantID, body); err != nil {
			s.logger.Error("mock emitter publish failed", "service", "scoring", "error", err)
		} else {
			s.logger.Info("mock transcript_ready emitted",
				"service", "scoring", "tenant_id", ping.TenantID, "call_id", ping.ExternalCallID)
		}

		if err := s.reader.Commit(ctx, msg); err != nil {
			return fmt.Errorf("mockemitter.Run commit: %w", err)
		}
	}
}
