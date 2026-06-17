package scoring

import (
	"errors"
	"hash/fnv"

	"github.com/Syed-Tajdar/calyx-ai/services/scoring/internal/models"
)

// AIProvider scores call transcripts.
type AIProvider interface {
	Score(tenantID, callID, transcript string) (models.ScoreResult, error)
}

// MockAI returns deterministic scores based on call_id hash.
type MockAI struct{}

// NewMockAI creates a mock AI provider.
func NewMockAI() *MockAI {
	return &MockAI{}
}

// Score returns a deterministic mock score.
func (m *MockAI) Score(_ string, callID, transcript string) (models.ScoreResult, error) {
	h := fnv.New32a()
	_, _ = h.Write([]byte(callID + transcript))
	score := float32(60+(h.Sum32()%40)) + 0.5
	sentiment := "neutral"
	if score >= 85 {
		sentiment = "positive"
	} else if score < 70 {
		sentiment = "negative"
	}
	return models.ScoreResult{
		OverallScore: score,
		Sentiment:    sentiment,
		RulesPassed:  8,
		RulesFailed:  2,
	}, nil
}

// OpenAIProvider is a placeholder for real OpenAI integration.
type OpenAIProvider struct {
	apiKey string
}

// NewOpenAIProvider creates an OpenAI scoring provider.
func NewOpenAIProvider(apiKey string) *OpenAIProvider {
	return &OpenAIProvider{apiKey: apiKey}
}

// Score is not implemented in Phase 2 skeleton — use mock mode for builds.
func (o *OpenAIProvider) Score(_ string, _ string, _ string) (models.ScoreResult, error) {
	return models.ScoreResult{}, errors.New("openai provider not implemented in phase 2 skeleton")
}

// Registry selects AI provider by name.
func Registry(name, apiKey string) AIProvider {
	if name == "openai" {
		return NewOpenAIProvider(apiKey)
	}
	return NewMockAI()
}
