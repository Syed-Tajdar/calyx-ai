# Calyx — Phase 2A-fix2: WebSocket Regression + Sync Single-Flight

**Scope:** close the red gate and a concurrency gap found in Phase 2A-fix. **No Phase 2B.
No status logic. No UI. No deletions. No write to Ringba.**

**Deliverable:** `docs/writeback-phase2a-fix2-report.md`

---

## ⛔ RULE

Do exactly this. Find causes by evidence before changing code. If a step is ambiguous or
reality differs from this document, STOP and report. Label claims `[PROVEN]` /
`[INFERRED]` / `[UNKNOWN]`. Evidence from rebuilt Docker images.

---
---

# PART 1 — WebSocket regression

## Why this is a regression, not a flake

| Run | WebSocket |
|---|---|
| Pre-Phase-2b gate | PASS |
| Phase 2A gate (sync live) | PASS |
| **Phase 2A-fix gate** | **FAIL — 0 events in 45000ms** |

Between the last PASS and this FAIL: platform-sync, consumer and ingestion were rebuilt;
two new nullable columns were added to the Kafka ping message; sync timeout went 2m → 10m;
a full backfill published ~2,450 messages to `callloom.pings.v1`.

Precedent: in Delta 2 (D5) the same symptom was the API live-feed consumer **crashed**
(KafkaJS Snappy codec), leaving consumer group `calyx-api-live-feed` with `MEMBERS=0`.

## 1.1 — Trace hop by hop (report evidence at each hop)

```
ws-smoke-test.mjs seeds signed webhook
  → ingestion :8080          hop 1: 200?
  → Kafka callloom.pings.v1  hop 2: message on topic?
  → API live consumer        hop 3: group state, MEMBERS, lag, offsets, API logs
  → WS gateway               hop 4: broadcast?
  → client /live             hop 5: connected, events received?
```

For each hop: **reached / not reached**, with the log line, offset, or group state.

## 1.2 — Candidate causes — rule each in or out with evidence

1. **Consumer crashed** (D5 pattern). `rpk group describe calyx-api-live-feed`: state,
   members, lag. API logs for `Consumer Crash` / KafkaJS errors.
2. **Consumer choked on the new message shape.** The ping message now carries
   `required_duration_seconds` / `max_required_duration_seconds` / the Phase 2A booleans,
   often as JSON `null`. Check whether the API live consumer's parsing/validation throws
   on `null` or an unknown key. Test by feeding it one real current-shape message.
3. **Backlog / lag.** The backfill put thousands of messages on the same topic the live
   feed reads. If the consumer is alive but behind, the smoke test's 3 events wait behind
   the backlog and miss the 45s window. Report lag at test time and drain rate.
4. **Test-data filter in the live path.** Smoke IDs now use `smoke-`. Confirm no
   server-side exclusion was added to the API live path (the exclusion belongs in the
   browser hook only). WS passed after the prefix change, so this is expected to be ruled
   out — confirm, don't assume.
5. **Anything else found.**

## 1.3 — Fix, then prove

- Fix the **named** cause.
- If the cause is **backlog (3)**, that is a product finding, not just a test issue: after
  any backfill, the dashboard's Live feed would lag for minutes while showing "Live".
  Report the measured lag and duration. **Propose** a fix (e.g. live feed ignoring
  republished historical calls, or a separate topic) — do not implement it in this phase.
  In that case, confirm WS passes once lag drains, and report both facts.
- `node scripts/ws-smoke-test.mjs` → ≥3 events (raw output).

**DoD 1:** hop table; each candidate ruled in/out with evidence; root cause in one
sentence; ws-smoke PASS.

---
---

# PART 2 — Per-integration sync single-flight

## Why

Phase 2A-fix reported: *"A concurrent second full walk finished immediately after
(`published=127`)"*. Ticks run every ~2 minutes; the sync timeout is now 10 minutes. A run
can therefore still be in progress when the next tick starts — **two syncs of the same
integration at once.**

Risk: both runs write `sync_cursor`. Whichever finishes last wins, even if its cursor is
older. A cursor moved backwards re-fetches; a cursor moved past un-fetched data **skips
calls silently** — the same failure class the pagination fix just closed.

## 2.1 — Report first

- Scheduler interval (`SyncInterval`) actual value.
- Whether any per-integration lock exists today, for scheduled ticks **and** for manual
  `POST /sync`.
- From platform-sync logs since Phase 2A-fix deployment: how many times two runs of the same
  integration overlapped. Evidence, not estimate.
- How `sync_cursor` is written at the end of a run — unconditional overwrite, or
  conditional?

## 2.2 — Implement single-flight

- One run per integration at a time, across scheduled ticks and manual `POST /sync`.
- **The lock must work across replicas** — production runs multiple pods, so an in-process
  mutex is not sufficient. Use a shared lock (Redis `SET NX` with TTL, or a Postgres
  advisory lock). Justify the choice.
- Lock TTL must exceed the maximum run time (the 10-minute timeout), with margin.
  A lock that expires mid-run re-opens the race.
- A tick that finds the lock held **skips** and logs it — it does not queue.
- Manual `POST /sync` while a run is in progress returns **409 already running**.
- On crash, the lock must release by TTL.

## 2.3 — Prove it

- Unit test: two concurrent runs for one integration → exactly one executes.
- Live: trigger two syncs of `ce6fd338` at the same moment → one runs, one reports
  skipped / 409. Show both log lines.
- After deploy, observe several ticks → no overlapping runs in the logs.

**DoD 2:** 2.1 facts; lock mechanism justified; unit + live proof.

---
---

# PART 3 — Deploy checklist

Add to `docs/deploy-checklist.md` under **Data**:

```
- [ ] **Retreaver pagination** — the Retreaver adapter pages with `page`/`per_page`
      over a changing `updated_at` window: the same offset-over-a-live-set pattern that
      silently skipped 431 Ringba calls. Apply the windowed approach before re-enabling
      Retreaver sync.
```

If Part 1 proves the backlog finding, also add a line recording that the Live feed lags
during backfills until the proposed fix ships.

---
---

# PART 4 — Gates, commit, scan

## 4.1 — Gates, with sync live

```
scripts/verify-system.ps1            → exit 0, Docker 15/15, Healthz 11/11, WebSocket PASS
scripts/verify-ingestion-slice.ps1   → exit 0
scripts/verify-presentation.ps1      → exit 0
```

## 4.2 — Commit locally

Commit all Phase 2A, 2A-fix and 2A-fix2 work. Report the hash and a clean `git status`.

## 4.3 — Secret scan — do NOT push

Scan everything in commits since `8f4c8f7` for: Ringba tokens / account credentials, real
`WRITEBACK_INTERNAL_TOKEN` values, encryption keys other than the known dev placeholder,
bearer tokens, caller phone numbers / ANI in `docs/evidence/`. List dev placeholders
separately.

**Do not push.** The operator approves the push after reading the scan.

## 🚦 STOP

Write `docs/writeback-phase2a-fix2-report.md` and stop. Do not start Phase 2B.

## Out of scope

- Phase 2B (status derivation, filter, list UI)
- Implementing the live-feed backlog fix if found (propose only)
- Retreaver adapter changes or activation
- Changing dedup identity; deleting rows; any write to Ringba
- Authenticating `/sync` and `/test-auth`
