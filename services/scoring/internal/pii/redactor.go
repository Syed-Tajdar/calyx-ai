package pii

import "regexp"

var (
	ssnPattern  = regexp.MustCompile(`\b\d{3}-\d{2}-\d{4}\b`)
	cardPattern = regexp.MustCompile(`\b(?:\d[ -]*?){13,16}\b`)
)

// Redact removes SSN and credit-card patterns from transcript text.
func Redact(text string) string {
	out := ssnPattern.ReplaceAllString(text, "[REDACTED_SSN]")
	out = cardPattern.ReplaceAllString(out, "[REDACTED_CARD]")
	return out
}
