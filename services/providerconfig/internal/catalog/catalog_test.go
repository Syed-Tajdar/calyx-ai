package catalog

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Syed-Tajdar/calyx-ai/services/providerconfig/internal/registry"
)

func TestValidateAgainstRegistryBoot(t *testing.T) {
	path := filepath.Join("..", "..", "..", "..", "shared", "providers", "catalog.json")
	if _, err := os.Stat(path); err != nil {
		t.Skip("catalog.json not found at repo path")
	}
	cat, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateAgainstRegistry(cat, registry.RegisteredAdapters()); err != nil {
		t.Fatal(err)
	}
}
