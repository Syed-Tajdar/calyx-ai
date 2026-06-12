package kafka

import (
	"context"
	"fmt"
	"time"

	"github.com/segmentio/kafka-go"
)

// Client wraps a Kafka producer with connection pooling semantics.
type Client struct {
	writer *kafka.Writer
	broker string
}

// NewClient creates a Kafka producer client.
func NewClient(brokers []string, topic string) *Client {
	if len(brokers) == 0 {
		brokers = []string{"localhost:9092"}
	}
	_ = topic
	return &Client{
		broker: brokers[0],
		writer: &kafka.Writer{
			Addr:         kafka.TCP(brokers...),
			Balancer:     &kafka.Hash{},
			RequiredAcks: kafka.RequireOne,
			Async:        false,
			BatchTimeout: 10 * time.Millisecond,
		},
	}
}

// Write publishes a message keyed by tenant_id for partition affinity.
func (c *Client) Write(ctx context.Context, topic, key string, value []byte) error {
	err := c.writer.WriteMessages(ctx, kafka.Message{
		Topic: topic,
		Key:   []byte(key),
		Value: value,
	})
	if err != nil {
		return fmt.Errorf("kafka.Write: %w", err)
	}
	return nil
}

// Flush ensures all buffered messages are sent.
func (c *Client) Flush(ctx context.Context) error {
	if err := c.writer.Close(); err != nil {
		return fmt.Errorf("kafka.Flush: %w", err)
	}
	return nil
}

// Ping verifies broker reachability.
func (c *Client) Ping(ctx context.Context) error {
	dialer := &kafka.Dialer{Timeout: 3 * time.Second}
	conn, err := dialer.DialContext(ctx, "tcp", c.broker)
	if err != nil {
		return fmt.Errorf("kafka.Ping: %w", err)
	}
	defer conn.Close()
	return nil
}

// Close shuts down the writer.
func (c *Client) Close() error {
	return c.writer.Close()
}
