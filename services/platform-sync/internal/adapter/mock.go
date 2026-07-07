package adapter

import (
	"context"
	"crypto/rand"
	"fmt"
	"time"

	"github.com/Syed-Tajdar/calyx-ai/services/platform-sync/internal/models"
)

type mockAdapter struct{}

func (m *mockAdapter) Platform() string { return "mock" }

func (m *mockAdapter) TestAuth(_ context.Context, _ map[string]string) error {
	return nil
}

func (m *mockAdapter) FetchPage(_ context.Context, creds map[string]string, _ string, _ string) (PageResult, error) {
	platform := creds["platform"]
	if platform == "" {
		platform = "unknown"
	}
	states := []string{"WA", "TX", "CA"}
	now := time.Now().UTC()
	randPart := make([]byte, 8)
	_, _ = rand.Read(randPart)
	callID := fmt.Sprintf("apisync-%s-%s-%x", platform, now.Format("20060102"), randPart)
	call := models.RemoteCall{
		ExternalCallID: callID,
		CallerID:       "15555550100",
		State:          states[now.Unix()%3],
		Revenue:        11.25,
		Payout:         0,
		EventType:      "incoming",
	}
	if platform == "retreaver" {
		call.CallKey = "sync-mock-key"
	}
	cursor := fmt.Sprintf("%d", now.Unix())
	return PageResult{
		Calls:      []models.RemoteCall{call},
		NextCursor: cursor,
		HasMore:    false,
	}, nil
}
