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
			CommitInterval: 0,
			StartOffset:    kafka.FirstOffset,
		}),
	}
}

// Fetch reads the next Kafka message without committing the offset.
func (r *Reader) Fetch(ctx context.Context) (kafka.Message, error) {
	msg, err := r.reader.FetchMessage(ctx)
	if err != nil {
		return kafka.Message{}, fmt.Errorf("kafka.Fetch: %w", err)
	}
	return msg, nil
}

// Commit persists offsets for successfully processed messages.
func (r *Reader) Commit(ctx context.Context, msgs ...kafka.Message) error {
	if len(msgs) == 0 {
		return nil
	}
	if err := r.reader.CommitMessages(ctx, msgs...); err != nil {
		return fmt.Errorf("kafka.Commit: %w", err)
	}
	return nil
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

// Writer publishes messages (DLQ).
type Writer struct {
	w *kafka.Writer
}

// NewWriter creates a kafka writer for the given topic.
func NewWriter(brokers []string, topic string) *Writer {
	return &Writer{
		w: &kafka.Writer{
			Addr:         kafka.TCP(brokers...),
			Topic:        topic,
			Balancer:     &kafka.LeastBytes{},
			RequiredAcks: kafka.RequireOne,
			Async:        false,
		},
	}
}

// Publish writes one message with key, value, and headers.
func (w *Writer) Publish(ctx context.Context, key, value []byte, headers []kafka.Header) error {
	msg := kafka.Message{Key: key, Value: value, Headers: headers}
	if err := w.w.WriteMessages(ctx, msg); err != nil {
		return fmt.Errorf("kafka.Publish: %w", err)
	}
	return nil
}

// Close shuts down the writer.
func (w *Writer) Close() error {
	return w.w.Close()
}
