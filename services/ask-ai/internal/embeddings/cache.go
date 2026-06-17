package embeddings

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"

	"github.com/Syed-Tajdar/call-loom-intelligence/services/ask-ai/internal/models"
	pgstore "github.com/Syed-Tajdar/call-loom-intelligence/services/ask-ai/pkg/postgres"
)

// Cache provides exact-question cache lookups per tenant.
type Cache struct {
	store *pgstore.Store
}

// NewCache creates a question similarity cache backed by Postgres.
func NewCache(store *pgstore.Store) *Cache {
	return &Cache{store: store}
}

// Lookup returns a cached answer for an identical normalized question.
func (c *Cache) Lookup(ctx context.Context, tenantID, question string) (*models.AskResponse, bool, error) {
	normalized := normalizeQuestion(question)
	rec, found, err := c.store.FindCachedQuestion(ctx, tenantID, normalized)
	if err != nil || !found {
		return nil, false, err
	}
	return &models.AskResponse{
		QueryID:    rec.ID,
		Question:   question,
		Answer:     rec.Answer,
		Intent:     rec.Intent,
		CacheHit:   true,
		RowCount:   rec.RowCount,
		TokensUsed: 0,
	}, true, nil
}

// QuestionHash returns a stable hash for analytics cache keys.
func QuestionHash(question string) string {
	sum := sha256.Sum256([]byte(normalizeQuestion(question)))
	return hex.EncodeToString(sum[:])
}

func normalizeQuestion(q string) string {
	return strings.Join(strings.Fields(strings.ToLower(strings.TrimSpace(q))), " ")
}
