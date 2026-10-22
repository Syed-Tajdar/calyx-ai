package transcribe

import (
	"strings"
	"testing"
)

func TestScrubRemovesCardsAndSSNs(t *testing.T) {
	cases := map[string]string{
		"my card is 4111 1111 1111 1111 thanks":    "my card is [REDACTED] thanks",
		"card 4111-1111-1111-1111":                 "card [REDACTED]",
		"social is 123-45-6789.":                   "social is [REDACTED].",
		"ssn 123456789":                            "ssn [REDACTED]",
		"Mi número de seguro es 5 5 5 1 2 3 4 5 6": "Mi número de seguro es [REDACTED]",
		"cuenta 0012.3456.7890":                    "cuenta [REDACTED]",
	}
	for in, want := range cases {
		if got, n := Scrub(in); got != want || n != 1 {
			t.Errorf("Scrub(%q) = %q (%d), want %q", in, got, n, want)
		}
	}
}

func TestScrubKeepsOrdinaryNumbers(t *testing.T) {
	for _, s := range []string{
		"the plan is $49.99 a month", "call me at 10:30", "I am 67 years old", "3 days ago", "zip 78701",
		"two policies, 2023 and 2024",
	} {
		if got, n := Scrub(s); got != s || n != 0 {
			t.Errorf("Scrub(%q) changed it to %q", s, got)
		}
	}
}

func TestScrubUtterances(t *testing.T) {
	us := []Utterance{{Text: "card 4111 1111 1111 1111"}, {Text: "hello"}, {Text: "ssn 123-45-6789"}}
	if n := ScrubUtterances(us); n != 2 || strings.Contains(us[0].Text+us[2].Text, "4111") || strings.Contains(us[2].Text, "6789") {
		t.Fatalf("n=%d %+v", n, us)
	}
}

func TestSpanishCampaigns(t *testing.T) {
	for name, want := range map[string]bool{"ACA Spanish": true, "Debt Español": true, "debt espanol": true, "Final Expense": false, "SSDI": false} {
		if IsSpanishCampaign(name, nil) != want {
			t.Errorf("%q: want %v", name, want)
		}
	}
}
