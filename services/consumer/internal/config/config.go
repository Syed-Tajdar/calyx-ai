package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config holds consumer service configuration.
type Config struct {
	Port             string
	KafkaBrokers     []string
	KafkaTopic       string
	KafkaDLQTopic    string
	ConsumerGroup    string
	ClickHouseAddr   string
	ClickHouseDB     string
	ClickHouseUser   string
	ClickHousePass   string
	BatchSize        int
	BatchFlushMillis int
}

// Load reads configuration from environment variables.
func Load() (Config, error) {
	batchSize := 500
	if v := os.Getenv("BATCH_SIZE"); v != "" {
		parsed, err := strconv.Atoi(v)
		if err != nil {
			return Config{}, fmt.Errorf("config.Load: BATCH_SIZE: %w", err)
		}
		batchSize = parsed
	}

	flushMs := 5000
	if v := os.Getenv("BATCH_FLUSH_MS"); v != "" {
		parsed, err := strconv.Atoi(v)
		if err != nil {
			return Config{}, fmt.Errorf("config.Load: BATCH_FLUSH_MS: %w", err)
		}
		flushMs = parsed
	}

	chAddr := envOrDefault("CLICKHOUSE_ADDR", "clickhouse:9000")
	chDB := envOrDefault("CLICKHOUSE_DB", "default")
	chUser := envOrDefault("CLICKHOUSE_USER", "default")
	chPass := envOrDefault("CLICKHOUSE_PASSWORD", "")

	return Config{
		Port:             envOrDefault("CONSUMER_PORT", "8081"),
		KafkaBrokers:     []string{envOrDefault("KAFKA_BROKER", "localhost:9092")},
		KafkaTopic:       envOrDefault("KAFKA_TOPIC_PINGS", "callloom.pings.v1"),
		KafkaDLQTopic:    envOrDefault("KAFKA_TOPIC_PINGS_DLQ", "callloom.pings.v1.dlq"),
		ConsumerGroup:    envOrDefault("KAFKA_CONSUMER_GROUP", "callloom-consumer"),
		ClickHouseAddr:   chAddr,
		ClickHouseDB:     chDB,
		ClickHouseUser:   chUser,
		ClickHousePass:   chPass,
		BatchSize:        batchSize,
		BatchFlushMillis: flushMs,
	}, nil
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// FlushInterval returns the batch flush duration.
func (c Config) FlushInterval() time.Duration {
	return time.Duration(c.BatchFlushMillis) * time.Millisecond
}
