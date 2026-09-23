# Phase 2A-fix report

Phase 2A-fix only. Phase 2B was not started. No status column, filter, or list UI. No rows deleted. No write to Ringba. Retreaver API sync stayed off.

Images rebuilt after the changes: `deploy-platform-sync`, `deploy-consumer`, `deploy-ingestion`. Evidence below is from those images and live ClickHouse / Ringba reads.

## Part 1 — Windowed pagination

### What shipped

Normal Ringba sync no longer walks one large window with offset pages. `FetchPage` calls `fetchWindowed`:

1. Split the requested range into day slices.
2. Request each slice with `size=1000`, `offset=0`.
3. If a slice returns a full page, split it in half and fetch both halves.
4. Repeat until every slice returns fewer than 1000 rows.
5. If a window of **1 minute** or shorter still fills a page, offset-page **that window only**, log `RINGBA_OFFSET_FALLBACK`, and count it in `offset_fallback_windows` / `offset_fallback_rows`.

File: `services/platform-sync/internal/adapter/ringba_windowing.go` (renamed from `ringba_windows.go` — Go treats `*_windows.go` as Windows-only, so the Linux Docker build omitted it until the rename). `[PROVEN]`

Ongoing ticks use the same path: the trailing overlap cursor is still a time range; `FetchPage` feeds that range into `fetchWindowed`. `[PROVEN]`

### `page1_refresh`

Registry still appends a page-1 refresh when `pagesFetched > 1` and `HasMore`. Windowed Ringba sets `HasMore=false` after one `FetchPage`, so **`page1_refresh` does not run for Ringba** (`page1_refresh=false` on every log line below). It remains for other platforms that still deep-page. It was not removed. `[PROVEN]`

### Sort / `orderByColumns`

Live probe: `orderByColumns: [{"column":"callDt","direction":"asc"}]` → **HTTP 422** `"Invalid sort column callDt"`. `[PROVEN]`

Empty `orderByColumns` is accepted. A documented valid sort column for this account was not confirmed. The request body does **not** send `orderByColumns`. Windowed single-page fetches do not rely on sort order. `[PROVEN]`

### Stability (closed window)

Window fully ended one hour before the walk: `2026-08-23T20:16:26Z`–`2026-09-22T20:16:26Z`.

| | Walk 1 | Walk 2 |
|---|---|---|
| rows | 4043 | 4043 |
| distinct `inboundCallId` | 4043 | 4043 |
| requests | 30 | 30 |
| offset fallback windows | 0 | 0 |
| ids only in walk 1 | 0 | — |
| ids only in walk 2 | — | 0 |

**Pass.** `[PROVEN]`

30-day backfill request count on the deployed sync: **30** (certified rate posture unchanged; ~40s of wall time for the fetch alone). `[PROVEN]`

### Rows vs distinct (1.3)

On the closed windowed walks: **rows = distinct = 4043**. `MULTI_ID=0` on a separate windowed walk of another 30-day closed range (4025 = 4025). `[PROVEN]`

The earlier offset walks’ row>distinct gap was pagination drift, not a second row per call in these windows. Revenue Recovery multi-target rows were **not** observed here. Dedup identity was not changed.

### Retreaver (report only)

Retreaver uses `page` / `per_page` over an `updated_at_start` window (`retreaver_adapter.go`). That is page-offset style over a changing set. **Not changed.** `api-sync-verify-024145` remains `is_active=false`. `[PROVEN]`

---

## Part 2 — Required duration columns

### Migration `000014`

| Column | Type | Source |
|---|---|---|
| `required_duration_seconds` | `Nullable(UInt32)` | `ringTreeWinningBidDynamicDuration` |
| `max_required_duration_seconds` | `Nullable(UInt32)` | `ringTreeWinningBidMaxDynamicDuration` |

Up and down ship. Compose init runs `000014` after `000013`. Applied on the running DB. `[PROVEN]`

### NULL ≠ 0

| Ringba record | Store |
|---|---|
| key absent | NULL |
| key present, `0` | 0 |
| key present, N | N |

Unit tests: `TestRingbaOptionalSecondsAbsentZeroPresent`, `TestPhase2RequiredDurationNullZeroPresent`, pingdedup hash distinguishes absent / 0 / N. `[PROVEN]`

### Raw JSON unit

Live samples: `"ringTreeWinningBidDynamicDuration":90` with Go type `float64` (also 15, 10). Whole numbers. Stored as seconds. `[PROVEN]`

### Mapper / hash

Both fields are on `valueColumns`, `RemoteCall`, Kafka `PingMessage`, consumer insert, and `pingdedup.Content` (nil → empty hash segment; present 0 → `"0"`).

---

## Part 3 — Backfill and proof

### Enable / clear

