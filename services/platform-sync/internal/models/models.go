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
	WritebackEnabled    bool
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
	// HasConverted: Ringba hasConverted present+true. Absent on Get Call Log means false.
	HasConverted          bool
	HasConvertedPresent   bool
	HasConnected          bool
	WasBlocked            bool
	BlockReason           string
	WasConversionAdjusted bool
	WasPayoutAdjusted     bool
	// OfflineConversionUploadedPresent is false when the key was silent-omitted.
	OfflineConversionUploadedPresent bool
	OfflineConversionUploaded        bool
	// RequiredDurationSeconds: Ringba ringTreeWinningBidDynamicDuration.
	// nil = key absent. pointer to 0 = present zero. pointer to N = N seconds.
	RequiredDurationSeconds    *uint32
	MaxRequiredDurationSeconds *uint32
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
	OccurredAt        int64   `json:"occurred_at_unix_ms,omitempty"`
	CallKey           string  `json:"call_key,omitempty"`
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
	HasConverted      bool    `json:"has_converted,omitempty"`
	// Phase 2A booleans are pointers so false is sent and a missing key stays NULL
	// on webhook rows (not provided). API sync always sets them.
	HasConnected          *bool  `json:"has_connected,omitempty"`
	WasBlocked            *bool  `json:"was_blocked,omitempty"`
	BlockReason           string `json:"block_reason,omitempty"`
	WasConversionAdjusted *bool  `json:"was_conversion_adjusted,omitempty"`
	WasPayoutAdjusted     *bool  `json:"was_payout_adjusted,omitempty"`
	// Required duration: nil omits the key (webhook / not provided). Pointer to 0 serializes as 0.
	RequiredDurationSeconds    *uint32 `json:"required_duration_seconds,omitempty"`
	MaxRequiredDurationSeconds *uint32 `json:"max_required_duration_seconds,omitempty"`
	// SyncPath: "api_sync" | "webhook" — UI empty-state distinction (P-B1.4).
	SyncPath string `json:"sync_path,omitempty"`
}
