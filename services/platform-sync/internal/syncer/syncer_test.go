package syncer

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Syed-Tajdar/calyx-ai/services/platform-sync/internal/dedup"
	"github.com/Syed-Tajdar/calyx-ai/services/platform-sync/internal/models"
	"github.com/redis/go-redis/v9"
)

type recordingPublisher struct {
	calls []models.RemoteCall
}

func (r *recordingPublisher) Publish(_ context.Context, _, _ string, call models.RemoteCall, _ bool, _ int64) error {
	r.calls = append(r.calls, call)
	return nil
}

// stubContentDedup mirrors F029: Evaluate does not store; Commit stores.
type stubContentDedup struct {
	hashes map[string]string
}

func (d *stubContentDedup) Evaluate(_ context.Context, tenantID, platform string, content dedup.Content) (dedup.Decision, error) {
	key := tenantID + ":" + platform + ":" + content.ExternalCallID
	h := dedup.Hash(content)
	prev, ok := d.hashes[key]
	if !ok {
		return dedup.PublishNew, nil
	}
	if prev == h {
		return dedup.Unchanged, nil
	}
	return dedup.PublishChanged, nil
}

func (d *stubContentDedup) Commit(_ context.Context, tenantID, platform string, content dedup.Content) error {
	key := tenantID + ":" + platform + ":" + content.ExternalCallID
	d.hashes[key] = dedup.Hash(content)
	return nil
}

func TestPublishSyncCallsSkipsEmptyExternalID(t *testing.T) {
	pub := &recordingPublisher{}
	d := &stubContentDedup{hashes: map[string]string{}}
	calls := []models.RemoteCall{
		{ExternalCallID: ""},
		{ExternalCallID: "valid-uuid"},
		{ExternalCallID: "  "},
	}
	skipped, published, unchanged, err := publishSyncCalls(context.Background(), "tenant", "retreaver", calls, d, pub, 0)
	if err != nil {
		t.Fatal(err)
	}
	if skipped != 2 {
		t.Fatalf("skipped=%d want 2", skipped)
	}
	if published != 1 || unchanged != 0 {
		t.Fatalf("published=%d unchanged=%d", published, unchanged)
	}
	if len(pub.calls) != 1 {
		t.Fatalf("published %d calls, want 1", len(pub.calls))
	}
	if pub.calls[0].ExternalCallID != "valid-uuid" {
		t.Fatalf("published id %q", pub.calls[0].ExternalCallID)
	}
}

func TestPublishSyncCallsSuppressesUnchanged(t *testing.T) {
	pub := &recordingPublisher{}
	d := &stubContentDedup{hashes: map[string]string{}}
	call := models.RemoteCall{ExternalCallID: "id-1", Revenue: 10}
	_, _, _, err := publishSyncCalls(context.Background(), "tenant", "retreaver",
		[]models.RemoteCall{call}, d, pub, 0)
	if err != nil {
		t.Fatal(err)
	}
	_, pubCount, unchanged, err := publishSyncCalls(context.Background(), "tenant", "retreaver",
		[]models.RemoteCall{call}, d, pub, 0)
	if err != nil {
		t.Fatal(err)
	}
	if pubCount != 0 || unchanged != 1 {
		t.Fatalf("second pass published=%d unchanged=%d", pubCount, unchanged)
	}
	if len(pub.calls) != 1 {
		t.Fatalf("want 1 published total, got %d", len(pub.calls))
	}
}

func TestPublishSyncCallsPublishesRevenueChange(t *testing.T) {
	pub := &recordingPublisher{}
	d := &stubContentDedup{hashes: map[string]string{}}
	call := models.RemoteCall{ExternalCallID: "id-1", Revenue: 10}
	_, _, _, _ = publishSyncCalls(context.Background(), "tenant", "retreaver",
		[]models.RemoteCall{call}, d, pub, 0)
	call.Revenue = 25
	_, published, unchanged, err := publishSyncCalls(context.Background(), "tenant", "retreaver",
		[]models.RemoteCall{call}, d, pub, 0)
	if err != nil {
		t.Fatal(err)
	}
	if published != 1 || unchanged != 0 {
		t.Fatalf("published=%d unchanged=%d", published, unchanged)
	}
	if len(pub.calls) != 2 {
		t.Fatalf("want 2 rows, got %d", len(pub.calls))
	}
}

