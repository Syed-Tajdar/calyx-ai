package writer

import (
	"encoding/json"
	"testing"

	"github.com/Syed-Tajdar/calyx-ai/services/consumer/internal/models"
)

func TestPhase2FalseIsDistinctFromMissing(t *testing.T) {
	f := false
	body, err := json.Marshal(models.PingMessage{
		ExternalCallID:        "api",
		HasConnected:          &f,
		WasBlocked:            &f,
		WasConversionAdjusted: &f,
	})
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	for _, key := range []string{`"has_connected":false`, `"was_blocked":false`, `"was_conversion_adjusted":false`} {
		if !contains(text, key) {
			t.Fatalf("missing %s in %s", key, text)
		}
	}
	var api models.PingMessage
	if err := json.Unmarshal(body, &api); err != nil {
		t.Fatal(err)
	}
	if api.HasConnected == nil || *api.HasConnected {
		t.Fatal("explicit false must stay false")
	}
	if u8ptr(api.HasConnected) == nil || *u8ptr(api.HasConnected) != 0 {
		t.Fatal("explicit false must store 0")
	}

	var webhook models.PingMessage
	if err := json.Unmarshal([]byte(`{"external_call_id":"hook","sync_path":"webhook"}`), &webhook); err != nil {
		t.Fatal(err)
	}
	if webhook.HasConnected != nil || webhook.WasBlocked != nil || webhook.WasConversionAdjusted != nil {
		t.Fatal("omitted keys must stay nil (not provided)")
	}
	if u8ptr(webhook.HasConnected) != nil {
		t.Fatal("nil must store NULL")
	}
}

func TestPhase2RequiredDurationNullZeroPresent(t *testing.T) {
	zero := uint32(0)
	ten := uint32(10)
	body, err := json.Marshal(models.PingMessage{
		ExternalCallID:             "api",
		RequiredDurationSeconds:    &zero,
		MaxRequiredDurationSeconds: &ten,
	})
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	if !contains(text, `"required_duration_seconds":0`) {
		t.Fatalf("present 0 must serialize: %s", text)
	}
	if !contains(text, `"max_required_duration_seconds":10`) {
		t.Fatalf("present 10 must serialize: %s", text)
	}
	var api models.PingMessage
	if err := json.Unmarshal(body, &api); err != nil {
		t.Fatal(err)
	}
	if api.RequiredDurationSeconds == nil || *api.RequiredDurationSeconds != 0 {
		t.Fatal("present 0 must stay 0")
	}
	if api.MaxRequiredDurationSeconds == nil || *api.MaxRequiredDurationSeconds != 10 {
		t.Fatal("present 10 must stay 10")
	}

	var webhook models.PingMessage
	if err := json.Unmarshal([]byte(`{"external_call_id":"hook","sync_path":"webhook"}`), &webhook); err != nil {
		t.Fatal(err)
	}
	if webhook.RequiredDurationSeconds != nil || webhook.MaxRequiredDurationSeconds != nil {
		t.Fatal("omitted keys must stay nil")
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 || indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
