package state

import "strings"

// validCodes is the set of recognized 2-letter US state/territory codes.
var validCodes = map[string]struct{}{
	"AL": {}, "AK": {}, "AZ": {}, "AR": {}, "CA": {}, "CO": {}, "CT": {}, "DE": {},
	"FL": {}, "GA": {}, "HI": {}, "ID": {}, "IL": {}, "IN": {}, "IA": {}, "KS": {},
	"KY": {}, "LA": {}, "ME": {}, "MD": {}, "MA": {}, "MI": {}, "MN": {}, "MS": {},
	"MO": {}, "MT": {}, "NE": {}, "NV": {}, "NH": {}, "NJ": {}, "NM": {}, "NY": {},
	"NC": {}, "ND": {}, "OH": {}, "OK": {}, "OR": {}, "PA": {}, "RI": {}, "SC": {},
	"SD": {}, "TN": {}, "TX": {}, "UT": {}, "VT": {}, "VA": {}, "WA": {}, "WV": {},
	"WI": {}, "WY": {}, "DC": {}, "PR": {}, "GU": {}, "VI": {},
}

// nameToCode maps full state/territory names (lowercase) to 2-letter codes.
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
}

// NormalizeState coerces an incoming state value to a 2-letter uppercase US code.
// Unknown values return "" — ingestion must never fail on bad geo data.
func NormalizeState(s string) string {
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		return ""
	}

	upper := strings.ToUpper(trimmed)
	if len(upper) == 2 {
		if _, ok := validCodes[upper]; ok {
			return upper
		}
		return ""
	}

	if code, ok := nameToCode[strings.ToLower(trimmed)]; ok {
		return code
	}

	return ""
}
