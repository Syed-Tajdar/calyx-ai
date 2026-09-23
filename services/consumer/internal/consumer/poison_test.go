package consumer

import (
	"errors"
	"testing"
)

func TestIsPoisonInsertError(t *testing.T) {
	poison := []string{
		`clickhouse [AppendRow]: tenant_id invalid UUID length: 3`,
		`FixedString size mismatch`,
		`clickhouse: batch is invalid. check appended data is correct`,
		`panic: invalid size`,
	}
	for _, s := range poison {
		if !isPoisonInsertError(errors.New(s)) {
			t.Fatalf("expected poison for %q", s)
		}
	}
	transient := []string{
		`dial tcp: lookup clickhouse on 127.0.0.11:53: no such host`,
		`dial tcp 172.18.0.5:9000: connect: connection refused`,
		`i/o timeout`,
		`context deadline exceeded`,
	}
	for _, s := range transient {
		if isPoisonInsertError(errors.New(s)) {
			t.Fatalf("expected transient for %q", s)
		}
	}
}
