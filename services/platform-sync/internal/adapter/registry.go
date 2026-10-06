package adapter

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"
)

const maxPagesPerSync = 10

// Registry resolves platform adapters and coordinates paginated fetch.
type Registry struct {
	http     *HTTPClient
	adapters map[string]PlatformAdapter
	mock     *mockAdapter
}

func NewRegistry() *Registry {
	http := NewHTTPClient()
	return &Registry{
		http: http,
		mock: &mockAdapter{},
		adapters: map[string]PlatformAdapter{
			"retreaver":  NewRetreaverAdapter(http),
			"ringba":     NewRingbaAdapter(http),
			"trackdrive": NewTrackDriveAdapter(http),
			"callgrid":   NewCallGridAdapter(http),
		},
	}
}

func (r *Registry) Get(platform string, syncMode string) (PlatformAdapter, error) {
	if syncMode != "real" {
		return r.mock, nil
	}
	ad, ok := r.adapters[platform]
	if !ok {
		return nil, fmt.Errorf("no REST adapter for platform %q", platform)
	}
	return ad, nil
}

// Platforms returns sorted registered REST adapter names (excludes mock).
func (r *Registry) Platforms() []string {
	out := make([]string, 0, len(r.adapters))
	for name := range r.adapters {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

func (r *Registry) TestAuth(ctx context.Context, platform string, syncMode string, creds map[string]string) error {
	ad, err := r.Get(platform, syncMode)
	if err != nil {
		return err
	}
	return ad.TestAuth(ctx, creds)
}

func (r *Registry) FetchAllPages(
	ctx context.Context,
	platform string,
	syncMode string,
	creds map[string]string,
	syncCursor string,
	lastSyncAt string,
	lookbackDays int,
) (PageResult, error) {
	ad, err := r.Get(platform, syncMode)
	if err != nil {
		return PageResult{}, err
	}
	final := PageResult{}
	cur := syncCursor
	pagesFetched := 0
	var firstWindowCursor string
	for page := 0; page < maxPagesPerSync; page++ {
		res, err := ad.FetchPage(ctx, creds, cur, lastSyncAt, lookbackDays)
		if err != nil {
			return PageResult{}, err
		}
		pagesFetched++
		if page == 0 {
			// Replay page 1 of this window after a deep walk (content-hash suppresses unchanged).
			firstWindowCursor = forceFirstPageCursor(platform, cur, res.NextCursor)
			final.WindowStart = res.WindowStart
			final.WindowEnd = res.WindowEnd
			final.WindowCapped = res.WindowCapped
		}
		final.Calls = append(final.Calls, res.Calls...)
		final.NextCursor = res.NextCursor
		final.HasMore = res.HasMore
		final.RequestCount += res.RequestCount
		final.OffsetFallbackWindows += res.OffsetFallbackWindows
		final.OffsetFallbackRows += res.OffsetFallbackRows
		if !res.HasMore {
			break
		}
		cur = res.NextCursor
	}
	if pagesFetched > 1 && firstWindowCursor != "" {
		refresh, err := ad.FetchPage(ctx, creds, firstWindowCursor, lastSyncAt, lookbackDays)
		if err != nil {
			return PageResult{}, err
		}
		final.Calls = append(final.Calls, refresh.Calls...)
		final.RequestCount += refresh.RequestCount
		final.OffsetFallbackWindows += refresh.OffsetFallbackWindows
		final.OffsetFallbackRows += refresh.OffsetFallbackRows
		final.Page1Refresh = true
		// Keep NextCursor from the deep walk, not the refresh.
	}
	return final, nil
}

// FetchRange runs an explicit time window for Ringba (sweep / backfill). Other platforms: error.
func (r *Registry) FetchRange(
	ctx context.Context,
	platform string,
	syncMode string,
	creds map[string]string,
	start, end time.Time,
) (PageResult, error) {
	ad, err := r.Get(platform, syncMode)
	if err != nil {
		return PageResult{}, err
	}
	rf, ok := ad.(RangeFetcher)
	if !ok {
		return PageResult{}, fmt.Errorf("windowed fetch not supported by the %s adapter (%T)", platform, ad)
	}
	return rf.FetchRange(ctx, creds, start, end)
}

// RangeFetcher is implemented by adapters that can fetch an explicit [start, end) window.
//
// Stage 4 R1 / D: this replaces the hardcoded `"ringba"` checks that gated sweeps and the
// day-windowed backfill. Ringba could do it because it slices by callDt; Retreaver can now do it
// because R0.3 proved `updated_at_end` is honoured. TrackDrive and CallGrid do not implement it
// and are unaffected — they simply keep the plain incremental path.
type RangeFetcher interface {
	FetchRange(ctx context.Context, creds map[string]string, start, end time.Time) (PageResult, error)
}

// SupportsRange reports whether this platform can run sweeps and windowed backfill.
func (r *Registry) SupportsRange(platform string, syncMode string) bool {
	ad, err := r.Get(platform, syncMode)
	if err != nil {
		return false
	}
	_, ok := ad.(RangeFetcher)
	return ok
}

// forceFirstPageCursor resets pagination to the first page of the same time window.
func forceFirstPageCursor(platform, startCursor, afterFirstNext string) string {
	switch platform {
	case "retreaver":
		var cs retreaverCursor
		src := afterFirstNext
		if startCursor != "" {
			src = startCursor
		}
		if src == "" {
			return ""
		}
		if err := DecodeCursor(src, &cs); err != nil {
			return ""
		}
		cs.normalize()
		cs.Page = 1
		enc, err := EncodeCursor(cs)
		if err != nil {
			return ""
		}
		return enc
	case "ringba":
		var cs ringbaCursor
		src := afterFirstNext
		if startCursor != "" {
			src = startCursor
		}
		if src == "" {
			return ""
		}
		if err := DecodeCursor(src, &cs); err != nil {
			return ""
		}
		cs.Offset = 0
		if cs.Size == 0 {
			cs.Size = 50
		}
		enc, err := EncodeCursor(cs)
		if err != nil {
			return ""
		}
		return enc
	case "trackdrive", "callgrid":
		// Opaque cursors: decode JSON and zero page/offset-like fields when present.
		src := afterFirstNext
		if startCursor != "" {
			src = startCursor
		}
		if src == "" {
			return ""
		}
		raw := []byte(src)
		var m map[string]any
		if err := json.Unmarshal(raw, &m); err != nil {
			return ""
		}
		if _, ok := m["page"]; ok {
			m["page"] = float64(1)
		}
		if _, ok := m["offset"]; ok {
			m["offset"] = float64(0)
		}
		if _, ok := m["cursor"]; ok {
			m["cursor"] = ""
		}
		enc, err := EncodeCursor(m)
		if err != nil {
			return ""
		}
		return enc
	default:
		return ""
	}
}
