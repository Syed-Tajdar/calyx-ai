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

// With heartbeat renewal, TTL only needs to cover a few missed renewals — not the full run.
// 90s: crash → next tick can run within ~TTL (not 45m). 30s renew: ≥2 renewals before expiry.
const (
	syncLockTTL           = 90 * time.Second
	syncLockRenewInterval = 30 * time.Second
)

// releaseIfOwnerScript deletes the key only when the value matches the caller's token.
// Prevents a late Release after TTL expiry from deleting another run's lock.
const releaseIfOwnerScript = `
if redis.call("get", KEYS[1]) == ARGV[1] then
  return redis.call("del", KEYS[1])
end
return 0
`

// renewIfOwnerScript extends TTL only when the caller still owns the lock value.
const renewIfOwnerScript = `
if redis.call("get", KEYS[1]) == ARGV[1] then
  return redis.call("pexpire", KEYS[1], ARGV[2])
end
return 0
`

type SyncLocker interface {
	// TryAcquire returns a unique ownership token when the lock is acquired.
	// reason is stored for Peek (e.g. "tick", "sweep", "deep_sweep", "backfill").
	TryAcquire(ctx context.Context, integrationID, reason string) (token string, ok bool, err error)
	// Renew extends TTL only if token still owns the lock. ok=false means ownership lost.
	Renew(ctx context.Context, integrationID, token string) (ok bool, err error)
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

func (l *RedisSyncLocker) Renew(ctx context.Context, integrationID, token string) (bool, error) {
	key := syncLockKey(integrationID)
	cur, err := l.rdb.Get(ctx, key).Result()
	if err == redis.Nil {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	_, heldTok := parseLockValue(cur)
	if heldTok != token {
		return false, nil
	}
	ms := int64(l.ttl / time.Millisecond)
	n, err := l.rdb.Eval(ctx, renewIfOwnerScript, []string{key}, cur, ms).Int64()
	if err != nil {
		return false, err
	}
	return n == 1, nil
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

// PTTL returns remaining lock TTL (for renew logging / ops).
// go-redis PTTL already returns a time.Duration (not raw milliseconds).
func (l *RedisSyncLocker) PTTL(ctx context.Context, integrationID string) (time.Duration, error) {
	d, err := l.rdb.PTTL(ctx, syncLockKey(integrationID)).Result()
	if err != nil {
		return 0, err
	}
	if d < 0 {
		return 0, nil
	}
	return d, nil
}

// TTLForTest exposes the configured TTL (tests / ops).
func (l *RedisSyncLocker) TTLForTest() time.Duration { return l.ttl }

// MemorySyncLocker is for unit tests (optional TTL for renew proofs).
type MemorySyncLocker struct {
	mu     sync.Mutex
	held   map[string]string // integrationID -> reason:token
	expiry map[string]time.Time
	ttl    time.Duration
	now    func() time.Time
}

func NewMemorySyncLocker() *MemorySyncLocker {
	return &MemorySyncLocker{
		held:   map[string]string{},
		expiry: map[string]time.Time{},
		ttl:    syncLockTTL,
		now:    time.Now,
	}
}

func (m *MemorySyncLocker) SetTTLForTest(d time.Duration) { m.ttl = d }
func (m *MemorySyncLocker) SetNowForTest(fn func() time.Time) {
	if fn != nil {
		m.now = fn
	}
}

func (m *MemorySyncLocker) purgeExpiredLocked(id string) {
	exp, ok := m.expiry[id]
	if ok && !exp.IsZero() && m.now().After(exp) {
		delete(m.held, id)
		delete(m.expiry, id)
	}
}

func (m *MemorySyncLocker) TryAcquire(_ context.Context, integrationID, reason string) (string, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.purgeExpiredLocked(integrationID)
	if _, ok := m.held[integrationID]; ok {
		return "", false, nil
	}
	token, err := newLockToken()
	if err != nil {
		return "", false, err
	}
	m.held[integrationID] = lockValue(reason, token)
	if m.ttl > 0 {
		m.expiry[integrationID] = m.now().Add(m.ttl)
	}
	return token, true, nil
}

func (m *MemorySyncLocker) Renew(_ context.Context, integrationID, token string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.purgeExpiredLocked(integrationID)
	cur, ok := m.held[integrationID]
	if !ok {
		return false, nil
	}
	_, heldTok := parseLockValue(cur)
	if heldTok != token {
		return false, nil
	}
	if m.ttl > 0 {
		m.expiry[integrationID] = m.now().Add(m.ttl)
	}
	return true, nil
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
		delete(m.expiry, integrationID)
	}
	return nil
}

func (m *MemorySyncLocker) Peek(_ context.Context, integrationID string) (string, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.purgeExpiredLocked(integrationID)
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
	m.purgeExpiredLocked(integrationID)
	_, tok := parseLockValue(m.held[integrationID])
	return tok
}

// ExpiryForTest returns the current expiry (tests only).
func (m *MemorySyncLocker) ExpiryForTest(integrationID string) time.Time {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.expiry[integrationID]
}

// SyncLockTTL and SyncLockRenewInterval are exported for ops/docs.
func SyncLockTTL() time.Duration           { return syncLockTTL }
func SyncLockRenewInterval() time.Duration { return syncLockRenewInterval }
