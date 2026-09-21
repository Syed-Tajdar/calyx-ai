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
}

func TestHashPlatformDuplicate(t *testing.T) {
	a := Content{ExternalCallID: "x", PlatformDuplicate: false}
	b := Content{ExternalCallID: "x", PlatformDuplicate: true}
	if Hash(a) == Hash(b) {
		t.Fatal("platform_duplicate must be part of hash")
	}
}
