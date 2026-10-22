package transcribe

import (
	"regexp"
	"strings"
)

// Calyx's own scrub, on EVERY transcript before it is stored (and later before it reaches OpenAI),
// whatever Deepgram redacted. Deepgram's pci/ssn entity redaction is English-only; for Spanish it can
// only redact numbers (docs/v2-intelligence-report.md §4.2). Same patterns as the scoring service's
// redactor (services/scoring/internal/pii/redactor.go), plus long spoken digit runs.

const Redacted = "[REDACTED]"

var (
	// SSN: 123-45-6789, 123 45 6789, 123456789 as one token.
	ssnRE = regexp.MustCompile(`\b\d{3}[- ]\d{2}[- ]\d{4}\b|\b\d{9}\b`)
	// A run of digits with optional spaces/dashes/dots, at least 9 digits long (cards, SSNs read in
	// groups, account numbers). Checked digit by digit below.
	digitRunRE = regexp.MustCompile(`\d(?:[\d\s.\-]{7,}\d)`)
)

// Scrub returns the text with sensitive numbers replaced by [REDACTED], and how many it replaced.
func Scrub(text string) (string, int) {
	n := 0
	out := ssnRE.ReplaceAllStringFunc(text, func(string) string { n++; return Redacted })
	out = digitRunRE.ReplaceAllStringFunc(out, func(m string) string {
		digits := 0
		for _, r := range m {
			if r >= '0' && r <= '9' {
				digits++
			}
		}
		if digits < 9 {
			return m // a time, a price, a short count: not sensitive
		}
		n++
		return Redacted
	})
	return out, n
}

// ScrubUtterances scrubs every utterance in place and returns the total count.
func ScrubUtterances(us []Utterance) int {
	total := 0
	for i := range us {
		t, n := Scrub(us[i].Text)
		us[i].Text = t
		total += n
	}
	return total
}

// IsSpanishCampaign: campaigns whose name says Spanish/Español get language=multi + numbers redaction.
func IsSpanishCampaign(name string, pattern *regexp.Regexp) bool {
	if pattern == nil {
		pattern = DefaultSpanishPattern
	}
	return pattern.MatchString(strings.ToLower(name))
}

var DefaultSpanishPattern = regexp.MustCompile(`spanish|español|espanol`)
