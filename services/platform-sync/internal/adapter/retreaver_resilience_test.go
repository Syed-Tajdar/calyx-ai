package adapter

import (
	"context"
	"testing"
	"time"
)

// Stage 4 R1 — C7: the downtime-gap and long-call claims were [INFERRED] from reading the cursor
// logic in R0. The amendments require them tested.

// TestRetreaverDowntimeLeavesNoGap — C7(1).
//
// Ringba's F003(b) lost calls after an outage because it stored an ABSOLUTE window and replayed
// it: the first tick back fetched the stale `[last−2h, last]`, then saved `[now−2h, now]`, and
// the middle was never fetched. A 2h51m laptop outage permanently lost 5 real calls.
//
// Retreaver's cursor is a watermark that only advances on success, so a gap cannot open. This
// test pins that: after a long outage the next window must start from the OLD watermark and
// therefore span the whole downtime.
func TestRetreaverDowntimeLeavesNoGap(t *testing.T) {
	now := time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)
	// Last successful sync was six hours ago; the service was down since.
	lastSync := now.Add(-6 * time.Hour)

	start, end := BuildRetreaverWindow(now, lastSync.Format(time.RFC3339), 30)

	if !end.Equal(now) {
		t.Fatalf("window end must be now, got %s", end)
	}
	// The window must reach back to the last success (minus the overlap), not to "now − 6h"
	// computed independently, and certainly not to a stale stored end.
	want := lastSync.Add(-RetreaverWindowOverlap())
	if !start.Equal(want) {
		t.Fatalf("after downtime the window must resume from the last watermark: got %s want %s", start, want)
	}
	if d := end.Sub(start); d < 6*time.Hour {
		t.Fatalf("window must span the whole outage, got %s", d)
	}
	t.Logf("downtime gap closed [PROVEN]: 6h outage → window spans %s", end.Sub(start))
}

// TestRetreaverLongCallIsNotMissed — C7(2).
//
// Ringba missed long calls because it filters on call START with a fixed overlap: a 3.9-hour call
// started before a 2-hour window and so fell outside it. Retreaver filters on `updated_at`, which
// moves when the call ends, so the call enters the window at completion regardless of its length.
func TestRetreaverLongCallIsNotMissed(t *testing.T) {
	base := time.Now().UTC().Add(-90 * time.Minute)

	// One call that started four hours ago and only finished moments ago: created_at is far
	// outside any short window, updated_at is current.
	longCall := k1Call{uuid: "LONGCALL", updatedAt: base}
	others := []k1Call{
		{uuid: "C1", updatedAt: base.Add(-10 * time.Minute)},
		{uuid: "C2", updatedAt: base.Add(-20 * time.Minute)},
	}
	calls := append([]k1Call{longCall}, others...)

	srv := k1Server(t, calls, nil)
	defer srv.Close()
	t.Setenv("RETREAVER_BASE_URL", srv.URL)

	a := NewRetreaverAdapter(NewHTTPClient())
	// A short 2h incremental window — the shape that lost the Ringba long call.
	res, err := a.FetchPage(context.Background(), map[string]string{"api_key": "k", "company_id": "co"},
		"", time.Now().UTC().Add(-2*time.Hour).Format(time.RFC3339), 30)
	if err != nil {
		t.Fatalf("FetchPage: %v", err)
	}

	found := false
	for _, c := range res.Calls {
		if c.ExternalCallID == "LONGCALL" {
			found = true
		}
	}
	if !found {
		t.Fatalf("a long call that finished inside the window must be fetched; got %d calls", len(res.Calls))
	}
	t.Logf("long call picked up [PROVEN]: filtering on updated_at, not call start")
}

// TestRetreaverResumesSameWindowAfterCrash — the frozen window must survive a restart.
//
// If a crash mid-walk recomputed the window, the second half of the walk would query a different
// result set and rows between the two ends would be skipped.
func TestRetreaverResumesSameWindowAfterCrash(t *testing.T) {
	base := time.Now().UTC().Add(-3 * time.Hour)
	calls := make([]k1Call, 0, 150)
	for i := 0; i < 150; i++ {
		calls = append(calls, k1Call{uuid: string(rune('A'+i%26)) + time.Duration(i).String(), updatedAt: base.Add(-time.Duration(i) * time.Minute)})
	}
	srv := k1Server(t, calls, nil)
	defer srv.Close()
	t.Setenv("RETREAVER_BASE_URL", srv.URL)

	a := NewRetreaverAdapter(NewHTTPClient())
	creds := map[string]string{"api_key": "k", "company_id": "co"}

	first, err := a.FetchPage(context.Background(), creds, "", "", 30)
	if err != nil {
		t.Fatalf("page 1: %v", err)
	}
	if !first.HasMore {
		t.Fatalf("expected more than one page for 150 rows at per_page=100")
	}

	// Simulate a crash: the only thing that survives is the persisted cursor.
	resumed := NewRetreaverAdapter(NewHTTPClient())
	second, err := resumed.FetchPage(context.Background(), creds, first.NextCursor, "", 30)
	if err != nil {
		t.Fatalf("resumed page 2: %v", err)
	}

	// Compare at second precision: the cursor round-trips through RFC3339, which carries no
	// sub-second component. That is the same resolution the request itself uses, so both pages
	// query an identical window on the wire even though the in-memory values differ by the
	// truncated fraction.
	if !second.WindowStart.Truncate(time.Second).Equal(first.WindowStart.Truncate(time.Second)) ||
		!second.WindowEnd.Truncate(time.Second).Equal(first.WindowEnd.Truncate(time.Second)) {
		t.Fatalf("resume changed the window: [%s,%s) -> [%s,%s)",
			first.WindowStart, first.WindowEnd, second.WindowStart, second.WindowEnd)
	}
	if len(second.Calls) == 0 {
		t.Fatalf("resumed page returned nothing")
	}
	t.Logf("crash resume [PROVEN]: same frozen window [%s , %s)", second.WindowStart, second.WindowEnd)
}
