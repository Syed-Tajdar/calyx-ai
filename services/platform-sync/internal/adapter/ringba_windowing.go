package adapter

import (
	"context"
	"log"
	"time"

	"github.com/Syed-Tajdar/calyx-ai/services/platform-sync/internal/models"
)

const ringbaWindowStart = 24 * time.Hour
const ringbaMinWindow = time.Minute

type ringbaWindowStats struct {
	Requests              int
	OffsetFallbackWindows int
	OffsetFallbackRows    int
}

// fetchWindowed loads [start, end) as day slices, then halves any slice that
// fills a page. Offset paging is used only when a window of ringbaMinWindow
// or shorter still returns a full page.
func (a *RingbaAdapter) fetchWindowed(ctx context.Context, creds map[string]string, start, end time.Time) ([]models.RemoteCall, ringbaWindowStats, error) {
	var stats ringbaWindowStats
	var all []models.RemoteCall
	for _, w := range ringbaDayWindows(start, end) {
		calls, err := a.fetchFitting(ctx, creds, w[0], w[1], &stats)
		if err != nil {
			return nil, stats, err
		}
		all = append(all, calls...)
	}
	return all, stats, nil
}

func ringbaDayWindows(start, end time.Time) [][2]time.Time {
	start, end = start.UTC(), end.UTC()
	var out [][2]time.Time
	cur := start
	for cur.Before(end) {
		next := cur.Add(ringbaWindowStart)
		if next.After(end) {
			next = end
		}
		out = append(out, [2]time.Time{cur, next})
		cur = next
	}
	return out
}

func (a *RingbaAdapter) fetchFitting(ctx context.Context, creds map[string]string, start, end time.Time, stats *ringbaWindowStats) ([]models.RemoteCall, error) {
	if !start.Before(end) {
		return nil, nil
	}
	calls, full, err := a.fetchWindowPage(ctx, creds, start, end, 0)
	if err != nil {
		return nil, err
	}
	stats.Requests++
	if !full {
		return calls, nil
	}
	if end.Sub(start) <= ringbaMinWindow {
		rest, pages, err := a.fetchOffsetRemainder(ctx, creds, start, end)
		stats.Requests += pages
		if err != nil {
			return nil, err
		}
		combined := append(calls, rest...)
		stats.OffsetFallbackWindows++
		stats.OffsetFallbackRows += len(combined)
		log.Printf("RINGBA_OFFSET_FALLBACK window_start=%s window_end=%s pages=%d rows=%d",
			start.UTC().Format(time.RFC3339), end.UTC().Format(time.RFC3339), pages+1, len(combined))
		return combined, nil
	}
	mid := start.Add(end.Sub(start) / 2)
	left, err := a.fetchFitting(ctx, creds, start, mid, stats)
	if err != nil {
		return nil, err
	}
	right, err := a.fetchFitting(ctx, creds, mid, end, stats)
	if err != nil {
		return nil, err
	}
	return append(left, right...), nil
}

func (a *RingbaAdapter) fetchOffsetRemainder(ctx context.Context, creds map[string]string, start, end time.Time) ([]models.RemoteCall, int, error) {
	var all []models.RemoteCall
	pages := 0
	offset := ringbaMaxPageSize
	for {
		if err := ctx.Err(); err != nil {
			return nil, pages, err
		}
		calls, full, err := a.fetchWindowPage(ctx, creds, start, end, offset)
		if err != nil {
			return nil, pages, err
		}
		pages++
		all = append(all, calls...)
		if !full {
			return all, pages, nil
		}
		offset += ringbaMaxPageSize
	}
}

func (a *RingbaAdapter) fetchWindowPage(ctx context.Context, creds map[string]string, start, end time.Time, offset int) ([]models.RemoteCall, bool, error) {
	cs := ringbaCursor{
		ReportStart: start.UTC().Format(time.RFC3339),
		ReportEnd:   end.UTC().Format(time.RFC3339),
		Offset:      offset,
		Size:        ringbaMaxPageSize,
	}
	return a.fetchPage(ctx, creds, cs)
}
