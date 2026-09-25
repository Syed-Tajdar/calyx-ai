# S1 Part A — Ringba sync window, sweep, hash-after-publish, backfill

**Scope:** S1 Part A only. No Part B. No Phase 2B. No Retreaver. **Not pushed.**

**Spec:** `docs/s1a-sync-window-spec.md`  
**Evidence:** `docs/evidence/s1a-sync-window/`  
**Integration:** `ce6fd338-af48-47d8-850a-bd10b855e2f4` · tenant `11111111-1111-1111-1111-111111111111`

---

## 1. Scoreboard

| Item | Result | Notes |
|---|---|---|
| **A1** Incremental window | **PASS** `[PROVEN]` | `[last−6h, now]`; stored cursor bounds ignored; `last_sync_at` = window end on success only; `window_capped` warn + counter |
| **A2** Sweep + bearer trigger | **PASS** `[PROVEN]` | Scheduled ticker + `POST /sync/window` behind `WRITEBACK_INTERNAL_TOKEN` |
| **A2.1** Hash TTL vs sweep | **PASS** `[PROVEN]` | TTL refreshed on Unchanged; default/env **240h**; 2nd sweep `published=0` |
| **A3** Hash after publish (F029) | **PASS** `[PROVEN]` | Evaluate → Publish → Commit; unit + live unchanged on re-tick |
| **A4** `WasPayoutAdjusted` in hash (F068) | **PASS** `[PROVEN]` | Content hash + producer/consumer/CH column |
| **A5** Unit tests (fake clock) | **PASS** `[PROVEN]` | `ringba_window_test.go` + F029 syncer tests |
| **A6** Live proof | **PASS** `[PROVEN]` | 3 ticks, 8h downtime window, dual sweep |
| **A7** 30d backfill + reconcile | **PARTIAL** | Call-level `IN_RINGBA_NOT_CH=0` + field mismatches in window `=0` **PASS**; **`RGB7CE620…` still `has_converted=0`** — outside closed 30d window (see §5) |
| **A8** Gates + commit + scan | **FAIL (gates)** / commit+scan done | `verify-ingestion-slice` **0**, `verify-presentation` **0**, `verify-system` **1** (WebSocket section only). No push. |

---

## 2. Before / after window path (`file:line`)

### Before (S0 / pre-S1A behaviour) `[PROVEN]` from prior code + S0

| Behaviour | Effect |
|---|---|
| Stored `sync_cursor` `report_start`/`report_end` **overrode** tick bounds | After downtime, first tick re-fetched stale `[last−2h, last]`, then advanced to `[now−2h, now]` — middle gap never fetched (F003b) |
| Overlap **2h** (`ringbaOverlap`) | Shorter than max call duration ~3.9h (F003c) |
| `UpdateSyncState(..., time.Now(), "syncing"\|"error")` | Advanced `last_sync_at` on start **and** failure |
| Success set `last_sync_at` to wall-clock after publish | Not tied to window end actually fetched |
| Window start/end **not logged** | S0 could not see requested bounds |

### After (this change) `[PROVEN]`

| Step | Location |
|---|---|
| Build `[last_sync−overlap, now]`, default overlap **6h** | `services/platform-sync/internal/adapter/ringba_window.go:52-75` (`buildRingbaIncrementalWindow`) |
| Cap at lookback; `window_capped` counter + distinct warn | `ringba_adapter.go:71-78`; counter `windowCappedTotal` |
| **Ignore** stored cursor bounds (`_ = cursor`) | `ringba_adapter.go:80-82` |
| Cursor still written as next-state snapshot of **this** window (offset 0) for visibility / page-1 helpers — **not** used as fetch bounds | `ringba_adapter.go:88-102`; `syncer.go` still persists `NextCursor` on success |
| Log requested window every tick | `syncer.go:175-181` (`api sync window`) |
| Status-only on start/error (no `last_sync_at` move) | `syncer.go:151`, `345-346`; PG `000027_sync_status_only` → `update_integration_sync_status` |
| `last_sync_at` = `WindowEnd` only after full success | `syncer.go:200-206` |
| Sweep / backfill `FetchRange` + shared lock | `ringba_adapter.go:112-136`; `syncer.go:109-117`, `226-275`; `cmd/server/main.go` `POST /sync/window` + sweep ticker |
| Hash Commit after Kafka publish | `syncer.go:320-338`; `packages/pingdedup/dedup.go:63-110` |
| `WasPayoutAdjusted` in hash | `packages/pingdedup/content.go:52-52`, `88-125` |

