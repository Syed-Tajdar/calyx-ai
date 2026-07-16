package adapter

import (
	"context"
	"testing"
)

type stubMultiPageAdapter struct {
	pages []PageResult
	calls int
}

func (s *stubMultiPageAdapter) Platform() string { return "retreaver" }
func (s *stubMultiPageAdapter) TestAuth(context.Context, map[string]string) error {
	return nil
}
func (s *stubMultiPageAdapter) FetchPage(_ context.Context, _ map[string]string, cursor string, _ string, _ int) (PageResult, error) {
	idx := s.calls
	s.calls++
	if idx < len(s.pages) {
		return s.pages[idx], nil
	}
	// refresh or extra
	if len(s.pages) > 0 {
		p := s.pages[0]
		p.HasMore = false
		return p, nil
	}
	return PageResult{}, nil
}

func TestFetchAllPagesPage1Refresh(t *testing.T) {
	c1, _ := EncodeCursor(retreaverCursor{UpdatedAtStart: "2026-07-01T00:00:00Z", Page: 1})
	c2, _ := EncodeCursor(retreaverCursor{UpdatedAtStart: "2026-07-01T00:00:00Z", Page: 2})
	cDone, _ := EncodeCursor(retreaverCursor{UpdatedAtStart: "2026-07-01T00:00:00Z", Page: 1})
	stub := &stubMultiPageAdapter{
		pages: []PageResult{
			{Calls: nil, NextCursor: c2, HasMore: true},
			{Calls: nil, NextCursor: cDone, HasMore: false},
		},
	}
	reg := &Registry{
		adapters: map[string]PlatformAdapter{"retreaver": stub},
		mock:     &mockAdapter{},
	}
	// Force real mode path
	res, err := reg.FetchAllPages(context.Background(), "retreaver", "real", map[string]string{}, c1, "", 30)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Page1Refresh {
		t.Fatalf("expected Page1Refresh=true, calls=%d", stub.calls)
	}
	if stub.calls < 3 {
		t.Fatalf("expected >=3 fetches (2 pages + refresh), got %d", stub.calls)
	}
}
