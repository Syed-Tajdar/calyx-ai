package kafka

import (
	"context"
	"fmt"
	"time"

	"github.com/segmentio/kafka-go"
)

// Reader wraps a Kafka consumer group reader.
type Reader struct {
	reader *kafka.Reader
}

// NewReader creates a Kafka consumer for the given topic and group.
func NewReader(brokers []string, topic, group string) *Reader {
	return &Reader{
		reader: kafka.NewReader(kafka.ReaderConfig{
			Brokers:        brokers,
			Topic:          topic,
			GroupID:        group,
			MinBytes:       1,
			MaxBytes:       10e6,
			CommitInterval: time.Second,
			StartOffset:    kafka.FirstOffset,
		}),
	}
}

// Fetch reads the next Kafka message.
func (r *Reader) Fetch(ctx context.Context) (kafka.Message, error) {
	msg, err := r.reader.FetchMessage(ctx)
	if err != nil {
		return kafka.Message{}, fmt.Errorf("kafka.Fetch: %w", err)
	}
	if err := r.reader.CommitMessages(ctx, msg); err != nil {
		return kafka.Message{}, fmt.Errorf("kafka.Fetch: commit: %w", err)
	}
	return msg, nil
}

// Ping verifies broker reachability.
func (r *Reader) Ping(ctx context.Context) error {
	dialer := &kafka.Dialer{Timeout: 3 * time.Second}
	conn, err := dialer.DialContext(ctx, "tcp", r.reader.Config().Brokers[0])
	if err != nil {
		return fmt.Errorf("kafka.Ping: %w", err)
	}
	defer conn.Close()
	return nil
}

// Close shuts down the reader.
func (r *Reader) Close() error {
	return r.reader.Close()
}
