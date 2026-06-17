package publisher

import (
	"context"
	"encoding/json"
	"time"

	"github.com/Syed-Tajdar/call-loom-intelligence/services/compliance-intelligence/internal/models"
	"github.com/Syed-Tajdar/call-loom-intelligence/services/compliance-intelligence/pkg/kafka"
	"github.com/google/uuid"
)

// Service publishes compliance Kafka events.
type Service struct {
	riskWriter      *kafka.Writer
	violationWriter *kafka.Writer
}

// NewService creates an event publisher.
func NewService(riskBrokers, violationBrokers []string, riskTopic, violationTopic string) *Service {
	return &Service{
		riskWriter:      kafka.NewWriter(riskBrokers, riskTopic),
		violationWriter: kafka.NewWriter(violationBrokers, violationTopic),
	}
}

// Close closes writers.
func (s *Service) Close() error {
	_ = s.riskWriter.Close()
	return s.violationWriter.Close()
}

// PublishRiskScored emits compliance_risk_scored event.
func (s *Service) PublishRiskScored(ctx context.Context, ping models.PingMessage, risk models.RiskResult) error {
	body, err := json.Marshal(map[string]any{
		"schema_version":      "1",
		"event_id":            uuid.NewString(),
		"tenant_id":           ping.TenantID,
		"call_id":             ping.ExternalCallID,
		"publisher_id":        ping.PublisherID,
		"state":               ping.State,
		"risk_score":          risk.RiskScore,
		"dnc_match":           risk.DNCMatched,
		"attestation_level":   risk.AttestationLevel,
		"exposure_usd":        risk.ExposureUSD,
		"scored_at_unix_ms":   time.Now().UnixMilli(),
	})
	if err != nil {
		return err
	}
	return s.riskWriter.Publish(ctx, ping.TenantID, body)
}

// PublishViolation emits compliance_violation_detected event.
func (s *Service) PublishViolation(ctx context.Context, ping models.PingMessage, risk models.RiskResult) error {
	if risk.ViolationType == "" {
		return nil
	}
	body, err := json.Marshal(map[string]any{
		"schema_version":      "1",
		"event_id":            uuid.NewString(),
		"tenant_id":           ping.TenantID,
		"call_id":             ping.ExternalCallID,
		"violation_type":      risk.ViolationType,
		"risk_score":          risk.RiskScore,
		"exposure_usd":        risk.ExposureUSD,
		"detected_at_unix_ms": time.Now().UnixMilli(),
	})
	if err != nil {
		return err
	}
	return s.violationWriter.Publish(ctx, ping.TenantID, body)
}
