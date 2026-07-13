# Phase 0 — Shared Foundation Report

**Date:** 2026-07-10  
**Verdict:** Phase 0 shared foundation **COMPLETE**  
**Scope:** A1/A2/A3 shared parts only. Ringba/TrackDrive/CallGrid `occurred_at` **not** mapped (remain pending).

---

## 0.1 Connector check (RAISED) + update-blindness fix

### Raw connector grep

```
callgrid.go:39:parsed.Revenue, parsed.Payout, parsed.IsDuplicate,
callgrid.go:76:parseBool(values, "is_duplicate", "duplicate"),
ping_builder.go:54:IsDuplicate:       false, // Redis dedup owns this; set in webhook handler
retreaver.go:42:parsed.Revenue, parsed.Payout, parsed.IsDuplicate,
retreaver.go:92:parseBool(values, "is_duplicate", "duplicate"),
ringba.go:24:IsDuplicate   bool
ringba.go:60:dupStr := firstValue(values, "is_duplicate", "IsDuplicate", "duplicate")
ringba.go:61:payload.IsDuplicate = strings.EqualFold(dupStr, "true") || dupStr == "1" || strings.EqualFold(dupStr, "yes")
ringba_connector.go:49:payload.Revenue, payload.Payout, payload.IsDuplicate,
trackdrive.go:62:parseBool(values, "is_duplicate", "duplicate"),
```

**Finding:** Connectors **do** read platform-declared duplicate from the webhook payload. That is a different concept from Redis ingestion dedup.

**Decision (split before changing aggregates):**
- `is_duplicate` = Redis ingestion dedup only (display/audit; no longer filters aggregates)
- `platform_duplicate` = webhook payload flag (new ClickHouse column `000009_pings_platform_duplicate`)
- Aggregates do **not** filter either column; identity + `argMax(..., received_at)` wins
- Product may later exclude `platform_duplicate=1` from metrics — not done silently

### Aggregate change

Removed `AND is_duplicate = 0` from:
- `services/api/src/analytics/analytics.service.ts` (`dedupedCallsSubquery`)
- `services/api/src/coverage/coverage.service.ts` (heatmap inner query)

### A1 two-row proof (raw)

```
=== INSERT TWO ROWS callId=phase0-update-blind-f029198b ===

=== CH rows for test call ===
phase0-update-blind-f029198b	0	0	2026-07-10 15:13:54
phase0-update-blind-f029198b	46.5	1	2026-07-10 15:14:04

=== BEFORE filter removed would have been ===
0

=== AFTER (no is_duplicate filter) ===
46.5

=== /pings search ===
total=2
id=phase0-update-blind-f029198b rev=0 dup=0
id=phase0-update-blind-f029198b rev=46.5 dup=1

=== ISOLATED QUERY (argMax for this call) ===
1	46.5
```

| Assert | Result |
|--------|--------|
| Metrics/argMax revenue = 46.5 | PASS |
| Call counted once | PASS (`distinct_calls=1`) |
| `/pings` shows both rows, second DUP | PASS |

---

## 0.2 Timeline dedupe

`web/src/lib/ping-timeline.ts` now keeps the latest row per `(platform, external_call_id)` before day-bucketing, so chart totals align with `analytics/metrics.total_pings` (distinct calls), not raw `/pings` row count.

---

## 0.3 Conformance harness

File: `services/platform-sync/internal/adapter/conformance_test.go`

- Loops registry via `Registry.Platforms()`
- Only asserts certified adapters (empty in Phase 0)
- Pending list (visible TODOs): `retreaver`, `ringba`, `trackdrive`, `callgrid`
- New adapter missing from both lists → build fail (`TestRegistryMembershipComplete`)

### Raw test output

```
--- PASS: TestRegistryMembershipComplete (0.00s)
=== RUN   TestCertifiedAdapterConformance
    conformance_test.go:69: no certified adapters yet — harness only (Phase 0)
--- PASS: TestCertifiedAdapterConformance (0.00s)
...
PASS
ok  	github.com/Syed-Tajdar/calyx-ai/services/platform-sync/internal/adapter	4.000s
PASS
ok  	github.com/Syed-Tajdar/calyx-ai/services/platform-sync/internal/redact	0.627s
PASS
ok  	github.com/Syed-Tajdar/calyx-ai/services/platform-sync/internal/syncer	0.996s
```

Header-auth redaction also covered in `redact` tests (`Bearer` / `Basic` → `[REDACTED]`).

---

## 0.4 Golden fixture format

- Path: `services/platform-sync/internal/adapter/testdata/{platform}_calls.json`
- Header comments: `source: …` + doc URL (stripped before JSON parse)
- Retreaver fixture present: `source: real-account (scrubbed)` (formalized for Phase 1)
- Ringba / TrackDrive / CallGrid fixtures: deferred to their phases (`occurred_at` mapping pending)

---

## 0.5 Go-live checklist

Written: `docs/platform-go-live-checklist.md` (D1–D12 + A1 detail endpoints).

Living status: `docs/platform-certification.md` (initialized; Retreaver not yet certified).

---

## Explicitly out of scope (Phase 0)

- Mapping `occurred_at` for ringba / trackdrive / callgrid
- Certifying any adapter (Phase 1 = Retreaver)

---

## One-line

**Phase 0 COMPLETE:** update-blindness fixed (after `platform_duplicate` split), timeline dedupes, conformance harness + checklist + fixture format ready; Retreaver certification is Phase 1.
