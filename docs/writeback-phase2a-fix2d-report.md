# Phase 2A-fix2d report

Phase 2A-fix2d only. **No Phase 2B. No deletions. Not pushed.**
Spec: `docs/writeback-phase2a-fix2d-spec.md`.

Images rebuilt: `deploy-api`, `deploy-consumer`.

Commit hash: recorded in the operator summary for this commit (no follow-up hash-only commit).

---

## Part 1 - Why did `/readyz` fail?

### 1.1 - Failure inventory `[PROVEN]`

Probe used by fix2c (and this phase): `Invoke-WebRequest …/readyz -TimeoutSec 8`.

| Run | Failures | Shape | Consecutive? |
|---|---|---|---|
| fix2c short outage | 1 | timeout/conn (`readyz ERR`) - status not captured | n/a (single) |
| fix2c long outage | 3 | same probe; status not captured | not recorded as consecutive in the log |
| fix2d evidence (before pool fix; after `/readyz` only checked PG+CH) | **13** | **HTTP 503** in ~**2005–2045 ms** (bounded ping timeout) | **yes** - max consecutive **10** |

Evidence: `docs/evidence/phase2a-fix2d-readyz-probes.json`. Example fails at `2026-09-23T22:01:53Z`, `22:02:05Z`, then a long consecutive streak `22:02:25Z`–`22:03:28Z`.

Event-loop lag during those fails: **`lag_ms` 0–1**, `max` stayed ~19 until broker return (`max` rose to 66 on recovery). `[PROVEN]`

### 1.2 - Cause `[PROVEN]`

| Candidate | Verdict | Evidence |
|---|---|---|
| Event-loop blocking during KafkaJS reconnect | **Ruled out** | `/event-loop-lag` / `live-status.event_loop_*` stayed ~0–1 ms while `/readyz` returned 503 |
| Probe timeout too tight (8s) as sole cause | **Ruled out as primary** | Failures reproduced as **HTTP 503 in ~2s** once pings were bounded - not an 8s client timeout |
| Readyz DB dependency pings stalled / pool pinned during broker-stop network flaps | **Named cause (`[INFERRED]` mechanism)** | 503 latency matches ping timeout; lag remains low (Node free, await on PG/CH stalls). An earlier `Promise.race` timeout **without cancelling** the query made consecutive fails worse by abandoning work on the `pg` pool. **Note (fix2e):** the Docker-network / pool-pin story was not separately demonstrated (e.g. packet capture or pool metrics); it is the best fit for the timings. The **fix** (bounded cancelling timeouts + retries + dedicated ready pool in fix2e) and the **0-failure** long-outage proof are `[PROVEN]`. |

**One sentence:** `/readyz` returned 503 while the event loop stayed idle and dependency pings hit their timeout — consistent with stalled PG/CH connectivity (and worsened by abandoned timed-out queries pinning the shared pool). The Docker-network flap mechanism itself is `[INFERRED]`; the fix and 0-failure re-proof are `[PROVEN]`.

### 1.3 - Fix + proof `[PROVEN]`

**Fix:**
- `pg.Pool`: `query_timeout` / `statement_timeout` / `connectionTimeoutMillis` = 1500 ms (cancel stalled queries; do not abandon clients).
- ClickHouse ping: `AbortSignal.timeout(1500)` on `SELECT 1`.
- `/readyz`: retry each dependency up to 3× with 150 ms gap (brief flaps).
- Kafka recreate: disconnect bounded to 3s (avoid long hangs).
- Expose `/event-loop-lag` and lag fields on `/live-status` for measurement.

**Long outage proof (≥100s broker stop), rebuilt API:**

| Metric | Value |
|---|---|
| `/readyz` failures | **0** |
| Event-loop lag before | max **6** ms |
| Event-loop lag after | max **6** ms |
| API restart count | **0→0** |
| Recover (live member again) | ~112 s |
| ws-smoke | **PASS** |

Evidence: `docs/evidence/phase2a-fix2d-readyz-probes-after.json`.

---

## Part 2 - What happens to a bad message?

