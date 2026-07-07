package adapter

import (
	"encoding/json"
	"testing"
)

func TestMapRetreaverCall(t *testing.T) {
	raw := json.RawMessage(`{
		"uuid": "f1abfb78-ab8a-4146-9946-8169fbcc6d6c",
		"caller": "+13015236555",
		"caller_state": "NC",
		"revenue": 5.0,
		"payout": 3.0,
		"campaign_id": 9872,
		"client_afid": "aff123",
		"target_id": 42,
		"status": "completed"
	}`)
	call := mapRetreaverCall(raw)
	if call.ExternalCallID != "f1abfb78-ab8a-4146-9946-8169fbcc6d6c" {
		t.Fatalf("uuid: %s", call.ExternalCallID)
	}
	if call.State != "NC" {
		t.Fatalf("state: %s", call.State)
	}
}

func TestRetreaverCursorRoundTrip(t *testing.T) {
	cs := retreaverCursor{CreatedAtStart: "2026-01-01T00:00:00Z", Page: 2}
	enc, err := EncodeCursor(cs)
	if err != nil {
		t.Fatal(err)
	}
	var out retreaverCursor
	if err := DecodeCursor(enc, &out); err != nil {
		t.Fatal(err)
	}
	if out.Page != 2 {
		t.Fatalf("page %d", out.Page)
	}
}
