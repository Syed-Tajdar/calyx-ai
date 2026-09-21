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
//   - sync_path: ingestion channel metadata; not call content.
//   - recording_known / has_recording pairing: has_recording alone is hashed; recording_known
//     is a CH presence flag for UI empty-states and must not drift the hash independently.
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
	// Ringba parity B1 (and TrackDrive/CallGrid target shape) — appended only.
	CampaignName       string
	PublisherName      string
	TargetName         string
	BuyerID            string
	BuyerName          string
	DurationSeconds    uint32 // Connected Call Length (talk time)
	CallLengthSeconds  uint32 // whole-call Duration
	HasRecording       bool
	ProfitGross        float64
	HasConverted       bool
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
	rec := "0"
	if c.HasRecording {
		rec = "1"
	}
	converted := "0"
	if c.HasConverted {
		converted = "1"
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
		norm(c.CampaignName),
		norm(c.PublisherName),
		norm(c.TargetName),
		norm(c.BuyerID),
		norm(c.BuyerName),
		fmt.Sprintf("%d", c.DurationSeconds),
		fmt.Sprintf("%d", c.CallLengthSeconds),
		rec,
		fmt.Sprintf("%.6f", c.ProfitGross),
		converted,
	}, "\x1f")
	sum := sha256.Sum256([]byte(payload))
	return hex.EncodeToString(sum[:])
}

func norm(s string) string {
	return strings.TrimSpace(s)
}
