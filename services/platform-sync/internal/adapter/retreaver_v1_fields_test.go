package adapter

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/Syed-Tajdar/calyx-ai/packages/pingdedup"
	"github.com/Syed-Tajdar/calyx-ai/services/platform-sync/internal/models"
	"github.com/Syed-Tajdar/calyx-ai/services/platform-sync/internal/producer"
)

// v1 mapping — campaign name, buyer, and recording booleans for Retreaver.
//
// recording_url is read only to derive has_recording / recording_known. The URL must never
// reach RemoteCall, the content hash, or the Kafka message. The value below is distinctive so a
// substring search proves it is absent.
const (
	testRecordingURL   = "https://recordings.example.invalid/rec/ZZ-REC-TOKEN-7f3a9c.mp3"
	testRecordingToken = "ZZ-REC-TOKEN-7f3a9c"
)

// fullContentOf mirrors syncer.remoteCallContent field for field (the real hash input).
func fullContentOf(c models.RemoteCall) pingdedup.Content {
	return pingdedup.Content{
		ExternalCallID:             c.ExternalCallID,
		Revenue:                    c.Revenue,
		Payout:                     c.Payout,
		State:                      c.State,
		EventType:                  c.EventType,
		OccurredAt:                 c.OccurredAt,
		CampaignID:                 c.CampaignID,
		PublisherID:                c.PublisherID,
		TargetID:                   c.TargetID,
		PlatformDuplicate:          c.PlatformDuplicate,
		CampaignName:               c.CampaignName,
		PublisherName:              c.PublisherName,
		TargetName:                 c.TargetName,
		BuyerID:                    c.BuyerID,
		BuyerName:                  c.BuyerName,
		DurationSeconds:            c.DurationSeconds,
		CallLengthSeconds:          c.CallLengthSeconds,
		HasRecording:               c.HasRecording,
		ProfitGross:                c.ProfitGross,
		HasConverted:               c.HasConverted,
		HasConnected:               c.HasConnected,
		WasBlocked:                 c.WasBlocked,
		BlockReason:                c.BlockReason,
		WasConversionAdjusted:      c.WasConversionAdjusted,
		WasPayoutAdjusted:          c.WasPayoutAdjusted,
		RequiredDurationSeconds:    c.RequiredDurationSeconds,
		MaxRequiredDurationSeconds: c.MaxRequiredDurationSeconds,
	}
}

// retreaverRowWithout builds retreaverRow(overrides) and then drops the named keys, so a test can
// model "key absent" (retreaverRow alone can only set values, including null).
func retreaverRowWithout(overrides map[string]any, drop ...string) json.RawMessage {
	var wrapped struct {
		Call map[string]any `json:"call"`
	}
	_ = json.Unmarshal(retreaverRow(overrides), &wrapped)
	for _, k := range drop {
		delete(wrapped.Call, k)
	}
	b, _ := json.Marshal(map[string]any{"call": wrapped.Call})
	return b
}

func TestRetreaverV1MapsCampaignName(t *testing.T) {
	c := mapRetreaverCall(retreaverRow(map[string]any{"campaign_name": "  Final Expense US  "}))
	if c.CampaignName != "Final Expense US" {
		t.Fatalf("CampaignName = %q, want %q", c.CampaignName, "Final Expense US")
	}
	// Absent → empty, same as before this change.
	if c := mapRetreaverCall(retreaverRow(nil)); c.CampaignName != "" {
		t.Fatalf("absent campaign_name must map to empty, got %q", c.CampaignName)
	}
}

