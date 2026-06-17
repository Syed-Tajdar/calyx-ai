package riskscoring

import (
	"github.com/Syed-Tajdar/calyx-ai/services/compliance-intelligence/internal/attestation"
	"github.com/Syed-Tajdar/calyx-ai/services/compliance-intelligence/internal/dnc"
	"github.com/Syed-Tajdar/calyx-ai/services/compliance-intelligence/internal/models"
)

// Engine computes compliance risk from DNC + attestation signals.
type Engine struct{}

// NewEngine creates a risk scoring engine.
func NewEngine() *Engine {
	return &Engine{}
}

// Score combines signals into a risk result.
func (e *Engine) Score(ping models.PingMessage, dncResult dnc.Result, attLevel string) models.RiskResult {
	score := float32(15)
	if dncResult.Matched {
		score += 55
	}
	switch attLevel {
	case "C":
		score += 25
	case "B":
		score += 10
	}
	if score > 100 {
		score = 100
	}

	exposure := attestation.CarrierBlockEstimate(attLevel, float32(ping.Revenue))
	if dncResult.Matched {
		exposure += float32(ping.Revenue) * 1.5
	}

	violation := ""
	if dncResult.Matched {
		violation = "dnc_match"
	} else if score >= 70 {
		violation = "high_risk_score"
	}

	return models.RiskResult{
		RiskScore:        score,
		DNCMatched:       dncResult.Matched,
		AttestationLevel: attLevel,
		ExposureUSD:      exposure,
		ViolationType:    violation,
	}
}
