package store

import (
	"context"
	"fmt"
	"strings"
)

// PauseIntegration is one row of get_publisher_pause_integration() (migration 000035). It also
// returns paused and Test rows so the pause service can refuse them with a clear reason.
type PauseIntegration struct {
	ID                  string
	TenantID            string
	Platform            string
	Name                string
	SyncMode            string
	IsActive            bool
	AllowPublisherPause bool
	APICredentialsBlob  []byte
}

// GetPublisherPauseIntegration loads the integration for a publisher pause / unpause.
func (s *IntegrationStore) GetPublisherPauseIntegration(ctx context.Context, tenantID, integrationID string) (*PauseIntegration, error) {
	row := s.pool.QueryRow(ctx,
		`SELECT id::text, tenant_id::text, platform, name, sync_mode, is_active, allow_publisher_pause, api_credentials_blob
		   FROM get_publisher_pause_integration($1::uuid, $2::uuid)`,
		tenantID, integrationID)
	var it PauseIntegration
	var name, syncMode *string
	if err := row.Scan(&it.ID, &it.TenantID, &it.Platform, &name, &syncMode,
		&it.IsActive, &it.AllowPublisherPause, &it.APICredentialsBlob); err != nil {
		return nil, fmt.Errorf("integration not found")
	}
	if name != nil {
		it.Name = *name
	}
	if syncMode != nil {
		it.SyncMode = *syncMode
	}
	return &it, nil
}

// PublisherAuditRow is one publisher_action_audit row. Message and RequestRedacted must already
// be redacted by the caller (the pause service does it); Numbers is a JSON object or empty.
type PublisherAuditRow struct {
	TenantID        string
	UserID          string
	IntegrationID   string
	Platform        string
	PublisherID     string
	Action          string // pause | unpause
	Mode            string // manual | auto | timer
	RuleID          string
	AlertEventID    string
	Outcome         string // confirmed | confirmed_by_response | failed | rejected
	ReadBack        string // ok | none | mismatch
	Message         string
	RequestRedacted string
	Numbers         string
}

// InsertPublisherActionAudit writes one immutable audit row via insert_publisher_action_audit().
func (s *IntegrationStore) InsertPublisherActionAudit(ctx context.Context, row PublisherAuditRow) (string, error) {
	numbers := strings.TrimSpace(row.Numbers)
	if numbers == "" || numbers == "null" {
		numbers = "{}"
	}
	readBack := row.ReadBack
	if readBack == "" {
		readBack = "none"
	}
	var id string
	err := s.pool.QueryRow(ctx, `
		SELECT insert_publisher_action_audit(
			$1::uuid, $2::uuid, $3::uuid, $4, $5, $6, $7, $8::uuid, $9::uuid, $10, $11, $12, $13, $14::jsonb
		)::text`,
		row.TenantID, nilUUID(row.UserID), nilUUID(row.IntegrationID), row.Platform, row.PublisherID,
		row.Action, row.Mode, nilUUID(row.RuleID), nilUUID(row.AlertEventID), row.Outcome, readBack,
		nullIfEmpty(row.Message), nullIfEmpty(row.RequestRedacted), numbers,
	).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("insert publisher action audit: %w", err)
	}
	return id, nil
}
