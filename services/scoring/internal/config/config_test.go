package config_test

import (
	"os"
	"testing"

	"github.com/Syed-Tajdar/call-loom-intelligence/services/scoring/internal/config"
)

func TestLoadMockModeDefault(t *testing.T) {
	os.Unsetenv("MOCK_MODE")
	os.Setenv("REDIS_URL", "redis://localhost:6379")
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if !cfg.MockMode {
		t.Fatal("expected MOCK_MODE default true")
	}
	if cfg.AIProvider != "mock" {
		t.Fatalf("expected mock AI provider, got %s", cfg.AIProvider)
	}
}

func TestLoadRealModeRequiresSecrets(t *testing.T) {
	os.Setenv("MOCK_MODE", "false")
	os.Setenv("TRANSCRIPTION_PROVIDER", "deepgram")
	os.Unsetenv("DEEPGRAM_API_KEY")
	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error when MOCK_MODE=false without DEEPGRAM_API_KEY")
	}
	os.Setenv("MOCK_MODE", "true")
}
