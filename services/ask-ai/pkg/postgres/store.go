package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/Syed-Tajdar/call-loom-intelligence/services/ask-ai/internal/models"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Store persists Ask AI query history in Postgres with RLS.
type Store struct {
	pool *pgxpool.Pool
}

// NewStore connects to Postgres.
func NewStore(databaseURL string) (*Store, error) {
	pool, err := pgxpool.New(context.Background(), databaseURL)
	if err != nil {
		return nil, fmt.Errorf("postgres.NewStore: %w", err)
	}
	return &Store{pool: pool}, nil
}

// Close closes the pool.
func (s *Store) Close() {
	s.pool.Close()
}

// Ping checks connectivity.
func (s *Store) Ping(ctx context.Context) error {
	return s.pool.Ping(ctx)
}

// InsertQueryLog stores a Q&A row and returns its id.
func (s *Store) InsertQueryLog(
	ctx context.Context,
	tenantID, question, intent, answer, sqlText string,
	rowCount int,
	cacheHit bool,
	tokens int,
) (string, error) {
	var id string
	err := s.withTenant(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		return tx.QueryRow(ctx, `
			INSERT INTO ask_ai_query_log
			(tenant_id, question, intent, answer, sql_text, row_count, cache_hit, tokens_used, updated_at)
			VALUES ($1::uuid, $2, $3, $4, $5, $6, $7, $8, NOW())
			RETURNING id::text
		`, tenantID, question, intent, answer, sqlText, rowCount, cacheHit, tokens).Scan(&id)
	})
	if err != nil {
		return "", fmt.Errorf("postgres.InsertQueryLog: %w", err)
	}
	return id, nil
}

// FindCachedQuestion returns the latest identical normalized question answer.
func (s *Store) FindCachedQuestion(ctx context.Context, tenantID, normalizedQuestion string) (models.HistoryRecord, bool, error) {
	var rec models.HistoryRecord
	err := s.withTenant(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		return tx.QueryRow(ctx, `
			SELECT id::text, question, answer, intent, cache_hit, row_count, tokens_used,
			       to_char(created_at AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS.US"Z"')
			FROM ask_ai_query_log
			WHERE tenant_id = $1::uuid
			  AND lower(regexp_replace(trim(question), '\s+', ' ', 'g')) = $2
			ORDER BY created_at DESC
			LIMIT 1
		`, tenantID, normalizedQuestion).Scan(
			&rec.ID, &rec.Question, &rec.Answer, &rec.Intent, &rec.CacheHit,
			&rec.RowCount, &rec.TokensUsed, &rec.CreatedAt,
		)
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return models.HistoryRecord{}, false, nil
	}
	if err != nil {
		return models.HistoryRecord{}, false, err
	}
	return rec, true, nil
}

// ListHistory returns recent Q&A for a tenant.
func (s *Store) ListHistory(ctx context.Context, tenantID string, limit int) ([]models.HistoryRecord, error) {
	if limit < 1 {
		limit = 1
	}
	if limit > 100 {
		limit = 100
	}
	var rows []models.HistoryRecord
	err := s.withTenant(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		result, err := tx.Query(ctx, `
			SELECT id::text, question, answer, intent, cache_hit, row_count, tokens_used,
			       to_char(created_at AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS.US"Z"')
			FROM ask_ai_query_log
			WHERE tenant_id = $1::uuid
			ORDER BY created_at DESC
			LIMIT $2
		`, tenantID, limit)
		if err != nil {
			return err
		}
		defer result.Close()
		for result.Next() {
			var rec models.HistoryRecord
			if err := result.Scan(
				&rec.ID, &rec.Question, &rec.Answer, &rec.Intent, &rec.CacheHit,
				&rec.RowCount, &rec.TokensUsed, &rec.CreatedAt,
			); err != nil {
				return err
			}
			rows = append(rows, rec)
		}
		return result.Err()
	})
	if err != nil {
		return nil, fmt.Errorf("postgres.ListHistory: %w", err)
	}
	return rows, nil
}

func (s *Store) withTenant(ctx context.Context, tenantID string, fn func(context.Context, pgx.Tx) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true)`, tenantID); err != nil {
		return err
	}
	if err := fn(ctx, tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
