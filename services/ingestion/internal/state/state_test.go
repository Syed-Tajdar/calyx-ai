package state

import "testing"

func TestNormalizeState(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Texas", "TX"},
		{"texas", "TX"},
		{"tx", "TX"},
		{"TX", "TX"},
		{"ZZ", ""},
		{"", ""},
		{"  ", ""},
		{"Puerto Rico", "PR"},
		{"puerto rico", "PR"},
		{"PR", "PR"},
		{"California", "CA"},
		{"New York", "NY"},
		{"District of Columbia", "DC"},
		{"Narnia", ""},
		{"  tx  ", "TX"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := NormalizeState(tt.input)
			if got != tt.expected {
				t.Errorf("NormalizeState(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}
