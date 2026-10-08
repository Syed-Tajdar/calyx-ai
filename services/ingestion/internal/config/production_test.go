package config

import (
	"strings"
	"testing"
)

func TestRefuseProductionPlaceholderNames(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("ANI_ENCRYPTION_KEY", "abababababababababababababababababababababababababababababababab")
	t.Setenv("DATABASE_URL", "postgres://callloom_ingestion:prodingest@postgres:5432/callloom")
	t.Setenv("REDIS_URL", "redis://:prodredis@redis:6379")
	if err := refuseProductionPlaceholders(); err != nil {
		t.Fatalf("good production env refused: %v", err)
	}

	cases := []struct {
		name  string
		value string
	}{
		{"ANI_ENCRYPTION_KEY", devANIKey},
		{"DATABASE_URL", "postgres://callloom_ingestion:ingestion_dev@postgres:5432/callloom"},
		{"REDIS_URL", "redis://redis:6379"},
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
