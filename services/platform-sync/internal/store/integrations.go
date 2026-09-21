package store

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Syed-Tajdar/calyx-ai/services/platform-sync/internal/models"
	"github.com/jackc/pgx/v5/pgxpool"
)

type IntegrationStore struct {
	pool *pgxpool.Pool
}

func NewIntegrationStore(pool *pgxpool.Pool) *IntegrationStore {
	return &IntegrationStore{pool: pool}
}

func (s *IntegrationStore) ListAPISync(ctx context.Context) ([]models.SyncIntegration, error) {
	rows, err := s.pool.Query(ctx, `SELECT * FROM list_api_sync_integrations()`)
	if err != nil {
		return nil, fmt.Errorf("ListAPISync: %w", err)
	}
	defer rows.Close()
	var out []models.SyncIntegration
	for rows.Next() {
		item, err := scanIntegration(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *IntegrationStore) GetAPISync(ctx context.Context, tenantID, integrationID string) (*models.SyncIntegration, error) {
	return s.getAPISync(ctx, `SELECT * FROM get_api_sync_integration($1::uuid, $2::uuid)`, tenantID, integrationID)
}

// GetAPISyncAny includes paused (is_active=false) api_sync rows — used by write-back only.
func (s *IntegrationStore) GetAPISyncAny(ctx context.Context, tenantID, integrationID string) (*models.SyncIntegration, error) {
	row := s.pool.QueryRow(ctx, `SELECT * FROM get_api_sync_integration_any($1::uuid, $2::uuid)`, tenantID, integrationID)
	var item models.SyncIntegration
	var lastSync *time.Time
	var cursor *string
	var syncStatus *string
	var syncError *string
	var lookbackDays int
	if err := row.Scan(
		&item.ID, &item.TenantID, &item.Platform, &item.Name, &item.SyncMode,
		&item.APICredentialsBlob, &cursor, &lastSync, &syncStatus, &syncError,
		&lookbackDays, &item.WritebackEnabled,
	); err != nil {
		return nil, fmt.Errorf("integration not found")
	}
	if cursor != nil {
		item.SyncCursor = *cursor
	}
	item.LastSyncAt = lastSync
	if syncStatus != nil {
		item.SyncStatus = *syncStatus
	}
	if syncError != nil {
		item.LastSyncError = *syncError
	}
	if item.SyncMode == "" {
		item.SyncMode = "mock"
	}
	if lookbackDays < 1 {
		lookbackDays = 30
	}
	item.InitialLookbackDays = lookbackDays
	return &item, nil
}

func (s *IntegrationStore) InsertWritebackAudit(ctx context.Context, row AuditRow) (string, error) {
	var id string
	err := s.pool.QueryRow(ctx, `
		SELECT insert_writeback_audit(
			$1::uuid, $2::uuid, $3::uuid, $4, $5, $6, $7, $8, $9, $10, $11
		)::text`,
		row.TenantID, nilUUID(row.UserID), nilUUID(row.IntegrationID),
		row.InboundCallID, row.Action, row.Amount, row.Outcome,
		nullIfEmpty(row.RingbaRef), nullIfEmpty(row.CampaignID),
		nullIfEmpty(row.Message), nullIfEmpty(row.OfflineUploaded),
	).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("insert writeback audit: %w", err)
	}
	return id, nil
}

type AuditRow struct {
	TenantID        string
	UserID          string
	IntegrationID   string
	InboundCallID   string
	Action          string
	Amount          float64
	Outcome         string
	RingbaRef       string
	CampaignID      string
	Message         string
	OfflineUploaded string
}

func nilUUID(s string) interface{} {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return s
}

func nullIfEmpty(s string) interface{} {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return s
}

func (s *IntegrationStore) getAPISync(ctx context.Context, query, tenantID, integrationID string) (*models.SyncIntegration, error) {
	rows, err := s.pool.Query(ctx, query, tenantID, integrationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		return nil, fmt.Errorf("integration not found")
	}
	item, err := scanIntegration(rows)
	if err != nil {
		return nil, err
	}
	return &item, nil
}

type scannable interface {
	Scan(dest ...interface{}) error
}

func scanIntegration(rows scannable) (models.SyncIntegration, error) {
	var item models.SyncIntegration
	var lastSync *time.Time
	var cursor *string
	var syncStatus *string
	var syncError *string
	var lookbackDays int
	if err := rows.Scan(
		&item.ID, &item.TenantID, &item.Platform, &item.Name, &item.SyncMode,
		&item.APICredentialsBlob, &cursor, &lastSync, &syncStatus, &syncError,
		&lookbackDays,
	); err != nil {
		return item, fmt.Errorf("scan integration: %w", err)
	}
	if cursor != nil {
		item.SyncCursor = *cursor
	}
	item.LastSyncAt = lastSync
	if syncStatus != nil {
		item.SyncStatus = *syncStatus
	}
	if syncError != nil {
		item.LastSyncError = *syncError
	}
	if item.SyncMode == "" {
		item.SyncMode = "mock"
	}
	if lookbackDays < 1 {
		lookbackDays = 30
	}
	item.InitialLookbackDays = lookbackDays
	return item, nil
}

func (s *IntegrationStore) UpdateSyncState(
	ctx context.Context,
	id, cursor string,
	syncAt time.Time,
	status string,
	errMsg *string,
) error {
	_, err := s.pool.Exec(ctx,
		`SELECT update_integration_sync_state($1::uuid, $2, $3, $4, $5)`,
		id, cursor, syncAt, status, errMsg,
	)
	return err
}

func (s *IntegrationStore) Ping(ctx context.Context) error {
	return s.pool.Ping(ctx)
}
