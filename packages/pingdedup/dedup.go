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
	// PublishNew: key was absent — publish with is_duplicate=0 and store hash.
	PublishNew Decision = iota
	// PublishChanged: stored hash differed — publish with is_duplicate=0 and update hash.
	PublishChanged
	// Unchanged: stored hash matched — do not publish.
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
// On ANY Redis error Evaluate fail-OPENs: returns PublishNew/PublishChanged with nil
// error so callers always publish (duplicates are OK; argMax protects aggregates).
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

// Evaluate compares content hash against Redis and updates the stored hash when publishing.
// Redis failures NEVER suppress publish (fail-OPEN).
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
		if setErr := c.client.Set(ctx, key, hash, c.ttl).Err(); setErr != nil {
			c.logError("pingdedup fail-OPEN: redis SET failed after miss — publishing",
				"tenant_id", tenantID, "platform", platform, "error", setErr.Error())
			return PublishNew, nil
		}
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
	if setErr := c.client.Set(ctx, key, hash, c.ttl).Err(); setErr != nil {
		c.logError("pingdedup fail-OPEN: redis SET failed after change — publishing",
			"tenant_id", tenantID, "platform", platform, "error", setErr.Error())
		return PublishChanged, nil
	}
	return PublishChanged, nil
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
