package adapter

import (
	"testing"
	"time"
)

func TestBuildRingbaIncrementalWindowNormalTick(t *testing.T) {
	now := time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC)
	last := now.Add(-2 * time.Minute).Format(time.RFC3339)
	start, end, capped := buildRingbaIncrementalWindow(last, 30, now)
	if capped {
		t.Fatal("must not cap")
	}
	if !end.Equal(now) {
		t.Fatalf("end=%v want %v", end, now)
	}
	wantStart := now.Add(-2*time.Minute - 6*time.Hour)
	if !start.Equal(wantStart) {
		t.Fatalf("start=%v want %v", start, wantStart)
	}
}

func TestBuildRingbaIncrementalWindowFailedRunKeepsLastSync(t *testing.T) {
	// Previous run failed → last_sync_at unchanged → next window still from old last−6h.
	now := time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC)
	oldLast := time.Date(2026, 9, 25, 10, 0, 0, 0, time.UTC) // 2h ago, unchanged after failure
	start, end, capped := buildRingbaIncrementalWindow(oldLast.Format(time.RFC3339), 30, now)
	if capped {
		t.Fatal("must not cap")
	}
	wantStart := oldLast.Add(-6 * time.Hour)
	if !start.Equal(wantStart) || !end.Equal(now) {
		t.Fatalf("start=%v end=%v want start=%v end=%v", start, end, wantStart, now)
	}
}

func TestBuildRingbaIncrementalWindowDowntime8h(t *testing.T) {
	now := time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC)
	last := now.Add(-8 * time.Hour).Format(time.RFC3339)
	start, end, capped := buildRingbaIncrementalWindow(last, 30, now)
	if capped {
		t.Fatal("must not cap for 8h+6h < 30d")
	}
	wantStart := now.Add(-8*time.Hour - 6*time.Hour)
	if !start.Equal(wantStart) || !end.Equal(now) {
		t.Fatalf("start=%v end=%v want start=%v end=%v", start, end, wantStart, now)
	}
}

func TestBuildRingbaIncrementalWindowCappedAtLookback(t *testing.T) {
	now := time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC)
	// last_sync so old that last−6h is before lookback (1 day for test)
	last := now.Add(-10 * 24 * time.Hour).Format(time.RFC3339)
	before := WindowCappedTotal()
	start, end, capped := buildRingbaIncrementalWindow(last, 1, now)
	if !capped {
		t.Fatal("want capped")
	}
	wantStart := now.Add(-24 * time.Hour)
	if !start.Equal(wantStart) || !end.Equal(now) {
		t.Fatalf("start=%v end=%v want start=%v end=%v", start, end, wantStart, now)
	}
	_ = before // counter incremented in FetchPage, not here
}

func TestBuildRingbaIncrementalWindowIgnoresStoredCursor(t *testing.T) {
	// Cursor is unused by buildRingbaIncrementalWindow — documented by absence of cursor param.
	now := time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC)
	last := now.Add(-30 * time.Minute).Format(time.RFC3339)
	start, end, _ := buildRingbaIncrementalWindow(last, 30, now)
	wantStart := now.Add(-30*time.Minute - 6*time.Hour)
	if !start.Equal(wantStart) || !end.Equal(now) {
		t.Fatalf("bounds must come from last_sync−overlap, not stored cursor; got %v–%v", start, end)
	}
}

func TestBuildRingbaSweepWindow(t *testing.T) {
	now := time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC)
	start, end := buildRingbaSweepWindow(now, 7)
	if !end.Equal(now) {
		t.Fatalf("end=%v", end)
	}
	wantStart := now.Add(-7 * 24 * time.Hour)
	if !start.Equal(wantStart) {
		t.Fatalf("start=%v want %v", start, wantStart)
	}
}

func TestRingbaDefaultCursorUsesSixHourOverlap(t *testing.T) {
	prev := ringbaNow
	t.Cleanup(func() { ringbaNow = prev })
	fixed := time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC)
	ringbaNow = func() time.Time { return fixed }

	last := "2026-09-25T11:58:00Z"
	cs := ringbaDefaultCursor(last, 30)
	start, err := time.Parse(time.RFC3339, cs.ReportStart)
	if err != nil {
		t.Fatal(err)
	}
	anchor, _ := time.Parse(time.RFC3339, last)
	want := anchor.Add(-6 * time.Hour)
	if !start.Equal(want) {
		t.Fatalf("start=%v want %v (6h overlap)", start, want)
	}
}
