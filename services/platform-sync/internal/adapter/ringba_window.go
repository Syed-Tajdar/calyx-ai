package adapter

import (
	"os"
	"strconv"
	"sync/atomic"
	"time"
)

// ringbaNow is overridable in unit tests (fake clock). Production uses UTC wall clock.
var ringbaNow = func() time.Time { return time.Now().UTC() }

// windowCappedTotal counts Ringba incremental windows that were capped at lookback.
var windowCappedTotal atomic.Uint64

// RingbaIncrementalOverlap returns the configured overlap (default 6h).
// Justified vs S0 max call duration 13,994s (~3.9h) with margin.
func RingbaIncrementalOverlap() time.Duration {
	if v := os.Getenv("RINGBA_INCREMENTAL_OVERLAP"); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			return d
		}
	}
	return 6 * time.Hour
}

// RingbaSweepInterval default 60m.
func RingbaSweepInterval() time.Duration {
	if v := os.Getenv("RINGBA_SWEEP_INTERVAL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			return d
		}
	}
	return 60 * time.Minute
}

// RingbaSweepDays default 7.
func RingbaSweepDays() int {
	if v := os.Getenv("RINGBA_SWEEP_DAYS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 1 {
			return n
		}
	}
	return 7
}

// WindowCappedTotal returns how many times an incremental window was capped.
func WindowCappedTotal() uint64 {
	return windowCappedTotal.Load()
}

// buildRingbaIncrementalWindow computes [last_sync−overlap, now] capped at lookback.
// Stored sync_cursor bounds are intentionally unused (F003b).
func buildRingbaIncrementalWindow(lastSyncAt string, lookbackDays int, now time.Time) (start, end time.Time, capped bool) {
	end = now.UTC()
	overlap := RingbaIncrementalOverlap()
	days := ringbaLookbackDays(lookbackDays)
	lookbackStart := end.Add(-time.Duration(days) * 24 * time.Hour)

	start = lookbackStart
	if lastSyncAt != "" {
		if t, err := time.Parse(time.RFC3339, lastSyncAt); err == nil {
			start = t.UTC().Add(-overlap)
		}
	}
	if start.Before(lookbackStart) {
		start = lookbackStart
		capped = true
	}
	if !start.Before(end) {
		// Degenerate: nudge start 1s before end so fetchWindowed has a range.
		start = end.Add(-time.Second)
	}
	return start, end, capped
}

// BuildRingbaSweepWindow returns [now−sweepDays, now].
func BuildRingbaSweepWindow(now time.Time, sweepDays int) (start, end time.Time) {
	return buildRingbaSweepWindow(now, sweepDays)
}

// buildRingbaSweepWindow returns [now−sweepDays, now].
func buildRingbaSweepWindow(now time.Time, sweepDays int) (start, end time.Time) {
	end = now.UTC()
	if sweepDays < 1 {
		sweepDays = RingbaSweepDays()
	}
	start = end.Add(-time.Duration(sweepDays) * 24 * time.Hour)
	return start, end
}
