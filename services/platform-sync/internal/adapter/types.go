package adapter

import (
	"context"

	"github.com/Syed-Tajdar/calyx-ai/services/platform-sync/internal/models"
)

// PlatformAdapter pulls calls from an external platform REST API.
type PlatformAdapter interface {
	Platform() string
	TestAuth(ctx context.Context, creds map[string]string) error
	// lookbackDays is the empty-cursor initial window (1..365). Adapters that ignore it use their own default.
	FetchPage(ctx context.Context, creds map[string]string, cursor string, lastSyncAt string, lookbackDays int) (PageResult, error)
}

// PageResult is one page of remote calls plus an opaque cursor for the next page.
type PageResult struct {
	Calls                 []models.RemoteCall
	NextCursor            string
	HasMore               bool
	Page1Refresh          bool // true when a deep-walk page=1 refresh pass was appended
	RequestCount          int  // HTTP requests made for this result
	OffsetFallbackWindows int  // 1-minute windows that still filled a page and used offset
	OffsetFallbackRows    int  // rows returned by those offset fallbacks
}
