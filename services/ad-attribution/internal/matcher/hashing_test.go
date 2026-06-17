package matcher

import (
	"testing"

	"github.com/Syed-Tajdar/calyx-ai/services/ad-attribution/internal/models"
)

func TestResolvePlatformFromGclid(t *testing.T) {
	got := ResolvePlatform(models.ConversionEvent{ClickID: "gclid-abc123"})
	if got != "google" {
		t.Fatalf("got %s want google", got)
	}
}

func TestHashPhoneSHA256Deterministic(t *testing.T) {
	a := HashPhoneSHA256("15551234567")
	b := HashPhoneSHA256("15551234567")
	if a == "" || a != b {
		t.Fatal("expected deterministic non-empty hash")
	}
}