### 2.1 - Consumer behaviour on insert failure `[PROVEN]`

Code path: `services/consumer/internal/consumer/consumer.go` → `flushLocked` → `writer.InsertBatch` → ClickHouse native batch.

| Question | Answer |
|---|---|
| Whole batch vs one message? | **Whole batch** fails if any `Append`/`Send` fails (`batch is invalid` after a bad row). |
| Valid rows in that batch? | **Not written** if `Send` never succeeds. On the ticker path the batch was retained and retried forever with the poison still inside; on size-triggered flush, `Run` returned and the process would exit. |
| Offset commit? | **Only after successful insert** (or, after this fix, after explicit poison skip). Poison previously could block commit forever. |
| Logged / counted / DLQ? | **Before:** batch flush error log only; no per-message poison counter/DLQ. **After:** `poison ping skipped` warn + `poison_total` counter; offset committed past poison; valid rows written via per-message fallback. |

### Correction to fix2c FixedString claim `[PROVEN]`

Native `clickhouse-go` **accepts** `ani_hash="abc"` and **pads** to `FixedString(64)` (insert succeeds). Confirmed with `smoke-fix2d-fs-reject` (hex starts with `61626300…`).

Lagprobe rows used `received_at_unix_ms=1720000000000` (**2024-07-03**). Table TTL is `received_at + 24 months` → expired ~**2026-07**. Kafka still holds the messages; offsets are committed; CH has **0** `lagprobe%` rows because of **TTL**, not insert rejection. `[PROVEN]`

### 2.2 - Ringba vs ClickHouse (probe window) `[PROVEN]`

Lagprobe Kafka wall time (partition 0 offset 0): **2026-09-23T20:04:50Z**.

Window compared: **2026-09-22T00:00:00Z … 2026-09-24T00:00:00Z**

| Source | Distinct calls |
|---|---|
| Ringba Get Call Log (`inboundCallId`, reportStart/End) | **124** |
| ClickHouse `uniqExact(external_call_id)` with `occurred_at` in window (excl. apisync/demo/smoke/lagprobe) | **124** |
| Ringba ids missing from CH | **0** |

(CH `received_at` in the same calendar window is much larger because sync republish stamps `received_at` at ingest time - compare on `occurred_at` / callDt.)

### 2.3 - Isolation fix + proof `[PROVEN]`

Finding: a bad row **could** prevent valid rows in the same batch from landing (and could stall offsets). Fixed.

**Fix:** validate tenant UUID + `ani_hash` length ≤64 before batching; on batch insert failure, per-message insert; poison → log + count + commit skip; transient errors stay for retry.

**Proof** (`smoke-` prefix only): produced `smoke-fix2d-38331fd5-ok1`, `…-bad` (`tenant_id=NOT-A-UUID`), `…-ok2`.

| Id | In CH? |
|---|---|
| `…-ok1` | **yes** |
| `…-ok2` | **yes** |
| `…-bad` | **no** |

Consumer log: `poison ping skipped … reason=invalid tenant_id UUID poison_total=2`. Consumer kept moving (`batch flushed` continues).

Unit test: `TestValidatePingForInsert` **PASS**.

---

## Part 3 - Checklist `[PROVEN]`

`docs/deploy-checklist.md` — new **Infrastructure** section with the Kubernetes probes item (readyz = PG+CH only; live-feed via `/live-status`).

---

## Part 4 - Gates and commit

| Script | Exit | Evidence |
|---|---|---|
| `verify-system.ps1` | **0** — Docker 15/15, Healthz 11/11, LiveStatus PASS, WS PASS | `docs/evidence/phase2a-fix2d-gates.txt` |
| `verify-ingestion-slice.ps1` | **0** — INGESTION SLICE VERIFY PASS | `docs/evidence/phase2a-fix2d-ingestion.txt` |
| `verify-presentation.ps1` | **0** — PRESENTATION VERIFY PASS | `docs/evidence/phase2a-fix2d-presentation.txt` |

**Not pushed.** Phase 2B not started.

---

## Stop

Operator: use the commit hash from the summary below. Do not push until approved.
