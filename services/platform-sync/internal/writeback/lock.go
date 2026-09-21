package writeback

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	inflightTTL = 45 * time.Second
	pendingTTL  = 15 * time.Minute
)

type Locker interface {
	Acquire(ctx context.Context, tenantID, action, callID string) (bool, error)
	Release(ctx context.Context, tenantID, action, callID string) error
	MarkPending(ctx context.Context, tenantID, action, callID string) error
}

type RedisLocker struct {
	rdb *redis.Client
}

func NewRedisLocker(rdb *redis.Client) *RedisLocker {
	return &RedisLocker{rdb: rdb}
}

func lockKey(tenantID, action, callID string) string {
	return fmt.Sprintf("writeback:lock:%s:%s:%s", tenantID, action, strings.TrimSpace(callID))
}

func (l *RedisLocker) Acquire(ctx context.Context, tenantID, action, callID string) (bool, error) {
	ok, err := l.rdb.SetNX(ctx, lockKey(tenantID, action, callID), "inflight", inflightTTL).Result()
	return ok, err
}

func (l *RedisLocker) Release(ctx context.Context, tenantID, action, callID string) error {
	return l.rdb.Del(ctx, lockKey(tenantID, action, callID)).Err()
}

func (l *RedisLocker) MarkPending(ctx context.Context, tenantID, action, callID string) error {
	return l.rdb.Set(ctx, lockKey(tenantID, action, callID), "pending", pendingTTL).Err()
}

// MemoryLocker is for tests.
type MemoryLocker struct {
	mu   sync.Mutex
	held map[string]string
}

func NewMemoryLocker() *MemoryLocker {
	return &MemoryLocker{held: map[string]string{}}
}

func (m *MemoryLocker) Acquire(_ context.Context, tenantID, action, callID string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	k := lockKey(tenantID, action, callID)
	if m.held[k] != "" {
		return false, nil
	}
	m.held[k] = "inflight"
	return true, nil
}

func (m *MemoryLocker) Release(_ context.Context, tenantID, action, callID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.held, lockKey(tenantID, action, callID))
	return nil
}

func (m *MemoryLocker) MarkPending(_ context.Context, tenantID, action, callID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.held[lockKey(tenantID, action, callID)] = "pending"
	return nil
}
