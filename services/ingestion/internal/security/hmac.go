package security

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"
)

// PayloadHash returns SHA-256 hex of the signed payload for audit logging.
func PayloadHash(body []byte, rawQuery string) string {
	h := sha256.New()
	if len(body) > 0 {
		h.Write(body)
	} else {
		h.Write([]byte(rawQuery))
	}
	return hex.EncodeToString(h.Sum(nil))
}

// VerifyRingbaHMAC validates X-Ringba-Signature against body or query string.
func VerifyRingbaHMAC(r *http.Request, body []byte, secret string) bool {
	sig := r.Header.Get("X-Ringba-Signature")
	if sig == "" || secret == "" {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	if len(body) > 0 {
		mac.Write(body)
	} else {
		mac.Write([]byte(r.URL.RawQuery))
	}
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(strings.ToLower(sig)), []byte(expected))
}

// ComputeRingbaHMAC produces a signature for tests and smoke scripts.
func ComputeRingbaHMAC(body []byte, rawQuery, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	if len(body) > 0 {
		mac.Write(body)
	} else {
		mac.Write([]byte(rawQuery))
	}
	return hex.EncodeToString(mac.Sum(nil))
}
