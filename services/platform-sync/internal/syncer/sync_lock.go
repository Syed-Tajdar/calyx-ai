package syncer

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// ErrAlreadyRunning is returned when a manual sync finds the per-integration lock held.
var ErrAlreadyRunning = errors.New("sync already running")

// Lock TTL must exceed the longest expected windowed run (deep sweep / backfill) with margin.
const syncLockTTL = 45 * time.Minute

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
	// reason is stored for Peek (e.g. "tick", "sweep", "deep_sweep", "backfill").
	TryAcquire(ctx context.Context, integrationID, reason string) (token string, ok bool, err error)
	// Release deletes the lock only if token still owns it (compare-and-delete).
	Release(ctx context.Context, integrationID, token string) error
	// Peek returns the holder reason when the lock is held (empty if free / unknown).
	Peek(ctx context.Context, integrationID string) (reason string, held bool, err error)
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

func lockValue(reason, token string) string {
	r := strings.TrimSpace(reason)
	if r == "" {
		r = "unknown"
	}
	return r + ":" + token
}

func parseLockValue(v string) (reason, token string) {
	i := strings.IndexByte(v, ':')
	if i <= 0 {
		return "unknown", v
	}
	return v[:i], v[i+1:]
}

type RedisSyncLocker struct {
	rdb *redis.Client
	ttl time.Duration
}

func NewRedisSyncLocker(rdb *redis.Client) *RedisSyncLocker {
	return &RedisSyncLocker{rdb: rdb, ttl: syncLockTTL}
}

func (l *RedisSyncLocker) TryAcquire(ctx context.Context, integrationID, reason string) (string, bool, error) {
	token, err := newLockToken()
	if err != nil {
		return "", false, err
	}
	ok, err := l.rdb.SetNX(ctx, syncLockKey(integrationID), lockValue(reason, token), l.ttl).Result()
	if err != nil {
		return "", false, err
	}
	if !ok {
		return "", false, nil
	}
	return token, true, nil
}

func (l *RedisSyncLocker) Release(ctx context.Context, integrationID, token string) error {
	// Value is reason:token — Eval compares full value; look up current and match token suffix.
	key := syncLockKey(integrationID)
	cur, err := l.rdb.Get(ctx, key).Result()
	if err == redis.Nil {
		return nil
	}
	if err != nil {
		return err
	}
	_, heldTok := parseLockValue(cur)
	if heldTok != token {
		return nil
	}
	_, err = l.rdb.Eval(ctx, releaseIfOwnerScript, []string{key}, cur).Result()
	return err
}

func (l *RedisSyncLocker) Peek(ctx context.Context, integrationID string) (string, bool, error) {
	v, err := l.rdb.Get(ctx, syncLockKey(integrationID)).Result()
	if err == redis.Nil {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	reason, _ := parseLockValue(v)
	return reason, true, nil
}

// MemorySyncLocker is for unit tests.
type MemorySyncLocker struct {
	mu   sync.Mutex
	held map[string]string // integrationID -> reason:token
}

func NewMemorySyncLocker() *MemorySyncLocker {
	return &MemorySyncLocker{held: map[string]string{}}
}

func (m *MemorySyncLocker) TryAcquire(_ context.Context, integrationID, reason string) (string, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.held[integrationID]; ok {
		return "", false, nil
	}
	token, err := newLockToken()
	if err != nil {
		return "", false, err
	}
	m.held[integrationID] = lockValue(reason, token)
	return token, true, nil
}

func (m *MemorySyncLocker) Release(_ context.Context, integrationID, token string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cur, ok := m.held[integrationID]
	if !ok {
		return nil
	}
	_, heldTok := parseLockValue(cur)
	if heldTok == token {
		delete(m.held, integrationID)
	}
	return nil
}

func (m *MemorySyncLocker) Peek(_ context.Context, integrationID string) (string, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	cur, ok := m.held[integrationID]
	if !ok {
		return "", false, nil
	}
	reason, _ := parseLockValue(cur)
	return reason, true, nil
}

// TokenForTest returns the held token (tests only).
func (m *MemorySyncLocker) TokenForTest(integrationID string) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, tok := parseLockValue(m.held[integrationID])
	return tok
}
