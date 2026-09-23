# Phase 2A-fix2b report

Phase 2A-fix2b only. **No Phase 2B.** No status logic, UI, deletions, or Ringba writes.
**Not pushed.** Spec: `docs/writeback-phase2a-fix2b-spec.md`.

Operator decision (recorded): the 2026-09-22 WebSocket failure is closed as
`[UNKNOWN — unrecoverable]`. It was not re-investigated.

Images rebuilt this phase: `deploy-api`, `deploy-platform-sync`, `deploy-enrichment`.

---

## Part A — Diagnosable ws-smoke

### What shipped

`scripts/ws-smoke-test.mjs`:

- **A1** Pass only with ≥3 `ping_received` whose `call_id` is in this run’s seeds. Other types are logged and ignored for the count.
- **A2** Timestamped connect, each seed (HTTP + sign/http ms), each event, exit summary (sent / not sent / received / not received).
- **A3** `rpk group describe calyx-api-live-feed` STATE / MEMBERS / TOTAL-LAG at start and on failure.

### A4 — Reproduce runs `[PROVEN]`

| Series | Result |
|---|---|
| 10× quiet (sync live) | **10/10 PASS** |
| 5× during real backfill (`sync_cursor` cleared + `POST /sync` HTTP 200, concurrent windowed publish) | **5/5 PASS** (TOTAL-LAG=0 on each start line) |

Earlier attempt used a mangled PowerShell `Start-Job` body → HTTP 400 (not a real backfill). Re-run with `--data-binary @file` produced a real sync (`published` / `requests=30` in platform-sync logs) before/during the five smokes. No fail to diagnose.

---

## Part B — Live consumer recovery

### B1 — Why restart ended in `Stopped` `[PROVEN]` / `[INFERRED]`

KafkaJS `onCrash` (installed `kafkajs` consumer):

- Treats `KafkaJSNumberOfRetriesExceeded` as retriable for restart purposes.
- Logs `Restarting the consumer in ${retryTime}ms`, then `setTimeout(() => start(onCrash), retryTime)`.
- Always `await disconnect()` first → **`Stopped` is expected** before the delayed restart.

On 2026-09-23 the log showed Restarting (7676ms) then Stopped, then **Starting only at +31m**. That matches a **hung/delayed `start()` after disconnect**, not `restartOnFailure` returning false (`[INFERRED]` from KafkaJS source + log gap; root delay inside start/connect not captured).

Compose **`api` had no `restart:` key** → Docker default **`restart: no`**. Process exit would not bring the container back. **Now:** `restart: unless-stopped` on `api`. `[PROVEN]`

### B2 — Recovery mechanism `[PROVEN]`

Shared `services/api/src/kafka/resilient-consumer.ts`:

- `restartOnFailure: async () => true`
- Track membership via GROUP_JOIN / CRASH / STOP / DISCONNECT
- Non-restartable CRASH → `process.exit(1)`
- Watchdog: not a group member for **60s** → `process.exit(1)` (covers the 31m hang class)
- Used by **live feed** and **alert-engine** (both `createKafkaClient()` consumers). Producer-only `KafkaService` unchanged.

Justification: Redis/membership cannot fix a stuck KafkaJS timer; exiting the process + compose restart is the reliable recovery. TTL-style release is N/A; container recycle is.

### B3 — `/readyz` `[PROVEN]`

When Kafka live is required (`MOCK_EMITTER_ENABLED=false` path), `/readyz` returns **503** if `calyx-api-live-feed` is not a group member.

### B4 — Proof `[PROVEN]`

1. `docker stop deploy-redpanda-1` → `/readyz` **503** after ~14s of still-200 (membership cleared on crash/disconnect).
2. `docker start deploy-redpanda-1` → `/readyz` **200**.
3. **Time-to-recover (not-ready → ready): ~34.2s.**
4. Group Stable MEMBERS=1 TOTAL-LAG=0; `ws-smoke` **PASS** (stricter A1).
5. `API_RESTART_COUNT=0` this run — KafkaJS rejoined within the 60s watchdog without process exit.

---

## Part C — Sync single-flight

### Choice `[PROVEN]`

**Redis `SET NX` + TTL 15m** (same Redis already used by platform-sync / write-back locks). Works across replicas. TTL **> 10m** sync timeout with margin. Crash releases by TTL.

### C1 — Behaviour `[PROVEN]`

- Scheduled tick: lock miss → log `api sync skipped — already running`, continue.
- `POST /sync`: lock miss → **409** + log `api sync rejected — already running`.

### C2 — Proof `[PROVEN]`

| Proof | Result |
|---|---|
| Unit `TestSingleFlightTwoConcurrentRunsOneExecutes` + `TestSyncOneReturnsErrAlreadyRunningWhenLockHeld` | **PASS** (`go test ./internal/syncer/`) |
| Live dual `POST /sync` | HTTP **200** + HTTP **409** `{"status":"already_running"}`; log `api sync rejected — already running` |
| Tick during long run | log `api sync skipped — already running` at `2026-09-23T20:46:52Z` while a manual/windowed run held the lock |

No overlapping `api sync completed` pair within 1s observed after the lock shipped (unlike 21:46:48 on Phase 2A-fix).

---

## Part D

### D1 — Checklist `[PROVEN]`

`docs/deploy-checklist.md` **Data** section includes Retreaver pagination item.

### D2 — Gates (sync live) `[PROVEN]`

| Script | Exit |
|---|---|
| `verify-system.ps1` | **0** — Docker **15/15**, Healthz **11/11**, WebSocket **PASS** (matching `ping_received`) |
| `verify-ingestion-slice.ps1` | **0** |
| `verify-presentation.ps1` | **0** |

Evidence: `docs/evidence/phase2a-fix2b-gates.txt` (and sibling ingestion/presentation transcripts).

**Side fix (gate unblock):** enrichment panicked on short `ani_hash` (`abc` from earlier lag-probe traffic) via `aniHash[:8]` in mock provider + consumer logs. Safe prefix logging/enrichment shipped; enrichment image rebuilt. `[PROVEN]`

### D3 — Commit `[PROVEN]`

- Hash: **`288f0ffc06b2950e3199af25c0916a6895ac3925`** (`288f0ff`)
- `git status`: clean working tree; branch **ahead of `origin/main` by 1** (not pushed)

### D4 — Secret scan (since `8f4c8f7`) — **do not push**

| Class | Finding |
|---|---|
| Ringba account tokens (`RAa…`) | **None** in scanned diffs/docs |
| Real `WRITEBACK_INTERNAL_TOKEN` | **None** — only `dev_writeback_internal_token_change_me` |
| Non-dev encryption keys | **None** — only `0123456789abcdef`×4 placeholders |
| Bearer JWTs in evidence | **None** in phase2a-fix2b evidence files |
| Caller ANI in evidence | Smoke uses **`15551234567`** (known fake) in scripts; not production ANI dumps |

**Dev placeholders (expected):** `0123456789abcdef…` keys, `dev_writeback_internal_token_change_me`, `callloom_dev_jwt_secret_…`, smoke `15551234567`.

---

## Stop

Phase 2B was not started. **Do not push** until the operator approves after reading this scan.
