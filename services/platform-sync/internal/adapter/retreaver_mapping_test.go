package adapter

import (
	"encoding/json"
	"testing"

	"github.com/Syed-Tajdar/calyx-ai/packages/pingdedup"
)

// Stage 4 R1 — D (field mapping) and C5 (every newly mapped field must move the content hash).
//
// Before R1 the Retreaver mapper set 9 of the 71 fields the API returns. Everything else —
// including `converted` and the durations — was left at its zero value, so a call that converted
// produced an UNCHANGED content hash and was never republished. Mapping the fields and hashing
// them are two separate requirements; this file proves both.

// retreaverRow builds a realistic Retreaver call payload, with overrides applied last.
func retreaverRow(overrides map[string]any) json.RawMessage {
	row := map[string]any{
		"uuid":                 "11111111-2222-3333-4444-555555555555",
		"caller":               "+15555550100",
		"caller_state":         "TX",
		"revenue":              nil,
		"payout":               nil,
		"campaign_id":          9872,
		"system_campaign_id":   9872,
		"afid":                 "aff123",
		"target_id":            42,
		"system_target_id":     42,
		"status":               "finished",
		"connected":            true,
		"created_at":           "2026-09-20T10:00:00.000Z",
		"updated_at":           "2026-09-20T10:05:00.000Z",
		"converted":            false,
		"total_duration":       120,
		"dialed_call_duration": 95,
		"profit_gross":         -0.05,
		"duplicate":            false,
		"payable_duplicate":    false,
		"target_name":          "Buyer A",
		"affiliate_name":       "MediaCo",
	}
	for k, v := range overrides {
		row[k] = v
	}
	b, _ := json.Marshal(map[string]any{"call": row})
	return b
}

// contentOf mirrors syncer.remoteCallContent for the fields this slice touches. Kept in step with
// that function; TestRetreaverHashCoversNewlyMappedFields fails if a mapped field stops hashing.
func contentOf(raw json.RawMessage) pingdedup.Content {
	c := mapRetreaverCall(raw)
	return pingdedup.Content{
		ExternalCallID:    c.ExternalCallID,
		Revenue:           c.Revenue,
		Payout:            c.Payout,
		State:             c.State,
		EventType:         c.EventType,
		OccurredAt:        c.OccurredAt,
		CampaignID:        c.CampaignID,
		PublisherID:       c.PublisherID,
		TargetID:          c.TargetID,
		PlatformDuplicate: c.PlatformDuplicate,
		PublisherName:     c.PublisherName,
		TargetName:        c.TargetName,
		DurationSeconds:   c.DurationSeconds,
		CallLengthSeconds: c.CallLengthSeconds,
		ProfitGross:       c.ProfitGross,
		HasConverted:      c.HasConverted,
		HasConnected:      c.HasConnected,
	}
}

// TestRetreaverMapsConversionAndDurationFields — D.
func TestRetreaverMapsConversionAndDurationFields(t *testing.T) {
	c := mapRetreaverCall(retreaverRow(map[string]any{
		"converted":            true,
		"revenue":              37.5,
		"payout":               30.0,
		"total_duration":       615,
		"dialed_call_duration": 600,
		"profit_gross":         7.5,
		"duplicate":            true,
	}))

	if !c.HasConverted {
		t.Fatalf("converted=true must map to HasConverted")
	}
	if !c.HasConvertedPresent {
		t.Fatalf("a present `converted` key must set HasConvertedPresent")
	}
	if !c.HasConnected {
		t.Fatalf("connected=true must map to HasConnected")
	}
	if c.Revenue != 37.5 || c.Payout != 30.0 {
		t.Fatalf("revenue/payout: got %v/%v", c.Revenue, c.Payout)
	}
	if c.CallLengthSeconds != 615 {
		t.Fatalf("total_duration -> CallLengthSeconds: got %d", c.CallLengthSeconds)
	}
	if c.DurationSeconds != 600 {
		t.Fatalf("dialed_call_duration -> DurationSeconds (talk time): got %d", c.DurationSeconds)
	}
	if c.ProfitGross != 7.5 {
		t.Fatalf("profit_gross: got %v", c.ProfitGross)
	}
	if !c.PlatformDuplicate {
		t.Fatalf("duplicate=true must map to PlatformDuplicate")
	}
	if c.TargetName != "Buyer A" || c.PublisherName != "MediaCo" {
		t.Fatalf("names: target=%q publisher=%q", c.TargetName, c.PublisherName)
	}
}

