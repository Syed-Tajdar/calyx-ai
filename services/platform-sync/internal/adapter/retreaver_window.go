package adapter

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/Syed-Tajdar/calyx-ai/services/platform-sync/internal/models"
)

// Stage 4 R1 — Retreaver frozen-window paging.
//
// K1 (audit F012) is caused by paging with page/per_page over a window that has a start but no
// end, sorted `updated_at desc`. The result set is live: a row whose `updated_at` changes during
// the walk jumps to position 1 and pushes every later row down one place, so a row due at the top
// of page 2 slides to the bottom of page 1 — already fetched — and is never returned.
//
// R0.3 proved `updated_at_end` IS honoured, so each walk can run inside a window whose end is
// fixed when the window opens. A row that updates mid-walk leaves the window entirely and is
// collected by the next window, which starts at `end − overlap`.
//
// R0.3 also proved `order=asc` is ignored on `updated_at`, so ascending keyset paging — the usual
// alternative — was never available. The frozen window is the only correct option.

const (
	// retreaverWindowOverlapDefault re-reads a little before the previous window's end so a row
	// updated in the instant the window closed is not stranded between two windows.
	retreaverWindowOverlapDefault = 15 * time.Minute

	// retreaverMaxPagesDefault bounds a single window's page walk.
	retreaverMaxPagesDefault = 200

	// retreaverMinWindow is the floor for window splitting. Below this we accept the window even
	// if it exceeds the page cap: splitting further cannot help, because the rows share a
	// timestamp granularity we cannot subdivide.
	retreaverMinWindow = time.Minute
)

func envDuration(name string, def time.Duration) time.Duration {
	if v := os.Getenv(name); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			return d
		}
	}
	return def
}

func envInt(name string, def int) int {
	if v := os.Getenv(name); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return def
}

// RetreaverWindowOverlap — `RETREAVER_WINDOW_OVERLAP` (default 15m).
func RetreaverWindowOverlap() time.Duration {
	return envDuration("RETREAVER_WINDOW_OVERLAP", retreaverWindowOverlapDefault)
}

// RetreaverMaxPagesPerWindow — `RETREAVER_MAX_PAGES_PER_WINDOW` (default 200).
func RetreaverMaxPagesPerWindow() int {
	return envInt("RETREAVER_MAX_PAGES_PER_WINDOW", retreaverMaxPagesDefault)
}

// RetreaverSweepDays — `RETREAVER_SWEEP_DAYS` (default 7): hourly re-check window.
func RetreaverSweepDays() int { return envInt("RETREAVER_SWEEP_DAYS", 7) }

// RetreaverSweepInterval — `RETREAVER_SWEEP_INTERVAL` (default 60m).
func RetreaverSweepInterval() time.Duration {
	return envDuration("RETREAVER_SWEEP_INTERVAL", time.Hour)
}

// RetreaverDeepSweepDays — `RETREAVER_DEEP_SWEEP_DAYS` (default 30): dispute-window re-check.
func RetreaverDeepSweepDays() int { return envInt("RETREAVER_DEEP_SWEEP_DAYS", 30) }

// RetreaverDeepSweepInterval — `RETREAVER_DEEP_SWEEP_INTERVAL` (default 24h).
func RetreaverDeepSweepInterval() time.Duration {
	return envDuration("RETREAVER_DEEP_SWEEP_INTERVAL", 24*time.Hour)
}

// BuildRetreaverWindow computes the frozen window for a tick.
//
// `lastSyncAt` empty (first ever sync) falls back to the per-integration lookback. The end is
// always `now`, captured once by the caller, so the window cannot drift while it is being paged.
func BuildRetreaverWindow(now time.Time, lastSyncAt string, lookbackDays int) (time.Time, time.Time) {
	end := now.UTC()
	overlap := RetreaverWindowOverlap()

	if lastSyncAt != "" {
		if t, err := time.Parse(time.RFC3339, lastSyncAt); err == nil {
			start := t.UTC().Add(-overlap)
			if start.Before(end) {
				return start, end
			}
			// Clock skew or a future last_sync: fall through to the lookback rather than
			// producing an inverted window.
		}
	}
	return end.Add(-retreaverLookbackDuration(lookbackDays)), end
}

// BuildRetreaverSweepWindow returns the [start, end) for a sweep of the last `days` days.
func BuildRetreaverSweepWindow(now time.Time, days int) (time.Time, time.Time) {
	end := now.UTC()
	if days < 1 {
		days = 1
	}
	return end.AddDate(0, 0, -days), end
}

// shrinkRetreaverWindow halves a window that holds more rows than the page cap can walk.
//
// C1: the previous design logged the overflow and "carried the remainder into the next window".
// That recreates K1 — with `order=desc` the rows left after the cap are the OLDEST in the window,
// and the next window starts at `end − overlap`, so they would never be fetched again. Shrinking
// the END instead is safe and self-healing: the part we drop is simply not yet covered, and the
// next window starts where this one ended.
//
// Returns the new end and whether a split happened.
func shrinkRetreaverWindow(start, end time.Time) (time.Time, bool) {
	span := end.Sub(start)
	if span <= retreaverMinWindow {
		return end, false
	}
	mid := start.Add(span / 2)
	if mid.Sub(start) < retreaverMinWindow {
		mid = start.Add(retreaverMinWindow)
	}
	if !mid.Before(end) {
		return end, false
	}
	return mid, true
}

// retreaverWindowFits reports whether `total` rows can be walked within the page cap.
func retreaverWindowFits(total, perPage, maxPages int) bool {
	if total <= 0 || perPage <= 0 {
		return true
	}
	return total <= perPage*maxPages
}

// FetchRange walks an explicit [start, end) window to completion.
//
// This is what makes sweeps and the day-windowed backfill work for Retreaver (Stage 4 R1 / D).
// It reuses the same frozen-window paging as the incremental path — the only difference is that
// the caller supplies the bounds instead of them being derived from the cursor.
func (a *RetreaverAdapter) FetchRange(
	ctx context.Context,
	creds map[string]string,
	start, end time.Time,
) (PageResult, error) {
	const perPage = 100
	all := []models.RemoteCall{}
	requests := 0
	maxPages := RetreaverMaxPagesPerWindow()

	for page := 1; page <= maxPages; page++ {
		calls, total, hasMore, err := a.fetchWindowPage(ctx, creds, start, end, page, perPage)
		if err != nil {
			return PageResult{}, err
		}
		requests++
		all = append(all, calls...)

		// C2: a short walk must be visible to the caller rather than silently accepted.
		if !hasMore {
			if total > 0 && len(all) < total {
				return PageResult{}, fmt.Errorf(
					"retreaver: window [%s, %s) returned %d of %d rows — refusing to advance",
					start.UTC().Format(time.RFC3339), end.UTC().Format(time.RFC3339), len(all), total)
			}
			break
		}
	}

	return PageResult{
		Calls:        all,
		HasMore:      false,
		RequestCount: requests,
		WindowStart:  start,
		WindowEnd:    end,
	}, nil
}
