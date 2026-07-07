package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

type CredentialDecryptor struct {
	key []byte
}

func NewCredentialDecryptor(keyHex string) (*CredentialDecryptor, error) {
	key, err := hex.DecodeString(keyHex)
	if err != nil {
		return nil, fmt.Errorf("decode key: %w", err)
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("key must be 32 bytes")
	}
	return &CredentialDecryptor{key: key}, nil
}

func (d *CredentialDecryptor) DecryptMap(blob []byte) (map[string]string, error) {
	plain, err := d.decrypt(blob)
	if err != nil {
		return nil, err
	}
	var out map[string]string
	if err := json.Unmarshal(plain, &out); err != nil {
		return nil, fmt.Errorf("unmarshal credentials: %w", err)
	}
	return out, nil
}

func (d *CredentialDecryptor) decrypt(blob []byte) ([]byte, error) {
	block, err := aes.NewCipher(d.key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonceSize := gcm.NonceSize()
	if len(blob) < nonceSize+16 {
		return nil, fmt.Errorf("blob too short")
	}
	nonce := blob[:nonceSize]
	tagStart := len(blob) - 16
	ciphertext := blob[nonceSize:tagStart]
	tag := blob[tagStart:]
	combined := append(ciphertext, tag...)
	return gcm.Open(nil, nonce, combined, nil)
}
