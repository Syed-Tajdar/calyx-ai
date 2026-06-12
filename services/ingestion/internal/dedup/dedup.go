package dedup

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// Checker performs Redis-backed ping deduplication.
type Checker struct {
	client *redis.Client
	ttl    time.Duration
}

// NewChecker creates a dedup checker with the given Redis client and TTL.
func NewChecker(client *redis.Client, ttl time.Duration) *Checker {
	return &Checker{client: client, ttl: ttl}
}

// IsDuplicate returns true if this ping was already seen within the TTL window.
func (c *Checker) IsDuplicate(ctx context.Context, tenantID, platform, externalCallID string) (bool, error) {
	key := fmt.Sprintf("%s:dedup:%s:%s", tenantID, platform, externalCallID)
	set, err := c.client.SetNX(ctx, key, "1", c.ttl).Result()
	if err != nil {
		return false, fmt.Errorf("dedup.IsDuplicate: %w", err)
	}
	return !set, nil
}

// Ping checks Redis connectivity.
func (c *Checker) Ping(ctx context.Context) error {
	if err := c.client.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("dedup.Ping: %w", err)
	}
	return nil
}
