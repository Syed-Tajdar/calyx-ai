package config

import (
	"strings"
	"testing"
)

func TestRefuseProductionPlaceholderNames(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("CLICKHOUSE_PASSWORD", "prod-ch")
	if err := refuseProductionPlaceholders(); err != nil {
		t.Fatalf("good production env refused: %v", err)
	}
	t.Setenv("CLICKHOUSE_PASSWORD", "callloom_dev")
	err := refuseProductionPlaceholders()
	if err == nil || !strings.Contains(err.Error(), "CLICKHOUSE_PASSWORD") {
		t.Fatalf("expected error naming CLICKHOUSE_PASSWORD")
	}
}
