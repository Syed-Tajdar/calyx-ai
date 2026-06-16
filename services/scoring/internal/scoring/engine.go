package scoring

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/Syed-Tajdar/call-loom-intelligence/services/scoring/internal/models"
	"github.com/redis/go-redis/v9"
)

// IdempotencyGuard prevents re-scoring the same call_id.
type IdempotencyGuard struct {
	redis *redis.Client
}

// NewIdempotencyGuard creates a Redis-backed idempotency guard.
func NewIdempotencyGuard(client *redis.Client) *IdempotencyGuard {
	return &IdempotencyGuard{redis: client}
}

func scoringKey(tenantID, callID string) string {
	return fmt.Sprintf("%s:scoring:scored:%s", tenantID, callID)
}

// AlreadyScored returns true if this call was previously scored.
func (g *IdempotencyGuard) AlreadyScored(ctx context.Context, tenantID, callID string) (bool, error) {
	exists, err := g.redis.Exists(ctx, scoringKey(tenantID, callID)).Result()
	if err != nil {
		return false, fmt.Errorf("IdempotencyGuard.AlreadyScored: %w", err)
	}
	return exists > 0, nil
}

// MarkScored records that a call has been scored.
func (g *IdempotencyGuard) MarkScored(ctx context.Context, tenantID, callID string) error {
	if err := g.redis.Set(ctx, scoringKey(tenantID, callID), "1", 0).Err(); err != nil {
		return fmt.Errorf("IdempotencyGuard.MarkScored: %w", err)
	}
	return nil
}

// PayloadHash returns SHA-256 hex of raw payload for DLQ metadata.
func PayloadHash(raw []byte) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

// Engine orchestrates transcript scoring.
type Engine struct {
	ai AIProvider
}

// NewEngine creates a scoring engine.
func NewEngine(ai AIProvider) *Engine {
	return &Engine{ai: ai}
}

// Score runs AI evaluation on redacted transcript text.
func (e *Engine) Score(ctx context.Context, tenantID, callID, transcript string) (models.ScoreResult, error) {
	_ = ctx
	return e.ai.Score(tenantID, callID, transcript)
}
