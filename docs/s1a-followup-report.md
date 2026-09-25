# S1A follow-up report

**Scope:** S1A follow-up only. No S1 Part B. No Phase 2B. **Not pushed.**

**Parent:** `docs/s1a-sync-window-report.md` · commit base `3e22959`  
**Evidence:** `docs/evidence/s1a-followup/`  
**Integration:** `ce6fd338-af48-47d8-850a-bd10b855e2f4`

---

## Scoreboard

| # | Item | Result |
|---|---|---|
| 1 | Reconcile method + 378 EXTRA | **PASS** — A7 used offset-only; windowed re-run **3890=3890**, EXTRA **0** |
| 2 | Full-history backfill + reconcile | **PASS** — `RGB7CE620…` `has_converted=1`; field mismatches **0** |
| 3 | Daily deep sweep | **PASS** — tier added; 2× manual `published=0`; tick skip logged; `last_sync_at` untouched |
| 4 | `verify-system` | **FAIL** (WS) — lag ruled out; seed-path `fetch failed` `[PROVEN]` |
| 5 | PG `000027` down | **PASS** — present |
| 6 | `enrichment_attrs` count vs FINAL | **PASS** — recorded |

---

## 1 — Reconciliation method

### What A7 used `[PROVEN]`

Temporary probe `probe-s1a-reconcile` (deleted after S1A) called Ringba with a **single** `[start,end]` body and **offset += 1000** pagination — **not** `fetchWindowed`.

Sync / backfill uses `RingbaAdapter.fetchWindowed` → day slices (24h), binary-halve on full pages, offset only at ≤1m floor (`ringba_windowing.go:20-34`).

### Windowed re-run of the same 30d closed window

Window: `2026-08-25T23:59:28Z` → `2026-09-24T23:59:28Z`  
Evidence: `f1_windowed_30d.txt`

| | Offset-only (A7) | Windowed (follow-up) |
|---|---|---|
| Ringba distinct | 3512 | **3890** |
| CH distinct | 3890 | **3890** |
| In Ringba, not CH | 0 | **0** |
| In CH, not Ringba | **378** | **0** |
| Field mismatches | 0 (on overlap) | **0** |
| Fetch raw / requests | n/a | 3890 / 30; `FETCH_DUP_IDS=0` |

**378 explained:** the offset-only probe **under-counted Ringba** (missed 378 ids the day-windowed path returns). Those ids were already in CH from the windowed backfill (`calls=3890`). They were not “extra CH invents” — they were **Ringba rows invisible to offset paging on a multi-day window**. Under the sync’s windowed method the gap **disappears** (`IN_CH_NOT_RINGBA=0`).

---

## 2 — Full-history backfill

| | |
|---|---|
| Earliest real Ringba `occurred_at` in CH | **`2026-08-19T17:05:49Z`** `[PROVEN]` |
| Backfill range | `2026-08-19T17:05:49Z` → `2026-09-25T01:37:46Z` |
| Log | `reason=backfill_full_history` `requests=37` `calls=4716` `published=821` `unchanged=3895` `duration_ms=106754` `last_sync_at_unchanged=true` |
| `last_sync_at` before | `2026-09-25 01:35:47.728609+00` |
| `last_sync_at` after | **identical** `[PROVEN]` |
| Tick during backfill | `api sync skipped — already running` `lock_holder=backfill_full_history` `[PROVEN]` |

### Windowed reconcile (full range) — `f2_full_history_reconcile.txt`

| | Count |
|---|---|
| Ringba distinct | **4716** |
| CH distinct | **4716** |
| In Ringba, not CH | **0** |
| In CH, not Ringba | **0** |
| Field mismatches (total) | **0** |

| Field | Mismatches |
|---|---|
| `has_converted` | 0 |
| revenue | 0 |
| payout | 0 |
| `was_conversion_adjusted` | 0 |
| `was_payout_adjusted` | 0 |

**`RGB7CE620F7AA7EBCC2928D7DE052E853A2EC94BA76RGB8802`:** Ringba `hasConverted=true` revenue=18 payout=14.4; CH `argMax(has_converted)=1` revenue=18 `received_at=2026-09-25 01:38:12` `[PROVEN]`.

**Remaining mismatches with ids:** **none.**

---

## 3 — Daily deep sweep

### Code

| Piece | Detail |
|---|---|
| `RINGBA_DEEP_SWEEP_DAYS` | default **30** (`ringba_window.go`) |
| `RINGBA_DEEP_SWEEP_INTERVAL` | default **24h** |
| Hourly sweep | unchanged (`RINGBA_SWEEP_DAYS=7`, interval 60m) |
| Lock | same Redis SETNX; reasons `tick` / `sweep` / `deep_sweep` / `backfill_*` |
| Tick skip log | `lock_holder=<reason>` when lock held |
| Lock TTL | raised **15m → 45m** for long windowed runs |

