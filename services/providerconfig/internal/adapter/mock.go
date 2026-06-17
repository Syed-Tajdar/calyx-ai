package adapter

import (
	"encoding/json"
	"fmt"
)

// Platform is a resolved provider adapter (Step 1b: mock only).
type Platform interface {
	ID() string
	Execute() (map[string]any, error)
}

// MockPlatform uses per-tenant config_json for deterministic variance.
type MockPlatform struct {
	Config map[string]any
}

// NewMockPlatform creates a mock adapter with tenant config.
func NewMockPlatform(config map[string]any) *MockPlatform {
	if config == nil {
		config = map[string]any{}
	}
	return &MockPlatform{Config: config}
}

// ID returns the mock provider id.
func (m *MockPlatform) ID() string {
	return "mock"
}

// Execute returns mock output including tenant config.
func (m *MockPlatform) Execute() (map[string]any, error) {
	label, _ := m.Config["label"].(string)
	variant, _ := m.Config["variant"].(string)
	return map[string]any{
		"provider": "mock",
		"label":    label,
		"variant":  variant,
		"status":   "ok",
	}, nil
}

// ConfigJSONString serializes config for logging tests.
func ConfigJSONString(cfg map[string]any) string {
	b, err := json.Marshal(cfg)
	if err != nil {
		return "{}"
	}
	return string(b)
}

// ParseConfigJSON parses cached binding config.
func ParseConfigJSON(raw []byte) (map[string]any, error) {
	if len(raw) == 0 {
		return map[string]any{}, nil
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("ParseConfigJSON: %w", err)
	}
	return out, nil
}