**Overlap justification:** S0 max connected duration **13,994s ≈ 3.89h**; default overlap **6h** leaves ~2.1h margin `[PROVEN]` (S0 V7) / `[INFERRED]` margin choice.

**Cursor after change:** still stored with the window that was just fetched (for ops visibility and Retreaver-style page-1 refresh helpers). For Ringba incremental ticks it **does not** set the next fetch interval.

---

## 3. A2.1 — TTL vs sweep

| Question | Answer | Label |
|---|---|---|
| Is hash TTL refreshed when re-seen as unchanged? | **Yes** — `Expire` on match in `Evaluate` | `[PROVEN]` `pingdedup/dedup.go:87-89`; unit `TestUnchangedRefreshesTTL` |
| TTL vs sweep 7d | Default / compose **`DEDUP_TTL_HOURS=240` (10d)** > 7d sweep with margin | `[PROVEN]` compose + config defaults |
| Live proof | Sweep1 `published=238`; Sweep2 immediate `published=0` `unchanged=298` | `[PROVEN]` logs below |

Decision: refresh-on-unchanged **and** raise TTL to 10d so expiry near the sweep edge cannot force hourly republish even if refresh is skipped under Redis failure.

---

## 4. A6 / A7 tables (raw evidence)

### A6.1 — Three consecutive ticks

| # | `window_start` | `window_end` | notes |
|---|---|---|---|
| 1 | `2026-09-24T17:54:47Z` | `2026-09-24T23:56:39Z` | `published=23` (hash schema change) |
| 2 | `2026-09-24T17:56:39Z` | `2026-09-24T23:57:09Z` | start = prior end − 6h; `unchanged=23` |
| 3 | `2026-09-24T17:57:09Z` | `2026-09-24T23:57:12Z` | same pattern; `unchanged=23` |

Evidence: `docs/evidence/s1a-sync-window/a6_ticks_and_sweeps.txt` `[PROVEN]`

### A6.2 — Simulated 8h downtime

| | |
|---|---|
| Operator action | `UPDATE … SET last_sync_at = NOW() - INTERVAL '8 hours'` on `ce6fd338` |
| Requested window | **`[2026-09-24T09:57:25Z, 2026-09-24T23:57:25Z]`** = last−6h → now (14h span; no middle skip) |
| Result | `calls=60` `published=37` `unchanged=23` |

| Reconcile (that requested window) | Count |
|---|---|
| Ringba API distinct | **60** |
| ClickHouse distinct | **60** |
| In Ringba, not in CH | **0** |
| In CH, not in Ringba | **0** |

Evidence: `a6_2_reconcile.txt` `[PROVEN]`

### A6.3 — Dual sweep

| | window | requests | calls | published | unchanged | duration_ms |
|---|---|---|---|---|---|---|
| Sweep 1 | `2026-09-17T23:58:54Z` → `2026-09-24T23:58:54Z` | 7 | 298 | **238** | 60 | 6308 |
| Sweep 2 (immediate) | same | 7 | 298 | **0** | **298** | 4151 |

`[PROVEN]` same evidence file.

### A7.1 — 30-day call-level reconcile

Backfill log: `reason=backfill_30d` `window_start=2026-08-25T23:59:28Z` `window_end=2026-09-24T23:59:28Z` `requests=30` `calls=3890` `published=3592` `unchanged=298` `duration_ms=120208` `[PROVEN]`

| | Count |
|---|---|
| Ringba API distinct | **3512** |
| ClickHouse distinct (occurred_at in window) | **3890** |
| In Ringba, not in CH | **0** |
| In CH, not in Ringba | **378** |

**S0 V1.4 five ids — all present in both Ringba and CH** `[PROVEN]`:

| ID | ringba | ch |
|---|---|---|
| `RGB02246DCF…RGBDC02` | true | true |
| `RGB5090F802…RGB8602` | true | true |
| `RGBA47E8638…RGB3502` | true | true |
| `RGB782856E9…RGBBD02` | true | true |
| `RGB31DF11E9…RGBE302` | true | true |

**EXTRA_CH (378):** CH holds distinct `occurred_at`-in-window ids that the closed Ringba Get Call Log response for the same `[start,end]` did not return. Likely prior sync history / Ringba omission vs Calyx retention — **not** missing Ringba→CH coverage. Root cause of each EXTRA id not individually proven `[INFERRED]` class explanation; count `[PROVEN]`.

