package catalog

import (
	"encoding/json"
	"fmt"
	"os"
)

// Provider describes a catalog entry (metadata only, no secrets).
type Provider struct {
	ID              string   `json:"id"`
	Domain          string   `json:"domain"`
	DisplayName     string   `json:"display_name"`
	RequiredSecrets []string `json:"required_secrets"`
	OptionalConfig  []string `json:"optional_config"`
	GoAdapter       string   `json:"go_adapter"`
}

// Catalog is the versioned provider catalog.
type Catalog struct {
	Version   string     `json:"version"`
	Providers []Provider `json:"providers"`
}

// Load reads catalog JSON from disk.
func Load(path string) (Catalog, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Catalog{}, fmt.Errorf("catalog.Load: read file: %w", err)
	}
	var c Catalog
	if err := json.Unmarshal(raw, &c); err != nil {
		return Catalog{}, fmt.Errorf("catalog.Load: unmarshal: %w", err)
	}
	if len(c.Providers) == 0 {
		return Catalog{}, fmt.Errorf("catalog.Load: empty providers list")
	}
	return c, nil
}

// ValidateAgainstRegistry ensures every catalog go_adapter is registered in code.
func ValidateAgainstRegistry(c Catalog, registered map[string]bool) error {
	for _, p := range c.Providers {
		if p.GoAdapter == "" {
			return fmt.Errorf("catalog.Validate: provider %q missing go_adapter", p.ID)
		}
		if !registered[p.GoAdapter] {
			return fmt.Errorf("catalog.Validate: go_adapter %q for provider %q not registered in code", p.GoAdapter, p.ID)
		}
	}
	for adapter := range registered {
		found := false
		for _, p := range c.Providers {
			if p.GoAdapter == adapter {
				found = true
				break
			}
		}
		if !found {
			// warning only — staged rollout
			fmt.Printf("catalog warning: code adapter %q has no catalog entry\n", adapter)
		}
	}
	return nil
}

// FindByDomain returns catalog providers for a domain.
func (c Catalog) FindByDomain(domain string) []Provider {
	out := make([]Provider, 0)
	for _, p := range c.Providers {
		if p.Domain == domain {
			out = append(out, p)
		}
	}
	return out
}