// Retreaver has no buyer field; the buyer is the target the caller was connected to.
func TestRetreaverV1MapsBuyerFromTarget(t *testing.T) {
	c := mapRetreaverCall(retreaverRow(map[string]any{"system_target_id": 27449, "target_id": 42, "target_name": "Buyer A"}))
	if c.BuyerID != "27449" || c.TargetID != "27449" {
		t.Fatalf("BuyerID=%q TargetID=%q, want both 27449 (system_target_id wins)", c.BuyerID, c.TargetID)
	}
	if c.BuyerName != "Buyer A" || c.TargetName != "Buyer A" {
		t.Fatalf("BuyerName=%q TargetName=%q, want both %q", c.BuyerName, c.TargetName, "Buyer A")
	}

	none := mapRetreaverCall(retreaverRowWithout(nil, "target_id", "system_target_id", "target_name"))
	if none.BuyerID != "" || none.BuyerName != "" {
		t.Fatalf("no target → buyer must be empty, got id=%q name=%q", none.BuyerID, none.BuyerName)
	}
	if none.TargetID != "0" {
		t.Fatalf("TargetID historical behaviour changed: got %q, want \"0\"", none.TargetID)
	}
}

func TestRetreaverV1RecordingFlags(t *testing.T) {
	for _, tc := range []struct {
		name      string
		raw       json.RawMessage
		wantHas   bool
		wantKnown bool
	}{
		{"url string", retreaverRow(map[string]any{"recording_url": testRecordingURL}), true, true},
		{"null", retreaverRow(map[string]any{"recording_url": nil}), false, true},
		{"empty string", retreaverRow(map[string]any{"recording_url": ""}), false, true},
		{"whitespace", retreaverRow(map[string]any{"recording_url": "   "}), false, true},
		{"key absent", retreaverRowWithout(nil, "recording_url"), false, false},
		{"unexpected type", retreaverRow(map[string]any{"recording_url": 123}), false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := mapRetreaverCall(tc.raw)
			if c.HasRecording != tc.wantHas || c.RecordingKnown != tc.wantKnown {
				t.Fatalf("has_recording=%v recording_known=%v, want %v/%v",
					c.HasRecording, c.RecordingKnown, tc.wantHas, tc.wantKnown)
			}
		})
	}
}

