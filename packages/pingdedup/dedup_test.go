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
			}
		}
	}
	if published != n {
		t.Fatalf("published=%d want %d (unchanged overlap suppressed)", published, n)
	}
}

func TestEvaluateFailOpenOnRedisUnreachable(t *testing.T) {
	// Nothing listening — Get/Set must fail; Evaluate must still publish (nil err + ShouldPublish).
	client := redis.NewClient(&redis.Options{Addr: "127.0.0.1:1", DialTimeout: 50 * time.Millisecond})
	t.Cleanup(func() { _ = client.Close() })
	c := NewChecker(client, time.Hour)
	d, err := c.Evaluate(context.Background(), "t1", "ringba", Content{
		ExternalCallID: "fail-open-1",
		Revenue:        1,
		EventType:      "incoming",
	})
	if err != nil {
		t.Fatalf("fail-OPEN must return nil error, got %v", err)
	}
	if !d.ShouldPublish() {
		t.Fatalf("fail-OPEN must publish, got %v", d)
	}
}

func TestEvaluateFailOpenOnSetAfterClose(t *testing.T) {
	c, mr := testChecker(t)
	ctx := context.Background()
	content := Content{ExternalCallID: "x", Revenue: 1, EventType: "incoming"}
	if _, err := c.Evaluate(ctx, "t1", "ringba", content); err != nil {
		t.Fatal(err)
	}
	mr.Close() // subsequent Get/Set fail
	changed := content
	changed.Revenue = 9
	d, err := c.Evaluate(ctx, "t1", "ringba", changed)
	if err != nil {
		t.Fatalf("fail-OPEN must return nil error after redis close, got %v", err)
	}
	if !d.ShouldPublish() {
		t.Fatalf("fail-OPEN must publish after redis close, got %v", d)
	}
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b [12]byte
	pos := len(b)
	for i > 0 {
		pos--
		b[pos] = byte('0' + i%10)
		i /= 10
	}
	return string(b[pos:])
}
