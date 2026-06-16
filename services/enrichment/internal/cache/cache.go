package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// Store provides Redis-backed enrichment dedup/cache.
type Store struct {
	client *redis.Client
	ttl    time.Duration
}

// NewStore creates enrichment cache store.
func NewStore(client *redis.Client, ttl time.Duration) *Store {
	return &Store{client: client, ttl: ttl}
}

func cacheKey(tenantID, aniHash string) string {
	return fmt.Sprintf("%s:enrichment:ani_hash:%s", tenantID, aniHash)
}

// IsCached returns true if ani_hash was recently enriched.
func (s *Store) IsCached(ctx context.Context, tenantID, aniHash string) (bool, error) {
	exists, err := s.client.Exists(ctx, cacheKey(tenantID, aniHash)).Result()
	if err != nil {
		return false, fmt.Errorf("cache.IsCached: %w", err)
	}
	return exists > 0, nil
}

// MarkCached records enrichment completion for ani_hash.
func (s *Store) MarkCached(ctx context.Context, tenantID, aniHash string) error {
	if err := s.client.Set(ctx, cacheKey(tenantID, aniHash), "1", s.ttl).Err(); err != nil {
		return fmt.Errorf("cache.MarkCached: %w", err)
	}
	return nil
}
