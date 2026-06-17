package models

// PingMessage mirrors ingestion ping envelope fields used by compliance.
type PingMessage struct {
	TenantID       string  `json:"tenant_id"`
	Platform       string  `json:"platform"`
	ExternalCallID string  `json:"external_call_id"`
	ANIHash        string  `json:"ani_hash"`
	PublisherID    string  `json:"publisher_id"`
	State          string  `json:"state"`
	IsDuplicate    bool    `json:"is_duplicate"`
	Revenue        float64 `json:"revenue"`
}

// RiskResult is the output of the risk scoring engine.
type RiskResult struct {
	RiskScore        float32
	DNCMatched       bool
	AttestationLevel string
	ExposureUSD      float32
	ViolationType    string
}
