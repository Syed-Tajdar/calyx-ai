# Calyx — v1 Retreaver mapping (additive)

**Branch:** `v1/retreaver-mapping` from `origin/main` @ `99618e0` · **Owner decision:** report Q4 (2026-10-19)
**Scope:** additive only. No change to counts, revenue, dedupe rules or any Ringba code.

## What changed

| Field (ClickHouse `pings`) | Source in the Retreaver v2 call | Note |
|---|---|---|
| `campaign_name` | `campaign_name` (trimmed) | v2 sends it flat; v4/v5 nest it in `campaign` (not read: we sync v2) [PROVEN-docs] https://learn.retreaver.com/api/calls |
| `buyer_id`, `buyer_name` | the target: `system_target_id` → `target_id` → `callpixels_target_id`; `target_name` | Retreaver has **no buyer field**; the docs describe the target as who "the caller was connected to" [INFERRED-docs]. Empty buyer when no target (TargetID keeps its old "0"). 🙋 Owner: keep buyer = target, or leave buyer empty? |
| `has_recording`, `recording_known` | whether `recording_url` is in the payload / non-empty | **Booleans only.** The URL is decoded into a local value, turned into two flags and cleared. It is never stored, logged, hashed or published |

`recording_url` → flags: string → `1/1`; `null`, `""`, whitespace or a non-string → `0/1`; key absent → `0/0`.

Files: `services/platform-sync/internal/adapter/retreaver_adapter.go` (+39), `internal/producer/producer.go`
(refactor only: message building split into `BuildMessageJSON` so a test can read the exact Kafka message;
`Publish` sends the same bytes), new test file `internal/adapter/retreaver_v1_fields_test.go`. Consumer and
ClickHouse untouched (the columns already exist).

## Content hash — what happens after deploy

The dedup hash (`packages/pingdedup/content.go`) already includes `CampaignName`, `BuyerID`, `BuyerName` and
`HasRecording` (not `recording_known`). Retreaver sent them empty until now, so **every Retreaver call that
platform-sync fetches again** (incremental `updated_at` window, hourly 7-day sweep, daily 30-day sweep) gets a
new hash and is **republished once** with the new values. Effect: one extra `pings` row per such call, same
`external_call_id`, same revenue and duplicate flags. Every read takes the latest row (`argMax`), so **counts
and revenue do not change**; `sync_count` on those calls goes up by 1. Older calls keep empty new fields until
something re-reads them. Dedupe itself is unchanged; null vs absent `recording_url` and a rotated URL do not
change the hash (tested).

## Tests [PROVEN]

New: `TestRetreaverV1MapsCampaignName`, `TestRetreaverV1MapsBuyerFromTarget`, `TestRetreaverV1RecordingFlags`
(6 cases), `TestRetreaverV1RecordingURLNeverLeaves` (URL, its host and token absent from the mapped call and the
Kafka JSON), `TestRetreaverV1NewFieldsDoNotTouchExistingMapping` (ids, revenue, payout, profit, duplicates,
converted, durations and times identical; golden fixture revenues unchanged), `TestRetreaverV1ContentHash`.
Full `go test ./...` in `services/platform-sync` and `packages/pingdedup`: ok. No existing test changed.

## Totals unchanged [PROVEN]

Laptop ClickHouse, read-only, laptop platform-sync stopped, no Retreaver API call:
**3,652 calls / $33,143.85** for Retreaver all time (same as the Stage 4 baseline). No laptop data changes until a
sync runs, which it does not on the laptop.

## Deploy (owner decides the merge)

Rebuild **platform-sync** only. No env or migration. After deploy Aryan runs the 7-day reconciliation (go-live
Step 2.4 queries): calls and revenue must be unchanged; `campaign_name`/`buyer_name`/`has_recording` start to fill
for re-fetched Retreaver calls.
