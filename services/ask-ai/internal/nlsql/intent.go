package nlsql

import "strings"

// Intent classifies a natural language question.
type Intent string

const (
	IntentTotalPings         Intent = "total_pings"
	IntentTotalRevenue       Intent = "total_revenue"
	IntentTopPublisher       Intent = "top_publisher"
	IntentTopState           Intent = "top_state"
	IntentComplianceExposure Intent = "compliance_exposure"
	IntentUnknown            Intent = "unknown"
)

// ParseIntent classifies question type from keywords (mock-safe classifier).
func ParseIntent(question string) Intent {
	q := strings.ToLower(strings.TrimSpace(question))
	switch {
	case strings.Contains(q, "exposure") || strings.Contains(q, "tcpa") || strings.Contains(q, "compliance risk"):
		return IntentComplianceExposure
	case strings.Contains(q, "publisher") && (strings.Contains(q, "top") || strings.Contains(q, "highest") || strings.Contains(q, "best") || strings.Contains(q, "cpa")):
		return IntentTopPublisher
	case strings.Contains(q, "state") && (strings.Contains(q, "top") || strings.Contains(q, "most") || strings.Contains(q, "highest")):
		return IntentTopState
	case strings.Contains(q, "revenue") || strings.Contains(q, "rpc"):
		return IntentTotalRevenue
	case strings.Contains(q, "ping") || strings.Contains(q, "call volume") || strings.Contains(q, "how many"):
		return IntentTotalPings
	default:
		return IntentUnknown
	}
}
