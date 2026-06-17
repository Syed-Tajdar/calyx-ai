package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds compliance-intelligence configuration.
type Config struct {
	Port                    string
	MockMode                bool
	RedisURL                string
	ProviderConfigURL       string
	KafkaBrokers            []string
	TopicPings              string
	TopicRiskScored         string
	TopicViolationDetected  string
	TopicComplianceDLQ      string
	ConsumerGroup           string
	ClickHouseAddr          string
	ClickHouseDB            string
	ClickHouseUser          string
	ClickHousePass          string
	DNCProvider             string
	DedupTTL                time.Duration
}

// Load reads configuration from environment.
func Load() (Config, error) {
	mockMode := parseBoolDefault(os.Getenv("MOCK_MODE"), true)
	dedupHours := 24
	if v := os.Getenv("COMPLIANCE_DEDUP_TTL_HOURS"); v != "" {
		parsed, err := strconv.Atoi(v)
		if err != nil {
			return Config{}, fmt.Errorf("config.Load COMPLIANCE_DEDUP_TTL_HOURS: %w", err)
		}
		dedupHours = parsed
	}

	cfg := Config{
		Port:                   envOrDefault("COMPLIANCE_PORT", "8085"),
		MockMode:               mockMode,
		RedisURL:               envOrDefault("REDIS_URL", "redis://localhost:6379"),
		ProviderConfigURL:      envOrDefault("PROVIDERCONFIG_URL", "http://localhost:8084"),
		KafkaBrokers:           []string{envOrDefault("KAFKA_BROKER", "localhost:9092")},
		TopicPings:             envOrDefault("KAFKA_TOPIC_PINGS", "callloom.pings.v1"),
		TopicRiskScored:        envOrDefault("KAFKA_TOPIC_COMPLIANCE_RISK_SCORED", "callloom.compliance.risk_scored.v1"),
		TopicViolationDetected: envOrDefault("KAFKA_TOPIC_COMPLIANCE_VIOLATION", "callloom.compliance.violation_detected.v1"),
		TopicComplianceDLQ:     envOrDefault("KAFKA_TOPIC_COMPLIANCE_DLQ", "callloom.compliance.dlq.v1"),
		ConsumerGroup:          envOrDefault("KAFKA_COMPLIANCE_GROUP", "callloom-compliance"),
		ClickHouseAddr:         envOrDefault("CLICKHOUSE_ADDR", "clickhouse:9000"),
		ClickHouseDB:           envOrDefault("CLICKHOUSE_DB", "default"),
		ClickHouseUser:         envOrDefault("CLICKHOUSE_USER", "default"),
		ClickHousePass:         envOrDefault("CLICKHOUSE_PASSWORD", ""),
		DNCProvider:            resolveProvider("DNC_PROVIDER", mockMode, "compliancepoint"),
		DedupTTL:               time.Duration(dedupHours) * time.Hour,
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
