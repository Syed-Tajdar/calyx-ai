package loader

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const bindingCacheTTL = 300 * time.Second

// RedisBindingCache implements BindingCache.
type RedisBindingCache struct {
	client *redis.Client
}

// NewRedisBindingCache creates a Redis binding cache.
func NewRedisBindingCache(client *redis.Client) *RedisBindingCache {
	return &RedisBindingCache{client: client}
}

func bindingCacheKey(tenantID, domain string) string {
	return fmt.Sprintf("%s:provider_binding:%s", tenantID, domain)
}

// Get reads a cached binding.
func (c *RedisBindingCache) Get(tenantID, domain string) (*Binding, bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	raw, err := c.client.Get(ctx, bindingCacheKey(tenantID, domain)).Bytes()
	if err == redis.Nil {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("RedisBindingCache.Get: %w", err)
	}
	b, err := UnmarshalBindingJSON(raw)
	if err != nil {
		return nil, false, fmt.Errorf("RedisBindingCache.Get: unmarshal: %w", err)
	}
	return b, true, nil
}

// Set stores a binding with TTL.
func (c *RedisBindingCache) Set(tenantID, domain string, binding *Binding) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	raw, err := MarshalBindingJSON(binding)
	if err != nil {
		return err
	}
	return c.client.Set(ctx, bindingCacheKey(tenantID, domain), raw, bindingCacheTTL).Err()
}

// Delete invalidates a cached binding.
func (c *RedisBindingCache) Delete(tenantID, domain string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return c.client.Del(ctx, bindingCacheKey(tenantID, domain)).Err()
}

// BindingCacheKey exposes the key pattern for API invalidation consistency.
func BindingCacheKey(tenantID, domain string) string {
	return bindingCacheKey(tenantID, domain)
}
