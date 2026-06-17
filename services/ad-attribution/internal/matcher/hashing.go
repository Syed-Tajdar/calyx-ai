package matcher

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// HashPhoneSHA256 normalizes and hashes phone material for ad platform APIs.
// Input must already be a hash in production paths — this re-hashes only when needed.
func HashPhoneSHA256(material string) string {
	normalized := strings.TrimSpace(material)
	if normalized == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(normalized))
	return hex.EncodeToString(sum[:])
}

// HashEmailSHA256 lowercases and SHA-256 hashes email for ad platforms.
func HashEmailSHA256(email string) string {
	normalized := strings.ToLower(strings.TrimSpace(email))
	if normalized == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(normalized))
	return hex.EncodeToString(sum[:])
}
