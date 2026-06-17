package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Config holds providerconfig service settings.
type Config struct {
	Port                string
	MockMode            bool
	SingleTenantMode    bool
	RedisURL            string
	DatabaseURL         string
	CatalogPath         string
	CredentialKeyHex    string
	AttributionProvider string
	DNCProvider         string
	LLMProvider         string
}

// Load reads configuration from environment variables.
func Load() (Config, error) {
	mockMode := parseBoolDefault(os.Getenv("MOCK_MODE"), true)
	cfg := Config{
		Port:                envOrDefault("PROVIDERCONFIG_PORT", "8084"),
		MockMode:            mockMode,
		SingleTenantMode:    parseBoolDefault(os.Getenv("SINGLE_TENANT_MODE"), false),
		RedisURL:            envOrDefault("REDIS_URL", "redis://localhost:6379"),
		DatabaseURL:         envOrDefault("DATABASE_URL", "postgres://callloom_api:api_dev@localhost:5432/callloom?sslmode=disable"),
		CatalogPath:         envOrDefault("PROVIDER_CATALOG_PATH", "../../../shared/providers/catalog.json"),
		CredentialKeyHex:    os.Getenv("CREDENTIAL_ENCRYPTION_KEY"),
		AttributionProvider: envOrDefault("ATTRIBUTION_PROVIDER", "mock"),
		DNCProvider:         envOrDefault("DNC_PROVIDER", "mock"),
		LLMProvider:         envOrDefault("LLM_PROVIDER", "mock"),
	}
	if !cfg.MockMode && cfg.CredentialKeyHex == "" {
		return Config{}, fmt.Errorf("config.Load: CREDENTIAL_ENCRYPTION_KEY required when MOCK_MODE=false")
	}
	return cfg, nil
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

// EnvProviderForDomain returns env fallback provider for a domain when SINGLE_TENANT_MODE is on.
func (c Config) EnvProviderForDomain(domain string) string {
	switch strings.ToLower(domain) {
	case "attribution":
		return c.AttributionProvider
	case "dnc":
		return c.DNCProvider
	case "llm":
		return c.LLMProvider
	default:
		return ""
	}
}
