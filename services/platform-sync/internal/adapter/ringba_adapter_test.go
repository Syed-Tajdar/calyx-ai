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
	if cs.Offset != 0 || cs.Size != 50 {
		t.Fatalf("cursor pagination %#v", cs)
	}
}
