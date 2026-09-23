package providers

import (
	"fmt"

	"github.com/Syed-Tajdar/calyx-ai/services/enrichment/internal/models"
)

// Provider enriches a phone number hash into attributes.
type Provider interface {
	Enrich(aniHash string) ([]models.Attribute, error)
}

// MockProvider returns 500+ deterministic fake attributes.
type MockProvider struct{}

// NewMockProvider creates a mock enrichment provider.
func NewMockProvider() *MockProvider {
	return &MockProvider{}
}

// Enrich generates fake attributes from ani_hash.
func (m *MockProvider) Enrich(aniHash string) ([]models.Attribute, error) {
	prefix := aniHash
	if len(prefix) > 8 {
		prefix = prefix[:8]
	}
	attrs := make([]models.Attribute, 0, 520)
	for i := 0; i < 520; i++ {
		attrs = append(attrs, models.Attribute{
			Key:   fmt.Sprintf("attr_%03d", i),
			Value: fmt.Sprintf("%s_val_%d", prefix, i%100),
		})
	}
	return attrs, nil
}

// EnrollHereProvider placeholder for real integration.
type EnrollHereProvider struct {
	apiKey string
}

// NewEnrollHereProvider creates enrollhere provider.
func NewEnrollHereProvider(apiKey string) *EnrollHereProvider {
	return &EnrollHereProvider{apiKey: apiKey}
}

func (e *EnrollHereProvider) Enrich(_ string) ([]models.Attribute, error) {
	return nil, fmt.Errorf("enrollhere provider not implemented in phase 2 skeleton")
}

// Registry selects enrichment provider.
func Registry(name, enrollKey, whitepagesKey string) Provider {
	switch name {
	case "enrollhere":
		return NewEnrollHereProvider(enrollKey)
	case "whitepages":
		return NewEnrollHereProvider(whitepagesKey)
	default:
		return NewMockProvider()
	}
}
