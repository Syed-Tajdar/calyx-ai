package connectors

import (
	"net/url"
	"testing"
)

func TestParseRingbaQuery_InboundCallIDRequired(t *testing.T) {
	_, err := ParseRingbaQuery(url.Values{"caller_id": {"15551234567"}})
	if err == nil {
		t.Fatal("expected error for missing inbound call id")
	}
}

func TestParseRingbaQuery_AllFields(t *testing.T) {
	values := url.Values{
		"CallID":       {"abc-123"},
		"caller_id":    {"+15551234567"},
		"campaign_id":  {"camp-1"},
		"publisher_id": {"pub-1"},
		"state":        {"TX"},
		"event":        {"incoming"},
		"call_revenue": {"12.50"},
		"payout":       {"8.00"},
		"is_duplicate": {"true"},
	}

	payload, err := ParseRingbaQuery(values)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if payload.InboundCallID != "abc-123" {
		t.Fatalf("call id: got %q", payload.InboundCallID)
	}
	if payload.CallerID != "+15551234567" {
		t.Fatalf("caller id: got %q", payload.CallerID)
	}
	if payload.Revenue != 12.5 {
		t.Fatalf("revenue: got %f", payload.Revenue)
	}
	if !payload.IsDuplicate {
		t.Fatal("expected duplicate true")
	}
}
