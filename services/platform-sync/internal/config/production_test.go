package config

import (
	"strings"
	"testing"
)

func TestRefuseProductionPlaceholderNames(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	good := map[string]string{
		"CREDENTIAL_ENCRYPTION_KEY": "abababababababababababababababababababababababababababababababab",
		"ANI_ENCRYPTION_KEY":        "cdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcd",
		"WRITEBACK_INTERNAL_TOKEN":  "prod-writeback",
		"DATABASE_URL":              "postgres://callloom_sync:prodsync@postgres:5432/callloom",
		"REDIS_URL":                 "redis://:prodredis@redis:6379",
	}
	for k, v := range good {
		t.Setenv(k, v)
	}
	if err := refuseProductionPlaceholders(); err != nil {
		t.Fatalf("good production env refused: %v", err)
	}

	cases := []struct {
		name  string
		value string
	}{
		{"CREDENTIAL_ENCRYPTION_KEY", devCredKey},
		{"ANI_ENCRYPTION_KEY", devCredKey},
		{"WRITEBACK_INTERNAL_TOKEN", devWriteback},
		{"DATABASE_URL", "postgres://callloom_sync:sync_dev@postgres:5432/callloom"},
		{"REDIS_URL", "redis://:redis_dev@redis:6379"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(tc.name, tc.value)
			err := refuseProductionPlaceholders()
			if err == nil || !strings.Contains(err.Error(), tc.name) {
				t.Fatalf("expected error naming %s", tc.name)
			}
		})
	}
}
