package models

// TranscriptReadyEvent is consumed from Kafka.
type TranscriptReadyEvent struct {
	SchemaVersion      string `json:"schema_version"`
	EventID            string `json:"event_id"`
	TenantID           string `json:"tenant_id"`
	CallID             string `json:"call_id"`
	Platform           string `json:"platform"`
	TranscriptText     string `json:"transcript_text"`
	DurationSeconds    float32 `json:"duration_seconds"`
	ReceivedAtUnixMs   int64  `json:"received_at_unix_ms"`
}

// CallScoredEvent is published after scoring.
type CallScoredEvent struct {
	SchemaVersion    string  `json:"schema_version"`
	EventID            string  `json:"event_id"`
	TenantID           string  `json:"tenant_id"`
	CallID             string  `json:"call_id"`
	OverallScore       float32 `json:"overall_score"`
	Sentiment          string  `json:"sentiment"`
	ScoredAtUnixMs     int64   `json:"scored_at_unix_ms"`
}

// PingMessage mirrors ingestion ping envelope for mock emitter.
type PingMessage struct {
	TenantID       string  `json:"tenant_id"`
	Platform       string  `json:"platform"`
	ExternalCallID string  `json:"external_call_id"`
	ANIHash        string  `json:"ani_hash"`
	EventType      string  `json:"event_type"`
	IsDuplicate    bool    `json:"is_duplicate"`
	ReceivedAt     int64   `json:"received_at_unix_ms"`
}

// ScoreResult is the output of AI scoring.
type ScoreResult struct {
	OverallScore float32
	Sentiment    string
	RulesPassed  uint16
	RulesFailed  uint16
}
