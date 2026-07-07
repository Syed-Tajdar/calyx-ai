package store



import (

	"context"

	"fmt"

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

	rows, err := s.pool.Query(ctx, `

		SELECT id, tenant_id, platform, name, api_credentials_blob, sync_cursor,

		       last_sync_at, sync_status, last_sync_error

		FROM integrations

		WHERE id = $1::uuid AND tenant_id = $2::uuid

		  AND connection_mode = 'api_sync' AND is_active = TRUE`,

		integrationID, tenantID,

	)

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

	if err := rows.Scan(

		&item.ID, &item.TenantID, &item.Platform, &item.Name,

		&item.APICredentialsBlob, &cursor, &lastSync, &syncStatus, &syncError,

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


