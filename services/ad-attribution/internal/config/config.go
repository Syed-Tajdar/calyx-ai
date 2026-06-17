package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds ad-attribution configuration.
type Config struct {
	Port                    string
	MockMode                bool
	MockBridgeEnabled       bool
	RedisURL                string
	ProviderConfigURL       string
	KafkaBrokers            []string
	TopicPings              string
	TopicConversions        string
	TopicAttributed         string
	TopicDLQ                string
	ConsumerGroup           string
	BridgeGroup             string
	ClickHouseAddr          string
	ClickHouseDB            string
	ClickHouseUser          string
	ClickHousePass          string
	AttributionProvider     string
	DedupTTL                time.Duration
	FireMaxRetries          int
}

// Load reads configuration from environment.
func Load() (Config, error) {
	mockMode := parseBoolDefault(os.Getenv("MOCK_MODE"), true)
	dedupHours := 24
	if v := os.Getenv("ATTRIBUTION_DEDUP_TTL_HOURS"); v != "" {
		parsed, err := strconv.Atoi(v)
		if err != nil {
			return Config{}, fmt.Errorf("config.Load ATTRIBUTION_DEDUP_TTL_HOURS: %w", err)
		}
		dedupHours = parsed
	}
	retries := 3
	if v := os.Getenv("ATTRIBUTION_FIRE_MAX_RETRIES"); v != "" {
		parsed, err := strconv.Atoi(v)
		if err != nil {
			return Config{}, fmt.Errorf("config.Load ATTRIBUTION_FIRE_MAX_RETRIES: %w", err)
		}
		retries = parsed
	}

	cfg := Config{
		Port:                envOrDefault("ATTRIBUTION_PORT", "8087"),
		MockMode:            mockMode,
		MockBridgeEnabled:   parseBoolDefault(os.Getenv("ATTRIBUTION_MOCK_BRIDGE"), mockMode),
		RedisURL:            envOrDefault("REDIS_URL", "redis://localhost:6379"),
		ProviderConfigURL:   envOrDefault("PROVIDERCONFIG_URL", "http://localhost:8084"),
		KafkaBrokers:        []string{envOrDefault("KAFKA_BROKER", "localhost:9092")},
		TopicPings:          envOrDefault("KAFKA_TOPIC_PINGS", "callloom.pings.v1"),
		TopicConversions:    envOrDefault("KAFKA_TOPIC_CONVERSIONS", "callloom.conversions.v1"),
		TopicAttributed:     envOrDefault("KAFKA_TOPIC_ATTRIBUTION_ATTRIBUTED", "callloom.attribution.attributed.v1"),
		TopicDLQ:            envOrDefault("KAFKA_TOPIC_ATTRIBUTION_DLQ", "callloom.attribution.dlq.v1"),
		ConsumerGroup:       envOrDefault("KAFKA_ATTRIBUTION_GROUP", "callloom-attribution"),
		BridgeGroup:         envOrDefault("KAFKA_ATTRIBUTION_BRIDGE_GROUP", "callloom-attribution-bridge"),
		ClickHouseAddr:      envOrDefault("CLICKHOUSE_ADDR", "clickhouse:9000"),
		ClickHouseDB:        envOrDefault("CLICKHOUSE_DB", "default"),
		ClickHouseUser:      envOrDefault("CLICKHOUSE_USER", "default"),
		ClickHousePass:      envOrDefault("CLICKHOUSE_PASSWORD", ""),
		AttributionProvider: resolveProvider("ATTRIBUTION_PROVIDER", mockMode, "meta_capi"),
		DedupTTL:            time.Duration(dedupHours) * time.Hour,
		FireMaxRetries:      retries,
	}
	return cfg, nil
}

func resolveProvider(envKey string, mockMode bool, realDefault string) string {
	if v := strings.TrimSpace(os.Getenv(envKey)); v != "" {
		return v
	}
	if mockMode {
		return "mock"
	}
	return realDefault
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func parseBoolDefault(value string, fallback bool) bool {
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}
