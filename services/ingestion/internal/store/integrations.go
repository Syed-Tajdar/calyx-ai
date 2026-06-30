package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

// ErrIntegrationNotFound is returned when no active integration matches the webhook token.
var ErrIntegrationNotFound = errors.New("integration not found")

// Integration maps a webhook token to a tenant and optional HMAC secret.
type Integration struct {
	TenantID      string
	Platform      string
	WebhookSecret string
}

// IntegrationStore loads integration config from PostgreSQL.
type IntegrationStore struct {
	db *sql.DB
}

// NewIntegrationStore opens a PostgreSQL connection pool.
func NewIntegrationStore(databaseURL string) (*IntegrationStore, error) {
	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return nil, fmt.Errorf("store.NewIntegrationStore: %w", err)
	}
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)
	return &IntegrationStore{db: db}, nil
}

// LookupByToken resolves tenant_id and secret for a webhook path token.
func (s *IntegrationStore) LookupByToken(ctx context.Context, token string) (Integration, error) {
	var integration Integration
	err := s.db.QueryRowContext(ctx, `
		SELECT tenant_id::text, platform, webhook_secret
		FROM lookup_integration_by_token($1)
	`, token).Scan(&integration.TenantID, &integration.Platform, &integration.WebhookSecret)
	if err == sql.ErrNoRows {
		return Integration{}, ErrIntegrationNotFound
	}
	if err != nil {
		return Integration{}, fmt.Errorf("store.LookupByToken: %w", err)
	}
	return integration, nil
}

// Ping checks database connectivity.
func (s *IntegrationStore) Ping(ctx context.Context) error {
	if err := s.db.PingContext(ctx); err != nil {
		return fmt.Errorf("store.Ping: %w", err)
	}
	return nil
}

// Close closes the database connection pool.
func (s *IntegrationStore) Close() error {
	return s.db.Close()
}
