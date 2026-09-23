# Phase 2A-fix2 report — STOP (Part 1 root cause not proven)

Phase 2A-fix2 only. **No Phase 2B.** No status logic, UI, deletions, or Ringba writes.

**STOP:** Part 1 cannot name a `[PROVEN]` root cause for the Phase 2A-fix WebSocket failure without guessing. Spec rule: *find causes by evidence before changing code; if ambiguous, STOP and report — do not infer.* Parts **2–4 were not started.** No product code was changed in this phase.

Spec file: `docs/writeback-phase2a-fix2-spec.md`.

---

## Part 1 — WebSocket regression

### Fail window (from gate log)

| | |
|---|---|
| Gate run | terminal `107036` — `verify-system.ps1` started `2026-09-22T22:02:40.435Z` |
| WS result | `WS SMOKE FAIL: only 0 events in 45000ms (need 3)` |
| After WS | Pipeline E2E 4/4 PASS (first ringba smoke in CH at `2026-09-22 22:08:51`) |
| Report | `docs/writeback-phase2a-fix-report.md` |

### 1.1 — Hop table

#### At Phase 2A-fix fail (`~2026-09-22T22:07Z`, reconstructed)

| Hop | Reached? | Evidence |
|---|---|---|
| 1 Ingestion `:8080` | **Not reached for WS seeds** `[PROVEN]` | ClickHouse has **no** `smoke-%` rows between `22:00` and `22:08:50`. First smoke row is pipeline `smoke-27cecdfd-…` at `22:08:51`. Ingestion logs for that minute are empty. |
| 2 Kafka `callloom.pings.v1` | **Not reached for WS seeds** `[INFERRED]` from hop 1 | No seed → nothing to publish. Topic was otherwise active (backfill earlier). |
| 3 API live consumer `calyx-api-live-feed` | **Alive** `[PROVEN]` | Joined `2026-09-21T22:10:03Z`; next Crash is `2026-09-23T19:24:35Z` — **after** this fail. No Crash / Stopped / Starting in the fail window. |
| 4 WS gateway broadcast | **Not reached for WS seeds** `[INFERRED]` | No seed messages to emit. |
| 5 Client `/live` | **Connected enough to wait 45s** `[PROVEN]` | Error is `only 0 events…`, not `connect_error` / `seed ping failed`. Whether `connect` fired and seeds started is **`[UNKNOWN]`** (stdout may be buffered; no Nest “live client” lines found — logger passes an object as the message). |

#### Now (this session, rebuilt images still running)

| Hop | Reached? | Evidence |
|---|---|---|
| 1 | Yes | Webhook HTTP 200; CH row for probe ids |
| 2 | Yes | Lag drains; offsets advance |
| 3 | Yes | `STATE=Stable` `MEMBERS=1` `TOTAL-LAG=0` |
| 4–5 | Yes | `node scripts/ws-smoke-test.mjs` → `WS SMOKE PASS: 3 events, types: ping_received` |

### 1.2 — Candidates

| # | Candidate | Verdict | Evidence |
|---|---|---|---|
| 1 | Consumer crashed (D5) | **Ruled out for the Sept 22 fail** `[PROVEN]` | No Crash in fail window. Group member continuous from Sept 21 join until Crash **next day**. |
| 2 | Choke on new message shape / nulls | **Ruled out** `[PROVEN]` | `kafka-live-consumer.service.ts` builds a fixed field set before zod; new duration / Phase 2A keys never enter `pingEventPayloadSchema`. Live probe: seed → `ping_received` for matching `call_id`. |
| 3 | Backlog / lag behind backfill | **Not proven as the Sept 22 cause** | Backfill **did** publish `published=2327` then `127` at `2026-09-22T21:46:48Z` (`[PROVEN]`). Fail was ~21 minutes later. Lag at fail time was **not recorded** (`[UNKNOWN]`). Reproduction: pause API → 3000 msgs → `TOTAL-LAG=3002` → unpause → lag **0 within ~2s**; ws-smoke still PASS. Drain rate too high for 2450 msgs to occupy a 45s window by themselves `[PROVEN]`. |
| 4 | Server-side `smoke-` filter on live path | **Ruled out** `[PROVEN]` | No `smoke-` / demo exclusion under `services/api/src/live/`. Exclusion is in `demo-rows.ts` (SQL / browser), not the Kafka→WS path. |
| 5 | Other | See below |

