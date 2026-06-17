package kafka

import (
	"context"
	"fmt"
	"time"

	"github.com/segmentio/kafka-go"
)

// Reader wraps Kafka consumer.
type Reader struct {
	reader *kafka.Reader
}

// NewReader creates a consumer group reader.
func NewReader(brokers []string, topic, group string) *Reader {
	return &Reader{
		reader: kafka.NewReader(kafka.ReaderConfig{
			Brokers: brokers, Topic: topic, GroupID: group,
			MinBytes: 1, MaxBytes: 10e6, CommitInterval: 0, StartOffset: kafka.FirstOffset,
		}),
	}
}

func (r *Reader) Fetch(ctx context.Context) (kafka.Message, error) {
	msg, err := r.reader.FetchMessage(ctx)
	if err != nil {
		return kafka.Message{}, fmt.Errorf("kafka.Fetch: %w", err)
	}
	return msg, nil
}

func (r *Reader) Commit(ctx context.Context, msgs ...kafka.Message) error {
	if len(msgs) == 0 {
		return nil
	}
	return r.reader.CommitMessages(ctx, msgs...)
}

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

// Writer publishes compliance events.
type Writer struct {
	writer *kafka.Writer
}

// NewWriter creates an async Kafka writer.
func NewWriter(brokers []string, topic string) *Writer {
	return &Writer{
		writer: &kafka.Writer{
			Addr:     kafka.TCP(brokers...),
			Topic:    topic,
			Balancer: &kafka.LeastBytes{},
		},
	}
}

func (w *Writer) Publish(ctx context.Context, key string, value []byte) error {
	err := w.writer.WriteMessages(ctx, kafka.Message{Key: []byte(key), Value: value})
	if err != nil {
		return fmt.Errorf("kafka.Publish: %w", err)
	}
	return nil
}

func (w *Writer) Close() error {
	return w.writer.Close()
}
