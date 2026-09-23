package pingdedup

import (
	"testing"
	"time"
)

func TestHashStableAndNormalized(t *testing.T) {
	base := Content{
		ExternalCallID: "  abc  ",
		Revenue:        12.5,
		Payout:         3.25,
		State:          " CA ",
		EventType:      "incoming",
		OccurredAt:     time.Date(2026, 7, 10, 15, 0, 0, 0, time.UTC),
		CampaignID:     "c1",
		PublisherID:    "p1",
		TargetID:       "t1",
	}
	h1 := Hash(base)
	h2 := Hash(Content{
		ExternalCallID: "abc",
		Revenue:        12.5,
		Payout:         3.25,
		State:          "CA",
		EventType:      "incoming",
		OccurredAt:     time.Date(2026, 7, 10, 15, 0, 0, 0, time.UTC),
		CampaignID:     "c1",
		PublisherID:    "p1",
		TargetID:       "t1",
	})
	if h1 != h2 {
		t.Fatalf("hash not stable after norm: %s vs %s", h1, h2)
	}
	changed := base
	changed.Revenue = 99
	if Hash(changed) == h1 {
		t.Fatal("revenue change must alter hash")
	}
	emptyOcc := base
	emptyOcc.OccurredAt = time.Time{}
	if Hash(emptyOcc) == h1 {
		t.Fatal("empty occurred_at must alter hash")
	}
}

func TestHashParityB1FieldsChangeHash(t *testing.T) {
	base := Content{ExternalCallID: "x", CampaignName: "A"}
	changed := Content{ExternalCallID: "x", CampaignName: "B"}
	if Hash(base) == Hash(changed) {
		t.Fatal("campaign_name must be part of hash")
	}
	d1 := Content{ExternalCallID: "x", DurationSeconds: 10}
	d2 := Content{ExternalCallID: "x", DurationSeconds: 20}
	if Hash(d1) == Hash(d2) {
		t.Fatal("duration_seconds must be part of hash")
	}
	c1 := Content{ExternalCallID: "x", HasConverted: false}
	c2 := Content{ExternalCallID: "x", HasConverted: true}
	if Hash(c1) == Hash(c2) {
		t.Fatal("has_converted must be part of hash")
	}
	conn1 := Content{ExternalCallID: "x", HasConnected: false}
	conn2 := Content{ExternalCallID: "x", HasConnected: true}
	if Hash(conn1) == Hash(conn2) {
		t.Fatal("has_connected must be part of hash")
	}
	blk := Content{ExternalCallID: "x", WasBlocked: true, BlockReason: "The bid has expired"}
	if Hash(blk) == Hash(conn1) {
		t.Fatal("was_blocked and block_reason must be part of hash")
	}
	adj := Content{ExternalCallID: "x", WasConversionAdjusted: true}
	if Hash(adj) == Hash(conn1) {
		t.Fatal("was_conversion_adjusted must be part of hash")
	}
	zero := uint32(0)
	ten := uint32(10)
	absent := Content{ExternalCallID: "x"}
	present0 := Content{ExternalCallID: "x", RequiredDurationSeconds: &zero}
	present10 := Content{ExternalCallID: "x", RequiredDurationSeconds: &ten}
	if Hash(absent) == Hash(present0) {
		t.Fatal("absent required_duration must differ from present 0")
	}
	if Hash(present0) == Hash(present10) {
		t.Fatal("required_duration 0 must differ from 10")
	}
	maxA := Content{ExternalCallID: "x", MaxRequiredDurationSeconds: &ten}
	if Hash(maxA) == Hash(absent) {
		t.Fatal("max_required_duration must be part of hash")
	}
}

func TestHashPlatformDuplicate(t *testing.T) {
	a := Content{ExternalCallID: "x", PlatformDuplicate: false}
	b := Content{ExternalCallID: "x", PlatformDuplicate: true}
	if Hash(a) == Hash(b) {
		t.Fatal("platform_duplicate must be part of hash")
	}
}
