package syncer

import (
	"context"
	"errors"
	"testing"

	"github.com/Syed-Tajdar/calyx-ai/services/platform-sync/internal/dedup"
	"github.com/Syed-Tajdar/calyx-ai/services/platform-sync/internal/models"
)

// Documents cursor safety: publishSyncCalls stops on first publish error after
// some success; syncOne markError keeps the OLD cursor (see syncer.go).
func TestPublishSyncCallsStopsMidBatchOnError(t *testing.T) {
	pub := &failAfterPublisher{ok: 1, err: errors.New("kafka down")}
	d := &stubContentDedup{hashes: map[string]string{}}
	calls := []models.RemoteCall{
		{ExternalCallID: "a", Revenue: 1},
		{ExternalCallID: "b", Revenue: 2},
		{ExternalCallID: "c", Revenue: 3},
	}
	_, published, _, err := publishSyncCalls(context.Background(), "t", "retreaver", calls, d, pub, 0)
	if err == nil {
		t.Fatal("expected error")
	}
	if published != 1 {
		t.Fatalf("published=%d want 1 before failure", published)
	}
	// Dedup already stored hash for "a"; "b" may have been Evaluate'd then Publish failed —
	// Evaluate runs before Publish, so "b" hash is stored even if publish failed.
	// That is intentional for content-hash; cursor must NOT advance (caller responsibility).
	_ = dedup.Hash(dedup.Content{})
}
