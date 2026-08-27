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
	// PlatformDuplicate is the platform's own duplicate-caller flag (Ringba isDuplicate).
	PlatformDuplicate bool
	CampaignName      string
	PublisherName     string
	TargetName        string
	BuyerID           string
	BuyerName         string
	// DurationSeconds is Connected Call Length (talk time). 0 if unknown / not provided.
	DurationSeconds uint32
	// CallLengthSeconds is whole-call Duration (includes ring/IVR). 0 if unknown.
	CallLengthSeconds uint32
	HasRecording      bool
	// RecordingKnown distinguishes "false from API" vs "webhook never sent the field".
	RecordingKnown bool
	ProfitGross    float64
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
	PlatformDuplicate bool    `json:"platform_duplicate"`
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
	// SyncPath: "api_sync" | "webhook" — UI empty-state distinction (P-B1.4).
	SyncPath string `json:"sync_path,omitempty"`
}
