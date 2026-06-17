package clickhouse

import (
	"context"
	"fmt"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/Syed-Tajdar/calyx-ai/services/enrichment/internal/models"
)

// Client wraps ClickHouse for enrichment writes.
type Client struct {
	conn clickhouse.Conn
}

// NewClient connects to ClickHouse.
func NewClient(addr, database, username, password string) (*Client, error) {
	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr: []string{addr},
		Auth: clickhouse.Auth{Database: database, Username: username, Password: password},
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

// InsertAttributes batch-inserts enrichment attributes.
func (c *Client) InsertAttributes(ctx context.Context, tenantID, aniHash, provider string, attrs []models.Attribute) error {
	batch, err := c.conn.PrepareBatch(ctx, `
		INSERT INTO enrichment_attrs (tenant_id, ani_hash, attribute_key, attribute_value, provider, created_at)
	`)
	if err != nil {
		return fmt.Errorf("clickhouse.InsertAttributes prepare: %w", err)
	}
	now := time.Now().UTC()
	for _, attr := range attrs {
		if err := batch.Append(tenantID, aniHash, attr.Key, attr.Value, provider, now); err != nil {
			return fmt.Errorf("clickhouse.InsertAttributes append: %w", err)
		}
	}
	if err := batch.Send(); err != nil {
		return fmt.Errorf("clickhouse.InsertAttributes send: %w", err)
	}
	return nil
}

// InsertDLQ stores failed enrichment events.
func (c *Client) InsertDLQ(ctx context.Context, tenantID, aniHash, eventType, errMsg, payloadHash string) error {
	err := c.conn.Exec(ctx, `
		INSERT INTO enrichment_dlq (tenant_id, ani_hash, event_type, error_message, payload_hash, failed_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, tenantID, aniHash, eventType, errMsg, payloadHash, time.Now().UTC())
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
