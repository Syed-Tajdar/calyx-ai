package adapter

import (
	"context"
	"crypto/rand"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/Syed-Tajdar/calyx-ai/services/platform-sync/internal/models"
)

type mockAdapter struct{}

func (m *mockAdapter) Platform() string { return "mock" }

func (m *mockAdapter) TestAuth(_ context.Context, _ map[string]string) error {
	return nil
}

func mockDedupProofEnabled() bool {
	v := os.Getenv("MOCK_DEDUP_PROOF")
	return v == "1" || v == "true" || v == "TRUE"
}

func (m *mockAdapter) FetchPage(_ context.Context, creds map[string]string, cursor string, _ string, _ int) (PageResult, error) {
	platform := creds["platform"]
	if platform == "" {
		platform = "unknown"
	}
	if mockDedupProofEnabled() {
		return m.fetchProofPage(platform, cursor)
	}
	return m.fetchRandomPage(platform)
}

func (m *mockAdapter) fetchRandomPage(platform string) (PageResult, error) {
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
		OccurredAt:     now,
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

func proofRevenue() float64 {
	if envRev := os.Getenv("MOCK_DEDUP_PROOF_REVENUE"); envRev != "" {
		if parsed, err := strconv.ParseFloat(envRev, 64); err == nil {
			return parsed
		}
	}
	return 10.0
}

// fetchProofPage returns stable IDs + multi-page walk for content-hash / page=1 proofs.
// Cursor is Retreaver-shaped JSON so Registry.forceFirstPageCursor can replay page 1.
func (m *mockAdapter) fetchProofPage(platform, cursor string) (PageResult, error) {
	cs := retreaverCursor{UpdatedAtStart: "2026-07-01T00:00:00Z", Page: 1}
	if cursor != "" {
		_ = DecodeCursor(cursor, &cs)
	}
	cs.normalize()
	rev := proofRevenue()
	mk := func(id string, revenue float64) models.RemoteCall {
		c := models.RemoteCall{
			ExternalCallID: fmt.Sprintf("proof-%s-%s", platform, id),
			CallerID:       "15555550100",
			State:          "WA",
			Revenue:        revenue,
			Payout:         1.0,
			EventType:      "incoming",
			OccurredAt:     time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC),
			CampaignID:     "camp-proof",
			PublisherID:    "pub-proof",
			TargetID:       "tgt-proof",
		}
		if platform == "retreaver" {
			c.CallKey = "sync-mock-key"
		}
		return c
	}
	if cs.Page <= 1 {
		next := cs
		next.Page = 2
		enc, err := EncodeCursor(next)
		if err != nil {
			return PageResult{}, err
		}
		return PageResult{
			Calls: []models.RemoteCall{
				mk("0", rev),
				mk("1", 10.0),
				mk("2", 10.0),
			},
			NextCursor: enc,
			HasMore:    true,
		}, nil
	}
	next := cs
	next.Page = 1
	enc, err := EncodeCursor(next)
	if err != nil {
		return PageResult{}, err
	}
	return PageResult{
		Calls: []models.RemoteCall{
			mk("3", 10.0),
			mk("4", 10.0),
		},
		NextCursor: enc,
		HasMore:    false,
	}, nil
}
