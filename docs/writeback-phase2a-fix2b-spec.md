# Calyx — Phase 2A-fix2b: Diagnosable Live Gate + Consumer Recovery + Single-Flight

**Scope:** make the live-feed gate honest and self-diagnosing, stop the live consumer from
dying silently, and prevent concurrent syncs. **No Phase 2B. No status logic. No UI. No
deletions. No write to Ringba.**

**Deliverable:** `docs/writeback-phase2a-fix2b-report.md`

## ⛔ RULE
Parts are **independent**. If one part blocks, report it and continue with the others —
do not stop the whole file. If a step is ambiguous, stop **that part** and report.
Label claims `[PROVEN]` / `[INFERRED]` / `[UNKNOWN]`. Evidence from rebuilt images.

## Operator decision (recorded)
The 2026-09-22 WebSocket failure has **no recoverable cause**: no lag, timestamps or seed
status were captured at the time. It is closed as `[UNKNOWN — unrecoverable]`. The
requirement is replaced by: make the next failure diagnose itself, and reproduce if
possible.

---

# PART A — Make ws-smoke honest and self-diagnosing

**A1 — Require the ping path.** Pass only if **≥3 `ping_received` events whose `call_id`
matches the seeds this run sent**. Other event types are logged but do not count.
This strengthens the check; it must not be relaxed anywhere else.

**A2 — Log every step with timestamps:** socket `connect` time; each seed's HTTP status
and elapsed time (including the signing step); each event received (type, call_id, time);
on exit, which seeds were sent / not sent and which were received / not received.

**A3 — Capture consumer state:** `calyx-api-live-feed` STATE / MEMBERS / TOTAL-LAG at test
start and on failure, printed in the output.

**A4 — Reproduce attempt.** Run ws-smoke **10 times** with sync live, then **5 times while a
backfill is publishing** (clear the Ringba cursor as in 2A-fix to trigger it). Report every
run's result. If any fails, the A2/A3 output must name where it stopped — report that.

# PART B — Live consumer must not die silently

`[PROVEN]` 2026-09-23 19:24:35Z: KafkaJS `Consumer Crash` (fetch timeout) → "Restarting in
7676ms" → `Stopped` → no rejoin until 19:55:30Z (**31 minutes**). D5 (Delta 2) was the same
class with a different trigger.

**B1 — Report first:** why KafkaJS's own restart ended in `Stopped` (retry exhausted?
`restartOnFailure` behaviour?), and the compose `restart:` policy for `api`.

**B2 — Recover:** on crash, the consumer must come back — reconnect with backoff, and if it
cannot, fail the process so the container restarts. Choose the mechanism and justify it.
Apply the same to every KafkaJS consumer in the API (`createKafkaClient()` users).

**B3 — Make it visible:** `/readyz` (already checked by `verify-system`) must report
not-ready when the live consumer is not a group member. A dead live feed must turn the gate
red on its own.

**B4 — Prove:** force a crash or broker interruption (e.g. briefly stop Redpanda) → show
`/readyz` go not-ready → consumer rejoins → ws-smoke passes. Report time-to-recover.

# PART C — Per-integration sync single-flight

Facts already `[PROVEN]` (fix2): `SYNC_INTERVAL_SEC=120`; timeout 10m; **no** lock on ticks
or `POST /sync`; three runs of `ce6fd338` finished within 1s at 21:46:48; `sync_cursor` is
overwritten unconditionally.

**C1 — Implement:** one run per integration across scheduled ticks and `POST /sync`.
**Must work across replicas** (Redis `SET NX` + TTL, or Postgres advisory lock — justify).
TTL > 10-minute timeout with margin. Held lock → tick **skips** and logs; `POST /sync`
returns **409 already running**. Crash releases by TTL.

**C2 — Prove:** unit test (two concurrent runs → one executes); live (two simultaneous
syncs → one runs, one skipped/409, both log lines); several ticks during a backfill → no
overlapping runs in logs.

# PART D — Checklist, gates, commit, scan

**D1 —** `docs/deploy-checklist.md`, under **Data**:
```
- [ ] **Retreaver pagination** — the Retreaver adapter pages with `page`/`per_page`
      over a changing `updated_at` window: the same pattern that silently skipped 431
      Ringba calls. Apply the windowed approach before re-enabling Retreaver sync.
```
**D2 —** Gates with sync live: `verify-system` (exit 0, 15/15, 11/11, WebSocket PASS with
the stricter A1 rule), `verify-ingestion-slice`, `verify-presentation`.
**D3 —** Commit locally; report hash + clean `git status`.
**D4 —** Secret scan of all commits since `8f4c8f7` (tokens, real internal token, non-dev
keys, bearer tokens, ANI/phone numbers in evidence). **Do NOT push.**

## Out of scope
Phase 2B · live-badge UI change · Retreaver adapter changes · dedup identity · deletions ·
any write to Ringba · authenticating `/sync` and `/test-auth`
