package nlquery

import "context"

// Provider generates NL answers from query results.
type Provider interface {
	FormatAnswer(ctx context.Context, question, intent string, rows []map[string]any) (string, int, error)
}
