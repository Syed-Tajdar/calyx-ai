package publisher

import (
	"context"
	"encoding/json"
	"time"

	"github.com/Syed-Tajdar/calyx-ai/services/ad-attribution/internal/models"
	"github.com/Syed-Tajdar/calyx-ai/services/ad-attribution/pkg/kafka"
)

// Service publishes attribution outcome events.
type Service struct {
	attributedWriter *kafka.Writer
	dlqWriter        *kafka.Writer
}

// NewService creates Kafka publishers.
func NewService(brokers []string, topicAttributed, topicDLQ string) *Service {
	return &Service{
		attributedWriter: kafka.NewWriter(brokers, topicAttributed),
		dlqWriter:        kafka.NewWriter(brokers, topicDLQ),
	}
}

// Close closes writers.
func (s *Service) Close() error {
	_ = s.attributedWriter.Close()
	return s.dlqWriter.Close()
}

type attributedEvent struct {
	TenantID     string  `json:"tenant_id"`
	EventID      string  `json:"event_id"`
	CallID       string  `json:"call_id"`
	Platform     string  `json:"platform"`
	ClickID      string  `json:"click_id,omitempty"`
	Revenue      float64 `json:"revenue"`
	Status       string  `json:"status"`
	HTTPStatus   int     `json:"http_status"`
	AttributedAt string  `json:"attributed_at"`
}

// PublishAttributed emits conversion_attributed event.
func (s *Service) PublishAttributed(ctx context.Context, evt models.ConversionEvent, platform string, res models.FireResult) error {
	status := "fired"
	if !res.Success {
		status = "failed"
	}
	body, err := json.Marshal(attributedEvent{
		TenantID:     evt.TenantID,
		EventID:      evt.EventID,
		CallID:       evt.CallID,
		Platform:     platform,
		ClickID:      evt.ClickID,
		Revenue:      evt.Revenue,
		Status:       status,
		HTTPStatus:   res.HTTPStatus,
		AttributedAt: time.Now().UTC().Format(time.RFC3339Nano),
	})
	if err != nil {
		return err
	}
	return s.attributedWriter.Publish(ctx, evt.TenantID, body)
}

type dlqEvent struct {
	TenantID  string `json:"tenant_id"`
	CallID    string `json:"call_id"`
	EventID   string `json:"event_id"`
	Stage     string `json:"stage"`
	Error     string `json:"error"`
	FailedAt  string `json:"failed_at"`
}

// PublishDLQ emits a DLQ message.
func (s *Service) PublishDLQ(ctx context.Context, evt models.ConversionEvent, stage, errMsg string) error {
	body, err := json.Marshal(dlqEvent{
		TenantID: evt.TenantID,
		CallID:   evt.CallID,
		EventID:  evt.EventID,
		Stage:    stage,
		Error:    errMsg,
		FailedAt: time.Now().UTC().Format(time.RFC3339Nano),
	})
	if err != nil {
		return err
	}
	return s.dlqWriter.Publish(ctx, evt.TenantID, body)
}