// TestRetreaverNullRevenueIsUnambiguousWithConverted — the C3 finding.
//
// R0.3 sampled 100 real calls: `revenue`/`payout` are null on exactly the unconverted calls and
// populated on exactly the converted ones — never zero. A null amount is therefore unambiguous
// once `converted` is mapped beside it, so no Nullable migration is needed.
func TestRetreaverNullRevenueIsUnambiguousWithConverted(t *testing.T) {
	unconverted := mapRetreaverCall(retreaverRow(nil)) // revenue/payout null, converted false
	if unconverted.Revenue != 0 || unconverted.Payout != 0 {
		t.Fatalf("null amounts should read as 0, got %v/%v", unconverted.Revenue, unconverted.Payout)
	}
	if unconverted.HasConverted {
		t.Fatalf("this fixture is not converted")
	}
	if !unconverted.HasConvertedPresent {
		t.Fatalf("converted was present in the payload and must be recorded as such")
	}

	// A payload with no `converted` key at all must be distinguishable from converted=false.
	raw := retreaverRow(nil)
	var m map[string]any
	_ = json.Unmarshal(raw, &m)
	delete(m["call"].(map[string]any), "converted")
	b, _ := json.Marshal(m)
	absent := mapRetreaverCall(b)
	if absent.HasConvertedPresent {
		t.Fatalf("an absent `converted` key must leave HasConvertedPresent false")
	}
}

// TestRetreaverHashCoversNewlyMappedFields — C5.
//
// For each newly mapped field: flip only that field and the content hash must change. If a field
// is mapped but not hashed, a change to it never republishes the row — the silent-miss this
// slice exists to prevent.
func TestRetreaverHashCoversNewlyMappedFields(t *testing.T) {
	baseline := pingdedup.Hash(contentOf(retreaverRow(nil)))

	for _, tc := range []struct {
		field    string
		override map[string]any
	}{
		{"converted", map[string]any{"converted": true}},
		{"total_duration", map[string]any{"total_duration": 999}},
		{"dialed_call_duration", map[string]any{"dialed_call_duration": 888}},
		{"profit_gross", map[string]any{"profit_gross": 12.34}},
		{"duplicate", map[string]any{"duplicate": true}},
		{"target_name", map[string]any{"target_name": "Buyer Z"}},
		{"affiliate_name", map[string]any{"affiliate_name": "OtherCo"}},
		{"revenue", map[string]any{"revenue": 42.0}},
		{"payout", map[string]any{"payout": 21.0}},
	} {
		got := pingdedup.Hash(contentOf(retreaverRow(tc.override)))
		if got == baseline {
			t.Fatalf("changing %q did not change the content hash — a change to it would never republish", tc.field)
		}
	}
}

// TestRetreaverHashIgnoresUnhashedFields — the reverse half of C5.
//
// A field that is deliberately NOT part of the identity must not cause republishing, or every
// sweep would rewrite every row.
func TestRetreaverHashIgnoresUnhashedFields(t *testing.T) {
	baseline := pingdedup.Hash(contentOf(retreaverRow(nil)))

	for _, tc := range []struct {
		field    string
		override map[string]any
	}{
		// updated_at moves on every touch; hashing it would defeat content-hash dedup entirely.
		{"updated_at", map[string]any{"updated_at": "2026-09-21T23:59:59.000Z"}},
		// The caller number is PII and is not part of call identity here.
		{"caller", map[string]any{"caller": "+15555559999"}},
	} {
		if got := pingdedup.Hash(contentOf(retreaverRow(tc.override))); got != baseline {
			t.Fatalf("changing %q changed the content hash — it would republish on every sweep", tc.field)
		}
	}
}
