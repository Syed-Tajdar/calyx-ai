package adapter

import (
	"context"
	"fmt"
)

const maxPagesPerSync = 10

// Registry resolves platform adapters and coordinates paginated fetch.
type Registry struct {
	mockMode bool
	http     *HTTPClient
	adapters map[string]PlatformAdapter
	mock     *mockAdapter
}

func NewRegistry(mockMode bool) *Registry {
	http := NewHTTPClient()
	return &Registry{
		mockMode: mockMode,
		http:     http,
		mock:     &mockAdapter{},
		adapters: map[string]PlatformAdapter{
			"retreaver":  NewRetreaverAdapter(http),
			"ringba":     NewRingbaAdapter(http),
			"trackdrive": NewTrackDriveAdapter(http),
			"callgrid":   NewCallGridAdapter(http),
		},
	}
}

func (r *Registry) Get(platform string) (PlatformAdapter, error) {
	if r.mockMode {
		return r.mock, nil
	}
	ad, ok := r.adapters[platform]
	if !ok {
		return nil, fmt.Errorf("no REST adapter for platform %q", platform)
	}
	return ad, nil
}

func (r *Registry) TestAuth(ctx context.Context, platform string, creds map[string]string) error {
	ad, err := r.Get(platform)
	if err != nil {
		return err
	}
	return ad.TestAuth(ctx, creds)
}

func (r *Registry) FetchAllPages(
	ctx context.Context,
	platform string,
	creds map[string]string,
	syncCursor string,
	lastSyncAt string,
) (PageResult, error) {
	ad, err := r.Get(platform)
	if err != nil {
		return PageResult{}, err
	}
	final := PageResult{}
	cur := syncCursor
	for page := 0; page < maxPagesPerSync; page++ {
		res, err := ad.FetchPage(ctx, creds, cur, lastSyncAt)
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
