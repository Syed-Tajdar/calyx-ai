package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"strings"
)

var digitsOnly = regexp.MustCompile(`\D`)

func HashANI(ani string) string {
	normalized := digitsOnly.ReplaceAllString(strings.TrimSpace(ani), "")
	sum := sha256.Sum256([]byte(normalized))
	return hex.EncodeToString(sum[:])
}

func EncryptANI(plaintext string, key []byte) ([]byte, error) {
	if plaintext == "" {
		return nil, nil
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err = rand.Read(nonce); err != nil {
		return nil, err
	}
	return gcm.Seal(nonce, nonce, []byte(plaintext), nil), nil
}

func EncryptSecret(plaintext string, key []byte) ([]byte, error) {
	return EncryptANI(plaintext, key)
}
