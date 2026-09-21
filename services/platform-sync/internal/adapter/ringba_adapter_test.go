package adapter

import (
	"testing"
	"time"
)

func TestMapRingbaCallLogEntryOccurredAt(t *testing.T) {
	raw := []byte(`{
		"columns": [
			{"name": "inboundCallId", "value": "id-1"},
			{"name": "callDt", "value": "2026-07-09T19:38:21Z"},
			{"name": "revenue", "value": "1.5"}
		]
	}`)
	got := mapRingbaCallLogEntry(raw)
	if got.ExternalCallID != "id-1" {
		t.Fatalf("id=%q", got.ExternalCallID)
	}
	want := time.Date(2026, 7, 9, 19, 38, 21, 0, time.UTC)
	if !got.OccurredAt.Equal(want) {
		t.Fatalf("OccurredAt=%v want %v", got.OccurredAt, want)
	}
	if got.Revenue != 1.5 {
		t.Fatalf("revenue=%v", got.Revenue)
	}
}

func TestMapRingbaFlatRecordEpochMillisAndConversionAmount(t *testing.T) {
	raw := []byte(`{
		"inboundCallId": "flat-id-1",
		"callDt": 1612219718965,
		"inboundPhoneNumber": "+15551212",
		"conversionAmount": 12.5,
		"payoutAmount": 10,
		"campaignId": "camp-1",
		"publisherId": "pub-1",
		"targetId": "tgt-1",
		"state": "California"
	}`)
	got := mapRingbaCallLogEntry(raw)
	if got.ExternalCallID != "flat-id-1" {
		t.Fatalf("id=%q", got.ExternalCallID)
	}
	want := time.UnixMilli(1612219718965).UTC()
	if !got.OccurredAt.Equal(want) {
		t.Fatalf("OccurredAt=%v want %v", got.OccurredAt, want)
	}
	if got.Revenue != 12.5 || got.Payout != 10 {
		t.Fatalf("money revenue=%v payout=%v", got.Revenue, got.Payout)
	}
	if got.State != "CA" {
		t.Fatalf("state=%q want CA", got.State)
	}
}

func TestParseRingbaCallLogResponseModern(t *testing.T) {
	body := []byte(`{"isSuccessful":true,"report":{"records":[{"inboundCallId":"a"},{"inboundCallId":"b"}]}}`)
	recs, err := parseRingbaCallLogResponse(body)
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) != 2 {
		t.Fatalf("len=%d", len(recs))
	}
}

func TestRingbaDefaultCursorOverlap(t *testing.T) {
	last := "2026-07-10T12:00:00Z"
	cs := ringbaDefaultCursor(last, 30)
	start, err := time.Parse(time.RFC3339, cs.ReportStart)
	if err != nil {
		t.Fatal(err)
	}
	anchor, _ := time.Parse(time.RFC3339, last)
	if !start.Equal(anchor.Add(-ringbaOverlap)) {
		t.Fatalf("start=%v want overlap from last_sync", start)
	}
	if cs.Offset != 0 || cs.Size != ringbaMaxPageSize {
		t.Fatalf("cursor pagination %#v", cs)
	}
}

func TestParseRingbaCallLogResponseEmptyRecords(t *testing.T) {
	body := []byte(`{"isSuccessful":true,"transactionId":"t1","report":{"records":[],"partialResult":false}}`)
	recs, err := parseRingbaCallLogResponse(body)
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) != 0 {
		t.Fatalf("len=%d want 0", len(recs))
	}
}

func TestMapRingbaParityB1Fields(t *testing.T) {
	raw := []byte(`{
		"inboundCallId": "parity-1",
		"callDt": 1612219718965,
		"conversionAmount": 30.6,
		"payoutAmount": 24.48,
		"campaignId": "camp-1",
		"campaignName": "Final Expense",
		"publisherId": "pub-1",
		"publisherName": "Optimize To Convert",
		"targetId": "tgt-1",
		"targetName": "RingbaX Target",
		"buyer": "RingbaX",
		"buyerId": "buy-1",
		"connectedCallLengthInSeconds": 244,
		"callLengthInSeconds": 247,
		"hasRecording": true,
		"isDuplicate": false,
		"profitGross": 6.12,
		"tag:InboundNumber:State": "CA"
	}`)
	got := mapRingbaCallLogEntry(raw)
	if got.CampaignName != "Final Expense" || got.PublisherName != "Optimize To Convert" {
		t.Fatalf("names campaign=%q publisher=%q", got.CampaignName, got.PublisherName)
	}
	if got.BuyerName != "RingbaX" || got.BuyerID != "buy-1" {
		t.Fatalf("buyer name=%q id=%q", got.BuyerName, got.BuyerID)
	}
	if got.DurationSeconds != 244 || got.CallLengthSeconds != 247 {
		t.Fatalf("duration connected=%d total=%d", got.DurationSeconds, got.CallLengthSeconds)
	}
	if !got.HasRecording || !got.RecordingKnown {
		t.Fatalf("recording has=%v known=%v", got.HasRecording, got.RecordingKnown)
	}
	if got.ProfitGross != 6.12 {
		t.Fatalf("profit=%v", got.ProfitGross)
	}
	if got.HasConverted {
		t.Fatal("hasConverted absent must map false")
	}
	if got.PlatformDuplicate {
		t.Fatal("isDuplicate false must map PlatformDuplicate=false")
	}
}

func TestMapRingbaTagInboundNumberState(t *testing.T) {
	raw := []byte(`{
		"inboundCallId": "tag-id-1",
		"callDt": 1612219718965,
		"tag:InboundNumber:State": "Texas",
		"tag:InboundNumber:Region": "South"
	}`)
	got := mapRingbaCallLogEntry(raw)
	if got.State != "TX" {
		t.Fatalf("state=%q want TX (State preferred over Region)", got.State)
	}
}
