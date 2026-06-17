package nlquery

import (
	"context"
	"fmt"
	"strings"
)

// MockProvider returns template-based natural language answers.
type MockProvider struct{}

// NewMockProvider creates a mock LLM formatter.
func NewMockProvider() *MockProvider {
	return &MockProvider{}
}

// FormatAnswer turns rows into a readable sentence.
func (m *MockProvider) FormatAnswer(_ context.Context, question, intent string, rows []map[string]any) (string, int, error) {
	switch intent {
	case "total_pings":
		if len(rows) == 0 {
			return "No ping data is available yet for your account.", 12, nil
		}
		return fmt.Sprintf("You have %v total pings in the system.", rows[0]["total_pings"]), 18, nil
	case "total_revenue":
		if len(rows) == 0 {
			return "No revenue data is available yet.", 10, nil
		}
		return fmt.Sprintf("Total revenue across all pings is $%v.", rows[0]["total_revenue"]), 20, nil
	case "top_publisher":
		if len(rows) == 0 {
			return "No publisher revenue data found yet.", 12, nil
		}
		pub := rows[0]["publisher_id"]
		rev := rows[0]["revenue"]
		if pub == "" || pub == nil {
			return fmt.Sprintf("Top revenue is $%v from an unnamed publisher.", rev), 22, nil
		}
		return fmt.Sprintf("Publisher %v leads with $%v in revenue.", pub, rev), 24, nil
	case "top_state":
		if len(rows) == 0 {
			return "No state-level ping data found yet.", 12, nil
		}
		return fmt.Sprintf("State %v has the most pings (%v).", rows[0]["state"], rows[0]["ping_count"]), 22, nil
	case "compliance_exposure":
		if len(rows) == 0 {
			return "No compliance exposure data is available yet.", 14, nil
		}
		return fmt.Sprintf("Estimated TCPA exposure is $%v across %v high-risk calls.",
			rows[0]["total_exposure_usd"], rows[0]["violation_count"]), 28, nil
	default:
		q := strings.ToLower(strings.TrimSpace(question))
		if strings.Contains(q, "cpa") || strings.Contains(q, "publisher") {
			return "In mock mode, try asking about total pings, total revenue, top publisher, top state, or compliance exposure.", 30, nil
		}
		return "I can answer questions about pings, revenue, publishers, states, and compliance exposure. Try: \"What is my total revenue?\"", 32, nil
	}
}

// Registry selects an LLM provider by name.
func Registry(name, apiKey string) Provider {
	if name == "mock" || name == "" {
		return NewMockProvider()
	}
	return NewOpenAIProvider(apiKey)
}
