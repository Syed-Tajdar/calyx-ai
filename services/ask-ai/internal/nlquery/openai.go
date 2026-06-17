package nlquery

import (
	"context"
	"errors"

	"github.com/Syed-Tajdar/calyx-ai/services/ask-ai/internal/models"
)

// OpenAIProvider is a placeholder for real OpenAI integration.
type OpenAIProvider struct {
	apiKey string
}

// NewOpenAIProvider creates an OpenAI formatter.
func NewOpenAIProvider(apiKey string) *OpenAIProvider {
	return &OpenAIProvider{apiKey: apiKey}
}

// FormatAnswer uses GPT to format rows (not implemented in Step 3 skeleton).
func (o *OpenAIProvider) FormatAnswer(_ context.Context, _ string, _ string, _ []map[string]any) (string, int, error) {
	return "", 0, errors.New("openai provider not implemented in phase 3 step 3 skeleton")
}

// Ensure OpenAIProvider satisfies Provider when wired later.
var _ Provider = (*OpenAIProvider)(nil)

// FormatFromResult is a helper for future OpenAI wiring.
func FormatFromResult(_ models.QueryResult) string {
	return ""
}
