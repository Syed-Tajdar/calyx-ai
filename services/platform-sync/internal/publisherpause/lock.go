package publisherpause

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// lockTTL bounds a crashed holder. It covers the slowest path: the Retreaver afid fallback
// walks up to 50 list pages at the shared 2/s limit.
const lockTTL = 2 * time.Minute

// Locker is the idempotency lock per tenant + platform + publisher. Pause and unpause share it.
type Locker interface {
	Acquire(ctx context.Context, tenantID, platform, publisherID string) (bool, error)
	Release(ctx context.Context, tenantID, platform, publisherID string) error
}

func lockKey(tenantID, platform, publisherID string) string {
	return fmt.Sprintf("publisherpause:lock:%s:%s:%s", tenantID, strings.ToLower(platform), strings.TrimSpace(publisherID))
}

// RedisLocker is the production Locker (SETNX with TTL), like writeback.RedisLocker.
type RedisLocker struct{ rdb *redis.Client }

func NewRedisLocker(rdb *redis.Client) *RedisLocker { return &RedisLocker{rdb: rdb} }

func (l *RedisLocker) Acquire(ctx context.Context, tenantID, platform, publisherID string) (bool, error) {
	return l.rdb.SetNX(ctx, lockKey(tenantID, platform, publisherID), "inflight", lockTTL).Result()
}

func (l *RedisLocker) Release(ctx context.Context, tenantID, platform, publisherID string) error {
	return l.rdb.Del(ctx, lockKey(tenantID, platform, publisherID)).Err()
}

// MemoryLocker is for tests.
type MemoryLocker struct {
	mu   sync.Mutex
	held map[string]bool
}

func NewMemoryLocker() *MemoryLocker { return &MemoryLocker{held: map[string]bool{}} }

func (m *MemoryLocker) Acquire(_ context.Context, tenantID, platform, publisherID string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	k := lockKey(tenantID, platform, publisherID)
	if m.held[k] {
		return false, nil
	}
	m.held[k] = true
	return true, nil
}

func (m *MemoryLocker) Release(_ context.Context, tenantID, platform, publisherID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.held, lockKey(tenantID, platform, publisherID))
	return nil
}

// Hold marks a key as held (tests: simulate a concurrent attempt).
func (m *MemoryLocker) Hold(tenantID, platform, publisherID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.held[lockKey(tenantID, platform, publisherID)] = true
}
