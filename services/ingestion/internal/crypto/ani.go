package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
)

var digitsOnly = regexp.MustCompile(`\D`)

// HashANI returns SHA-256 hex of normalized E.164 digits (no plus sign).
func HashANI(ani string) string {
	normalized := NormalizeANI(ani)
	sum := sha256.Sum256([]byte(normalized))
	return hex.EncodeToString(sum[:])
}

// NormalizeANI strips non-digits from a phone number for consistent hashing.
func NormalizeANI(ani string) string {
	return digitsOnly.ReplaceAllString(strings.TrimSpace(ani), "")
}

// EncryptANI encrypts caller ANI with AES-256-GCM. Never log the plaintext.
func EncryptANI(plaintext string, key []byte) ([]byte, error) {
	if plaintext == "" {
		return nil, nil
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("crypto.EncryptANI: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("crypto.EncryptANI: %w", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err = rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("crypto.EncryptANI: nonce: %w", err)
	}
	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return ciphertext, nil
}
