package producer

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Syed-Tajdar/calyx-ai/services/ingestion/internal/models"
	"github.com/Syed-Tajdar/calyx-ai/services/ingestion/pkg/kafka"
)

// PingMessage is the Kafka JSON envelope for normalized pings.
type PingMessage struct {
	TenantID       string  `json:"tenant_id"`
	Platform       string  `json:"platform"`
	ExternalCallID string  `json:"external_call_id"`
	ANIHash        string  `json:"ani_hash"`
	CampaignID     string  `json:"campaign_id"`
	PublisherID    string  `json:"publisher_id"`
	TargetID       string  `json:"target_id"`
	State          string  `json:"state"`
	EventType      string  `json:"event_type"`
	Revenue        float64 `json:"revenue"`
	Payout         float64 `json:"payout"`
	IsDuplicate    bool    `json:"is_duplicate"`
	ReceivedAt     int64   `json:"received_at_unix_ms"`
}

// Producer publishes normalized pings to Kafka.
type Producer struct {
	client *kafka.Client
	topic  string
}

// NewProducer creates a Kafka ping producer.
func NewProducer(client *kafka.Client, topic string) *Producer {
	return &Producer{client: client, topic: topic}
}

// Publish sends a ping to Kafka. ANI encrypted blob is intentionally omitted from Kafka
// for Phase 1 — only hash travels to analytics; encrypted blob stays out of hot path logs.
func (p *Producer) Publish(ctx context.Context, ping models.Ping) error {
	msg := PingMessage{
		TenantID:       ping.TenantID,
		Platform:       ping.Platform,
		ExternalCallID: ping.ExternalCallID,
		ANIHash:        ping.ANIHash,
		CampaignID:     ping.CampaignID,
		PublisherID:    ping.PublisherID,
		TargetID:       ping.TargetID,
		State:          ping.State,
		EventType:      ping.EventType,
		Revenue:        ping.Revenue,
		Payout:         ping.Payout,
		IsDuplicate:    ping.IsDuplicate,
		ReceivedAt:     ping.ReceivedAt.UTC().UnixMilli(),
	}
	body, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("producer.Publish: marshal: %w", err)
	}
	if err := p.client.Write(ctx, p.topic, ping.TenantID, body); err != nil {
		return fmt.Errorf("producer.Publish: %w", err)
	}
	return nil
}

// Flush is a no-op for synchronous producer; messages are already acked on Publish.
func (p *Producer) Flush(_ context.Context) error {
	return nil
}

// Ping checks Kafka connectivity.
func (p *Producer) Ping(ctx context.Context) error {
	return p.client.Ping(ctx)
}
