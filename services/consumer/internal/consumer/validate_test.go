package consumer

import (
	"testing"

	"github.com/Syed-Tajdar/calyx-ai/services/consumer/internal/models"
)

func TestValidatePingForInsert(t *testing.T) {
	ok := models.PingMessage{
		TenantID:       "11111111-1111-1111-1111-111111111111",
		ANIHash:        "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		ExternalCallID: "smoke-ok",
	}
	if got := ValidatePingForInsert(ok); got != "" {
		t.Fatalf("expected ok, got %q", got)
	}

	badTenant := ok
	badTenant.TenantID = "BAD"
	if got := ValidatePingForInsert(badTenant); got == "" {
		t.Fatal("expected invalid tenant_id")
	}

	longAni := ok
	longAni.ANIHash = string(make([]byte, 65))
	if got := ValidatePingForInsert(longAni); got == "" {
		t.Fatal("expected long ani_hash rejection")
	}

	shortAni := ok
	shortAni.ANIHash = "abc"
	if got := ValidatePingForInsert(shortAni); got != "" {
		t.Fatalf("short ani_hash is padded by CH driver; expected allow, got %q", got)
	}
}
