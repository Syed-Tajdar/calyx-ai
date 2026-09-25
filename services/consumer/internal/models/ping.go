package models

import "time"

// PingMessage mirrors the Kafka JSON envelope from ingestion.
type PingMessage struct {
	TenantID          string  `json:"tenant_id"`
	Platform          string  `json:"platform"`
	ExternalCallID    string  `json:"external_call_id"`
	ANIHash           string  `json:"ani_hash"`
	CampaignID        string  `json:"campaign_id"`
	PublisherID       string  `json:"publisher_id"`
	TargetID          string  `json:"target_id"`
	State             string  `json:"state"`
	EventType         string  `json:"event_type"`
	Revenue           float64 `json:"revenue"`
	Payout            float64 `json:"payout"`
	IsDuplicate       bool    `json:"is_duplicate"`
	PlatformDuplicate bool    `json:"platform_duplicate"`
	ReceivedAt        int64   `json:"received_at_unix_ms"`
	OccurredAt        int64   `json:"occurred_at_unix_ms,omitempty"`
	CallKey           string  `json:"call_key,omitempty"`
	CampaignName      string  `json:"campaign_name,omitempty"`
	PublisherName     string  `json:"publisher_name,omitempty"`
	TargetName        string  `json:"target_name,omitempty"`
	BuyerID           string  `json:"buyer_id,omitempty"`
	BuyerName         string  `json:"buyer_name,omitempty"`
	DurationSeconds   uint32  `json:"duration_seconds,omitempty"`
	CallLengthSeconds uint32  `json:"call_length_seconds,omitempty"`
	HasRecording      bool    `json:"has_recording,omitempty"`
	RecordingKnown    bool    `json:"recording_known,omitempty"`
	ProfitGross       float64 `json:"profit_gross,omitempty"`
	HasConverted      bool    `json:"has_converted,omitempty"`
	// Nil when the producer did not request the field (webhook, or a pre-Phase-2A message).
	HasConnected          *bool  `json:"has_connected,omitempty"`
	WasBlocked            *bool  `json:"was_blocked,omitempty"`
	BlockReason           string `json:"block_reason,omitempty"`
	WasConversionAdjusted *bool  `json:"was_conversion_adjusted,omitempty"`
	WasPayoutAdjusted     *bool  `json:"was_payout_adjusted,omitempty"`
	RequiredDurationSeconds    *uint32 `json:"required_duration_seconds,omitempty"`
	MaxRequiredDurationSeconds *uint32 `json:"max_required_duration_seconds,omitempty"`
	SyncPath                   string  `json:"sync_path,omitempty"`
}

// ReceivedTime converts the unix millis field to UTC time.
func (m PingMessage) ReceivedTime() time.Time {
	return time.UnixMilli(m.ReceivedAt).UTC()
}

// OccurredTime returns the source call time when present; otherwise zero.
func (m PingMessage) OccurredTime() time.Time {
	if m.OccurredAt <= 0 {
		return time.Time{}
	}
	return time.UnixMilli(m.OccurredAt).UTC()
}
