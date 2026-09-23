package syncer

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// ErrAlreadyRunning is returned when a manual sync finds the per-integration lock held.
var ErrAlreadyRunning = errors.New("sync already running")

// Lock TTL must exceed the 10-minute sync timeout with margin so a mid-run expiry
// cannot reopen the concurrent-cursor race.
const syncLockTTL = 15 * time.Minute

type SyncLocker interface {
	// TryAcquire returns true if this caller holds the lock.
	TryAcquire(ctx context.Context, integrationID string) (bool, error)
	Release(ctx context.Context, integrationID string) error
}

func syncLockKey(integrationID string) string {
	return fmt.Sprintf("platform-sync:lock:%s", integrationID)
}

type RedisSyncLocker struct {
	rdb *redis.Client
	ttl time.Duration
}

func NewRedisSyncLocker(rdb *redis.Client) *RedisSyncLocker {
	return &RedisSyncLocker{rdb: rdb, ttl: syncLockTTL}
}

func (l *RedisSyncLocker) TryAcquire(ctx context.Context, integrationID string) (bool, error) {
	return l.rdb.SetNX(ctx, syncLockKey(integrationID), "1", l.ttl).Result()
}

func (l *RedisSyncLocker) Release(ctx context.Context, integrationID string) error {
	return l.rdb.Del(ctx, syncLockKey(integrationID)).Err()
}

// MemorySyncLocker is for unit tests.
type MemorySyncLocker struct {
	mu   sync.Mutex
	held map[string]struct{}
}

func NewMemorySyncLocker() *MemorySyncLocker {
	return &MemorySyncLocker{held: map[string]struct{}{}}
}

func (m *MemorySyncLocker) TryAcquire(_ context.Context, integrationID string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.held[integrationID]; ok {
		return false, nil
	}
	m.held[integrationID] = struct{}{}
	return true, nil
}

func (m *MemorySyncLocker) Release(_ context.Context, integrationID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.held, integrationID)
	return nil
}
