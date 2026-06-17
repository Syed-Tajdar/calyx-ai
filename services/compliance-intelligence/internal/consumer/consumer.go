package consumer

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/Syed-Tajdar/calyx-ai/services/compliance-intelligence/internal/attestation"
	"github.com/Syed-Tajdar/calyx-ai/services/compliance-intelligence/internal/config"
	"github.com/Syed-Tajdar/calyx-ai/services/compliance-intelligence/internal/dedup"
	"github.com/Syed-Tajdar/calyx-ai/services/compliance-intelligence/internal/dnc"
	"github.com/Syed-Tajdar/calyx-ai/services/compliance-intelligence/internal/models"
	"github.com/Syed-Tajdar/calyx-ai/services/compliance-intelligence/internal/providerconfig"
	"github.com/Syed-Tajdar/calyx-ai/services/compliance-intelligence/internal/publisher"
	"github.com/Syed-Tajdar/calyx-ai/services/compliance-intelligence/internal/riskscoring"
	"github.com/Syed-Tajdar/calyx-ai/services/compliance-intelligence/pkg/clickhouse"
	"github.com/Syed-Tajdar/calyx-ai/services/compliance-intelligence/pkg/kafka"
	kafkago "github.com/segmentio/kafka-go"
)

// Service consumes pings and runs TCPA compliance intelligence.
type Service struct {
	cfg       config.Config
	reader    *kafka.Reader
	ch        *clickhouse.Client
	dedup     *dedup.Checker
	dnc       dnc.Provider
	engine    *riskscoring.Engine
	publisher *publisher.Service
	resolver  providerconfig.Resolver
	logger    *slog.Logger
}

// NewService creates the compliance consumer.
func NewService(
	cfg config.Config,
	reader *kafka.Reader,
	ch *clickhouse.Client,
	dedupChecker *dedup.Checker,
	dncProvider dnc.Provider,
	pub *publisher.Service,
	resolver providerconfig.Resolver,
	logger *slog.Logger,
) *Service {
	return &Service{
		cfg:       cfg,
		reader:    reader,
		ch:        ch,
		dedup:     dedupChecker,
		dnc:       dncProvider,
		engine:    riskscoring.NewEngine(),
		publisher: pub,
		resolver:  resolver,
		logger:    logger,
	}
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

		if err := s.handleMessage(ctx, msg); err != nil {
			s.logger.Error("compliance handle failed", "service", "compliance-intelligence", "error", err)
		}
		if err := s.reader.Commit(ctx, msg); err != nil {
			return fmt.Errorf("consumer.Run commit: %w", err)
		}
	}
}

func (s *Service) handleMessage(ctx context.Context, msg kafkago.Message) error {
	var ping models.PingMessage
	if err := json.Unmarshal(msg.Value, &ping); err != nil {
		return nil
	}
	if ping.IsDuplicate || ping.TenantID == "" || ping.ExternalCallID == "" || ping.ANIHash == "" {
		return nil
	}

	processed, err := s.dedup.AlreadyProcessed(ctx, ping.TenantID, ping.ExternalCallID)
	if err != nil {
		return err
	}
	if processed {
		return nil
	}

	traceID := ping.ExternalCallID
	if s.resolver != nil {
		resolveCtx, resolveCancel := context.WithTimeout(ctx, 3*time.Second)
		_, _ = s.resolver.Resolve(resolveCtx, ping.TenantID, "dnc", traceID)
		resolveCancel()
	}

	dncResult, err := s.dnc.CheckNumber(ctx, ping.ANIHash)
	if err != nil {
		_ = s.ch.InsertDLQ(ctx, ping.TenantID, ping.ExternalCallID, "dnc_check", err.Error(), payloadHash(msg.Value))
		return err
	}

	attLevel := attestation.Level(ping.ExternalCallID)
	risk := s.engine.Score(ping, dncResult, attLevel)

	if err := s.ch.InsertDNCEvent(ctx, ping, dncResult.ListName, dncResult.Matched); err != nil {
		_ = s.ch.InsertDLQ(ctx, ping.TenantID, ping.ExternalCallID, "dnc_write", err.Error(), payloadHash(msg.Value))
		return err
	}
	blockEst := attestation.CarrierBlockEstimate(attLevel, float32(ping.Revenue))
	if err := s.ch.InsertAttestation(ctx, ping.TenantID, ping.ExternalCallID, attLevel, blockEst); err != nil {
		return err
	}
	if err := s.ch.InsertRiskScore(ctx, ping, risk); err != nil {
		_ = s.ch.InsertDLQ(ctx, ping.TenantID, ping.ExternalCallID, "risk_write", err.Error(), payloadHash(msg.Value))
		return err
	}
	if risk.ViolationType != "" {
		if err := s.ch.InsertViolation(ctx, ping.TenantID, ping.ExternalCallID, risk.ViolationType, risk); err != nil {
			return err
		}
	}

	_ = s.publisher.PublishRiskScored(ctx, ping, risk)
	_ = s.publisher.PublishViolation(ctx, ping, risk)

	if err := s.dedup.MarkProcessed(ctx, ping.TenantID, ping.ExternalCallID); err != nil {
		return err
	}

	s.logger.Info("compliance scored",
		"service", "compliance-intelligence",
		"tenant_id", ping.TenantID,
		"call_id", ping.ExternalCallID,
		"risk_score", risk.RiskScore,
		"dnc_match", risk.DNCMatched,
		"attestation", attLevel,
		"violation", risk.ViolationType,
	)
	return nil
}

func payloadHash(raw []byte) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}
