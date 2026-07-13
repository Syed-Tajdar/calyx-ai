# Retreaver Real Pull — Verification Report

**Date:** 2026-07-10  
**Credentials:** never printed (redacted). Verification used encrypted blob via `get_api_sync_integration` + `cmd/verify-retreaver-call`.

---

## PART A — Data is genuinely Retreaver’s

### A1 — Fetch stored UUID from Retreaver call-detail API

ClickHouse sample UUID: `5b5662f2-faee-43c1-9c11-3c1bbb37d31b` (CH `state=IL`, `received_at=2026-07-10 12:51:19`).

```
GET /api/v2/calls/{uuid}.json
uuid=5b5662f2-faee-43c1-9c11-3c1bbb37d31b
http_status=200
state=IL
status=no-target
created_at=2026-07-09T19:38:21.244Z
revenue=0
payout=0
caller_present=true
```

**Verdict:** HTTP **200** + matching state → `external_call_id` is a **genuine Retreaver call UUID**. The UI list often hides this UUID (detail URL / Call Logs, not RTB logs).

Also proves the time bug: API `created_at` is **2026-07-09 19:38**, while CH `received_at` is **sync time 2026-07-10 12:51**.

### A2 — One API page vs ClickHouse (set match)

Fetched `calls.json?per_page=10` (HTTP 200). All **10/10** UUIDs present in ClickHouse (each with `n=2` from overlapping re-syncs — expected with store-and-flag dedup).

| API uuid (prefix) | In CH? |
|-------------------|--------|
| a962cc9c-… | yes (2) |
| 7d985e43-… | yes (2) |
| be72596d-… | yes (2) |
| 4940f700-… | yes (2) |
| 09e5ad79-… | yes (2) |
| 3da80522-… | yes (2) |
| 6589e4f4-… | yes (2) |
| ffd44a66-… | yes (2) |
| 09245555-… | yes (2) |
| cec954d2-… | yes (2) |

### A3 — UI reconciliation steps (for the user)

1. Open Retreaver → **Call Logs** (not RTB logs).
2. From API/CH pick one call’s **caller** + **`created_at`** (e.g. from A1: status `no-target`, state `IL`, time `2026-07-09T19:38:21Z`).
3. Filter Call Logs to that date/time window and caller.
4. Open the call **detail** page — the UUID appears in the URL (or a details field).
5. Compare to Calyx `external_call_id` — they match.

Do **not** search the list view by UUID alone; many Retreaver UIs show a shorter display id.

### A4 — Volume sanity

```
rows=457  calls(uniq)=241
min(received_at)=2026-06-25 05:27:03
max(received_at)=2026-07-10 12:51:19
nonzero_rev=91  nonzero_payout=76
```

Empty-cursor first sync pulled a wide history window → hundreds of rows / many states is expected. `rows > uniq` because re-syncs store duplicates with `is_duplicate=1`. Compare account Call Logs total for the same window in Retreaver UI; expect similar order of magnitude (exact match depends on cursor/pagination and filters).

---

## PART B — FIX: source call time (`created_at` → `occurred_at`)

### Before

Adapter did **not** map `created_at`. Producer set `received_at` = sync wall clock → Ping Feed showed one timestamp per batch.

### After

| Layer | Change |
|-------|--------|
| CH migration `000008_pings_occurred_at` | `occurred_at DateTime('UTC') DEFAULT 0` |
| `mapRetreaverCall` | parses `created_at` → `RemoteCall.OccurredAt` |
| Kafka JSON | `occurred_at_unix_ms` |
| Consumer | writes `occurred_at` |
| API `/pings` | returns `occurred_at` (coalesced display time) + keeps `received_at` as ingestion |
| Filters / analytics dates | use `if(occurred_at > 0, occurred_at, received_at)` |
| Web Ping Feed / timeline / date filter | use `pingEventAt()` = `occurred_at \|\| received_at` |

Webhook path: connectors do not currently send a call timestamp → `occurred_at` stays 0 → UI falls back to `received_at` (unchanged behavior).

### Existing rows

Rows pulled **before** this fix have `occurred_at = 0` and still display sync-time until a **re-sync** (or backfill). After next Live sync, new/updated publishes carry real `created_at`.

### Evidence (A1)

| Field | Value |
|-------|-------|
| Retreaver `created_at` | `2026-07-09T19:38:21.244Z` |
| Old CH `received_at` | `2026-07-10 12:51:19` |

Unit test: `TestMapRetreaverCallNestedV2ListItem` asserts `OccurredAt` from `created_at`.

---

## PART C — Revenue $0: genuine, not a mapping bug

| Evidence | Result |
|----------|--------|
| Recent API page (10 calls) | `revenue` JSON type **`null`** for all 10; `nonzero_revenue_in_page=0/10` |
| CH overall | `91` rows with `revenue > 0` (mostly `finished`) |
| Today’s sync by event | `no-target` 266 @ $0; `finished` 160 with **66** nonzero; `no-answer` 6 @ $0 |
| Cross-check UUID `75f1e19d-…` | API `revenue=46.5` `payout=37.2` = CH exactly |

**Verdict: revenue genuinely 0 (or null) for many calls** — especially `no-target` / `no-answer`. Mapping is correct when Retreaver sends a number. No mapper fix.

If the Ping Feed looks “all $0”, filter to `finished` or sort by revenue — many recent rows are no-target.

---

## PART D — Aggregate dedupe identity

| Query | Identity |
|-------|----------|
| analytics metrics / P&L / RPC | `GROUP BY platform, external_call_id` + `argMax(revenue\|payout, received_at)` |
| platform-mix | `GROUP BY platform, external_call_id` then outer by platform |
| coverage heatmap | `GROUP BY platform, external_call_id` + `argMax(state\|revenue, received_at)` |
| `/pings` list | raw rows (unchanged) |
| dashboard ping timeline | client buckets by `pingEventAt` (source time when present) |

No further aggregate fixes required.

---

## Verify scripts (post-deploy)

| Script | Result |
|--------|--------|
| `verify-presentation.ps1` | **20/20 PASS** |
| `smoke-test-all-platforms.ps1` | **4/4 PASS** |
| `verify-api-sync.ps1` | **PASS** |
| `ws-smoke-test.mjs` | **PASS** |

Deployed: `platform-sync`, `consumer`, `api`, `web` rebuilt with `occurred_at` support. CH column confirmed present.

---

## Tooling

`services/platform-sync/cmd/verify-retreaver-call` — decrypts credentials in-process, prints only safe fields (`uuid`, status, state, `created_at`, revenue). Never prints `api_key`.

---

**Retreaver real data verified: YES**  
**FIX B (source call time): DONE** (re-sync needed to backfill existing rows)  
**FIX C (revenue): NO BUG** — genuine zeros/nulls from Retreaver; mapping matches API
