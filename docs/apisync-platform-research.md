# API Sync — Platform Calls API Research (STEP 0)

Verified against official docs as of 2026-07. Used to implement `services/platform-sync` REST adapters.

## Summary

| Platform | Pull API | Webhook-only fallback |
|----------|----------|----------------------|
| Retreaver | Yes | No |
| Ringba | Yes | No |
| TrackDrive | Yes | No |
| CallGrid | Yes | No |
| Call Loom native | N/A (direct source) | — |

---

## Retreaver

**Docs:** https://retreaver.github.io/core-api-docs/

| Item | Detail |
|------|--------|
| Endpoint | `GET https://api.retreaver.com/api/v2/calls.json` |
| Detail by id | `GET https://api.retreaver.com/api/v2/calls/{uuid}.json` |
| Auth | Query `api_key` + `company_id` (Settings → API Access / My Company) |
| Incremental | `updated_at_start` (RFC3339); empty cursor lookback **30 days**; legacy `created_at_start` migrated |
| Sort | `sort_by=updated_at` (API forces desc), `order=desc` |
| Pagination | `page`, `per_page`; multi-run backfill when `maxPagesPerSync` hits mid-window |
| Rate limits | Not documented — use 500ms backoff on errors |
| `call_key` | **Not in API** — webhook-only for conversion postbacks |

**Does the API support fetching by last-modified?** **YES**

