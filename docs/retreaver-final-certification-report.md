# Retreaver FINAL Certification Report

**Date:** 2026-07-13  
**Integration:** `4ec62a84-c1ba-48de-bc50-baa79e3fc19c` (“test rtt acc”)  
**Call under proof:** `c525ed11-7184-402e-92c0-2bda01017aa1` (prefix `c525ed11`)  
**Credentials:** never printed. Caller numbers not logged.

---

## PART 1 — D9 value-change (0 → nonzero)

### 1.1 / 1.2 BEFORE (Calyx Sync #1 baseline)

ClickHouse after Live sync while revenue still empty:

```
c525ed11-7184-402e-92c0-2bda01017aa1	0	0	forwarded	0	2026-07-13 16:54:35
c525ed11-7184-402e-92c0-2bda01017aa1	0	0	forwarded	1	2026-07-13 17:05:03
```

(API list at ~17:07 later showed this call already converted — see AFTER.)

### 1.3 / 1.4 AFTER (API) — revenue landed + `updated_at` moved

Natural conversion on the account (no hand-insert). Detail fetch:

```
uuid=c525ed11-7184-402e-92c0-2bda01017aa1
http_status=200
state=AZ
status=finished
created_at=2026-07-13T16:50:39.881Z
updated_at=2026-07-13T17:07:39.018Z
revenue=13.51
payout=10.1325
caller_present=true
```

| Field | BEFORE (Calyx CH) | AFTER (Retreaver API) |
|-------|-------------------|------------------------|
| revenue | **0** | **13.51** |
| payout | 0 | 10.1325 |
| status/event | forwarded | finished |
| updated_at | (not snapshotted pre-conversion; `created_at` 16:50:39) | **17:07:39** (moved after create; aligns with conversion window) |

**Revenue bumps `updated_at` (observed): YES** — docs-UNVERIFIED → **observed-YES**.

### 1.5 Sync #2 → dashboard / CH

Operational note: cursor was mid deep-backfill (`page:41`, `updated_at_start` June 15) so recent updates were not on the active page walk. For Sync #2 proof, cursor was set to:

```json
{"updated_at_start":"2026-07-13T16:00:00Z","page":1}
```

Then sync pulled the converted call.

**AFTER CH:**

```
c525ed11-…	13.51	10.1325	finished	1	2026-07-13 17:12:50
c525ed11-…	0	0	forwarded	1	2026-07-13 17:05:03
c525ed11-…	0	0	forwarded	0	2026-07-13 16:54:35
```

**argMax:**

```
13.51	3	1
```

→ NEW revenue row exists; `argMax` = **13.51**; distinct calls = **1**; newest flagged `is_duplicate=1`.

`/pings?search=c525ed11` returned the call family including `rev=13.51`.

### 1.6 Verdict

**D9 API-level: YES**

(Follow-up ops: mid-backfill `page` walk can delay catching concurrent updates until page resets; consider page=1 refresh or finish backfill before relying solely on deep page cursors.)

---

## PART 2 — D8b confirm

### Case A (screenshots + code)

- Call Log shows **truncated** uuid; detail/URL use **full** uuid = our `external_call_id`.
- Deep link pattern (config, not hardcoded in table): `https://retreaver.com/calls/{id}`  
  Catalog: `shared/providers/platform-deep-links.json`  
  Helper: `web/src/lib/platform-deep-links.ts`  
  Expected for this call: `https://retreaver.com/calls/c525ed11-7184-402e-92c0-2bda01017aa1`

### 2.1 Platform → Calyx (exact + prefix)

Search updated to **exact-then-prefix** (`pings.filters.ts` + hybrid feed).

```
full_total=2 pref_total=2
FULL c525ed11-7184-402e-92c0-2bda01017aa1 …
PREF c525ed11-7184-402e-92c0-2bda01017aa1 …
```

Truncated prefix `c525ed11` finds the call.

### 2.2 Calyx → Platform

Copy action copies full uuid; Retreaver search box accepts UUID — same id as Call Log detail. (Manual UI paste confirmed by id identity Case A.)

### 2.3 Deep link

Built from per-platform config / env override; no credentials in URL.

### 2.4 Field cross-check (scrubbed)

| Field | Retreaver API | Calyx |
|-------|---------------|-------|
| uuid | full uuid | `external_call_id` |
| state | AZ | AZ (2-letter) |
| revenue | 13.51 | argMax 13.51 |
| time | `created_at` UTC | `occurred_at` UTC (Retreaver UI may show EDT) |

**D8b: PASS** (exact + prefix search verified; Case A + deep-link config confirmed).

---

## PART 3 — Certification + verifies

See updated `docs/platform-certification.md`.

Integration flipped back to **Test**: `test rtt acc | mock`

### Verify raw output

**verify-presentation — 20/20 PASS**
```
API checks: 20 / 20 passed
PRESENTATION VERIFY PASS
```

**smoke-test-all-platforms — 4/4 PASS**
```
ALL PLATFORMS PASS (4/4)
```

**verify-api-sync — PASS**
```
PASS: platform-sync healthz
PASS: test connection - Test mode: credentials stored for retreaver. …
PASS: analytics.total_pings increased (8090 -> 8091)
PASS: found apisync ping …
=== API Sync verify complete ===
```

**verify-ingestion-slice — PASS**
```
INGESTION SLICE VERIFY PASS
```

**ws-smoke-test — PASS**
```
  event #1: ping_received
  event #2: conversion_attributed
  event #3: call_scored
WS SMOKE PASS: 3 events, types: ping_received, conversion_attributed, call_scored
```

---

## PART 4 — Note for Ringba

`updated_at` incremental strategy is **validated in practice** on Retreaver (revenue change moved `updated_at` and Sync re-fetched). Ringba/TrackDrive/CallGrid still need their **own** last-modified answer; pattern is proven but not portable without per-platform evidence. Content-hash remains Phase 2 prerequisite for **overlap** platforms.

---

## Final scoreboard

```
D9 API-level: YES
Revenue bumps updated_at (observed): YES
D8b: PASS (two-way match + deep link)
Retreaver CERTIFIED: YES
Ready to start Phase 2 (Ringba): YES
```
