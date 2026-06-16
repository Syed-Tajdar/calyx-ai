package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds enrichment service configuration.
type Config struct {
	Port                 string
	MockMode             bool
	RedisURL             string
	KafkaBrokers         []string
	TopicPings           string
	TopicEnrichmentDLQ   string
	ConsumerGroup        string
	ClickHouseAddr       string
	ClickHouseDB         string
	ClickHouseUser       string
	ClickHousePass       string
	EnrichmentProvider   string
	EnrollHereAPIKey     string
	WhitepagesAPIKey     string
	CacheTTL             time.Duration
}

// Load reads and validates configuration.
func Load() (Config, error) {
	mockMode := parseBoolDefault(os.Getenv("MOCK_MODE"), true)
	ttlHours := 24
	if v := os.Getenv("ENRICHMENT_CACHE_TTL_HOURS"); v != "" {
		parsed, err := strconv.Atoi(v)
		if err != nil {
			return Config{}, fmt.Errorf("config.Load ENRICHMENT_CACHE_TTL_HOURS: %w", err)
		}
		ttlHours = parsed
	}

	cfg := Config{
		Port:               envOrDefault("ENRICHMENT_PORT", "8083"),
		MockMode:           mockMode,
		RedisURL:           envOrDefault("REDIS_URL", "redis://localhost:6379"),
		KafkaBrokers:       []string{envOrDefault("KAFKA_BROKER", "localhost:9092")},
		TopicPings:         envOrDefault("KAFKA_TOPIC_PINGS", "callloom.pings.v1"),
		TopicEnrichmentDLQ: envOrDefault("KAFKA_TOPIC_ENRICHMENT_DLQ", "callloom.enrichment.dlq.v1"),
		ConsumerGroup:      envOrDefault("KAFKA_ENRICHMENT_GROUP", "callloom-enrichment"),
		ClickHouseAddr:     envOrDefault("CLICKHOUSE_ADDR", "clickhouse:9000"),
		ClickHouseDB:       envOrDefault("CLICKHOUSE_DB", "default"),
		ClickHouseUser:     envOrDefault("CLICKHOUSE_USER", "default"),
		ClickHousePass:     envOrDefault("CLICKHOUSE_PASSWORD", ""),
		EnrichmentProvider: resolveProvider("ENRICHMENT_PROVIDER", mockMode, "enrollhere"),
		EnrollHereAPIKey:   os.Getenv("ENROLLHERE_API_KEY"),
		WhitepagesAPIKey:   os.Getenv("WHITEPAGES_API_KEY"),
		CacheTTL:           time.Duration(ttlHours) * time.Hour,
	}

	if err := cfg.validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (c Config) validate() error {
	if c.MockMode {
		return nil
	}
	if c.EnrichmentProvider == "enrollhere" && c.EnrollHereAPIKey == "" {
		return fmt.Errorf("config.validate: ENROLLHERE_API_KEY required when MOCK_MODE=false and ENRICHMENT_PROVIDER=enrollhere")
	}
	if c.EnrichmentProvider == "whitepages" && c.WhitepagesAPIKey == "" {
		return fmt.Errorf("config.validate: WHITEPAGES_API_KEY required when MOCK_MODE=false and ENRICHMENT_PROVIDER=whitepages")
	}
	return nil
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
