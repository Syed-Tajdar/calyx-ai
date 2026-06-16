package models

// PingMessage mirrors ingestion ping envelope.
type PingMessage struct {
	TenantID       string `json:"tenant_id"`
	Platform       string `json:"platform"`
	ExternalCallID string `json:"external_call_id"`
	ANIHash        string `json:"ani_hash"`
	IsDuplicate    bool   `json:"is_duplicate"`
}

// Attribute is a single enrichment key-value pair.
type Attribute struct {
	Key   string
	Value string
}
