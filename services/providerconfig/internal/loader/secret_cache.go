package loader

import (
	"time"

	lru "github.com/hashicorp/golang-lru/v2/expirable"
)

const secretCacheTTL = 60 * time.Second
const secretCacheMaxEntries = 1000

// LRUSecretCache is a bounded in-memory decrypt cache.
type LRUSecretCache struct {
	cache *lru.LRU[string, map[string]string]
}

// NewLRUSecretCache creates a bounded TTL secret cache.
func NewLRUSecretCache() *LRUSecretCache {
	return &LRUSecretCache{
		cache: lru.NewLRU[string, map[string]string](secretCacheMaxEntries, nil, secretCacheTTL),
	}
}

// Get returns cached secrets if present and not evicted.
func (c *LRUSecretCache) Get(key string) (map[string]string, bool) {
	return c.cache.Get(key)
}

// Set stores decrypted secrets in the LRU cache.
func (c *LRUSecretCache) Set(key string, secrets map[string]string) {
	c.cache.Add(key, secrets)
}

// SecretCacheKey builds a cache key for decrypted secrets.
func SecretCacheKey(tenantID, providerID, updatedAt string) string {
	return tenantID + ":" + providerID + ":" + updatedAt
}