Cleared `sync_cursor` and `last_sync_at` on `ce6fd338` only. Left `sync_mode=real`, `is_active=true`, `writeback_enabled=true`. `[PROVEN]`

### Timeout (found while backfilling)

Scheduled `RunAll` was **2 minutes**. A windowed 30-day fetch (~30 requests) plus thousands of Kafka publishes exceeded that (`kafka.Write: context deadline exceeded`). `[PROVEN]`

Raised scheduled sync and `POST /sync` timeouts to **10 minutes** in `cmd/server/main.go`, rebuilt platform-sync, re-cleared cursor, re-ran. Report only — not a pagination change.

### Full backfill (rebuilt image)

```
calls=4055 published=2327 unchanged=1728 page1_refresh=false requests=30
offset_fallback_windows=0 offset_fallback_rows=0
```

A concurrent second full walk finished immediately after (`published=127 unchanged=3929 requests=30`). No `RINGBA_OFFSET_FALLBACK` log lines. `[PROVEN]`

### Reconciliation (same closed window)

| | Count |
|---|---|
| Ringba API distinct | 4043 |
| ClickHouse distinct (`occurred_at` in window, `RGB%`) | 4043 |
| In API, not in ClickHouse | **0** |
| In ClickHouse, not in API | **0** |

**Pass.** Phase 2A’s 431 missed ids are in this set (difference is zero). `[PROVEN]`

### Steady-state (no republish loop)

After backfill settled, consecutive 2h ticks (no new old-call churn):

| Time (UTC) | calls | published | unchanged |
|---|---|---|---|
| 21:54:42 | 18 | **0** | 18 |
| 21:56:41 | 18 | **0** | 18 |
| 21:58:41 | 18 | **0** | 18 |
| 22:00:41 | 18 | **0** | 18 |
| 22:02:41 | 18 | **0** | 18 |

Earlier ticks published 1–2 when `calls` rose 15→16→18 (new live calls). Old calls stayed **unchanged**. No hash ping-pong observed. `[PROVEN]`

---

## Part 4 — Rule on reconciled ClickHouse

Latest row per call in the closed window. Converted = `has_converted` and revenue > 0. Candidate = connected, not converted-with-amount, not blocked, not adjusted.

### Connected duration (`duration_seconds`)

| | required present, dur ≥ req | required present, dur < req | required NULL |
|---|---|---|---|
| Converted | **712** | **0** | **0** |
| Connected, not converted, not blocked, not adjusted | **1** | 2066 | 4 |

Converted below requirement: **0**. `[PROVEN]`

True ⚠ Miss candidate (dur ≥ required):

| Call | Target | dur | call_len | required | revenue |
|---|---|---|---|---|---|
| `RGB587DCF02F86A16B85286995AC559F7D4576BEDD1RGBD002` | RingbaX Final Expense paid-search RTT | 40 | 51 | 35 | 0 |

Same id as the Phase 2A probe’s single exception. `[PROVEN]`

`required = 0`: **4** calls; **2** of those converted. `[PROVEN]`

`required` NULL pool: **723** calls; **0** converted; duration min 0 / median 0 / max 96. `[PROVEN]`

### Whole-call length (`call_length_seconds`)

| | ≥ required | < required |
|---|---|---|
| Converted | 712 | 0 |
| Candidate | **112** | 1955 |

112 candidates meet the field on call length; only 1 on connected duration. The field aligns with **connected / talk time**, not whole-call length, on this data. `[PROVEN]`

---

## Enrichment

| | Count |
|---|---|
| Before republish (`count()`) | 3,643,180 |
| After raw `count()` | 4,003,540 |
| After `FINAL` | 2,187,180 |

ReplacingMergeTree collapsed duplicate versions; logical size is the FINAL figure, not the raw peak. `[PROVEN]`

---

## Gates (sync live)

Unit tests for windowing and NULL/0/N: adapter, pingdedup, consumer writer — **pass** before deploy. `[PROVEN]`

Full gate scripts (run with sync live):

| Script | Exit | Notes |
|---|---|---|
| `verify-system.ps1` | **1** | Docker 15/15, Healthz 11/11, unit tests PASS, web build PASS. **WebSocket smoke FAIL** (`only 0 events in 45000ms`). Pipeline E2E 4/4 PASS. |
| `verify-ingestion-slice.ps1` | **0** | PASS |
| `verify-presentation.ps1` | **0** | PASS |

WS failure is reported, not fixed in this phase. Not the default-KPI flake from 2A.9.

---

## Found, not productized

- Sync timeout had to move from 2m → 10m for a full 30-day windowed publish to finish.
- `orderByColumns` + `callDt` is rejected live.
- `page1_refresh` is idle for Ringba under windowing.
- One miss candidate remains on reconciled data; status UI is Phase 2B.

## Stop

Phase 2B was not started.
