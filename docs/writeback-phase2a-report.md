# Write-back Phase 2A report

Phase 2A only. Phase 2B was not started. No status column, filter, or call-list UI.

Ringba Live `ce6fd338-af48-47d8-850a-bd10b855e2f4` is `is_active=true`, `sync_mode=real`, `writeback_enabled=true`. Retreaver `api-sync-verify-024145` is still `is_active=false`. No other integration row was changed except clearing this row's cursor (below).

Images rebuilt before sync was enabled: `deploy-platform-sync`, `deploy-consumer`, `deploy-ingestion`. Global `mock_mode` in the log is the process default; this integration's `sync_mode=real` selects the Ringba adapter. `[PROVEN]` the first sync's fetched row count matches a live Get Call Log walk (4025 + page-1 refresh).

## Migrations

| Migration | What |
|---|---|
| `000012_pings_has_converted.down.sql` | `DROP COLUMN has_converted`. Checklist item ticked. |
| `000013_pings_phase2a_call_flags.up.sql` | applied on the running ClickHouse |
| `000013_pings_phase2a_call_flags.down.sql` | ships with the up; not executed |
| `deploy/docker-compose.yml` | fresh init runs `000013` after `000012` |

`has_converted` stays `UInt8`. The new booleans are `Nullable(UInt8)`. `block_reason` is `String DEFAULT ''`.

## Null versus false

**Choice:** `NULL` = not provided. `0` = false. `1` = true.

Webhook JSON and every row written before this phase omit the keys. ClickHouse default for a new `Nullable` column is `NULL`, and the consumer writes `NULL` when the pointer is nil. The API-sync producer always sends the bool, including `false`, because `omitempty` on a Go `bool` would drop `false` and make it look like not provided. `[PROVEN]` unit test `TestPhase2FalseIsDistinctFromMissing`. `[PROVEN]` a webhook row written by the rebuilt consumer after the gates, `smoke-9ca76e18-e28a-48a8-a48d-559f7a72423a`, has all three flags `NULL`.

The API mapper requests `hasConnected`, `wasBlocked`, `blockReason`, and `wasConversionAdjusted`. A missing key on that response is stored as `false`, not `NULL`.

### Omission check, before sync, real records

30-day window `2026-08-22T23:17:28Z`–`2026-09-21T23:17:28Z`. No phone column requested. `[PROVEN]`

| Check | Result |
|---|---|
| `hasConnected` absent | 1260 |
| of those, connected duration 0 | 1260 |
| of those, `wasBlocked` true | 708 |
| absent and duration > 0 | **0** |
| `wasBlocked` absent | 3317 |
| of those, a non-empty `blockReason` | **0** |
| of those, `hasConnected` true | 2765 |
| explicit `false` for either flag | 0 |

Ringba omitted the false value instead of sending `false`. Absence of `hasConnected` was only on zero-duration calls, including the blocked ones. Absence of `wasBlocked` never carried a block reason, and 2765 of those calls were connected. A second walk of the sync window (`…23:31:29Z`) repeated the same two zero-failure checks (`absent_dur_positive=0`, `absent_with_reason=0`). The absent→false rule holds for these two fields on this account. `[PROVEN]`

## Volume before sync

`initial_lookback_days` is 30. The stored cursor was the 10-character token `1789751187`, which is not the JSON cursor `DecodeCursor` expects, and `last_sync_at` was `2026-09-18T17:06:27Z`. Left alone, the next fetch would have been `last_sync − 2h`, the window B1 already showed does not backfill new fields. Phase 2A says the republish is the same as B1, so at enable time this row's `sync_cursor` and `last_sync_at` were cleared and `is_active` was set true. `sync_mode` and `writeback_enabled` were not changed.

| | Count |
|---|---|
| API rows in the 30-day window | 4025 |
| API distinct `inboundCallId` | 3384 |
| Pages at `size=1000` | 5 (last page 25) |
| Certified rate | 5 requests. At 5/sec that is about 1s of spacing. 80/min does not bind. The read itself took 6s with an 800ms pause between pages. |

Expected republish: every distinct call in the walk, because the four fields are appended to the content hash, so the previous hash cannot match. About 3384 publishes, plus duplicate page hits marked unchanged.

## What the sync did

Rebuilt platform-sync log:

- `calls=5025 published=3284 unchanged=1741 page1_refresh=true` (5025 = 4025 + the 1000-row page-1 refresh)
- next tick, cursor already the trailing 2h window: `calls=15 published=4 unchanged=11`

