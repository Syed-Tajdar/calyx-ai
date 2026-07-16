package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config holds ingestion service configuration loaded from environment variables.
type Config struct {
	Port            string
	RedisURL        string
	KafkaBrokers    []string
	KafkaTopic      string
	DatabaseURL     string
	RequestTimeout  time.Duration
	DedupTTL        time.Duration
	MaxBodyBytes    int64
	EncryptionKey   []byte
}

// Load reads and validates configuration from the environment.
func Load() (Config, error) {
	port := envOrDefault("INGESTION_PORT", "8080")
	redisURL := envOrDefault("REDIS_URL", "redis://localhost:6379")
	kafkaBroker := envOrDefault("KAFKA_BROKER", "localhost:9092")
	kafkaTopic := envOrDefault("KAFKA_TOPIC_PINGS", "callloom.pings.v1")
	databaseURL := envOrDefault("DATABASE_URL", "postgres://callloom:callloom_dev@localhost:5432/callloom?sslmode=disable")

	keyHex := os.Getenv("ANI_ENCRYPTION_KEY")
	if keyHex == "" {
		return Config{}, fmt.Errorf("config.Load: ANI_ENCRYPTION_KEY is required (32-byte hex)")
	}
	key, err := decodeHexKey(keyHex)
	if err != nil {
		return Config{}, fmt.Errorf("config.Load: ANI_ENCRYPTION_KEY: %w", err)
	}

	maxBody := int64(1 << 20)
	if v := os.Getenv("MAX_BODY_BYTES"); v != "" {
		parsed, parseErr := strconv.ParseInt(v, 10, 64)
		if parseErr != nil {
			return Config{}, fmt.Errorf("config.Load: MAX_BODY_BYTES: %w", parseErr)
		}
		maxBody = parsed
	}

	dedupHours := 168 // align with platform-sync (7d content-hash window)
	if v := os.Getenv("DEDUP_TTL_HOURS"); v != "" {
		parsed, parseErr := strconv.Atoi(v)
		if parseErr != nil {
			return Config{}, fmt.Errorf("config.Load: DEDUP_TTL_HOURS: %w", parseErr)
		}
		dedupHours = parsed
	}

	requestTimeoutSec := 10
	if v := os.Getenv("INGESTION_REQUEST_TIMEOUT_SEC"); v != "" {
		parsed, parseErr := strconv.Atoi(v)
		if parseErr != nil {
			return Config{}, fmt.Errorf("config.Load: INGESTION_REQUEST_TIMEOUT_SEC: %w", parseErr)
		}
		requestTimeoutSec = parsed
	}

	return Config{
		Port:           port,
		RedisURL:       redisURL,
		KafkaBrokers:   []string{kafkaBroker},
		KafkaTopic:     kafkaTopic,
		DatabaseURL:    databaseURL,
		RequestTimeout: time.Duration(requestTimeoutSec) * time.Second,
		DedupTTL:       time.Duration(dedupHours) * time.Hour,
		MaxBodyBytes:   maxBody,
		EncryptionKey:  key,
	}, nil
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func decodeHexKey(hexKey string) ([]byte, error) {
	if len(hexKey) != 64 {
		return nil, fmt.Errorf("expected 64 hex chars (32 bytes), got %d", len(hexKey))
	}
	key := make([]byte, 32)
	for i := 0; i < 32; i++ {
		var b byte
		_, err := fmt.Sscanf(hexKey[i*2:i*2+2], "%02x", &b)
		if err != nil {
			return nil, fmt.Errorf("invalid hex at position %d: %w", i, err)
		}
		key[i] = b
	}
	return key, nil
}
