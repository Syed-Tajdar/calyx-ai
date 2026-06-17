package models

// PingMessage mirrors ingestion ping envelope for mock bridge.
type PingMessage struct {
	TenantID       string  `json:"tenant_id"`
	Platform       string  `json:"platform"`
	ExternalCallID string  `json:"external_call_id"`
	ANIHash        string  `json:"ani_hash"`
	EventType      string  `json:"event_type"`
	IsDuplicate    bool    `json:"is_duplicate"`
	Revenue        float64 `json:"revenue"`
	Payout         float64 `json:"payout"`
}

// ConversionEvent is the Kafka conversion payload.
type ConversionEvent struct {
	SchemaVersion     string  `json:"schema_version"`
	EventID           string  `json:"event_id"`
	TenantID          string  `json:"tenant_id"`
	CallID            string  `json:"call_id"`
	Revenue           float64 `json:"revenue"`
	Payout            float64 `json:"payout"`
	ClickID           string  `json:"click_id,omitempty"`
	ANIHash           string  `json:"ani_hash,omitempty"`
	PlatformHint      string  `json:"platform_hint,omitempty"`
	ConvertedAtUnixMs int64   `json:"converted_at_unix_ms"`
}

// FireRequest is sent to ad platforms.
type FireRequest struct {
	TenantID   string
	CallID     string
	EventID    string
	Platform   string
	ClickID    string
	Revenue    float64
	PhoneHash  string
	EmailHash  string
}

// FireResult is the platform response.
type FireResult struct {
	Success    bool
	HTTPStatus int
	Error      string
}

// PlatformStatus summarizes fired/failed counts.
type PlatformStatus struct {
	Platform     string `json:"platform"`
	FiredCount   uint64 `json:"fired_count"`
	FailedCount  uint64 `json:"failed_count"`
	LastFiredAt  string `json:"last_fired_at,omitempty"`
}
