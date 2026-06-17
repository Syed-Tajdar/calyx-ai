package mockbridge

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/Syed-Tajdar/calyx-ai/services/ad-attribution/internal/config"
	"github.com/Syed-Tajdar/calyx-ai/services/ad-attribution/internal/matcher"
	"github.com/Syed-Tajdar/calyx-ai/services/ad-attribution/internal/models"
	"github.com/Syed-Tajdar/calyx-ai/services/ad-attribution/pkg/kafka"
	"github.com/google/uuid"
	kafkago "github.com/segmentio/kafka-go"
)

// Service bridges pings to conversion events in mock mode.
type Service struct {
	cfg    config.Config
	reader *kafka.Reader
	writer *kafka.Writer
	logger *slog.Logger
}

// NewService creates a mock ping→conversion bridge.
func NewService(cfg config.Config, reader *kafka.Reader, writer *kafka.Writer, logger *slog.Logger) *Service {
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

		if err := s.handlePing(ctx, msg); err != nil {
			s.logger.Error("mock bridge failed", "service", "ad-attribution", "error", err)
		}
		if err := s.reader.Commit(ctx, msg); err != nil {
			return fmt.Errorf("mockbridge.Run commit: %w", err)
		}
	}
}

func (s *Service) handlePing(ctx context.Context, msg kafkago.Message) error {
	var ping models.PingMessage
	if err := json.Unmarshal(msg.Value, &ping); err != nil {
		return nil
	}
	if ping.IsDuplicate || ping.TenantID == "" || ping.ExternalCallID == "" {
		return nil
	}
	if ping.Revenue <= 0 {
		return nil
	}

	platform := matcher.ResolvePlatform(models.ConversionEvent{CallID: ping.ExternalCallID})
	clickPrefix := "fbclid"
	switch platform {
	case "google":
		clickPrefix = "gclid"
	case "tiktok":
		clickPrefix = "ttclid"
	}

	suffix := ping.ExternalCallID
	if len(suffix) > 8 {
		suffix = suffix[:8]
	}
	evt := models.ConversionEvent{
		SchemaVersion:     "1",
		EventID:           uuid.NewString(),
		TenantID:          ping.TenantID,
		CallID:            ping.ExternalCallID,
		Revenue:           ping.Revenue,
		Payout:            ping.Payout,
		ANIHash:           ping.ANIHash,
		ClickID:           fmt.Sprintf("%s-mock-%s", clickPrefix, suffix),
		PlatformHint:      platform,
		ConvertedAtUnixMs: time.Now().UTC().UnixMilli(),
	}
	body, err := json.Marshal(evt)
	if err != nil {
		return err
	}
	if err := s.writer.Publish(ctx, ping.TenantID, body); err != nil {
		return err
	}
	s.logger.Info("mock conversion emitted",
		"service", "ad-attribution",
		"tenant_id", ping.TenantID,
		"call_id", ping.ExternalCallID,
		"platform", platform,
	)
	return nil
}
