package security

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestVerifyRingbaHMAC_QueryString(t *testing.T) {
	secret := "test-secret"
	rawQuery := "CallID=abc&caller_id=15551234567"
	sig := ComputeRingbaHMAC(nil, rawQuery, secret)

	req := httptest.NewRequest(http.MethodGet, "/webhooks/ringba/token?"+rawQuery, nil)
	req.Header.Set("X-Ringba-Signature", sig)

	if !VerifyRingbaHMAC(req, nil, secret) {
		t.Fatal("expected valid HMAC")
	}
}

func TestVerifyRingbaHMAC_RejectsMissingHeader(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/webhooks/ringba/token?CallID=abc", nil)
	if VerifyRingbaHMAC(req, nil, "secret") {
		t.Fatal("expected invalid HMAC without header")
	}
}

func TestVerifyRingbaHMAC_RejectsWrongSecret(t *testing.T) {
	rawQuery := "CallID=abc"
	sig := ComputeRingbaHMAC(nil, rawQuery, "right-secret")

	req := httptest.NewRequest(http.MethodGet, "/webhooks/ringba/token?"+rawQuery, nil)
	req.Header.Set("X-Ringba-Signature", sig)

	if VerifyRingbaHMAC(req, nil, "wrong-secret") {
		t.Fatal("expected invalid HMAC for wrong secret")
	}
}

func TestPayloadHash_Deterministic(t *testing.T) {
	a := PayloadHash(nil, "CallID=abc")
	b := PayloadHash(nil, "CallID=abc")
	if a != b || len(a) != 64 {
		t.Fatalf("unexpected hash: %s", a)
	}
}
