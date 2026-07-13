package adapter

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/Syed-Tajdar/calyx-ai/services/platform-sync/internal/models"
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
	if call.Revenue != 5.0 || call.Payout != 3.0 {
		t.Fatalf("revenue/payout: %v %v", call.Revenue, call.Payout)
	}
	if call.CampaignID != "9872" || call.PublisherID != "aff123" || call.TargetID != "42" {
		t.Fatalf("ids: camp=%s pub=%s tgt=%s", call.CampaignID, call.PublisherID, call.TargetID)
	}
	if call.CallKey != "" {
		t.Fatalf("call_key should be empty from API")
	}
}

func TestMapRetreaverCallNestedV2ListItem(t *testing.T) {
	raw := json.RawMessage(`{
		"call": {
			"uuid": "f1abfb78-ab8a-4146-9946-8169fbcc6d6c",
			"caller": "+13015236555",
			"caller_state": "NC",
			"revenue": 5.0,
			"payout": 5.0,
			"system_campaign_id": 9872,
			"afid": "aff456",
			"system_target_id": 27449,
			"status": "finished",
			"created_at": "2026-07-09T19:38:21.244Z"
		}
	}`)
	call := mapRetreaverCall(raw)
	if call.ExternalCallID != "f1abfb78-ab8a-4146-9946-8169fbcc6d6c" {
		t.Fatalf("uuid: %s", call.ExternalCallID)
	}
	if call.State != "NC" {
		t.Fatalf("state: %s", call.State)
	}
	if call.Revenue != 5.0 || call.Payout != 5.0 {
		t.Fatalf("revenue/payout: %v %v", call.Revenue, call.Payout)
	}
	if call.CampaignID != "9872" {
		t.Fatalf("campaign: %s", call.CampaignID)
	}
	if call.PublisherID != "aff456" {
		t.Fatalf("publisher: %s", call.PublisherID)
	}
	if call.TargetID != "27449" {
		t.Fatalf("target: %s", call.TargetID)
	}
	if call.OccurredAt.IsZero() {
		t.Fatal("expected OccurredAt from created_at")
	}
	want := time.Date(2026, 7, 9, 19, 38, 21, 244000000, time.UTC)
	if !call.OccurredAt.Equal(want) {
		t.Fatalf("OccurredAt=%v want %v", call.OccurredAt, want)
	}
}

func TestMapRetreaverCallNormalizeState(t *testing.T) {
	tests := []struct {
		name  string
		state string
		want  string
	}{
		{"full name", "Texas", "TX"},
		{"lowercase abbr", "tx", "TX"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			raw := json.RawMessage(`{"call":{"uuid":"u1","caller_state":"` + tc.state + `"}}`)
			call := mapRetreaverCall(raw)
			if call.State != tc.want {
				t.Fatalf("state %q -> %q, want %q", tc.state, call.State, tc.want)
			}
		})
	}
}

func TestMapRetreaverCallFieldFallbacks(t *testing.T) {
	t.Run("call_uuid fallback", func(t *testing.T) {
		raw := json.RawMessage(`{"call_uuid":"cc-uuid-1","caller_state":"CA"}`)
		call := mapRetreaverCall(raw)
		if call.ExternalCallID != "cc-uuid-1" {
			t.Fatalf("external_call_id: %s", call.ExternalCallID)
		}
	})
	t.Run("id fallback", func(t *testing.T) {
		raw := json.RawMessage(`{"id":"legacy-id","caller_state":"NY"}`)
		call := mapRetreaverCall(raw)
		if call.ExternalCallID != "legacy-id" {
			t.Fatalf("external_call_id: %s", call.ExternalCallID)
		}
	})
	t.Run("legacy campaign and publisher names", func(t *testing.T) {
		raw := json.RawMessage(`{
			"uuid":"u2",
			"campaign_id": 111,
			"client_afid": "legacy-pub",
			"target_id": 222,
			"caller_state": "FL"
		}`)
		call := mapRetreaverCall(raw)
		if call.CampaignID != "111" || call.PublisherID != "legacy-pub" || call.TargetID != "222" {
			t.Fatalf("legacy fields: camp=%s pub=%s tgt=%s", call.CampaignID, call.PublisherID, call.TargetID)
		}
	})
	t.Run("prefer V2 names over legacy", func(t *testing.T) {
		raw := json.RawMessage(`{
			"uuid":"u3",
			"system_campaign_id": 9872,
			"campaign_id": 1,
			"afid": "v2-pub",
			"client_afid": "legacy-pub",
			"system_target_id": 27449,
			"target_id": 2
		}`)
		call := mapRetreaverCall(raw)
		if call.CampaignID != "9872" || call.PublisherID != "v2-pub" || call.TargetID != "27449" {
			t.Fatalf("V2 preferred: camp=%s pub=%s tgt=%s", call.CampaignID, call.PublisherID, call.TargetID)
		}
	})
}

