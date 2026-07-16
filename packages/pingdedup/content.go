package pingdedup

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

// Content is the canonical mapped-ping slice used for Redis content-hash dedup.
// Field order in Hash is stable and intentional — do not reorder without a key migration.
//
// INTENTIONALLY EXCLUDED (documented blindness — do not add without Syed approval):
//   - call_key: webhook-only on Retreaver; API-sync always empty → hashing it causes
//     webhook↔API ping-pong republish loop. Read path must use
//     argMaxIf(call_key, received_at, call_key != '') so empty API rows never clobber.
//   - ani_hash: enrichment identity; not part of financial/content identity for dedup.
type Content struct {
	ExternalCallID    string
	Revenue           float64
	Payout            float64
	State             string
	EventType         string
	OccurredAt        time.Time // zero → empty in hash
	CampaignID        string
	PublisherID       string
	TargetID          string
	PlatformDuplicate bool
}

// Hash returns sha256 hex of the canonical content encoding.
func Hash(c Content) string {
	occurred := ""
	if !c.OccurredAt.IsZero() {
		occurred = c.OccurredAt.UTC().Format(time.RFC3339)
	}
	dup := "0"
	if c.PlatformDuplicate {
		dup = "1"
	}
	payload := strings.Join([]string{
		norm(c.ExternalCallID),
		fmt.Sprintf("%.6f", c.Revenue),
		fmt.Sprintf("%.6f", c.Payout),
		norm(c.State),
		norm(c.EventType),
		occurred,
		norm(c.CampaignID),
		norm(c.PublisherID),
		norm(c.TargetID),
		dup,
	}, "\x1f")
	sum := sha256.Sum256([]byte(payload))
	return hex.EncodeToString(sum[:])
}

func norm(s string) string {
	return strings.TrimSpace(s)
}
