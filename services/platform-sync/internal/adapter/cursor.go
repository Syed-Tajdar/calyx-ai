package adapter

import (
	"encoding/json"
	"fmt"
)

// DecodeCursor unmarshals an opaque cursor JSON string into dest.
func DecodeCursor(cursor string, dest interface{}) error {
	if cursor == "" {
		return nil
	}
	if err := json.Unmarshal([]byte(cursor), dest); err != nil {
		return fmt.Errorf("decode cursor: %w", err)
	}
	return nil
}

// EncodeCursor marshals cursor state to JSON.
func EncodeCursor(v interface{}) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", fmt.Errorf("encode cursor: %w", err)
	}
	return string(b), nil
}
