package state

import "strings"

// Country for a known North America region code.
type Country string

const (
	CountryUS Country = "US"
	CountryCA Country = "CA"
	CountryUnknown Country = ""
)

// US states (50) + DC + territories that appear in pay-per-call geo.
var usCodes = map[string]struct{}{
	"AL": {}, "AK": {}, "AZ": {}, "AR": {}, "CA": {}, "CO": {}, "CT": {}, "DE": {},
	"FL": {}, "GA": {}, "HI": {}, "ID": {}, "IL": {}, "IN": {}, "IA": {}, "KS": {},
	"KY": {}, "LA": {}, "ME": {}, "MD": {}, "MA": {}, "MI": {}, "MN": {}, "MS": {},
	"MO": {}, "MT": {}, "NE": {}, "NV": {}, "NH": {}, "NJ": {}, "NM": {}, "NY": {},
	"NC": {}, "ND": {}, "OH": {}, "OK": {}, "OR": {}, "PA": {}, "RI": {}, "SC": {},
	"SD": {}, "TN": {}, "TX": {}, "UT": {}, "VT": {}, "VA": {}, "WA": {}, "WV": {},
	"WI": {}, "WY": {}, "DC": {},
	"PR": {}, "GU": {}, "VI": {}, "AS": {}, "MP": {},
}

// Canadian provinces and territories.
var caCodes = map[string]struct{}{
	"ON": {}, "QC": {}, "BC": {}, "AB": {}, "MB": {}, "SK": {}, "NS": {}, "NB": {},
	"NL": {}, "PE": {}, "NT": {}, "YT": {}, "NU": {},
}

// nameToCode maps full region names (lowercase) → 2-letter codes (US + Canada).
// Note: "CA" always means California (US). Canada is never bare "CA" — use ON/BC/….
var nameToCode = map[string]string{
	"alabama": "AL", "alaska": "AK", "arizona": "AZ", "arkansas": "AR",
	"california": "CA", "colorado": "CO", "connecticut": "CT", "delaware": "DE",
	"florida": "FL", "georgia": "GA", "hawaii": "HI", "idaho": "ID",
	"illinois": "IL", "indiana": "IN", "iowa": "IA", "kansas": "KS",
	"kentucky": "KY", "louisiana": "LA", "maine": "ME", "maryland": "MD",
	"massachusetts": "MA", "michigan": "MI", "minnesota": "MN", "mississippi": "MS",
	"missouri": "MO", "montana": "MT", "nebraska": "NE", "nevada": "NV",
	"new hampshire": "NH", "new jersey": "NJ", "new mexico": "NM", "new york": "NY",
	"north carolina": "NC", "north dakota": "ND", "ohio": "OH", "oklahoma": "OK",
	"oregon": "OR", "pennsylvania": "PA", "rhode island": "RI", "south carolina": "SC",
	"south dakota": "SD", "tennessee": "TN", "texas": "TX", "utah": "UT",
	"vermont": "VT", "virginia": "VA", "washington": "WA", "west virginia": "WV",
	"wisconsin": "WI", "wyoming": "WY",
	"district of columbia": "DC", "washington dc": "DC", "washington d.c.": "DC",
	"puerto rico": "PR", "guam": "GU", "u.s. virgin islands": "VI", "virgin islands": "VI",
	"american samoa": "AS", "northern mariana islands": "MP",
	// Canada
	"ontario": "ON", "quebec": "QC", "québec": "QC",
	"british columbia": "BC", "alberta": "AB", "manitoba": "MB", "saskatchewan": "SK",
	"nova scotia": "NS", "new brunswick": "NB", "newfoundland and labrador": "NL",
	"newfoundland": "NL", "prince edward island": "PE",
	"northwest territories": "NT", "yukon": "YT", "yukon territory": "YT", "nunavut": "NU",
}

// NormalizeState coerces geo input to a known US or Canadian 2-letter region code.
// Unknown values (junk, non-NA, invalid) return "". Never fails ingestion on bad geo.
// "CA" is California (US). Canadian volume uses province codes (ON, BC, …) only.
func NormalizeState(s string) string {
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		return ""
	}

	upper := strings.ToUpper(trimmed)
	if len(upper) == 2 {
		if _, ok := usCodes[upper]; ok {
			return upper
		}
		if _, ok := caCodes[upper]; ok {
			return upper
		}
		return ""
	}

	if code, ok := nameToCode[strings.ToLower(trimmed)]; ok {
		return code
	}

	return ""
}

// CountryFor returns US or CA for a normalized region code, else "".
func CountryFor(code string) Country {
	c := strings.ToUpper(strings.TrimSpace(code))
	if _, ok := usCodes[c]; ok {
		return CountryUS
	}
	if _, ok := caCodes[c]; ok {
		return CountryCA
	}
	return CountryUnknown
}

// IsKnownRegion reports whether code is in the US+CA allowlist.
func IsKnownRegion(code string) bool {
	return CountryFor(code) != CountryUnknown
}
