package clickhouse

import (
	"context"
	"fmt"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/shopspring/decimal"
)

// PingRow is a single row for the pings table.
type PingRow struct {
	TenantID       string
	Platform       string
	ExternalCallID string
	ANIHash        string
	CampaignID     string
	PublisherID    string
	TargetID       string
	State          string
	EventType      string
	Revenue        float64
	Payout         float64
	IsDuplicate    uint8
	ReceivedAt     time.Time
}

// Client wraps the ClickHouse native connection.
type Client struct {
	conn clickhouse.Conn
}

// NewClient connects to ClickHouse.
func NewClient(addr, database, username, password string) (*Client, error) {
	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr: []string{addr},
		Auth: clickhouse.Auth{
			Database: database,
			Username: username,
			Password: password,
		},
		Settings: clickhouse.Settings{
			"max_execution_time": 60,
		},
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		return nil, fmt.Errorf("clickhouse.NewClient: open: %w", err)
	}
	if err := conn.Ping(context.Background()); err != nil {
		return nil, fmt.Errorf("clickhouse.NewClient: ping: %w", err)
	}
	return &Client{conn: conn}, nil
}

// InsertPings batch-inserts ping rows.
func (c *Client) InsertPings(ctx context.Context, rows []PingRow) error {
	batch, err := c.conn.PrepareBatch(ctx, `
		INSERT INTO pings (
			tenant_id, platform, external_call_id, ani_hash,
			campaign_id, publisher_id, target_id, state,
			event_type, revenue, payout, is_duplicate, received_at
		)
	`)
	if err != nil {
		return fmt.Errorf("clickhouse.InsertPings: prepare: %w", err)
	}

	for _, row := range rows {
		if err := batch.Append(
			row.TenantID,
			row.Platform,
			row.ExternalCallID,
			row.ANIHash,
			row.CampaignID,
			row.PublisherID,
			row.TargetID,
			row.State,
			row.EventType,
			decimal.NewFromFloat(row.Revenue),
			decimal.NewFromFloat(row.Payout),
			row.IsDuplicate,
			row.ReceivedAt,
		); err != nil {
			return fmt.Errorf("clickhouse.InsertPings: append: %w", err)
		}
	}

	if err := batch.Send(); err != nil {
		return fmt.Errorf("clickhouse.InsertPings: send: %w", err)
	}
	return nil
}

// Ping checks connectivity.
func (c *Client) Ping(ctx context.Context) error {
	if err := c.conn.Ping(ctx); err != nil {
		return fmt.Errorf("clickhouse.Ping: %w", err)
	}
	return nil
}

// Close closes the connection.
func (c *Client) Close() error {
	return c.conn.Close()
}