func TestPublishSyncCallsFailOpenStillPublishes(t *testing.T) {
	pub := &recordingPublisher{}
	client := redis.NewClient(&redis.Options{Addr: "127.0.0.1:1", DialTimeout: 50 * time.Millisecond})
	t.Cleanup(func() { _ = client.Close() })
	d := dedup.NewChecker(client, time.Hour)
	_, published, unchanged, err := publishSyncCalls(context.Background(), "tenant", "retreaver",
		[]models.RemoteCall{{ExternalCallID: "id-fail-open", Revenue: 3}}, d, pub, 0)
	if err != nil {
		t.Fatalf("publishSyncCalls must not fail on redis error: %v", err)
	}
	if published != 1 || unchanged != 0 {
		t.Fatalf("published=%d unchanged=%d want published=1", published, unchanged)
	}
	if len(pub.calls) != 1 {
		t.Fatalf("want 1 published call, got %d", len(pub.calls))
	}
}

func TestPublishSyncCallsFailsOnPublishError(t *testing.T) {
	pub := &failPublisher{err: errors.New("kafka down")}
	d := &stubContentDedup{hashes: map[string]string{}}
	_, _, _, err := publishSyncCalls(context.Background(), "tenant", "retreaver",
		[]models.RemoteCall{{ExternalCallID: "id-1"}}, d, pub, 0)
	if err == nil {
		t.Fatal("expected error")
	}
	if len(d.hashes) != 0 {
		t.Fatal("F029: hash must be absent when publish fails")
	}
}

func TestPublishSyncCallsF029RetryAfterPublishFail(t *testing.T) {
	fail := &failPublisher{err: errors.New("kafka down")}
	d := &stubContentDedup{hashes: map[string]string{}}
	call := models.RemoteCall{ExternalCallID: "retry-1", Revenue: 7}
	_, _, _, err := publishSyncCalls(context.Background(), "tenant", "ringba",
		[]models.RemoteCall{call}, d, fail, 0)
	if err == nil {
		t.Fatal("expected publish error")
	}
	if len(d.hashes) != 0 {
		t.Fatal("hash absent after failed publish")
	}
	ok := &recordingPublisher{}
	_, published, unchanged, err := publishSyncCalls(context.Background(), "tenant", "ringba",
		[]models.RemoteCall{call}, d, ok, 0)
	if err != nil || published != 1 || unchanged != 0 {
		t.Fatalf("retry published=%d unchanged=%d err=%v", published, unchanged, err)
	}
	_, published2, unchanged2, err := publishSyncCalls(context.Background(), "tenant", "ringba",
		[]models.RemoteCall{call}, d, ok, 0)
	if err != nil || published2 != 0 || unchanged2 != 1 {
		t.Fatalf("after success published=%d unchanged=%d", published2, unchanged2)
	}
}

func TestPublishSyncCallsWasPayoutAdjustedRepublishes(t *testing.T) {
	pub := &recordingPublisher{}
	d := &stubContentDedup{hashes: map[string]string{}}
	call := models.RemoteCall{ExternalCallID: "pay-1", Payout: 10}
	_, _, _, _ = publishSyncCalls(context.Background(), "t", "ringba", []models.RemoteCall{call}, d, pub, 0)
	call.WasPayoutAdjusted = true
	_, published, _, err := publishSyncCalls(context.Background(), "t", "ringba", []models.RemoteCall{call}, d, pub, 0)
	if err != nil || published != 1 {
		t.Fatalf("payout adjust must republish: published=%d err=%v", published, err)
	}
}

type failAfterPublisher struct {
	ok  int
	n   int
	err error
}

func (f *failAfterPublisher) Publish(context.Context, string, string, models.RemoteCall, bool, int64) error {
	if f.n < f.ok {
		f.n++
		return nil
	}
	return f.err
}

type failPublisher struct {
	err error
}

func (f *failPublisher) Publish(context.Context, string, string, models.RemoteCall, bool, int64) error {
	return f.err
}
