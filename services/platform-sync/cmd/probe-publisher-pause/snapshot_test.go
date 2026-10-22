package main

import (
	"reflect"
	"testing"
)

func TestSnapshotIgnoresOnlyTheFlagAndTimestamps(t *testing.T) {
	ringba := map[string]any{"id": "PI1", "name": "CALYX-TEST", "enabled": true, "updatedAt": "x", "payout": map[string]any{"b": 2, "a": 1}}
	s := Snapshot("ringba", ringba)
	if _, ok := s["enabled"]; ok {
		t.Fatal("enabled must be ignored for Ringba")
	}
	if _, ok := s["updatedAt"]; ok {
		t.Fatal("updated timestamps must be ignored")
	}
	if s["payout"] != `{"a":1,"b":2}` {
		t.Fatalf("nested objects must be canonical, got %s", s["payout"])
	}
	if got := IgnoredKeys("ringba", ringba); !reflect.DeepEqual(got, []string{"enabled", "updatedAt"}) {
		t.Fatalf("ignored keys %v", got)
	}
	retreaver := map[string]any{"id": 71582, "afid": "11111", "paused": true, "updated_at": "y", "enabled": true}
	rs := Snapshot("retreaver", retreaver)
	if _, ok := rs["paused"]; ok {
		t.Fatal("paused must be ignored for Retreaver")
	}
	if _, ok := rs["enabled"]; !ok {
		t.Fatal("enabled is a normal field for Retreaver and must be compared")
	}
}

func TestDiffSnapshotsProvesNothingElseChanged(t *testing.T) {
	before := Snapshot("ringba", map[string]any{"id": "PI1", "name": "T", "enabled": true, "updatedAt": "1"})
	afterPause := Snapshot("ringba", map[string]any{"id": "PI1", "name": "T", "enabled": false, "updatedAt": "2"})
	if d := DiffSnapshots(before, afterPause); len(d) != 0 {
		t.Fatalf("only the flag and timestamp changed, diff must be empty, got %v", d)
	}
	if Fingerprint(before) != Fingerprint(afterPause) {
		t.Fatal("fingerprints must match when only ignored fields change")
	}
	sideEffect := Snapshot("ringba", map[string]any{"id": "PI1", "name": "RENAMED", "enabled": false, "extra": 1})
	want := []string{"added: extra", "changed: name"}
	if d := DiffSnapshots(before, sideEffect); !reflect.DeepEqual(d, want) {
		t.Fatalf("diff %v, want %v", d, want)
	}
	if d := DiffSnapshots(before, map[string]string{"id": `"PI1"`}); !reflect.DeepEqual(d, []string{"removed: name"}) {
		t.Fatalf("removed field must show, got %v", d)
	}
}
