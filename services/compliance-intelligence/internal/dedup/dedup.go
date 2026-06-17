package dedup

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// Checker provides idempotent event processing via Redis.
type Checker struct {
	client *redis.Client
	ttl    time.Duration
}

// NewChecker creates a dedup checker.
func NewChecker(client *redis.Client, ttl time.Duration) *Checker {
	return &Checker{client: client, ttl: ttl}
}

func key(tenantID, callID string) string {
	return fmt.Sprintf("%s:compliance:processed:%s", tenantID, callID)
}

// AlreadyProcessed returns true if call was already scored.
func (c *Checker) AlreadyProcessed(ctx context.Context, tenantID, callID string) (bool, error) {
	n, err := c.client.Exists(ctx, key(tenantID, callID)).Result()
	if err != nil {
		return false, fmt.Errorf("dedup.AlreadyProcessed: %w", err)
	}
	return n > 0, nil
}

// MarkProcessed records successful processing.
func (c *Checker) MarkProcessed(ctx context.Context, tenantID, callID string) error {
	if err := c.client.Set(ctx, key(tenantID, callID), "1", c.ttl).Err(); err != nil {
		return fmt.Errorf("dedup.MarkProcessed: %w", err)
	}
	return nil
}
