# Phase 2.0 — Content-hash dedup report

**Date:** 2026-07-14  
**Status:** PASS

## What shipped

| Item | Detail |
|------|--------|
| Shared module | `packages/pingdedup` — `Content` / `Hash` / `Evaluate` → `PublishNew` \| `PublishChanged` \| `Unchanged` |
| Wire-up | Ingestion `webhook.go` + `ringba.go`; platform-sync `publishSyncCalls` |
| Wrappers | Thin `internal/dedup` in both services (`replace` → local package) |
| TTL | Both services default **168h** via `DEDUP_TTL_HOURS` |
| Page=1 refresh | `Registry.FetchAllPages` appends first-page replay when `pagesFetched > 1` |

## Policy change

- Unchanged content: **no Kafka publish** (previously always published with `is_duplicate=1`).
- Changed content: publish row with `is_duplicate=0` (update semantics; analytics `argMax` still applies).
- Legacy DUP rows remain readable in ClickHouse.

## Canonical hash fields (stable order)

`external_call_id`, `revenue`, `payout`, `state`, `event_type`, `occurred_at` (RFC3339 UTC or empty), `campaign_id`, `publisher_id`, `target_id`, `platform_duplicate`.

## Tests

```
go test ./packages/pingdedup/...          PASS
go test ./services/platform-sync/internal/syncer/...  PASS
go test ./services/platform-sync/internal/adapter/... PASS
```

| Case | Result |
|------|--------|
| Identical content twice → 1 publish | PASS (unit) |
| Revenue change → PublishChanged | PASS (unit + syncer) |
| Overlap 100 × 5 syncs → 100 publishes | PASS (unit) |
| Empty external id skipped | PASS (syncer) |

## Verify checklist

- [x] Shared package + both services compile against `replace`
- [x] Ingestion skips Publish on Unchanged
- [x] Syncer skips Publish on Unchanged
- [x] TTL aligned 168h
- [x] Deep-walk page=1 refresh hooked
