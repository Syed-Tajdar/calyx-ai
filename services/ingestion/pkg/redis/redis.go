package redis

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
)

// NewLazyClient parses a Redis URL and returns a client without connecting. Connections are
// dialled on first use and re-dialled after failures, so a redis that is down (or not yet
// resolvable) at startup is picked up by itself once it comes back.
func NewLazyClient(redisURL string) (*redis.Client, error) {
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("redis.NewLazyClient: parse url: %w", err)
	}
	return redis.NewClient(opts), nil
}

// NewClient parses a Redis URL and returns a connected client.
func NewClient(redisURL string) (*redis.Client, error) {
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("redis.NewClient: parse url: %w", err)
	}
	client := redis.NewClient(opts)
	if err := client.Ping(context.Background()).Err(); err != nil {
		return nil, fmt.Errorf("redis.NewClient: ping: %w", err)
	}
	return client, nil
}
