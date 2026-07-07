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
| Auth | Query `api_key` + `company_id` (Settings → API Access / My Company) |
| Incremental | `created_at_start` (RFC3339), optional `created_at_end` |
| Sort | `sort_by=created_at`, `order=asc` |
| Pagination | `page`, `per_page` |
| Rate limits | Not documented — use 500ms backoff on errors |
| `call_key` | **Not in API** — webhook-only for conversion postbacks |

**Response fields → internal:**

| Internal | JSON field |
|----------|------------|
| `external_call_id` | `uuid` |
| caller | `caller` |
| `state` | `caller_state` |
| `revenue` | `revenue` |
| `payout` | `payout` |
| `campaign_id` | `campaign_id` |
| `publisher_id` | `client_afid` |
| `target_id` | `target_id` |
| `event_type` | derived from `status` / `connected` |

---

## Ringba

**Docs:** https://github.com/diablomedia/ringba-php-sdk (mirrors `https://api.ringba.com`)

| Item | Detail |
|------|--------|
| Endpoint | `POST https://api.ringba.com/{accountId}/CallLogs/Date` |
| Auth | Header `Authorization: {API_ACCESS_TOKEN}` (Bearer deprecated Jun 2025) |
| Body | `reportStart`, `reportEnd` (ISO), `size`, `offset` |
| Incremental | Sliding window from `sync_cursor` / `last_sync_at` |
| Pagination | `offset` += `size` until `callLog.data` empty |
| Rate limits | 5/sec, 80/min ([support](https://support.ringba.com/hc/en-us/articles/23785949612695)) |

**Response:** `callLog.data[]` — each call has `columns`, `events`, `tags` arrays.

**Field extraction:** tag/column lookup by name (caller state, revenue, inboundCallId, campaign/publisher/target ids).

---

## TrackDrive

**Docs:** https://trackdrive.com/api/docs/1.0/calls/index.html

| Item | Detail |
|------|--------|
| Endpoint | `GET https://{subdomain}.trackdrive.com/api/v1/calls` |
| Auth | `Authorization: Basic base64(public_key:private_key)` |
| Incremental | `created_at_from`, `created_at_to` |
| Pagination | `cursor` (-1 first), `metadata.next_cursor`; `per_page` ≤ 10 without cursor |
| Rate limits | Not documented — backoff on 429 |

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

**Response:** `data[]` array; field names confirmed at map time (id, caller, state, revenue, etc.).

---

## Call Loom

Direct ingestion at `/sources/callloom-native/v1/pings` — **no REST pull adapter**.
