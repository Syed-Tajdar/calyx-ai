package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Port                    string
	DatabaseURL             string
	RedisURL                string
	KafkaBrokers            []string
	KafkaTopic              string
	CredentialEncryptionKey string
	ANIEncryptionKey        []byte
	SyncInterval            time.Duration
	DedupTTL                time.Duration
	MockMode                bool
	WritebackEnabled        bool
	WritebackAuthToken      string
}

func Load() (Config, error) {
	if err := refuseProductionPlaceholders(); err != nil {
		return Config{}, err
	}
	port := envOrDefault("PLATFORM_SYNC_PORT", "8088")
	databaseURL := envOrDefault("DATABASE_URL", "postgres://callloom_sync:sync_dev@localhost:5432/callloom?sslmode=disable")
	redisURL := envOrDefault("REDIS_URL", "redis://localhost:6379")
	kafkaBroker := envOrDefault("KAFKA_BROKER", "localhost:9092")
	kafkaTopic := envOrDefault("KAFKA_TOPIC_PINGS", "callloom.pings.v1")
	credKey := os.Getenv("CREDENTIAL_ENCRYPTION_KEY")
	if credKey == "" {
		return Config{}, fmt.Errorf("CREDENTIAL_ENCRYPTION_KEY is required")
	}
	aniHex := os.Getenv("ANI_ENCRYPTION_KEY")
	if aniHex == "" {
		return Config{}, fmt.Errorf("ANI_ENCRYPTION_KEY is required")
	}
	aniKey, err := decodeHexKey(aniHex)
	if err != nil {
		return Config{}, fmt.Errorf("ANI_ENCRYPTION_KEY: %w", err)
	}
	intervalSec := 60
	if v := os.Getenv("SYNC_INTERVAL_SEC"); v != "" {
		parsed, parseErr := strconv.Atoi(v)
		if parseErr != nil {
			return Config{}, fmt.Errorf("SYNC_INTERVAL_SEC: %w", parseErr)
		}
		intervalSec = parsed
	}
	mockMode := true
	if v := os.Getenv("MOCK_MODE"); v == "false" || v == "0" {
		mockMode = false
	}
	dedupHours := 240
	if v := os.Getenv("DEDUP_TTL_HOURS"); v != "" {
		parsed, parseErr := strconv.Atoi(v)
		if parseErr != nil {
			return Config{}, fmt.Errorf("DEDUP_TTL_HOURS: %w", parseErr)
		}
		dedupHours = parsed
	}
	writebackEnabled := false
	if v := os.Getenv("WRITEBACK_ENABLED"); v == "true" || v == "1" {
		writebackEnabled = true
	}
	return Config{
		Port:                    port,
		DatabaseURL:             databaseURL,
		RedisURL:                redisURL,
		KafkaBrokers:            []string{kafkaBroker},
		KafkaTopic:              kafkaTopic,
		CredentialEncryptionKey: credKey,
		ANIEncryptionKey:        aniKey,
		SyncInterval:            time.Duration(intervalSec) * time.Second,
		DedupTTL:                time.Duration(dedupHours) * time.Hour,
		MockMode:                mockMode,
		WritebackEnabled:        writebackEnabled,
		WritebackAuthToken:      os.Getenv("WRITEBACK_INTERNAL_TOKEN"),
	}, nil
}

const (
	devCredKey    = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	devWriteback  = "dev_writeback_internal_token_change_me"
)

func refuseProductionPlaceholders() error {
	if os.Getenv("APP_ENV") != "production" {
		return nil
	}
	var bad []string
	if v := os.Getenv("CREDENTIAL_ENCRYPTION_KEY"); v == "" || v == devCredKey {
		bad = append(bad, "CREDENTIAL_ENCRYPTION_KEY")
	}
	if v := os.Getenv("ANI_ENCRYPTION_KEY"); v == "" || v == devCredKey {
		bad = append(bad, "ANI_ENCRYPTION_KEY")
	}
	if v := os.Getenv("WRITEBACK_INTERNAL_TOKEN"); v == "" || v == devWriteback {
		bad = append(bad, "WRITEBACK_INTERNAL_TOKEN")
	}
	if v := os.Getenv("DATABASE_URL"); v == "" || strings.Contains(v, "sync_dev") || strings.Contains(v, "callloom_dev") {
		bad = append(bad, "DATABASE_URL")
	}
	if len(bad) > 0 {
		return fmt.Errorf("APP_ENV=production refuses to start with placeholder/missing secrets: %v", bad)
	}
	return nil
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func decodeHexKey(hexKey string) ([]byte, error) {
	if len(hexKey) != 64 {
		return nil, fmt.Errorf("expected 64 hex chars, got %d", len(hexKey))
	}
	key := make([]byte, 32)
	for i := 0; i < 32; i++ {
		var b byte
		if _, err := fmt.Sscanf(hexKey[i*2:i*2+2], "%02x", &b); err != nil {
			return nil, err
		}
		key[i] = b
	}
	return key, nil
}
