package models

import "time"

type SyncIntegration struct {
	ID                  string
	TenantID            string
	Platform            string
	Name                string
	SyncMode            string
	APICredentialsBlob  []byte
	SyncCursor          string
	LastSyncAt          *time.Time
	SyncStatus          string
	LastSyncError       string
	InitialLookbackDays int
}

type RemoteCall struct {
	ExternalCallID string
	CallerID       string
	State          string
	Revenue        float64
	Payout         float64
	EventType      string
	CallKey        string
	CampaignID     string
	PublisherID    string
	TargetID       string
	// OccurredAt is the platform source call time (e.g. Retreaver created_at). Zero if unknown.
	OccurredAt time.Time
	// UpdatedAt is the platform last-modified time (e.g. Retreaver updated_at). Zero if unknown.
	UpdatedAt time.Time
}

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
	// OccurredAt: platform source call time (unix ms). 0 if unknown — consumers fall back to ReceivedAt.
	OccurredAt int64  `json:"occurred_at_unix_ms,omitempty"`
	CallKey    string `json:"call_key,omitempty"`
}