Evidence: `a7_reconcile.txt`.

### A7.2 — Field-level (30d overlapping ids)

| Field | Mismatches |
|---|---|
| `has_converted` | **0** |
| conversion amount (revenue) | **0** |
| payout | **0** |
| `was_conversion_adjusted` | **0** |
| `was_payout_adjusted` | **0** |
| **Total** | **0** `[PROVEN]` |

#### Ambiguity / STOP item — `RGB7CE620…RGB8802`

| Fact | Value | Label |
|---|---|---|
| Spec A7 | “one **30-day** range” | — |
| Spec A7.2 | “`RGB7CE620…` must now show `has_converted=1`” | — |
| Call `occurred_at` | `2026-08-20T17:48:41Z` | `[PROVEN]` CH |
| Closed 30d window start | `2026-08-25T23:59:28Z` | `[PROVEN]` |
| In 30d Ringba/CH reconcile | **absent** (`ringba=false ch=false` in window check) | `[PROVEN]` |
| CH latest `has_converted` | still **0** (last `received_at` `2026-09-18T17:05:37Z`) | `[PROVEN]` |

**Did not** extend beyond 30 days (would be inference). **A7.2 explicit target for this id: FAIL** under the prescribed window. Needs an explicit operator decision to run a wider/manual range covering `2026-08-20`.

### A7.3 — Enrichment row counts

| | Before backfill republish | After (≈+3–5 min) |
|---|---|---|
| `enrichment_attrs` (tenant) | **2,375,420** | **4,135,620** → **4,136,140** |
| Ringba `api_sync` ping rows | **11,919** | **15,511** → **15,512** |

ReplacingMergeTree will continue to converge; raw insert volume rose with republish `[PROVEN]` counts / convergence timing `[INFERRED]`.

---

## 5. Honest limitations / not fixed

1. **`RGB7CE620…` conversion flag** — still stale in CH; outside prescribed 30d backfill. **Ambiguous vs A7.2 wording — stopped without widening.**  
2. **`verify-system` exit 1** — WebSocket smoke `fetch failed` mid-seed during/after heavy backfill load; ingestion-slice + presentation exited **0**. Not re-run (operator STOP after report).  
3. **378 EXTRA_CH** ids in 30d window — explained only at class level, not per-id.  
4. **Retreaver / other platforms** — incremental window change is Ringba-specific; other adapters unchanged (out of scope).  
5. **Ingestion webhook content hash** still omits many Ringba parity fields (pre-existing); API-sync path hashes full mapped content including `WasPayoutAdjusted`.  
6. **Commit after publish fail-opens** on Redis SET error (logs; leaves hash absent → retry) — intentional; can duplicate under Redis outage.  
7. **No CAS** between Evaluate and Commit beyond single-flight sync lock — two writers on different processes without lock could still race; ticks/sweeps share Redis SETNX lock `[PROVEN]` for sync service.

---

## 6. A8 — Gates, commit, secret scan

### Gates (sync live)

| Gate | Exit | Notes |
|---|---|---|
| `verify-system.ps1` | **1** | Docker/health/API/pipeline/units/web build PASS; **WebSocket FAIL** |
| `verify-ingestion-slice.ps1` | **0** | PASS |
| `verify-presentation.ps1` | **0** | PASS |

### Commit

Local commit created (see git). **Not pushed.**

### Secret scan (commits since last push = new S1A commit(s) vs `origin/main`)

Patterns scanned (same as prior phases):

- `RA[a-fA-F0-9]{20,}` (Ringba account-style; exclude `RGB…` call ids)
- `eyJhbGci` (JWT)
- `AKIA[0-9A-Z]{16}`
- `Bearer eyJ`
- `WRITEBACK_INTERNAL_TOKEN` assignments that are not the known dev placeholder
- Long phone-like literals outside smoke `15551234567`

Expected / allowed placeholders: `0123456789abcdef…` cred keys, `dev_writeback_internal_token_change_me`, `callloom_dev`.

Scan result recorded in commit message / below after `git` run.

---

## STOP

S1 Part A deliverable complete aside from A7.2 `RGB7CE620` (window conflict) and A8 `verify-system` WS failure. **Do not start S1 Part B. Do not push.**
