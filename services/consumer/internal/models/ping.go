package models

import "time"

// PingMessage mirrors the Kafka JSON envelope from ingestion.
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
	CallKey        string  `json:"call_key,omitempty"`
}

// ReceivedTime converts the unix millis field to UTC time.
func (m PingMessage) ReceivedTime() time.Time {
	return time.UnixMilli(m.ReceivedAt).UTC()
}
