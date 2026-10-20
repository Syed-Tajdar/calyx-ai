package main

// Whole-object comparison for the sandbox plan (owner fix 2): read steps a, c and e compare EVERY
// field of the publisher / affiliate object except the paused flag itself and "updated" timestamps,
// to prove the PATCH / PUT changes nothing else. Values are compared as canonical JSON.

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"strings"
)

// ignoredField reports whether a top-level key is excluded from the comparison: the field the write
// is meant to change, and timestamps that move on every edit.
func ignoredField(platform, key string) bool {
	k := strings.ToLower(key)
	if platform == "ringba" && k == "enabled" {
		return true
	}
	if platform == "retreaver" && k == "paused" {
		return true
	}
	return strings.Contains(k, "updated") || strings.Contains(k, "modified")
}

// Snapshot maps every compared top-level key to its canonical JSON (encoding/json sorts map keys,
// so nested objects compare by content, not order).
func Snapshot(platform string, obj map[string]any) map[string]string {
	out := make(map[string]string, len(obj))
	for k, v := range obj {
		if ignoredField(platform, k) {
			continue
		}
		b, err := json.Marshal(v)
		if err != nil {
			b = []byte("<unmarshalable>")
		}
		out[k] = string(b)
	}
	return out
}

// IgnoredKeys lists the keys of obj that Snapshot leaves out (for the report).
func IgnoredKeys(platform string, obj map[string]any) []string {
	var out []string
	for k := range obj {
		if ignoredField(platform, k) {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}

// Fingerprint is a short SHA-256 of a snapshot, so two reads can be compared at a glance without
// printing any value.
func Fingerprint(s map[string]string) string {
	keys := make([]string, 0, len(s))
	for k := range s {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	h := sha256.New()
	for _, k := range keys {
		h.Write([]byte(k))
		h.Write([]byte{0})
		h.Write([]byte(s[k]))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))[:16]
}

// DiffSnapshots lists every difference between two snapshots as "added: k", "removed: k" or
// "changed: k". Empty means the objects are identical apart from the ignored fields.
func DiffSnapshots(before, after map[string]string) []string {
	var out []string
	for k, v := range before {
		w, ok := after[k]
		switch {
		case !ok:
			out = append(out, "removed: "+k)
		case w != v:
			out = append(out, "changed: "+k)
		}
	}
	for k := range after {
		if _, ok := before[k]; !ok {
			out = append(out, "added: "+k)
		}
	}
	sort.Strings(out)
	return out
}
