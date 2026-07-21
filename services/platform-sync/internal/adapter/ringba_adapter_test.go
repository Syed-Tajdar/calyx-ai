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
