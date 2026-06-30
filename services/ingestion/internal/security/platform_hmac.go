package security

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"
)

// VerifyRetreaverHMAC validates X-Retreaver-Signature (HMAC-SHA256 hex of body or query).
func VerifyRetreaverHMAC(r *http.Request, body []byte, secret string) bool {
	sig := r.Header.Get("X-Retreaver-Signature")
	if sig == "" {
		sig = r.Header.Get("X-Signature")
	}
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

// ComputeRetreaverHMAC produces a signature for tests and smoke scripts.
func ComputeRetreaverHMAC(body []byte, rawQuery, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	if len(body) > 0 {
		mac.Write(body)
	} else {
		mac.Write([]byte(rawQuery))
	}
	return hex.EncodeToString(mac.Sum(nil))
}

// VerifyCallGridHMAC validates X-CallGrid-Signature (HMAC-SHA256 hex of raw query or body).
func VerifyCallGridHMAC(r *http.Request, body []byte, secret string) bool {
	sig := r.Header.Get("X-CallGrid-Signature")
	if sig == "" {
		sig = r.Header.Get("X-Signature")
	}
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

// ComputeCallGridHMAC produces a signature for tests and smoke scripts.
func ComputeCallGridHMAC(body []byte, rawQuery, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	if len(body) > 0 {
		mac.Write(body)
	} else {
		mac.Write([]byte(rawQuery))
	}
	return hex.EncodeToString(mac.Sum(nil))
}
