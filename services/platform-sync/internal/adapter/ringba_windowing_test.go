package adapter

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestRingbaOptionalSecondsAbsentZeroPresent(t *testing.T) {
	absent := mustMapRingba(t, []byte(`{"inboundCallId":"a1"}`))
	if absent.RequiredDurationSeconds != nil || absent.MaxRequiredDurationSeconds != nil {
		t.Fatalf("absent key must be nil, got req=%v max=%v", absent.RequiredDurationSeconds, absent.MaxRequiredDurationSeconds)
	}

	zero := mustMapRingba(t, []byte(`{
		"inboundCallId":"a2",
		"ringTreeWinningBidDynamicDuration": 0
	}`))
	if zero.RequiredDurationSeconds == nil || *zero.RequiredDurationSeconds != 0 {
		t.Fatalf("present 0 must store 0, got %v", zero.RequiredDurationSeconds)
	}

	n := mustMapRingba(t, []byte(`{
		"inboundCallId":"a3",
		"ringTreeWinningBidDynamicDuration": 90,
		"ringTreeWinningBidMaxDynamicDuration": 200
	}`))
	if n.RequiredDurationSeconds == nil || *n.RequiredDurationSeconds != 90 {
		t.Fatalf("present 90 got %v", n.RequiredDurationSeconds)
	}
	if n.MaxRequiredDurationSeconds == nil || *n.MaxRequiredDurationSeconds != 200 {
		t.Fatalf("max 200 got %v", n.MaxRequiredDurationSeconds)
	}

	str := mustMapRingba(t, []byte(`{
		"inboundCallId":"a4",
		"ringTreeWinningBidDynamicDuration": "15"
	}`))
	if str.RequiredDurationSeconds == nil || *str.RequiredDurationSeconds != 15 {
		t.Fatalf("string 15 got %v", str.RequiredDurationSeconds)
	}

	names := map[string]bool{}
	for _, col := range ringbaValueColumns() {
		names[col["column"]] = true
	}
	for _, want := range []string{"ringTreeWinningBidDynamicDuration", "ringTreeWinningBidMaxDynamicDuration"} {
		if !names[want] {
			t.Fatalf("valueColumns missing %s", want)
		}
	}
}

func TestRingbaDayWindows(t *testing.T) {
	start := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	end := time.Date(2026, 9, 3, 6, 0, 0, 0, time.UTC)
	ws := ringbaDayWindows(start, end)
	if len(ws) != 2 {
		t.Fatalf("windows=%d want 2", len(ws))
	}
	if !ws[0][0].Equal(start) || !ws[0][1].Equal(start.Add(24*time.Hour)) {
		t.Fatalf("first=%v", ws[0])
	}
	if !ws[1][0].Equal(start.Add(24*time.Hour)) || !ws[1][1].Equal(end) {
		t.Fatalf("second=%v", ws[1])
	}
}

func TestFetchWindowedSplitsFullPage(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		var body struct {
			ReportStart string `json:"reportStart"`
			ReportEnd   string `json:"reportEnd"`
			Size        int    `json:"size"`
			Offset      int    `json:"offset"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		n := 3
		start, _ := time.Parse(time.RFC3339, body.ReportStart)
		end, _ := time.Parse(time.RFC3339, body.ReportEnd)
		// Full page only for the initial day-sized window.
		if end.Sub(start) >= 24*time.Hour {
			n = body.Size
		}
		recs := make([]map[string]interface{}, n)
		for i := 0; i < n; i++ {
			recs[i] = map[string]interface{}{
				"inboundCallId": fmtCall(body.ReportStart, i),
				"callDt":        start.UnixMilli(),
			}
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"report": map[string]interface{}{"records": recs},
		})
	}))
	defer srv.Close()
	t.Setenv("RINGBA_BASE_URL", srv.URL)

	a := NewRingbaAdapter(NewHTTPClient())
	start := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)
	calls, stats, err := a.fetchWindowed(context.Background(), map[string]string{
		"api_token": "t", "account_id": "A1",
	}, start, end)
	if err != nil {
		t.Fatal(err)
	}
	if stats.OffsetFallbackWindows != 0 {
		t.Fatalf("offset fallback=%d", stats.OffsetFallbackWindows)
	}
	if stats.Requests < 3 {
		t.Fatalf("expected split requests, got %d hits=%d", stats.Requests, hits)
	}
	if len(calls) < 3 {
		t.Fatalf("calls=%d", len(calls))
	}
	for _, c := range calls {
		if c.ExternalCallID == "" {
			t.Fatal("empty id")
		}
	}
}

func TestFetchWindowedMinWindowOffsetFallback(t *testing.T) {
	var offsets []int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Size   int `json:"size"`
			Offset int `json:"offset"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		offsets = append(offsets, body.Offset)
		n := body.Size
		if body.Offset > 0 {
			n = 2
		}
		recs := make([]map[string]interface{}, n)
		for i := 0; i < n; i++ {
			recs[i] = map[string]interface{}{
				"inboundCallId": fmtCall("min", body.Offset+i),
				"callDt":        time.Now().UnixMilli(),
			}
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"report": map[string]interface{}{"records": recs},
		})
	}))
	defer srv.Close()
	t.Setenv("RINGBA_BASE_URL", srv.URL)

	a := NewRingbaAdapter(NewHTTPClient())
	start := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	end := start.Add(time.Minute)
	calls, stats, err := a.fetchWindowed(context.Background(), map[string]string{
		"api_token": "t", "account_id": "A1",
	}, start, end)
	if err != nil {
		t.Fatal(err)
	}
	if stats.OffsetFallbackWindows != 1 {
		t.Fatalf("fallback windows=%d", stats.OffsetFallbackWindows)
	}
	if len(offsets) < 2 || offsets[0] != 0 || offsets[1] != ringbaMaxPageSize {
		t.Fatalf("offsets=%v", offsets)
	}
	if len(calls) != ringbaMaxPageSize+2 {
		t.Fatalf("calls=%d", len(calls))
	}
}

func fmtCall(prefix string, i int) string {
	return prefix + "-" + time.Unix(int64(i), 0).UTC().Format("150405") + "-" + itoa(i)
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b [20]byte
	pos := len(b)
	for i > 0 {
		pos--
		b[pos] = byte('0' + i%10)
		i /= 10
	}
	return string(b[pos:])
}
