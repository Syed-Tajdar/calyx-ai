package registry

import (
	"fmt"

	"github.com/Syed-Tajdar/calyx-ai/services/providerconfig/internal/adapter"
)

// RegisteredAdapters lists code adapters available at runtime.
func RegisteredAdapters() map[string]bool {
	return map[string]bool{
		"mock": true,
	}
}

// NewPlatform constructs a platform adapter by go_adapter name.
func NewPlatform(goAdapter, providerID string, config map[string]any, _ map[string]string) (adapter.Platform, error) {
	switch goAdapter {
	case "mock":
		return adapter.NewMockPlatform(config), nil
	default:
		return nil, fmt.Errorf("registry.NewPlatform: unknown go_adapter %q", goAdapter)
	}
}

// GoAdapterForProvider maps a catalog provider id to go_adapter (Step 1b: always mock).
func GoAdapterForProvider(providerID string) string {
	if providerID == "mock" {
		return "mock"
	}
	return providerID
}
