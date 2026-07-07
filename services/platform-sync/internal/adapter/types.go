package adapter

import (
	"context"

	"github.com/Syed-Tajdar/calyx-ai/services/platform-sync/internal/models"
)

// PlatformAdapter pulls calls from an external platform REST API.
type PlatformAdapter interface {
	Platform() string
	TestAuth(ctx context.Context, creds map[string]string) error
	FetchPage(ctx context.Context, creds map[string]string, cursor string, lastSyncAt string) (PageResult, error)
}

// PageResult is one page of remote calls plus an opaque cursor for the next page.
type PageResult struct {
	Calls      []models.RemoteCall
	NextCursor string
	HasMore    bool
}
