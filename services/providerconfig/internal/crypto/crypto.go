package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
)

// Decryptor decrypts AES-256-GCM credential blobs.
type Decryptor struct {
	key []byte
}

// NewDecryptor creates a decryptor from a 32-byte hex key.
func NewDecryptor(keyHex string) (*Decryptor, error) {
	key, err := hex.DecodeString(keyHex)
	if err != nil {
		return nil, fmt.Errorf("NewDecryptor: decode key: %w", err)
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("NewDecryptor: key must be 32 bytes")
	}
	return &Decryptor{key: key}, nil
}

// Decrypt decrypts nonce+ciphertext blob.
func (d *Decryptor) Decrypt(blob []byte) ([]byte, error) {
	block, err := aes.NewCipher(d.key)
	if err != nil {
		return nil, fmt.Errorf("Decrypt: cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("Decrypt: gcm: %w", err)
	}
	nonceSize := gcm.NonceSize()
	if len(blob) < nonceSize {
		return nil, fmt.Errorf("Decrypt: blob too short")
	}
	nonce, ciphertext := blob[:nonceSize], blob[nonceSize:]
	plain, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("Decrypt: open: %w", err)
	}
	return plain, nil
}

// Encrypt encrypts plaintext for storage (used by API when writing credentials).
func (d *Decryptor) Encrypt(plain []byte) ([]byte, error) {
	block, err := aes.NewCipher(d.key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	return gcm.Seal(nonce, nonce, plain, nil), nil
}
