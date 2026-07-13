package syncer

import (
	"context"
	"errors"
	"testing"

	"github.com/Syed-Tajdar/calyx-ai/services/platform-sync/internal/models"
)

type recordingPublisher struct {
	calls []models.RemoteCall
}

func (r *recordingPublisher) Publish(_ context.Context, _, _ string, call models.RemoteCall, _ bool, _ int64) error {
	r.calls = append(r.calls, call)
	return nil
}

type stubDedup struct {
	seen map[string]bool
}

func (d *stubDedup) IsDuplicate(_ context.Context, tenantID, platform, externalCallID string) (bool, error) {
	key := tenantID + ":" + platform + ":" + externalCallID
	if d.seen[key] {
		return true, nil
	}
	d.seen[key] = true
	return false, nil
}

func TestPublishSyncCallsSkipsEmptyExternalID(t *testing.T) {
	pub := &recordingPublisher{}
	dedup := &stubDedup{seen: map[string]bool{}}
	calls := []models.RemoteCall{
		{ExternalCallID: ""},
		{ExternalCallID: "valid-uuid"},
		{ExternalCallID: "  "},
	}
	skipped, err := publishSyncCalls(context.Background(), "tenant", "retreaver", calls, dedup, pub, 0)
	if err != nil {
		t.Fatal(err)
	}
	if skipped != 2 {
		t.Fatalf("skipped=%d want 2", skipped)
	}
	if len(pub.calls) != 1 {
		t.Fatalf("published %d calls, want 1", len(pub.calls))
	}
	if pub.calls[0].ExternalCallID != "valid-uuid" {
		t.Fatalf("published id %q", pub.calls[0].ExternalCallID)
	}
}

func TestPublishSyncCallsPropagatesPublishError(t *testing.T) {
	pub := &failPublisher{err: errors.New("kafka down")}
	dedup := &stubDedup{seen: map[string]bool{}}
	_, err := publishSyncCalls(context.Background(), "tenant", "retreaver",
		[]models.RemoteCall{{ExternalCallID: "id-1"}}, dedup, pub, 0)
	if err == nil {
		t.Fatal("expected error")
	}
}

type failPublisher struct {
	err error
}

func (f *failPublisher) Publish(context.Context, string, string, models.RemoteCall, bool, int64) error {
	return f.err
}
