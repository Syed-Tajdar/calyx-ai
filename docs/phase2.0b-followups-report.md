# Phase 2.0b — Deferred Retreaver follow-ups

**Date:** 2026-07-14  
**Status:** PASS

## 1. Drop `aniHash` from call-detail URL

| Change | File |
|--------|------|
| Href without `aniHash` | `web/src/components/dashboard/PingsTable.tsx` → `/calls/{id}?platform=…` |
| Stop reading query `aniHash` | `web/src/app/(app)/calls/[callId]/page.tsx` |
| Enrichment from deep view | `deepView.ping.ani_hash` → `EnrichmentPanel` |
| Journey `hasEnrichment` | ani_hash **or** enrichment attribute_count |

No caller-derived query params remain on call detail links.

## 2. Default-hide `apisync-%` in Ping Feed

| Layer | Behavior |
|-------|----------|
| API `buildPingWhereClause` | Default `external_call_id NOT LIKE 'apisync-%'`; `show_demo=true` lifts |
| Applied to | list totals, distinct_total, list rows, CSV export |
| Web | **Show demo** checkbox on Ping Feed (off by default) |
| Live WS merge | `pingMatchesFilters` also excludes `apisync-%` unless show_demo |

## Checklist

- [x] Call links carry `platform` only
- [x] Enrichment works from deep-view `ani_hash`
- [x] Demo rows hidden by default
- [x] Show demo toggle wires `show_demo=true`
