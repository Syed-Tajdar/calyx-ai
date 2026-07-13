package adapter

import (
	"context"
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
	for page := 0; page < maxPagesPerSync; page++ {
		res, err := ad.FetchPage(ctx, creds, cur, lastSyncAt, lookbackDays)
		if err != nil {
			return PageResult{}, err
		}
		final.Calls = append(final.Calls, res.Calls...)
		final.NextCursor = res.NextCursor
		final.HasMore = res.HasMore
		if !res.HasMore {
			break
		}
		cur = res.NextCursor
	}
	return final, nil
}
