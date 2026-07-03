package crypto

import (
	"bytes"
	"testing"
)

func TestHashANI_NormalizesDigits(t *testing.T) {
	a := HashANI("+1 (555) 123-4567")
	b := HashANI("15551234567")
	if a != b {
		t.Fatalf("hash mismatch: %s vs %s", a, b)
	}
}

func TestEncryptANI_RoundTripKeyLength(t *testing.T) {
	key := bytes.Repeat([]byte{0xab}, 32)
	enc, err := EncryptANI("15551234567", key)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	if len(enc) == 0 {
		t.Fatal("expected ciphertext")
	}
}

func TestEncryptANI_EmptyReturnsNil(t *testing.T) {
	key := bytes.Repeat([]byte{0xab}, 32)
	enc, err := EncryptANI("", key)
	if err != nil {
		t.Fatalf("encrypt empty: %v", err)
	}
	if enc != nil {
		t.Fatal("expected nil ciphertext for empty ANI")
	}
}

func TestEncryptSecret_RoundTrip(t *testing.T) {
	key := bytes.Repeat([]byte{0xcd}, 32)
	plain := "retreaver-credential-value"
	enc, err := EncryptSecret(plain, key)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	if len(enc) < 32 {
		t.Fatal("expected non-trivial ciphertext length")
	}
	got, err := DecryptSecret(enc, key)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if got != plain {
		t.Fatal("round-trip mismatch")
	}
}
