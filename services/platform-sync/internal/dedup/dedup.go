package dedup

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type Checker struct {
	client *redis.Client
	ttl    time.Duration
}

func NewChecker(client *redis.Client, ttl time.Duration) *Checker {
	return &Checker{client: client, ttl: ttl}
}

func (c *Checker) IsDuplicate(ctx context.Context, tenantID, platform, externalCallID string) (bool, error) {
	key := fmt.Sprintf("%s:dedup:%s:%s", tenantID, platform, externalCallID)
	set, err := c.client.SetNX(ctx, key, "1", c.ttl).Result()
	if err != nil {
		return false, fmt.Errorf("dedup.IsDuplicate: %w", err)
	}
	return !set, nil
}

func (c *Checker) Ping(ctx context.Context) error {
	return c.client.Ping(ctx).Err()
}