### Does the hourly (or deep) sweep touch `last_sync_at`?

**Before follow-up:** `RunWindow` / `runWindowLocked` never called `UpdateSyncState` — only Publish + log. **Did not** advance `last_sync_at` `[PROVEN]` (S1A code).  
**After:** same; log field `last_sync_at_unchanged=true`. Live: deep-sweep before/after `last_sync_at` identical.

### TTL vs daily 30d sweep

`DEDUP_TTL_HOURS=240` with **Expire on Unchanged**. Deep sweep re-sees every call in the 30d range **daily** → TTL refreshed daily ≫ 24h interval → **no hourly/daily republish storm** from TTL expiry `[PROVEN]` reasoning + live 2× deep `published=0`.

### Live proof — `f3_deep_sweep_logs.txt`

| Run | window | duration_ms | published | unchanged |
|---|---|---|---|---|
| Deep 1 | `2026-08-26T01:46:10Z`→`2026-09-25T01:46:10Z` | **40866** | **0** | 3887 |
| Deep 2 (immediate) | same | **33334** | **0** | 3887 |

Tick skipped while deep held lock: **1** observed  
`api sync skipped — already running` `lock_holder=deep_sweep` at `2026-09-25T01:46:48Z` `[PROVEN]`.

(First deep also `published=0` because full-history backfill had already hashed the range — still satisfies “second ~0” and shows no storm.)

### Deploy checklist

Added under **Data**:  
*Ringba deep-sweep range (default 30 days) — confirm against the dispute window in buyer contracts; raise `RINGBA_DEEP_SWEEP_DAYS` if contracts allow longer.*

---

## 4 — Gate (`verify-system.ps1`)

| Section | Result |
|---|---|
| Docker / healthz / live-status / API / web / units / build / pipeline | **PASS** |
| WebSocket | **FAIL** |
| Overall exit | **1** |

Evidence: `f4_verify_system.txt`, `f4_ws_smoke_diag.txt`, `f4_ws_smoke_127.txt`.

### WS investigation (not marked load-related)

| Observation | Label |
|---|---|
| At failure: `STATE=Stable` `MEMBERS=1` `TOTAL-LAG=0` | `[PROVEN]` — **not** consumer lag / backlog |
| Seed 1 always `http=200` + matching `ping_received` | `[PROVEN]` |
| Fail message: `WS SMOKE FAIL: fetch failed` during **seed 2/3 in progress** | `[PROVEN]` |
| `sign_ms` via `go run ./cmd/platformsig` ≈ **14–25s** per seed | `[PROVEN]` |
| 3 rapid webhooks with **prebuilt** `platformsig.exe` all **200** | `[PROVEN]` — ingestion accepts sequential seeds |
| `WS_SMOKE_TIMEOUT_MS=180000` + `127.0.0.1` still same failure pattern | `[PROVEN]` |

**Verdict:** failure is in the **ws-smoke seed path** (Node `fetch` after / during slow `go run` signing), **not** live-feed lag or Kafka load. Exact undici errno for `fetch failed` not captured in smoke (`err.cause` not logged) → **`[UNKNOWN]`** at the TCP layer; load ruled out.

No S1 Part B / smoke-script fix applied in this follow-up.

---

## 5 — Postgres migration `000027` down

`db/postgres/migrations/000027_sync_status_only.down.sql`:

```sql
DROP FUNCTION IF EXISTS update_integration_sync_status(UUID, TEXT, TEXT);
```

**Present.** `[PROVEN]`

---

## 6 — `enrichment_attrs` count vs FINAL

Tenant `11111111-1111-1111-1111-111111111111`:

| When | `count()` | `count() … FINAL` |
|---|---|---|
| Before full-history backfill | 4,139,260 | 2,264,660 |
| After (post deep sweeps) | 4,532,380 | 2,303,140 |

Raw `count()` includes ReplacingMergeTree versions; `FINAL` is merged logical rows. Delta (raw − FINAL) ≈ **2.2M** versions still pending merge `[PROVEN]`.

---

## Commit / secret scan

Local follow-up commit (hash in git). Secret scan patterns: `AKIA…`, `eyJhbGci`, `Bearer eyJ`, non-dev `WRITEBACK_INTERNAL_TOKEN`, RA account ids (excl. `RGB…`). Dev placeholders only. **Not pushed.** No hash-only follow-up.

---

## STOP

S1A follow-up complete. Do not start S1 Part B. Do not push.
