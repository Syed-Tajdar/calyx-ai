package riskscoring

import (
	"testing"

	"github.com/Syed-Tajdar/calyx-ai/services/compliance-intelligence/internal/dnc"
	"github.com/Syed-Tajdar/calyx-ai/services/compliance-intelligence/internal/models"
)

func TestScoreDNCMatchHighRisk(t *testing.T) {
	engine := NewEngine()
	ping := models.PingMessage{Revenue: 12.5}
	risk := engine.Score(ping, dnc.Result{Matched: true, ListName: "mock_federal"}, "C")
	if !risk.DNCMatched {
		t.Fatal("expected dnc match")
	}
	if risk.RiskScore < 70 {
		t.Fatalf("expected high risk score, got %f", risk.RiskScore)
	}
	if risk.ViolationType != "dnc_match" {
		t.Fatalf("expected dnc_match violation, got %s", risk.ViolationType)
	}
}

func TestScoreCleanLowRisk(t *testing.T) {
	engine := NewEngine()
	ping := models.PingMessage{Revenue: 5}
	risk := engine.Score(ping, dnc.Result{Matched: false}, "A")
	if risk.DNCMatched {
		t.Fatal("expected no dnc match")
	}
	if risk.ViolationType != "" {
		t.Fatalf("expected no violation, got %s", risk.ViolationType)
	}
}
