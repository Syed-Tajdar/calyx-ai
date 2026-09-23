package syncer

import (
	"context"
	"crypto/rand"
	"encoding/hex"
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

// releaseIfOwnerScript deletes the key only when the value matches the caller's token.
// Prevents a late Release after TTL expiry from deleting another run's lock.
const releaseIfOwnerScript = `
if redis.call("get", KEYS[1]) == ARGV[1] then
  return redis.call("del", KEYS[1])
end
return 0
`

type SyncLocker interface {
	// TryAcquire returns a unique ownership token when the lock is acquired.
	TryAcquire(ctx context.Context, integrationID string) (token string, ok bool, err error)
	// Release deletes the lock only if token still owns it (compare-and-delete).
	Release(ctx context.Context, integrationID, token string) error
}

func syncLockKey(integrationID string) string {
	return fmt.Sprintf("platform-sync:lock:%s", integrationID)
}

func newLockToken() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}

type RedisSyncLocker struct {
	rdb *redis.Client
	ttl time.Duration
}

func NewRedisSyncLocker(rdb *redis.Client) *RedisSyncLocker {
	return &RedisSyncLocker{rdb: rdb, ttl: syncLockTTL}
}

func (l *RedisSyncLocker) TryAcquire(ctx context.Context, integrationID string) (string, bool, error) {
	token, err := newLockToken()
	if err != nil {
		return "", false, err
	}
	ok, err := l.rdb.SetNX(ctx, syncLockKey(integrationID), token, l.ttl).Result()
	if err != nil {
		return "", false, err
	}
	if !ok {
		return "", false, nil
	}
	return token, true, nil
}

func (l *RedisSyncLocker) Release(ctx context.Context, integrationID, token string) error {
	_, err := l.rdb.Eval(ctx, releaseIfOwnerScript, []string{syncLockKey(integrationID)}, token).Result()
	return err
}

// MemorySyncLocker is for unit tests.
type MemorySyncLocker struct {
	mu   sync.Mutex
	held map[string]string // integrationID -> token
}

func NewMemorySyncLocker() *MemorySyncLocker {
	return &MemorySyncLocker{held: map[string]string{}}
}

func (m *MemorySyncLocker) TryAcquire(_ context.Context, integrationID string) (string, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.held[integrationID]; ok {
		return "", false, nil
	}
	token, err := newLockToken()
	if err != nil {
		return "", false, err
	}
	m.held[integrationID] = token
	return token, true, nil
}

func (m *MemorySyncLocker) Release(_ context.Context, integrationID, token string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.held[integrationID] == token {
		delete(m.held, integrationID)
	}
	return nil
}

// TokenForTest returns the held token (tests only).
func (m *MemorySyncLocker) TokenForTest(integrationID string) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.held[integrationID]
}
