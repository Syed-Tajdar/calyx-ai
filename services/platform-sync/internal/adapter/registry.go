package adapter

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
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
		}
		final.Calls = append(final.Calls, res.Calls...)
		final.NextCursor = res.NextCursor
		final.HasMore = res.HasMore
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
		final.Page1Refresh = true
		// Keep NextCursor from the deep walk, not the refresh.
	}
	return final, nil
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
