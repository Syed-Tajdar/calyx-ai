package producer

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"

	"github.com/Syed-Tajdar/calyx-ai/services/ingestion/internal/crypto"
)

func TestCallKeyKafkaJSON_DoesNotContainPlaintext(t *testing.T) {
	key := bytes.Repeat([]byte{0xef}, 32)
	plain := "secret-key-xyz"

	enc, err := crypto.EncryptSecret(plain, key)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	stored := hex.EncodeToString(enc)

	body, err := json.Marshal(PingMessage{CallKey: stored})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(body), plain) {
		t.Fatal("kafka JSON must not contain plaintext call_key")
	}
	if len(stored) <= len(plain) {
		t.Fatal("expected ciphertext hex longer than plaintext")
	}
}
