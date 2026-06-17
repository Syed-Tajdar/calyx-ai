package config

import (
	"os"
	"testing"
)

func TestLoadMockModeDefault(t *testing.T) {
	os.Unsetenv("MOCK_MODE")
	os.Unsetenv("CREDENTIAL_ENCRYPTION_KEY")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.MockMode {
		t.Fatal("expected MOCK_MODE default true")
	}
}

func TestLoadRealModeRequiresKey(t *testing.T) {
	os.Setenv("MOCK_MODE", "false")
	os.Unsetenv("CREDENTIAL_ENCRYPTION_KEY")
	_, err := Load()
	if err == nil {
		t.Fatal("expected error without CREDENTIAL_ENCRYPTION_KEY")
	}
	os.Setenv("MOCK_MODE", "true")
}