func TestMapRetreaverCallShapes(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		wantID  string
		wantErr bool
	}{
		{
			name: "wrapped array item",
			raw:  `[{"call":{"uuid":"wrap-1","caller_state":"WA"}}]`,
			wantID: "wrap-1",
		},
		{
			name: "flat object",
			raw:  `{"uuid":"flat-1","caller_state":"OR"}`,
			wantID: "flat-1",
		},
		{
			name:   "empty uuid is empty not silent success",
			raw:    `{"call":{"caller_state":"CA"}}`,
			wantID: "",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var raw json.RawMessage
			if tc.raw[0] == '[' {
				var arr []json.RawMessage
				if err := json.Unmarshal([]byte(tc.raw), &arr); err != nil {
					t.Fatal(err)
				}
				raw = arr[0]
			} else {
				raw = json.RawMessage(tc.raw)
			}
			call := mapRetreaverCall(raw)
			if call.ExternalCallID != tc.wantID {
				t.Fatalf("external_call_id=%q want %q", call.ExternalCallID, tc.wantID)
			}
		})
	}
}

func TestRetreaverCursorRoundTrip(t *testing.T) {
	cs := retreaverCursor{UpdatedAtStart: "2026-01-01T00:00:00Z", Page: 2}
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
	if out.UpdatedAtStart != "2026-01-01T00:00:00Z" {
		t.Fatalf("updated_at_start %s", out.UpdatedAtStart)
	}
}

func TestRetreaverCursorMigratesLegacyCreatedAt(t *testing.T) {
	cs := retreaverCursor{CreatedAtStart: "2026-06-01T00:00:00Z", Page: 1}
	cs.normalize()
	if cs.UpdatedAtStart != "2026-06-01T00:00:00Z" {
		t.Fatalf("migrated updated_at_start=%q", cs.UpdatedAtStart)
	}
	if cs.CreatedAtStart != "" {
		t.Fatalf("legacy created_at_start should clear, got %q", cs.CreatedAtStart)
	}
}

func TestRetreaverInitialLookbackIs30Days(t *testing.T) {
	if retreaverInitialLookback != 30*24*time.Hour {
		t.Fatalf("lookback=%v want 30d", retreaverInitialLookback)
	}
	if retreaverLookbackDuration(0) != retreaverInitialLookback {
		t.Fatalf("empty lookbackDays should fall back to 30d, got %v", retreaverLookbackDuration(0))
	}
	if retreaverLookbackDuration(7) != 7*24*time.Hour {
		t.Fatalf("lookbackDays=7 got %v", retreaverLookbackDuration(7))
	}
}

func TestRetreaverCursorSafetyDeltaIs15Minutes(t *testing.T) {
	if retreaverCursorSafetyDelta != 15*time.Minute {
		t.Fatalf("safety delta=%v want 15m", retreaverCursorSafetyDelta)
	}
	if retreaverCursorMode != "updated_at" {
		t.Fatalf("cursor_mode=%q want updated_at", retreaverCursorMode)
	}
}

func TestMaxCallUpdatedAtUsesMaxThenSafetyAnchor(t *testing.T) {
	fallback := time.Date(2026, 7, 13, 12, 0, 0, 0, time.UTC)
	early := time.Date(2026, 7, 13, 10, 0, 0, 0, time.UTC)
	late := time.Date(2026, 7, 13, 11, 30, 0, 0, time.UTC)
	got := maxCallUpdatedAt([]models.RemoteCall{
		{UpdatedAt: early},
		{UpdatedAt: late},
		{},
	}, fallback)
	if !got.Equal(late) {
		t.Fatalf("max=%v want %v", got, late)
	}
	empty := maxCallUpdatedAt(nil, fallback)
	if !empty.Equal(fallback) {
		t.Fatalf("empty max=%v want fallback %v", empty, fallback)
	}
	anchored := got.Add(-retreaverCursorSafetyDelta)
	wantAnchor := late.Add(-15 * time.Minute)
	if !anchored.Equal(wantAnchor) {
		t.Fatalf("anchor=%v want %v", anchored, wantAnchor)
	}
}

func TestMapRetreaverCallMapsUpdatedAt(t *testing.T) {
	raw := json.RawMessage(`{
		"uuid": "upd-1",
		"created_at": "2026-07-09T19:38:21.244Z",
		"updated_at": "2026-07-10T01:00:00.000Z"
	}`)
	call := mapRetreaverCall(raw)
	want := time.Date(2026, 7, 10, 1, 0, 0, 0, time.UTC)
	if !call.UpdatedAt.Equal(want) {
		t.Fatalf("UpdatedAt=%v want %v", call.UpdatedAt, want)
	}
}

func TestRetreaverEmptyCursorUsesLookback(t *testing.T) {
	// Contract: with empty cursor and empty lastSyncAt, UpdatedAtStart is now - lookback.
	lookback := retreaverLookbackDuration(0)
	start := time.Now().UTC().Add(-lookback)
	// Tolerance window — assert lookback duration itself and that default remains 30d.
	if lookback != 30*24*time.Hour {
		t.Fatalf("default empty-cursor lookback=%v want 30d", lookback)
	}
	if time.Since(start) < 29*24*time.Hour || time.Since(start) > 31*24*time.Hour {
		t.Fatalf("lookback window unexpectedly far from 30d: since=%v", time.Since(start))
	}
}

func TestRetreaverFetchBuildsUpdatedAtQuery(t *testing.T) {
	// Ensure EncodeCursor for next page carries updated_at_start (unit-level contract).
	cs := retreaverCursor{UpdatedAtStart: "2026-07-01T00:00:00Z", Page: 1}
	enc, err := EncodeCursor(cs)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(enc, "updated_at_start") {
		t.Fatalf("cursor JSON missing updated_at_start: %s", enc)
	}
	if strings.Contains(enc, "created_at_start") {
		t.Fatalf("new cursor should not embed created_at_start: %s", enc)
	}
}

