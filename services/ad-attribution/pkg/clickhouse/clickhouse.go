package clickhouse

import (
	"context"
	"fmt"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/Syed-Tajdar/call-loom-intelligence/services/ad-attribution/internal/models"
)

// Client writes attribution analytics to ClickHouse.
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

// Close closes the connection.
func (c *Client) Close() error {
	return c.conn.Close()
}

// Ping checks connectivity.
func (c *Client) Ping(ctx context.Context) error {
	return c.conn.Ping(ctx)
}

// InsertFiredEvent logs an attribution fire attempt.
func (c *Client) InsertFiredEvent(ctx context.Context, evt models.ConversionEvent, platform string, res models.FireResult) error {
	status := "fired"
	if !res.Success {
		status = "failed"
	}
	err := c.conn.Exec(ctx, `
		INSERT INTO attribution_fired_events
		(tenant_id, call_id, event_id, platform, click_id, revenue, status, http_status, error_message, fired_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, evt.TenantID, evt.CallID, evt.EventID, platform, evt.ClickID, evt.Revenue,
		status, uint16(res.HTTPStatus), res.Error, time.Now().UTC())
	if err != nil {
		return fmt.Errorf("clickhouse.InsertFiredEvent: %w", err)
	}
	return nil
}

// InsertDLQ stores a failed attribution stage.
func (c *Client) InsertDLQ(ctx context.Context, evt models.ConversionEvent, stage, errMsg, payloadHash string) error {
	err := c.conn.Exec(ctx, `
		INSERT INTO attribution_dlq
		(tenant_id, call_id, event_id, stage, error_message, payload_hash, failed_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, evt.TenantID, evt.CallID, evt.EventID, stage, errMsg, payloadHash, time.Now().UTC())
	if err != nil {
		return fmt.Errorf("clickhouse.InsertDLQ: %w", err)
	}
	return nil
}
