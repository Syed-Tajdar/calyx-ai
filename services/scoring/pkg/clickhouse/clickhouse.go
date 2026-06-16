package clickhouse

import (
	"context"
	"fmt"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
)

// Client wraps ClickHouse connection for scoring writes.
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

// InsertTranscript stores a redacted transcript.
func (c *Client) InsertTranscript(ctx context.Context, tenantID, callID, text string, duration float32) error {
	wordCount := uint32(len(splitWords(text)))
	err := c.conn.Exec(ctx, `
		INSERT INTO transcripts (tenant_id, call_id, transcript_text, duration_seconds, word_count, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, tenantID, callID, text, duration, wordCount, time.Now().UTC())
	if err != nil {
		return fmt.Errorf("clickhouse.InsertTranscript: %w", err)
	}
	return nil
}

// InsertScore stores a call score row.
func (c *Client) InsertScore(ctx context.Context, tenantID, callID, platform string, score float32, sentiment string, passed, failed uint16) error {
	err := c.conn.Exec(ctx, `
		INSERT INTO call_scores (tenant_id, call_id, platform, overall_score, sentiment, rules_passed, rules_failed, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, tenantID, callID, platform, score, sentiment, passed, failed, time.Now().UTC())
	if err != nil {
		return fmt.Errorf("clickhouse.InsertScore: %w", err)
	}
	return nil
}

// InsertDLQ stores a failed scoring event.
func (c *Client) InsertDLQ(ctx context.Context, tenantID, callID, eventType, errMsg, payloadHash string) error {
	err := c.conn.Exec(ctx, `
		INSERT INTO scoring_dlq (tenant_id, call_id, event_type, error_message, payload_hash, failed_at)
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

func splitWords(s string) []string {
	var words []string
	start := -1
	for i, r := range s {
		if r == ' ' || r == '\n' || r == '\t' {
			if start >= 0 {
				words = append(words, s[start:i])
				start = -1
			}
			continue
		}
		if start < 0 {
			start = i
		}
	}
	if start >= 0 {
		words = append(words, s[start:])
	}
	return words
}
