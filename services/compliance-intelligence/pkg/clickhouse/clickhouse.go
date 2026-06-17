package clickhouse

import (
	"context"
	"fmt"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/Syed-Tajdar/call-loom-intelligence/services/compliance-intelligence/internal/models"
)

// Client wraps ClickHouse compliance writes.
type Client struct {
	conn clickhouse.Conn
}

// NewClient connects to ClickHouse.
func NewClient(addr, database, username, password string) (*Client, error) {
	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr:        []string{addr},
		Auth:        clickhouse.Auth{Database: database, Username: username, Password: password},
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		return nil, fmt.Errorf("clickhouse.NewClient: %w", err)
	}
	if err := conn.Ping(context.Background()); err != nil {
		return nil, fmt.Errorf("clickhouse.NewClient ping: %w", err)
	}
	return &Client{conn: conn}, nil
}

// InsertRiskScore stores a compliance risk score row.
func (c *Client) InsertRiskScore(ctx context.Context, ping models.PingMessage, risk models.RiskResult) error {
	dncFlag := uint8(0)
	if risk.DNCMatched {
		dncFlag = 1
	}
	err := c.conn.Exec(ctx, `
		INSERT INTO compliance_risk_scores
		(tenant_id, call_id, publisher_id, state, risk_score, dnc_match, attestation_level, exposure_usd, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, ping.TenantID, ping.ExternalCallID, ping.PublisherID, ping.State, risk.RiskScore, dncFlag,
		risk.AttestationLevel, risk.ExposureUSD, time.Now().UTC())
	if err != nil {
		return fmt.Errorf("clickhouse.InsertRiskScore: %w", err)
	}
	return nil
}

// InsertDNCEvent stores a DNC check event.
func (c *Client) InsertDNCEvent(ctx context.Context, ping models.PingMessage, listName string, matched bool) error {
	flag := uint8(0)
	if matched {
		flag = 1
	}
	err := c.conn.Exec(ctx, `
		INSERT INTO dnc_match_events
		(tenant_id, call_id, ani_hash, publisher_id, list_name, matched, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, ping.TenantID, ping.ExternalCallID, ping.ANIHash, ping.PublisherID, listName, flag, time.Now().UTC())
	if err != nil {
		return fmt.Errorf("clickhouse.InsertDNCEvent: %w", err)
	}
	return nil
}

// InsertAttestation stores attestation analytics.
func (c *Client) InsertAttestation(ctx context.Context, tenantID, callID, level string, blockEst float32) error {
	err := c.conn.Exec(ctx, `
		INSERT INTO attestation_events
		(tenant_id, call_id, attestation_level, carrier_block_est, created_at)
		VALUES (?, ?, ?, ?, ?)
	`, tenantID, callID, level, blockEst, time.Now().UTC())
	if err != nil {
		return fmt.Errorf("clickhouse.InsertAttestation: %w", err)
	}
	return nil
}

// InsertViolation stores a compliance violation row.
func (c *Client) InsertViolation(ctx context.Context, tenantID, callID, violationType string, risk models.RiskResult) error {
	err := c.conn.Exec(ctx, `
		INSERT INTO compliance_violations
		(tenant_id, call_id, violation_type, risk_score, exposure_usd, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, tenantID, callID, violationType, risk.RiskScore, risk.ExposureUSD, time.Now().UTC())
	if err != nil {
		return fmt.Errorf("clickhouse.InsertViolation: %w", err)
	}
	return nil
}

// InsertDLQ stores failed compliance events.
func (c *Client) InsertDLQ(ctx context.Context, tenantID, callID, eventType, errMsg, payloadHash string) error {
	err := c.conn.Exec(ctx, `
		INSERT INTO compliance_dlq (tenant_id, call_id, event_type, error_message, payload_hash, failed_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, tenantID, callID, eventType, errMsg, payloadHash, time.Now().UTC())
	if err != nil {
		return fmt.Errorf("clickhouse.InsertDLQ: %w", err)
	}
	return nil
}

func (c *Client) Ping(ctx context.Context) error {
	return c.conn.Ping(ctx)
}

func (c *Client) Close() error {
	return c.conn.Close()
}
