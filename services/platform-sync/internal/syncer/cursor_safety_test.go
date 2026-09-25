package syncer

import (
	"context"
	"errors"
	"testing"

	"github.com/Syed-Tajdar/calyx-ai/services/platform-sync/internal/models"
)

// Documents cursor safety: publishSyncCalls stops on first publish error after
// some success; syncOne markError keeps the OLD last_sync_at (status-only).
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
	// F029: "a" committed; "b" evaluated but publish failed → no hash for "b".
	if _, ok := d.hashes["t:retreaver:a"]; !ok {
		t.Fatal("a must be committed after successful publish")
	}
	if _, ok := d.hashes["t:retreaver:b"]; ok {
		t.Fatal("b must not have hash after failed publish")
	}
}
