package adapter

import (
	"testing"
	"time"
)

// Stage 4 R1 — C1 (window splitting) and C9 (per-account throttle).

// TestShrinkWindowHalvesAndStopsAtFloor covers the split arithmetic.
//
// C1: when a window holds more rows than the page cap can walk we shrink the END, never
// "carry the remainder forward". With order=desc the leftover rows are the OLDEST in the
// window, and the next window starts at end−overlap, so carrying them forward would lose them —
// K1 again, created by the fix.
func TestShrinkWindowHalvesAndStopsAtFloor(t *testing.T) {
	start := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)

	end := start.Add(24 * time.Hour)
	got, ok := shrinkRetreaverWindow(start, end)
	if !ok {
		t.Fatalf("a 24h window must be splittable")
	}
	if want := start.Add(12 * time.Hour); !got.Equal(want) {
		t.Fatalf("halve: got %s want %s", got, want)
	}

	// Shrinking must always move the end backwards, never past the start.
	cur := end
	for i := 0; i < 40; i++ {
		next, ok := shrinkRetreaverWindow(start, cur)
		if !ok {
			break
		}
		if !next.After(start) {
			t.Fatalf("shrunk end must stay after start (iteration %d)", i)
		}
		if !next.Before(cur) {
			t.Fatalf("shrunk end must move backwards (iteration %d)", i)
		}
		cur = next
	}
	if cur.Sub(start) > retreaverMinWindow {
		t.Fatalf("shrinking should reach the floor, got span %s", cur.Sub(start))
	}

	// At the floor it must refuse rather than collapse to an empty window.
	if _, ok := shrinkRetreaverWindow(start, start.Add(retreaverMinWindow)); ok {
		t.Fatalf("a floor-sized window must not split further")
	}
	if _, ok := shrinkRetreaverWindow(start, start); ok {
		t.Fatalf("an empty window must not split")
	}
}

// TestRetreaverWindowFitsRespectsPageCap pins the split trigger.
func TestRetreaverWindowFitsRespectsPageCap(t *testing.T) {
	cases := []struct {
		total, perPage, maxPages int
		fits                     bool
	}{
		{0, 100, 200, true},      // Total absent
		{100, 100, 200, true},    // one page
		{20000, 100, 200, true},  // exactly the cap
		{20001, 100, 200, false}, // one row over → must split
		{96747, 100, 200, false}, // an account-sized window (R0.3 observed ~96.7k rows)
	}
	for _, c := range cases {
		if got := retreaverWindowFits(c.total, c.perPage, c.maxPages); got != c.fits {
			t.Fatalf("total=%d perPage=%d maxPages=%d: got fits=%v want %v",
				c.total, c.perPage, c.maxPages, got, c.fits)
		}
	}
}

// TestBuildRetreaverWindowUsesOverlapAndLookback covers window construction.
func TestBuildRetreaverWindowUsesOverlapAndLookback(t *testing.T) {
	now := time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)

	// With a last sync, the window starts one overlap before it.
	last := now.Add(-3 * time.Hour)
	start, end := BuildRetreaverWindow(now, last.Format(time.RFC3339), 30)
	if !end.Equal(now) {
		t.Fatalf("end must be now, got %s", end)
	}
	if want := last.Add(-RetreaverWindowOverlap()); !start.Equal(want) {
		t.Fatalf("start: got %s want %s", start, want)
	}

	// No last sync → fall back to the lookback.
	start, end = BuildRetreaverWindow(now, "", 30)
	if want := now.Add(-30 * 24 * time.Hour); !start.Equal(want) {
		t.Fatalf("lookback start: got %s want %s", start, want)
	}

	// A last sync in the future (clock skew) must not produce an inverted window.
	start, end = BuildRetreaverWindow(now, now.Add(48*time.Hour).Format(time.RFC3339), 30)
	if !start.Before(end) {
		t.Fatalf("future last_sync produced an inverted window: %s .. %s", start, end)
	}
}

// TestRetreaverLimiterIsPerCompanyID — C9.
//
// Two integrations on the same Retreaver account must share one limiter, or together they would
// exceed a limit that is enforced per account. Different accounts must not interfere.
func TestRetreaverLimiterIsPerCompanyID(t *testing.T) {
	l := NewRetreaverAccountLimiters()

	a1 := l.ForAccount("TESTCO_AAAA")
	a2 := l.ForAccount("TESTCO_AAAA")
	if a1 != a2 {
		t.Fatalf("same company_id must share one limiter")
	}

	b := l.ForAccount("TESTCO_BBBB")
	if a1 == b {
		t.Fatalf("different company_ids must not share a limiter")
	}

	// A missing company_id must still be bounded, not unlimited.
	if l.ForAccount("") == nil {
		t.Fatalf("empty company_id must still get a limiter")
	}
}

// TestRetreaverThrottleDefaults documents the chosen numbers.
//
// Retreaver publishes no numeric limit — only a documented 429 and "Highly concurrent requests
// are frowned upon" (https://retreaver.github.io/core-api-docs/#introduction). R0.3 observed
// 20 sequential GETs at ~3.0 req/s with zero 429s. 2/sec · 60/min is a deliberate conservative
// default, approved by the owner (amendments A4), not a measured ceiling.
func TestRetreaverThrottleDefaults(t *testing.T) {
	if got := RetreaverRatePerSec(); got != 2 {
		t.Fatalf("default per-second rate: got %d want 2", got)
	}
	if got := RetreaverRatePerMin(); got != 60 {
		t.Fatalf("default per-minute rate: got %d want 60", got)
	}
	t.Setenv("RETREAVER_RATE_PER_SEC", "5")
	if got := RetreaverRatePerSec(); got != 5 {
		t.Fatalf("env override ignored: got %d", got)
	}
}

// TestRetreaverThrottleActuallyDelays proves the limiter is wired, not just constructed.
func TestRetreaverThrottleActuallyDelays(t *testing.T) {
	lim := NewDualWindowLimiter(2, 60)
	start := time.Now()
	for i := 0; i < 4; i++ {
		lim.Wait()
	}
	// 4 requests at 2/sec cannot complete instantly.
	if elapsed := time.Since(start); elapsed < 900*time.Millisecond {
		t.Fatalf("2/sec limiter let 4 requests through in %s", elapsed)
	}
}

// TestRetreaverSweepDefaults pins the sweep tiers (mirrors Ringba's, per amendments D).
func TestRetreaverSweepDefaults(t *testing.T) {
	if got := RetreaverSweepDays(); got != 7 {
		t.Fatalf("sweep days: got %d want 7", got)
	}
	if got := RetreaverDeepSweepDays(); got != 30 {
		t.Fatalf("deep sweep days: got %d want 30", got)
	}
	if got := RetreaverSweepInterval(); got != time.Hour {
		t.Fatalf("sweep interval: got %s want 1h", got)
	}
	if got := RetreaverDeepSweepInterval(); got != 24*time.Hour {
		t.Fatalf("deep sweep interval: got %s want 24h", got)
	}
	if got := RetreaverMaxPagesPerWindow(); got != 200 {
		t.Fatalf("max pages: got %d want 200", got)
	}

	start, end := BuildRetreaverSweepWindow(time.Date(2026, 9, 30, 0, 0, 0, 0, time.UTC), 7)
	if d := end.Sub(start); d != 7*24*time.Hour {
		t.Fatalf("sweep window span: got %s want 168h", d)
	}
}
