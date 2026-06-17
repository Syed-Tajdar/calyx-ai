package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Config holds ask-ai service configuration.
type Config struct {
	Port              string
	MockMode          bool
	DatabaseURL       string
	ProviderConfigURL string
	ClickHouseAddr    string
	ClickHouseDB      string
	ClickHouseUser    string
	ClickHousePass    string
	LLMProvider       string
	OpenAIAPIKey      string
	TokenBudgetDaily  int
	QueryTimeoutSec   int
	MaxRows           int
}

// Load reads configuration from environment.
func Load() (Config, error) {
	mockMode := parseBoolDefault(os.Getenv("MOCK_MODE"), true)
	budget := 100000
	if v := os.Getenv("ASK_AI_TOKEN_BUDGET_DAILY"); v != "" {
		parsed, err := strconv.Atoi(v)
		if err != nil {
			return Config{}, fmt.Errorf("config.Load ASK_AI_TOKEN_BUDGET_DAILY: %w", err)
		}
		budget = parsed
	}
	timeoutSec := 10
	if v := os.Getenv("ASK_AI_QUERY_TIMEOUT_SEC"); v != "" {
		parsed, err := strconv.Atoi(v)
		if err != nil {
			return Config{}, fmt.Errorf("config.Load ASK_AI_QUERY_TIMEOUT_SEC: %w", err)
		}
		timeoutSec = parsed
	}
	maxRows := 100
	if v := os.Getenv("ASK_AI_MAX_ROWS"); v != "" {
		parsed, err := strconv.Atoi(v)
		if err != nil {
			return Config{}, fmt.Errorf("config.Load ASK_AI_MAX_ROWS: %w", err)
		}
		maxRows = parsed
	}

	cfg := Config{
		Port:              envOrDefault("ASK_AI_PORT", "8086"),
		MockMode:          mockMode,
		DatabaseURL:       envOrDefault("DATABASE_URL", "postgres://callloom_api:api_dev@localhost:5432/callloom?sslmode=disable"),
		ProviderConfigURL: envOrDefault("PROVIDERCONFIG_URL", "http://localhost:8084"),
		ClickHouseAddr:    envOrDefault("CLICKHOUSE_ADDR", "clickhouse:9000"),
		ClickHouseDB:      envOrDefault("CLICKHOUSE_DB", "default"),
		ClickHouseUser:    envOrDefault("CLICKHOUSE_USER", "default"),
		ClickHousePass:    envOrDefault("CLICKHOUSE_PASSWORD", ""),
		LLMProvider:       resolveProvider("LLM_PROVIDER", mockMode, "openai"),
		OpenAIAPIKey:      os.Getenv("OPENAI_API_KEY"),
		TokenBudgetDaily:  budget,
		QueryTimeoutSec:   timeoutSec,
		MaxRows:           maxRows,
	}

	if !cfg.MockMode && cfg.LLMProvider == "openai" && cfg.OpenAIAPIKey == "" {
		return Config{}, fmt.Errorf("config.Load: OPENAI_API_KEY required when MOCK_MODE=false and LLM_PROVIDER=openai")
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
