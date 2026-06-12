package redis

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
)

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
