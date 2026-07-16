package handlers

import (
	"context"
	"testing"
	"time"

	"github.com/Syed-Tajdar/calyx-ai/services/ingestion/internal/dedup"
	"github.com/redis/go-redis/v9"
)

// Mirrors webhook.go Evaluate → publish decision: Redis errors must NOT abort publish.
func TestWebhookPathFailOpenStillPublishes(t *testing.T) {
	client := redis.NewClient(&redis.Options{Addr: "127.0.0.1:1", DialTimeout: 50 * time.Millisecond})
	t.Cleanup(func() { _ = client.Close() })
	checker := dedup.NewChecker(client, time.Hour)

	decision, err := checker.Evaluate(context.Background(), "tenant", "ringba", dedup.Content{
		ExternalCallID: "webhook-fail-open",
		Revenue:        7.25,
		EventType:      "incoming",
	})
	if err != nil {
		t.Fatalf("webhook path fail-OPEN: Evaluate must return nil err, got %v", err)
	}
	if decision == dedup.Unchanged || !decision.ShouldPublish() {
		t.Fatalf("webhook path fail-OPEN: must publish, got %v", decision)
	}
	// webhook.go: if err != nil → 500 without Publish; if Unchanged → skip Publish.
	// Here we assert the publish branch is taken (err==nil && ShouldPublish).
}