// The URL must not appear anywhere in what platform-sync produces.
func TestRetreaverV1RecordingURLNeverLeaves(t *testing.T) {
	c := mapRetreaverCall(retreaverRow(map[string]any{"recording_url": testRecordingURL, "campaign_name": "Camp"}))
	if !c.HasRecording || !c.RecordingKnown {
		t.Fatalf("precondition: expected has_recording and recording_known true")
	}

	asGo := fmt.Sprintf("%#v", c)
	asJSON, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	msg, err := producer.BuildMessageJSON("tenant-1", "retreaver", c, false, 1_700_000_000_000, make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	for name, s := range map[string]string{
		"RemoteCall (%#v)":   asGo,
		"RemoteCall (json)":  string(asJSON),
		"Kafka message json": string(msg),
	} {
		for _, needle := range []string{testRecordingURL, testRecordingToken, "recordings.example.invalid", "recording_url"} {
			if strings.Contains(s, needle) {
				t.Fatalf("%s contains %q", name, needle)
			}
		}
	}

	var decoded map[string]any
	if err := json.Unmarshal(msg, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded["has_recording"] != true || decoded["recording_known"] != true {
		t.Fatalf("Kafka message: has_recording=%v recording_known=%v, want true/true",
			decoded["has_recording"], decoded["recording_known"])
	}
	if decoded["campaign_name"] != "Camp" || decoded["buyer_name"] != "Buyer A" || decoded["buyer_id"] != "42" {
		t.Fatalf("Kafka message: campaign_name=%v buyer_name=%v buyer_id=%v",
			decoded["campaign_name"], decoded["buyer_name"], decoded["buyer_id"])
	}
}

// The new keys must not move money, counts, identity, time or duplicate flags.
func TestRetreaverV1NewFieldsDoNotTouchExistingMapping(t *testing.T) {
	type core struct {
		ID, State, Event, CampaignID, PublisherID, TargetID, TargetName, PublisherName string
		Revenue, Payout, Profit                                                        float64
		Dup, Converted, ConvPresent, Connected                                         bool
		Talk, Whole                                                                    uint32
		Occurred, Updated                                                              int64
	}
	coreOf := func(c models.RemoteCall) core {
		return core{c.ExternalCallID, c.State, c.EventType, c.CampaignID, c.PublisherID, c.TargetID,
			c.TargetName, c.PublisherName, c.Revenue, c.Payout, c.ProfitGross, c.PlatformDuplicate,
			c.HasConverted, c.HasConvertedPresent, c.HasConnected, c.DurationSeconds,
			c.CallLengthSeconds, c.OccurredAt.UnixNano(), c.UpdatedAt.UnixNano()}
	}
	extra := map[string]any{"campaign_name": "Camp", "recording_url": testRecordingURL}

	for _, ov := range []map[string]any{
		nil,
		{"revenue": 42.5, "payout": 30.0, "converted": true},
		{"duplicate": true},
		{"duplicate": nil, "payable_duplicate": true},
	} {
		with := map[string]any{}
		for k, v := range ov {
			with[k] = v
		}
		for k, v := range extra {
			with[k] = v
		}
		if a, b := coreOf(mapRetreaverCall(retreaverRow(ov))), coreOf(mapRetreaverCall(retreaverRow(with))); a != b {
			t.Fatalf("adding campaign_name/recording_url changed core mapping:\n%+v\n%+v", a, b)
		}
	}

	fix := loadGoldenFixture(t, "retreaver")
	for name, tc := range map[string]struct {
		raw     json.RawMessage
		revenue float64
	}{
		"valid_call":     {fix.ValidCall, 46.5},
		"revenue_null":   {fix.RevenueNull, 0},
		"revenue_number": {fix.RevenueNumber, 12.5},
		"extra_fields":   {fix.ExtraFields, 0},
	} {
		c := mapRetreaverCall(tc.raw)
		if c.Revenue != tc.revenue || c.PlatformDuplicate {
			t.Fatalf("%s: revenue=%v dup=%v, want %v/false", name, c.Revenue, c.PlatformDuplicate, tc.revenue)
		}
		// The fixtures carry no campaign_name / recording_url, so those stay at their zero value.
		if c.CampaignName != "" || c.HasRecording || c.RecordingKnown {
			t.Fatalf("%s: unexpected new fields campaign=%q has=%v known=%v", name, c.CampaignName, c.HasRecording, c.RecordingKnown)
		}
	}
}

// Content hash: what moves it and what must not.
func TestRetreaverV1ContentHash(t *testing.T) {
	hash := func(raw json.RawMessage) string { return pingdedup.Hash(fullContentOf(mapRetreaverCall(raw))) }
	base := hash(retreaverRowWithout(nil, "recording_url"))

	// A real change to a newly mapped field republishes the call.
	for name, raw := range map[string]json.RawMessage{
		"campaign_name set":  retreaverRow(map[string]any{"campaign_name": "Camp"}),
		"recording appeared": retreaverRow(map[string]any{"recording_url": testRecordingURL}),
		"target changed":     retreaverRow(map[string]any{"system_target_id": 99}),
	} {
		if hash(raw) == base {
			t.Fatalf("%s: content hash did not change", name)
		}
	}

	// recording_known is not hashed: key absent vs null must not republish (no ping-pong).
	if got := hash(retreaverRow(map[string]any{"recording_url": nil})); got != base {
		t.Fatalf("recording_url null vs absent changed the hash")
	}
	// The URL is not hashed: a re-signed / rotated URL must not republish.
	a := hash(retreaverRow(map[string]any{"recording_url": testRecordingURL}))
	b := hash(retreaverRow(map[string]any{"recording_url": testRecordingURL + "?sig=rotated"}))
	if a != b {
		t.Fatalf("a different recording URL changed the hash — the URL must not be part of the content")
	}
}
