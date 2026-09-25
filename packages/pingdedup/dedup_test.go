package pingdedup

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func testChecker(t *testing.T) (*Checker, *miniredis.Miniredis) {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(mr.Close)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	return NewChecker(client, time.Hour), mr
}

func TestEvaluateIdenticalTwiceUnchanged(t *testing.T) {
	c, _ := testChecker(t)
	ctx := context.Background()
	content := Content{ExternalCallID: "call-1", Revenue: 10, State: "TX", EventType: "incoming"}

	d1, err := c.Evaluate(ctx, "t1", "retreaver", content)
	if err != nil {
		t.Fatal(err)
	}
	if d1 != PublishNew || !d1.ShouldPublish() {
		t.Fatalf("first=%v want PublishNew", d1)
	}
	if err := c.Commit(ctx, "t1", "retreaver", content); err != nil {
		t.Fatal(err)
	}

	d2, err := c.Evaluate(ctx, "t1", "retreaver", content)
	if err != nil {
		t.Fatal(err)
	}
	if d2 != Unchanged || d2.ShouldPublish() {
		t.Fatalf("second=%v want Unchanged", d2)
	}
}

func TestEvaluateRevenueChangePublishes(t *testing.T) {
	c, _ := testChecker(t)
	ctx := context.Background()
	base := Content{ExternalCallID: "call-2", Revenue: 10, EventType: "incoming"}
	if _, err := c.Evaluate(ctx, "t1", "ringba", base); err != nil {
		t.Fatal(err)
	}
	if err := c.Commit(ctx, "t1", "ringba", base); err != nil {
		t.Fatal(err)
	}
	changed := base
	changed.Revenue = 25
	d, err := c.Evaluate(ctx, "t1", "ringba", changed)
	if err != nil {
		t.Fatal(err)
	}
	if d != PublishChanged {
		t.Fatalf("got %v want PublishChanged", d)
	}
}

func TestEvaluateOverlapSim(t *testing.T) {
	c, _ := testChecker(t)
	ctx := context.Background()
	const n = 100
	const syncs = 5
	published := 0
	for s := 0; s < syncs; s++ {
		for i := 0; i < n; i++ {
			content := Content{
				ExternalCallID: "call-" + itoa(i),
				Revenue:        float64(i),
				EventType:      "incoming",
			}
			d, err := c.Evaluate(ctx, "tenant", "retreaver", content)
			if err != nil {
				t.Fatal(err)
			}
			if d.ShouldPublish() {
				published++
				if err := c.Commit(ctx, "tenant", "retreaver", content); err != nil {
					t.Fatal(err)
				}
			}
		}
	}
	if published != n {
		t.Fatalf("published=%d want %d (unchanged overlap suppressed)", published, n)
	}
}

func TestEvaluateFailOpenOnRedisUnreachable(t *testing.T) {
	client := redis.NewClient(&redis.Options{Addr: "127.0.0.1:1", DialTimeout: 50 * time.Millisecond})
	t.Cleanup(func() { _ = client.Close() })
	c := NewChecker(client, time.Hour)
	d, err := c.Evaluate(context.Background(), "t1", "ringba", Content{
		ExternalCallID: "fail-open-1",
		Revenue:        1,
		EventType:      "incoming",
	})
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if !d.ShouldPublish() {
		t.Fatalf("want publish on fail-OPEN, got %v", d)
	}
}

func TestF029PublishFailHashAbsentThenRetry(t *testing.T) {
	c, mr := testChecker(t)
	ctx := context.Background()
	content := Content{ExternalCallID: "f029", Revenue: 5, EventType: "incoming"}

	d1, err := c.Evaluate(ctx, "t1", "ringba", content)
	if err != nil || d1 != PublishNew {
		t.Fatalf("first evaluate: d=%v err=%v", d1, err)
	}
	// Simulate Kafka publish failure: do NOT Commit.
	key := Key("t1", "ringba", "f029")
	if mr.Exists(key) {
		t.Fatal("hash must be absent when publish fails")
	}

	d2, err := c.Evaluate(ctx, "t1", "ringba", content)
	if err != nil || d2 != PublishNew {
		t.Fatalf("retry evaluate: d=%v err=%v want PublishNew", d2, err)
	}
	if err := c.Commit(ctx, "t1", "ringba", content); err != nil {
		t.Fatal(err)
	}
	d3, err := c.Evaluate(ctx, "t1", "ringba", content)
	if err != nil || d3 != Unchanged {
		t.Fatalf("after commit: d=%v err=%v want Unchanged", d3, err)
	}
}

func TestUnchangedRefreshesTTL(t *testing.T) {
	c, mr := testChecker(t)
	ctx := context.Background()
	content := Content{ExternalCallID: "ttl-1", Revenue: 1, EventType: "incoming"}
	if err := c.Commit(ctx, "t1", "ringba", content); err != nil {
		t.Fatal(err)
	}
	key := Key("t1", "ringba", "ttl-1")
	mr.FastForward(30 * time.Minute)
	d, err := c.Evaluate(ctx, "t1", "ringba", content)
	if err != nil || d != Unchanged {
		t.Fatalf("d=%v err=%v", d, err)
	}
	// After refresh, FastForward almost a full TTL should still leave the key.
	mr.FastForward(c.ttl - time.Minute)
	if !mr.Exists(key) {
		t.Fatal("TTL must be refreshed on Unchanged so sweep edge does not expire early")
	}
}

func TestWasPayoutAdjustedChangesHash(t *testing.T) {
	base := Content{ExternalCallID: "p1", EventType: "incoming"}
	adj := base
	adj.WasPayoutAdjusted = true
	if Hash(base) == Hash(adj) {
		t.Fatal("WasPayoutAdjusted must change hash")
	}
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b [16]byte
	n := len(b)
	for i > 0 {
		n--
		b[n] = byte('0' + i%10)
		i /= 10
	}
	return string(b[n:])
}
