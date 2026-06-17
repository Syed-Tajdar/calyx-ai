package dnc

import (
	"context"
	"hash/fnv"
)

// Result is a DNC lookup outcome.
type Result struct {
	Matched  bool
	ListName string
}

// Provider checks numbers against DNC lists.
type Provider interface {
	CheckNumber(ctx context.Context, phoneHash string) (Result, error)
}

// MockProvider returns deterministic ~1% DNC matches from hash.
type MockProvider struct{}

// NewMockProvider creates a mock DNC provider.
func NewMockProvider() *MockProvider {
	return &MockProvider{}
}

// CheckNumber flags ~1% of hashes for realistic mock behavior.
func (m *MockProvider) CheckNumber(_ context.Context, phoneHash string) (Result, error) {
	h := fnv.New32a()
	_, _ = h.Write([]byte(phoneHash))
	matched := h.Sum32()%100 == 0
	list := "mock_clean"
	if matched {
		list = "mock_federal"
	}
	return Result{Matched: matched, ListName: list}, nil
}

// Registry selects DNC provider by name.
func Registry(name string) Provider {
	if name == "mock" || name == "" {
		return NewMockProvider()
	}
	// Real providers (compliancepoint, dnc_com) added in later steps.
	return NewMockProvider()
}
