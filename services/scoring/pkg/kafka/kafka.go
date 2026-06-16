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

// NewReader creates a Kafka consumer.
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

// Fetch reads the next message without committing.
func (r *Reader) Fetch(ctx context.Context) (kafka.Message, error) {
	msg, err := r.reader.FetchMessage(ctx)
	if err != nil {
		return kafka.Message{}, fmt.Errorf("kafka.Fetch: %w", err)
	}
	return msg, nil
}

// Commit persists offsets.
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

func (r *Reader) Close() error {
	return r.reader.Close()
}

// Writer publishes Kafka messages.
type Writer struct {
	writer *kafka.Writer
	broker string
}

// NewWriter creates a Kafka producer.
func NewWriter(brokers []string) *Writer {
	broker := "localhost:9092"
	if len(brokers) > 0 {
		broker = brokers[0]
	}
	return &Writer{
		broker: broker,
		writer: &kafka.Writer{
			Addr:         kafka.TCP(brokers...),
			Balancer:     &kafka.Hash{},
			RequiredAcks: kafka.RequireOne,
		},
	}
}

// Write publishes a keyed message.
func (w *Writer) Write(ctx context.Context, topic, key string, value []byte) error {
	if err := w.writer.WriteMessages(ctx, kafka.Message{
		Topic: topic,
		Key:   []byte(key),
		Value: value,
	}); err != nil {
		return fmt.Errorf("kafka.Write: %w", err)
	}
	return nil
}

func (w *Writer) Ping(ctx context.Context) error {
	dialer := &kafka.Dialer{Timeout: 3 * time.Second}
	conn, err := dialer.DialContext(ctx, "tcp", w.broker)
	if err != nil {
		return fmt.Errorf("kafka.Ping: %w", err)
	}
	defer conn.Close()
	return nil
}

func (w *Writer) Close() error {
	return w.writer.Close()
}
