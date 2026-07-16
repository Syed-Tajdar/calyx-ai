package state

import "testing"

func TestNormalizeState(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Texas", "TX"},
		{"tx", "TX"},
		{"ZZ", ""},
		{"UK", ""},
		{"ON", "ON"},
		{"Ontario", "ON"},
		{"British Columbia", "BC"},
		{"CA", "CA"},
		{"California", "CA"},
		{"PR", "PR"},
	}
	for _, tt := range tests {
		got := NormalizeState(tt.input)
		if got != tt.expected {
			t.Errorf("NormalizeState(%q)=%q want %q", tt.input, got, tt.expected)
		}
	}
}

func TestCountryFor(t *testing.T) {
	if CountryFor("CA") != CountryUS {
		t.Fatal("CA must be California / US, not Canada")
	}
	if CountryFor("ON") != CountryCA {
		t.Fatal("ON must be Canada")
	}
	if CountryFor("XX") != CountryUnknown {
		t.Fatal("XX unknown")
	}
}
