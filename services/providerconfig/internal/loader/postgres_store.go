package loader

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

// PostgresBindingStore reads tenant_provider_bindings with RLS tenant context.
type PostgresBindingStore struct {
	db *sql.DB
}

// NewPostgresBindingStore opens a Postgres connection.
func NewPostgresBindingStore(databaseURL string) (*PostgresBindingStore, error) {
	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return nil, fmt.Errorf("NewPostgresBindingStore: open: %w", err)
	}
	db.SetMaxOpenConns(10)
	db.SetConnMaxLifetime(5 * time.Minute)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("NewPostgresBindingStore: ping: %w", err)
	}
	return &PostgresBindingStore{db: db}, nil
}

// Close closes the database pool.
func (s *PostgresBindingStore) Close() error {
	return s.db.Close()
}

// Ping checks database connectivity.
func (s *PostgresBindingStore) Ping(ctx context.Context) error {
	return s.db.PingContext(ctx)
}

// GetActiveBinding returns the highest-priority active binding for tenant+domain.
func (s *PostgresBindingStore) GetActiveBinding(tenantID, domain string) (*Binding, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("GetActiveBinding: begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `SELECT set_config('app.current_tenant', $1, true)`, tenantID); err != nil {
		return nil, fmt.Errorf("GetActiveBinding: set tenant: %w", err)
	}

	row := tx.QueryRowContext(ctx, `
		SELECT id::text, tenant_id::text, domain, provider_id, config_json, updated_at
		FROM tenant_provider_bindings
		WHERE tenant_id = $1::uuid AND domain = $2 AND is_active = TRUE
		ORDER BY priority DESC, updated_at DESC
		LIMIT 1
	`, tenantID, domain)

	var b Binding
	var configRaw []byte
	if err := row.Scan(&b.ID, &b.TenantID, &b.Domain, &b.ProviderID, &configRaw, &b.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("GetActiveBinding: scan: %w", err)
	}
	b.ConfigJSON = configRaw
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("GetActiveBinding: commit: %w", err)
	}
	return &b, nil
}

// CachedBinding is the Redis-serializable binding payload.
type CachedBinding struct {
	ID         string    `json:"id"`
	TenantID   string    `json:"tenant_id"`
	Domain     string    `json:"domain"`
	ProviderID string    `json:"provider_id"`
	ConfigJSON []byte    `json:"config_json"`
	UpdatedAt  time.Time `json:"updated_at"`
}

func bindingToCached(b *Binding) CachedBinding {
	return CachedBinding{
		ID:         b.ID,
		TenantID:   b.TenantID,
		Domain:     b.Domain,
		ProviderID: b.ProviderID,
		ConfigJSON: b.ConfigJSON,
		UpdatedAt:  b.UpdatedAt,
	}
}

func cachedToBinding(c CachedBinding) *Binding {
	return &Binding{
		ID:         c.ID,
		TenantID:   c.TenantID,
		Domain:     c.Domain,
		ProviderID: c.ProviderID,
		ConfigJSON: c.ConfigJSON,
		UpdatedAt:  c.UpdatedAt,
	}
}

// MarshalBindingJSON marshals binding for Redis.
func MarshalBindingJSON(b *Binding) ([]byte, error) {
	return json.Marshal(bindingToCached(b))
}

// UnmarshalBindingJSON unmarshals binding from Redis.
func UnmarshalBindingJSON(raw []byte) (*Binding, error) {
	var c CachedBinding
	if err := json.Unmarshal(raw, &c); err != nil {
		return nil, err
	}
	return cachedToBinding(c), nil
}
