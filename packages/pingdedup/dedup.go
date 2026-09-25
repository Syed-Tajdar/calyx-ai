package pingdedup

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"
)

// Decision is the outcome of a content-hash evaluate.
type Decision int

const (
	// PublishNew: key was absent — publish with is_duplicate=0 then Commit hash.
	PublishNew Decision = iota
	// PublishChanged: stored hash differed — publish then Commit hash.
	PublishChanged
	// Unchanged: stored hash matched — do not publish (TTL refreshed).
	Unchanged
)

// ShouldPublish reports whether Kafka/CH should receive a row for this decision.
func (d Decision) ShouldPublish() bool {
	return d != Unchanged
}

func (d Decision) String() string {
	switch d {
	case PublishNew:
		return "new"
	case PublishChanged:
		return "changed"
	case Unchanged:
		return "unchanged"
	default:
		return fmt.Sprintf("decision(%d)", int(d))
	}
}

// Checker performs Redis-backed content-hash ping deduplication.
// On ANY Redis GET error Evaluate fail-OPENs: returns PublishNew/PublishChanged with nil
// error so callers always publish (duplicates are OK; argMax protects aggregates).
// Hash is written only via Commit after a successful publish (F029).
type Checker struct {
	client *redis.Client
	ttl    time.Duration
	logger *slog.Logger
}

// NewChecker creates a content-hash dedup checker.
func NewChecker(client *redis.Client, ttl time.Duration) *Checker {
	return &Checker{client: client, ttl: ttl, logger: slog.Default()}
}

// Key builds the Redis key: {tenant}:dedup:{platform}:{external_call_id}.
func Key(tenantID, platform, externalCallID string) string {
	return fmt.Sprintf("%s:dedup:%s:%s", tenantID, platform, externalCallID)
}

// Evaluate compares content hash against Redis without storing.
// On match, refreshes TTL (so sweep re-sees do not expire hashes early).
// Call Commit after a successful Kafka publish for PublishNew/PublishChanged.
func (c *Checker) Evaluate(ctx context.Context, tenantID, platform string, content Content) (Decision, error) {
	if c == nil || c.client == nil {
		c.logError("pingdedup fail-OPEN: nil checker — publishing", "tenant_id", tenantID, "platform", platform)
		return PublishNew, nil
	}
	id := norm(content.ExternalCallID)
	if id == "" {
		return PublishNew, errors.New("pingdedup: empty external_call_id")
	}
	hash := Hash(content)
	key := Key(tenantID, platform, id)

	prev, err := c.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return PublishNew, nil
	}
	if err != nil {
		c.logError("pingdedup fail-OPEN: redis GET failed — publishing",
			"tenant_id", tenantID, "platform", platform, "error", err.Error())
		return PublishNew, nil
	}
	if prev == hash {
		_ = c.client.Expire(ctx, key, c.ttl).Err()
		return Unchanged, nil
	}
	return PublishChanged, nil
}

// Commit stores the content hash after a successful publish.
func (c *Checker) Commit(ctx context.Context, tenantID, platform string, content Content) error {
	if c == nil || c.client == nil {
		return nil
	}
	id := norm(content.ExternalCallID)
	if id == "" {
		return errors.New("pingdedup.Commit: empty external_call_id")
	}
	key := Key(tenantID, platform, id)
	if err := c.client.Set(ctx, key, Hash(content), c.ttl).Err(); err != nil {
		c.logError("pingdedup Commit: redis SET failed",
			"tenant_id", tenantID, "platform", platform, "error", err.Error())
		return fmt.Errorf("pingdedup.Commit: %w", err)
	}
	return nil
}

func (c *Checker) logError(msg string, args ...any) {
	logger := slog.Default()
	if c != nil && c.logger != nil {
		logger = c.logger
	}
	logger.Error(msg, args...)
}

// Ping checks Redis connectivity.
func (c *Checker) Ping(ctx context.Context) error {
	if err := c.client.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("pingdedup.Ping: %w", err)
	}
	return nil
}
