# Part 1 Delta 2 Report (D5–D6)

**Date:** 2026-07-23  
**Scope:** Final close-out before Part 2 (Ringba parity A + B1).  
**Proof:** rebuilt Docker `api` / `web` / `enrichment` on `:3000` / `:3001` / enrichment service.

---

## Summary scoreboard

| Item | Result |
|------|--------|
| D5 — WebSocket live feed regression | **PASS** |
| D6 — Enrichment write guard / ReplacingMergeTree | **PASS** (implemented) |

---

## D5 — WebSocket live feed regression

### Root cause (one sentence)

**KafkaJS crashed consumer group `calyx-api-live-feed` with `Snappy compression not implemented` after a Snappy-compressed message was written to `callloom.pings.v1` by `rpk topic produce` (default `-z snappy`) during the D1.2 write-path proof at `2026-07-23T16:11:40Z` — leaving `MEMBERS=0` so hops 4–5 never received events.**

This was **not** caused by C3 demo predicates or D1 enrichment Go changes. It was an environmental side-effect of the D1 evidence produce. Filing it as roadmap I was incorrect triage.

### D5.1 — Hop-by-hop trace

| Hop | Path | Result | Evidence |
|-----|------|--------|----------|
| 1 | Ingestion `:8080` | **Reached** | Signed Ringba webhook → `200` `{"status":"ok"}` (e.g. `callId=64551e69-…`) |
| 2 | Kafka `callloom.pings.v1` | **Reached** | `rpk topic consume` found the same `external_call_id` on the topic |
| 3 | API live consumer | **Not reached (failure point)** | `rpk group describe calyx-api-live-feed` → `STATE=Empty`, `MEMBERS=0`, lag on partitions; API log `KafkaJSNotImplemented: Snappy compression not implemented` / `Consumer Crash` for `calyx-api-live-feed` at `16:11:40.555Z` |
| 4 | WS gateway | N/A (no consume) | Gateway code intact; no broadcast without hop 3 |
| 5 | Client `/live` | Connect OK, **0 events** | Smoke seeded 3 webhooks; handshake did not `connect_error`; `live_event` count stayed 0 |

**Failure point: hop 3 — live Kafka consumer crashed and left the group.**

### D5.2 — Candidate causes

| Candidate | Verdict | Evidence |
|-----------|---------|----------|
| 1. Demo predicate in live path | **Ruled out** | No `apiSync`/`apisync`/`demoExclusion` under `services/api/src/live/`. Smoke IDs are UUIDs, not `apisync-%`. |
| 2. Kafka consumer state | **Ruled in** | Empty group, Crash log, Snappy error. After fix: `STATE=Stable`, `MEMBERS=1`. |
| 3. Gateway / auth | **Ruled out as primary** | Client connects (smoke reaches seed loop after `connect`); no `connect_error`. Badge path uses socket `connected`. |
| 4. Ingestion rejection | **Ruled out** | Hop 1 returns 200. |
| 5. Enrichment stealing partitions | **Ruled out** | Enrichment group `callloom-enrichment` is separate; enrichment continued (`enrichment complete` / previously `skip existing`). Crash was KafkaJS-only on API group. |

### D5.3 — Regression window (isolation)

| Check | Result |
|-------|--------|
| Current images before fix | FAIL (0 events) — confirmed in Delta 1 |
| Part 1 credibility report | WS PASS — before D1.2 `rpk` produce |
| Crash timestamp vs D1 produce | **Exact match** `16:11:40Z` with enrichment skip log for same produce |
| Temporary revert C3 or D1 code? | **Not required / would not restore** — consumer was dead from Snappy regardless of those source trees. Isolation is proven by: **same C3+D1 code + Snappy codec fix → WS PASS** without reverting product changes. |

### D5.4 — Fix

- Added `kafkajs-snappy` and `createKafkaClient()` registering Snappy (`services/api/src/kafka/kafka-client.ts`).
- Wired live consumer, `KafkaService`, and alert engine through that helper.
- Rebuild/recreate `api` → consumer Stable.

### Live badge honesty

Dashboard already maps:

- `connected` (socket.io) → **Live**
- `!connected && poll.isLive` → **Polling**

(`web/src/app/(app)/dashboard/page.tsx` + `use-hybrid-ping-feed.ts`). Socket was up while Kafka was dead — badge correctly reflected **socket** state, not Kafka health. After D5 fix, socket connection again implies events can flow. No badge change required.

### Proof (raw)

