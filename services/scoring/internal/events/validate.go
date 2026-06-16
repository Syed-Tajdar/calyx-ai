package events

import (
	"encoding/json"
	"fmt"
)

// ValidateTranscriptReady ensures required fields are present.
func ValidateTranscriptReady(raw []byte) error {
	var evt struct {
		SchemaVersion  string `json:"schema_version"`
		EventID        string `json:"event_id"`
		TenantID       string `json:"tenant_id"`
		CallID         string `json:"call_id"`
		TranscriptText string `json:"transcript_text"`
	}
	if err := json.Unmarshal(raw, &evt); err != nil {
		return fmt.Errorf("events.ValidateTranscriptReady: %w", err)
	}
	if evt.SchemaVersion == "" || evt.EventID == "" || evt.TenantID == "" || evt.CallID == "" || evt.TranscriptText == "" {
		return fmt.Errorf("events.ValidateTranscriptReady: missing required fields")
	}
	return nil
}