#### Other findings (not the named Sept 22 cause)

1. **KafkaJS Crash recovery gap** `[PROVEN]` — At `2026-09-23T19:24:35Z`, `calyx-api-live-feed` logged `Consumer Crash: … Fetch … timed out`, then `Restarting the consumer in 7676ms`, then `Stopped`. Next `Starting` / join is `2026-09-23T19:55:30Z` (**~31 minutes**). During that window the live feed would miss events (D5-class symptom). **This is after the Phase 2A-fix gate** and does not explain it.
2. **ws-smoke can false-PASS on non-ping traffic** `[PROVEN]` — Earlier this session, smoke counted `conversion_attributed` / `call_scored` and exited before seeded `ping_received` mattered. Gate is not ping-specific.
3. **Missing CH smoke rows in the fail window** `[PROVEN]` — Distinct from a dead consumer (which would still show seeds in CH via the CH consumer group). Pattern fits “seeds never completed before `process.exit`”, but **why** (hung `/live` connect vs hung first `go run` vs other) is **`[UNKNOWN]`**. Warm `go run` ×3 + delays ≈ 13s `[PROVEN]` — not enough alone to explain a 45s miss under current load.

### 1.3 — Fix

**No fix applied.** A green ws-smoke rerun without a named cause is not a fix (spec). Backlog product proposal is deferred until backlog is proven as the fail cause.

**Proposed (not implemented):**

- Capture `rpk group describe calyx-api-live-feed` (STATE / MEMBERS / TOTAL-LAG) at the start of `ws-smoke-test.mjs` and on failure.
- Watchdog / explicit restart when KafkaJS emits Crash for `calyx-api-live-feed` (addresses the proven 31-minute dead window).
- Optionally require ≥1 `ping_received` in smoke so attribution noise cannot mask a dead ping path.

### DoD 1

| Required | Status |
|---|---|
| Hop table | Done (fail + now) |
| Candidates ruled in/out | Done |
| Root cause in one sentence | **FAIL — `[UNKNOWN]`** |
| ws-smoke PASS | Passes **now**; does not close the regression without a named cause |

**Root cause (required one sentence):** **`[UNKNOWN]` — evidence rules out crash / message-shape / live `smoke-` filter for the Sept 22 fail; seeds did not land in ClickHouse during the WS window, but the mechanism that prevented seeding is not identified; lag-at-fail-time was not measured.**

---

## Part 2 — Per-integration sync single-flight

**Not implemented (STOP).** Facts gathered while investigating:

| Fact | Value | Label |
|---|---|---|
| `SyncInterval` | `SYNC_INTERVAL_SEC=120` (2 minutes) on `deploy-platform-sync-1` | `[PROVEN]` |
| Run timeout | `10 * time.Minute` on scheduled ticks and `POST /sync` | `[PROVEN]` (`cmd/server/main.go`) |
| Per-integration lock today | **None** for ticks or `POST /sync` (Redis SetNX exists only for write-back) | `[PROVEN]` |
| Overlapping runs (ce6fd338) since 2A-fix deploy | At least **one** clear triple finish: `21:46:48.714` `published=2327`, `21:46:48.906` `published=127`, `21:46:49.185` `published=0` — finish deltas &lt; 1s | `[PROVEN]` |
| `sync_cursor` write | Unconditional `UPDATE integrations SET sync_cursor = p_cursor … WHERE id = p_id` via `update_integration_sync_state` | `[PROVEN]` |

---

## Part 3 — Deploy checklist

**Not updated (STOP).**

---

## Part 4 — Gates, commit, scan

**Not run / not committed / not pushed (STOP).**

---

## Stop

Phase 2B was not started. Operator decision needed before any guess-fix or before continuing Parts 2–4 under a waived Part 1 cause.