```
=== ws-smoke ===
  event #1: ping_received
  event #2: conversion_attributed
  event #3: call_scored
WS SMOKE PASS: 3 events, types: ping_received, conversion_attributed, call_scored
ws_exit=0

=== verify-system (excerpt) ===
=== WebSocket live feed ===
  event #1: ping_received
  event #2: conversion_attributed
  event #3: call_scored
WS SMOKE PASS: 3 events, …

SYSTEM VERIFY PASS
system_exit=0
```

Logs: `docs/verify-logs/delta2-verify-system.txt`

### DoD D5

- [x] Hop-by-hop with evidence; failure at hop 3  
- [x] Candidates ruled in/out  
- [x] Regression window (D1 rpk Snappy, not C3/D1 code)  
- [x] One-sentence root cause  
- [x] `ws-smoke-test.mjs` ≥3 events  
- [x] `verify-system` WS PASS  
- [x] Live badge already socket-accurate  

---

## D6 — Enrichment write guard freezes attributes

### Decision: **IMPLEMENTED** (not roadmapped)

ReplacingMergeTree migration is straightforward on this stack; one migration collapses the ~1.4M excess rows **and** allows updates. Write guard removed.

### D6.1–D6.2 — Migration design + cost

| Item | Choice |
|------|--------|
| Engine | `ReplacingMergeTree(created_at)` |
| ORDER BY | `(tenant_id, ani_hash, attribute_key)` — same as before |
| Version | `created_at` — newest insert wins |
| Mechanics | Create `enrichment_attrs_rmt` → `INSERT SELECT` → rename swap → `OPTIMIZE … FINAL` |
| Downtime | Brief rename (~seconds). Copy+OPTIMIZE of ~4.6M rows ≈ **19s** observed locally |
| Files | `db/clickhouse/migrations/000010_enrichment_attrs_replacing.{up,down}.sql` + compose `clickhouse-init` phase10 |

**Row counts**

| When | `count()` | `uniqExact(tenant,ani,key)` | Engine |
|------|-----------|------------------------------|--------|
| Before | **4,630,140** | **3,197,540** | MergeTree |
| After OPTIMIZE | **3,197,540** | **3,197,540** | ReplacingMergeTree |

Excess duplicates removed: **~1,432,600** (matches D1 report order of magnitude).

**Read path:** Keep `GROUP BY attribute_key` + `argMax(..., created_at)` — still correct without `FINAL`; not redundant for safety under unmerged parts.

**Write guard:** Removed. Comment at former call site documents why updates must flow (Chapter 2). Dead `HasAttributes` helper removed.

### D6.3 — Proofs

**Update visible**

```
attr_000_before=d6736136_val_0
attr_000_after=d6_update_proof_value   # INSERT with now() then GET /enrichment
```

**Growth bounded (3 enrich cycles, same ANI, cache cleared each time)**

```
rows_ani_t0=521
rows_ani_after_3_cycles_raw=2081          # temporary parts before merge
rows_ani_after_optimize=520 uniq_keys=520 # collapsed
enrichment complete ×3 (no "skip existing")
```

Produce used `-z none` so Snappy regression is not reintroduced during proof.

### DoD D6

- [x] Design + cost reported  
- [x] Read-path assessed (keep GROUP BY/argMax)  
- [x] **Implemented** (not gated)  
- [x] Update-visible + growth-bound + duplicate reduction proofs  
- [x] Code comment at former guard site  

---

## Verify commands (required re-run)

```text
ws-smoke-test.mjs          → PASS (3 events)
verify-ingestion-slice.ps1 → PASS (exit 0)
verify-system.ps1          → SYSTEM VERIFY PASS (exit 0; WS PASS)
```

---

## Honest limitations

1. ReplacingMergeTree collapses on **merge**/`OPTIMIZE FINAL`; raw `count()` can temporarily rise between merges (shown in 3-cycle raw=2081).
2. `verify-api-sync.ps1` still stale vs C3 (roadmap H) — not run as a greenwash; product exclusion stands.
3. Live badge does not separately surface “Kafka consumer healthy”; it tracks socket connection (sufficient once hop 3 is fixed).

---

## Found but not fixed

- Remaining active mock API-sync integrations (`Test 1`, `Real R. Test`) still grow `apisync-%` rows  
- Publishers card hardcoded names  
- Roadmap H (`verify-api-sync` assertion + teardown)  
- Coverage period / dashboard gap items (roadmap C)  

---

## Part 2 readiness

| Gate | Status |
|------|--------|
| D5 WS live path | **PASS** |
| D6 enrichment updates + growth | **PASS** (ReplacingMergeTree; guard removed) |
| Part 2 (Ringba A + B1) | **Unblocked** after this report is accepted |