ClickHouse rows with `received_at >= 2026-09-21 23:31:00` and a non-null `has_connected`: **3288** distinct calls. That equals 3284 + 4. The consumer did not drop publishes. `[PROVEN]`

## Reconciliation

A second complete Get Call Log walk of `2026-08-22T23:31:29Z`–`2026-09-21T23:31:29Z` also returned **4025 rows** and a last page of 25, but **3563** distinct ids. The pre-sync walk was also 4025 rows and **3384** distinct. Same row count, different distinct sets. `[PROVEN]` offset pagination on this account is not a stable id set. That is the same class of skip as the earlier 45-call miss.

| | Count |
|---|---|
| API distinct on the later walk of the sync window | 3563 |
| ClickHouse distinct republished (`has_connected` not null) | 3288 |
| In the later API set, not republished | 431 |
| Republished, not in the later API set | 156 |

All **431** missing ids already exist in ClickHouse from before this phase, and their latest `has_connected` is `NULL`. The sync walk never returned them, so they were not republished. They are not insert failures. The 156 extras are ids that walk returned and the later walk did not. `[PROVEN]`

## Field population

Latest row per call.

Republished Ringba calls (3288):

| Field | true | false | not provided |
|---|---|---|---|
| `has_connected` | 2256 | 1032 | 0 |
| `was_blocked` | 557 | 2731 | 0 |
| `was_conversion_adjusted` | 2 | 3286 | 0 |
| `has_converted` | 571 | 2717 | 0 |

`has_converted` cannot store "not provided". Its type was not changed. On these rows the mapper wrote 0 or 1 (absent → false). The nullable columns are what mark not provided.

`block_reason` on those 3288 calls: empty 2731, `The bid has expired` 557. The 557 matches `was_blocked` true.

Outside the republish — latest `has_connected` is null, `RGB%`: **1047** calls. `was_blocked` and `was_conversion_adjusted` are null on all 1047. Zero of them are stored as false. `[PROVEN]`

## Duration (`duration_seconds`)

Latest values on the 3288 republished calls.

**Converted** (`has_converted` true and revenue > 0): n=570, min 13, p10 107.8, p25 186, median 611, p75 1856, max 12567.

| 0–10s | 10–30s | 30–60s | 60–120s | 120–300s | 300s+ |
|---|---|---|---|---|---|
| 0 | 8 | 13 | 53 | 116 | 380 |

**Candidate** (connected, not converted-with-amount, not blocked, not adjusted): n=1684, min 0, p10 6, p25 14, median 25, p75 50, max 105.

| 0–10s | 10–30s | 30–60s | 60–120s | 120–300s | 300s+ |
|---|---|---|---|---|---|
| 263 | 678 | 441 | 302 | 0 | 0 |

The pre-sync API walk reported 0 rows with `hasConverted` and `conversionAmount>0`. The sync walk stored 570. That disagreement is the same unstable page set, not a second mapper. The distribution above is from the rows that were inserted.

**Proposed threshold: 13 seconds.** It is the shortest connected duration among those 570 converted calls. A cut at the converted p10 (~108s) would sit above 8+13+53 sales that are shorter than that. 13s does not separate the pools: candidate median is 25s and candidate max is 105s, so most candidates are already at or above 13s. Not productized. 2B waits for an approved number.

## Enrichment

`enrichment_attrs` is `ReplacingMergeTree`.

| | Rows |
|---|---|
| Before republish | 2,014,540 |
| Raw peak after | 3,636,940 |
| After background merge (`count()` = `FINAL`) | 2,120,620 |

Logical growth is about 106,080 rows, not the raw peak. The merge collapsed the duplicate versions. `[PROVEN]`

## Gates, sync left on

| Script | Exit |
|---|---|
| `scripts/verify-system.ps1` | 0 (Docker 15/15, Healthz 11/11) |
| `scripts/verify-ingestion-slice.ps1` | 0 |
| `scripts/verify-presentation.ps1` | 0 |

`verify-ingestion-slice.ps1` lines 75–78 assert the default KPI total does not change across the smoke ping (`4335 → 4335` this run). A real Ringba insert between those two reads would fail that line. It did not fail. No script was changed. If it flakes, compare the default total excluding rows received during the test, instead of requiring the absolute total to hold still.

## Stop

2B was not started.