- Sample/throughput section of the Core API docs states the API accepts `updated_at_start` / `updated_at_end` (in addition to `created_at_*`): [Retreaver Core API docs](https://retreaver.github.io/core-api-docs/) (enumerate/throughput notes).
- Formal V2 query table documents `created_at_start` / `created_at_end` and `sort_by=updated_at` (“If you only want recently updated Calls, sort by `updated_at`”).
- **Caveat:** treat `updated_at_*` as documented-but-should-be-live-probed before shipping a cursor change (confirm filter semantics vs sort-only).

**Response fields → internal:**

| Internal | JSON field |
|----------|------------|
| `external_call_id` | `uuid` |
| caller | `caller` |
| `state` | `caller_state` |
| `revenue` | `revenue` |
| `payout` | `payout` |
| `campaign_id` | `campaign_id` / `system_campaign_id` |
| `publisher_id` | `afid` / `client_afid` |
| `target_id` | `target_id` / `system_target_id` |
| `event_type` | derived from `status` / `connected` |
| `occurred_at` | `created_at` |

---

## Ringba

**Docs:** https://github.com/diablomedia/ringba-php-sdk (mirrors `https://api.ringba.com`); support rate limits: https://support.ringba.com/hc/en-us/articles/23785949612695

| Item | Detail |
|------|--------|
| Endpoint | `POST https://api.ringba.com/v2/{accountId}/calllogs` (official Get Call Log; legacy `…/CallLogs/Date` → HTTP 301 docs redirect) |
| Auth | Header `Authorization: Token {apiToken}` — [API Access Tokens](https://support.ringba.com/hc/en-us/articles/31661837696023-API-Access-Tokens). Bearer was deprecated Jun 2025 and replaced by the **`Token` prefix**, not a bare token. |
| Body | `reportStart`, `reportEnd` (ISO UTC), `filters` (array), `size` (max **1000**), `offset`; optional `valueColumns` for money/geo tags |
| Response | `report.records[]` flat objects (`inboundCallId`, `callDt` epoch ms, `tag:InboundNumber:State`, `conversionAmount`, …); wrapper also has `isSuccessful`, `transactionId`, `report.partialResult` |
| Incremental | Sliding window from `sync_cursor` / `last_sync_at` |
| Pagination | `offset` += `size` until page shorter than `size` |
| Rate limits | Get Logs: **5/sec or 80/min account-wide**; Export CSV: **5/min, 20/hr, 200/day per user** (export path not implemented until support Q3 answered) |

**Does the API support fetching by last-modified?** **NO** (documented surface is call-time date range)

- `CallLogs/Date` is driven by `reportStart` / `reportEnd` (call/report time window), not an `updated_at` cursor: [CallLogApi.md](https://github.com/diablomedia/ringba-php-sdk/blob/HEAD/docs/Api/CallLogApi.md).
- Filters exist on columns/tags, but no documented “last modified since” incremental mode.
- **Implication:** same D9 gap as created-at cursors — revenue updates after the call’s report time require an **overlap window** (and/or webhooks) to re-fetch.

**Response:** `callLog.data[]` — each call has `columns`, `events`, `tags` arrays.

**Field extraction:** tag/column lookup by name (caller state, revenue, inboundCallId, campaign/publisher/target ids). **`occurred_at` ← `callDt`** (Phase 2.A/B).

**Cursor:** overlap window (`last_sync_at − 2h`) + content-hash (Phase 2.0); no last-mod filter (A3=NO).

**A1 / detail-by-id:** Call Logs detail with `InboundCallIds` (and annotate/get-call by `inboundCallId`).

---

## TrackDrive

**Docs:** https://trackdrive.com/api/docs/1.0/calls/index.html

| Item | Detail |
|------|--------|
| Endpoint | `GET https://{subdomain}.trackdrive.com/api/v1/calls` |
| Auth | `Authorization: Basic base64(public_key:private_key)` |
| Incremental | `created_at_from`, `created_at_to` (also `ended_at_from` / `ended_at_to`) |
| Pagination | `cursor` (-1 first), `metadata.next_cursor`; `per_page` ≤ 10 without cursor |
| Rate limits | Not documented — backoff on 429 |

**Does the API support fetching by last-modified?** **NO** (as a filter)

- Documented time filters: `created_at_from` / `created_at_to`, `ended_at_from` / `ended_at_to` — [calls index](https://trackdrive.com/api/docs/1.0/calls/index.html).
- `updated_at` exists as a **sortable** column (`order=updated_at`) and as a response field, but there is **no** `updated_at_from` / `updated_at_to` query param in the published params list.
- **Implication:** use created-at (or ended-at) **overlap window** for conversion/revenue refresh; optional webhook path if the account posts conversions.

**Response fields → internal:**

| Internal | JSON field |
|----------|------------|
| `external_call_id` | `uuid` |
| caller | `caller_number` |
| `state` | `caller_state` or token |
| `revenue` | `revenue`, `buyer_revenue` |
| `payout` | `payout`, `traffic_source_payout` |
| `campaign_id` | `offer_id` |
| `publisher_id` | `traffic_source_id` |
| `target_id` | `buyer_id` |
| `event_type` | `status`, `buyer_converted` |

---

## CallGrid

**Docs:** https://callgrid.com/api

| Item | Detail |
|------|--------|
| Endpoint | `GET https://api.callgrid.com/api/call` |
| Auth | `Authorization: Bearer {API_KEY}` |
| Incremental | `startDate`, `endDate` (ISO) |
| Pagination | `useCursor=true`, `maxItems`, `searchAfter` from `nextCursor` |
| Rate limits | 80 req/min; 429 + `RateLimit-*` headers |

**Does the API support fetching by last-modified?** **NO** (documented surface is call date range)

- Official download examples filter with `startDate` / `endDate` only: [CallGrid API](https://callgrid.com/api).
- Optional JSON `filters` exist for tags/rules; no documented last-modified / updated-at incremental cursor.
- **Implication:** overlap window on `startDate`/`endDate` (and/or webhooks) for post-call revenue updates.

**Response:** `data[]` array; field names confirmed at map time (id, caller, state, revenue, etc.).

---

## Call Loom

Direct ingestion at `/sources/callloom-native/v1/pings` — **no REST pull adapter**.

---

## Fetch-by-last-modified (Phase 1.6 official sources)

| Platform | Last-modified fetch? | Param(s) | Official doc URL | Confidence |
|----------|----------------------|----------|------------------|------------|
| Retreaver | **YES** | `updated_at_start`, `updated_at_end`; `sort_by=updated_at` | https://retreaver.github.io/core-api-docs/ | **HIGH** (docs + live probe) |
| Ringba | Ambiguous on last-mod cursor | Reporting filters are attribute filters; Call Logs date-range — no vendor-documented “modified since” param found | https://support.ringba.com/hc/en-us/articles/17994137377815-Reporting-Filters ; https://support.ringba.com/hc/en-us/articles/23787558523159 | **LOW — confirm by live probe in Ringba phase** (do not treat third-party PHP SDK alone as NO) |
| TrackDrive | **NO** as filter | `created_at_*` / `ended_at_*`; `updated_at` order/column only | https://trackdrive.com/api/docs/1.0/calls/index.html | **HIGH** |
| CallGrid | **NO** as documented last-mod filter | `startDate` / `endDate` (+ optional tag filters); no `updated_at_*` query param in vendor download docs | https://callgrid.com/api | **MEDIUM** |

Phase 1.5 table below retained for history; prefer the Phase 1.6 table above for go-live decisions.

## Fetch-by-last-modified (Phase 1.5 summary — historical)

| Platform | Last-modified fetch? | Doc |
|----------|----------------------|-----|
| Retreaver | **YES** (`updated_at_start` / `updated_at_end` + `sort_by=updated_at`) | https://retreaver.github.io/core-api-docs/ |
| Ringba | **NO** (call-time `reportStart`/`reportEnd` only) — *source was third-party SDK; superseded by Phase 1.6 LOW confidence* | https://github.com/diablomedia/ringba-php-sdk/blob/HEAD/docs/Api/CallLogApi.md |
| TrackDrive | **NO** as filter (`created_at_*` / `ended_at_*`; `updated_at` sort only) | https://trackdrive.com/api/docs/1.0/calls/index.html |
| CallGrid | **NO** (`startDate`/`endDate` only) | https://callgrid.com/api |

**Design property:** conversion/update mechanism must be **per-adapter configurable** (updated_at cursor vs overlap window vs webhook). Not implemented until product picks — see `docs/phase1.5-close-gaps-report.md`.
