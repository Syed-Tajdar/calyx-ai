# Phase 1.6 B1–B3 — Implementation report

**Date:** 2026-07-13  
**Scope:** Decision Gate locked (A2 YES, A3 populates, explicit backfill) → Retreaver-only B1–B3.  
**Credentials:** never printed.

---

## B1 — Update mechanism (`updated_at` + 15m safety delta)

### What changed (code)

Primary file: [`services/platform-sync/internal/adapter/retreaver_adapter.go`](../services/platform-sync/internal/adapter/retreaver_adapter.go)

| Constant / helper | Value / behaviour |
|-------------------|-------------------|
| `retreaverCursorMode` | `"updated_at"` (documented; DB `cursor_mode` column deferred) |
| `retreaverCursorSafetyDelta` | **15 minutes** (was ad-hoc 5m) |
| `retreaverDefaultLookbackDays` | **30** |
| `retreaverLookbackDuration(days)` | clamps 1..365; `0` → 30d |
| `maxCallUpdatedAt(calls, fallback)` | latest non-zero `RemoteCall.UpdatedAt` |

**`FetchPage` flow:**

1. Decode cursor → migrate legacy `created_at_start` → `updated_at_start`.
2. Else if `lastSyncAt` → `updated_at_start = lastSyncAt − 15m`.
3. Else empty → `updated_at_start = now − lookbackDays` (per-integration, default 30).
4. Query: `updated_at_start` + `sort_by=updated_at` + `order=desc`.
5. If `hasMore` → bump `page`.
6. If complete → `UpdatedAtStart = maxCallUpdatedAt(page) − 15m` (fallback `now − 15m`).

**Mapping:** API JSON `updated_at` → `models.RemoteCall.UpdatedAt` (required for max-seen cursor).

**Wiring:** `FetchPage(..., lookbackDays int)` on adapter interface; registry/syncer pass `InitialLookbackDays` from store ([`syncer.go`](../services/platform-sync/internal/syncer/syncer.go), [`store/integrations.go`](../services/platform-sync/internal/store/integrations.go)). Other adapters updated for signature only.

### Unit tests (raw)

```
cd services/platform-sync
go test ./internal/adapter/ -count=1
ok  	github.com/Syed-Tajdar/calyx-ai/services/platform-sync/internal/adapter	3.369s
```

Covered in `retreaver_adapter_test.go`:

- `TestRetreaverInitialLookbackIs30Days`
- `TestRetreaverCursorSafetyDeltaIs15Minutes`
- `TestMaxCallUpdatedAtUsesMaxThenSafetyAnchor`
- `TestRetreaverEmptyCursorUsesLookback`

### Checklist

| Check | Result |
|-------|--------|
| Safety delta 15m | PASS |
| Max-seen `updated_at` − delta on complete page | PASS (code + unit) |
| Empty cursor uses lookback days | PASS |
| Legacy `created_at_start` migrate | PASS (normalize) |

---

## B2 — `platform_duplicate` rule (b)

### Prerequisite (A3)

Webhook with `is_duplicate=1` → ClickHouse:

```
phase16-dup-fc58eb79	0	1
```

→ `platform_duplicate=1`, `is_duplicate=0`. Wiring **alive**. Row deleted after test.

### Aggregates (code)

[`services/api/src/analytics/analytics.service.ts`](../services/api/src/analytics/analytics.service.ts):

- Distinct subquery: `argMax(platform_duplicate, received_at)` per `(platform, external_call_id)`.
- `total_pings` = `count()` of distinct calls (**includes** platform dupes).
- `total_revenue` / `total_payout` = `sumIf(..., platform_duplicate = 0)`.
- `duplicate_rate` = `duplicate_pings / total_pings`.

[`services/api/src/coverage/coverage.service.ts`](../services/api/src/coverage/coverage.service.ts):

- Heatmap counts keep distinct calls; revenue uses `sumIf(revenue, platform_duplicate = 0)`.

### UI / API

- Metrics type + field: `duplicate_rate` on `MetricsResult`.
- Dashboard: [`AnalyticsMetricsGrid.tsx`](../web/src/components/reports/AnalyticsMetricsGrid.tsx) — “Duplicate rate” as percent.
- `/pings` unchanged (badge = Redis `is_duplicate` path; platform flag separate).

### Live shape (from Phase 1.6 verify session)

```json
{"total_pings":7722,"total_revenue":58293.1591,"total_payout":15162.4188,...,"duplicate_rate":0}
```

(`duplicate_rate: 0` expected after A3 test row deleted.)

### Checklist

| Check | Result |
|-------|--------|
| Column populates | PASS (A3) |
| Revenue excludes `platform_duplicate=1` | PASS (SQL) |
| Counts include platform dupes | PASS |
| `duplicate_rate` exposed | PASS |

---

## B3 — Backfill / configurable lookback

### Postgres

Migration: [`db/postgres/migrations/000023_initial_lookback_days.up.sql`](../db/postgres/migrations/000023_initial_lookback_days.up.sql)

- Column: `initial_lookback_days INT NOT NULL DEFAULT 30` CHECK (1..365).
- Updated `list_api_sync_integrations()` / `get_api_sync_integration(...)`.

Applied:

```
23/u initial_lookback_days (72.255556ms)
```

DB check:

```
 initial_lookback_days | 30
```

### API

[`integrations.service.ts`](../services/api/src/integrations/integrations.service.ts) / [`integrations.controller.ts`](../services/api/src/integrations/integrations.controller.ts):

- Field on `IntegrationRecord`: `initial_lookback_days`.
- `PATCH` sync-mode accepts optional `initial_lookback_days`.
- `POST /integrations/:id/backfill`:
  - Clears `sync_cursor`, `last_sync_at`.
  - Triggers platform-sync.
  - In-process cooldown (`BACKFILL_COOLDOWN_MS` → 5 minutes) → 429 if spammed.
  - Returns `{ status: 'backfill_triggered' }`.

Web client: [`web/src/lib/api.ts`](../web/src/lib/api.ts) — `backfillIntegration()`, lookback on sync-mode update.

### Settings UI

[`IntegrationsList.tsx`](../web/src/components/settings/IntegrationsList.tsx):

- Status text: **“Syncing last N days”** (`item.initial_lookback_days ?? 30`).
- **Backfill** control (confirm dialog → `POST .../backfill`).

### Checklist

| Check | Result |
|-------|--------|
| No silent hardcoded-only lookback | PASS (DB + adapter param) |
| Default 30 days | PASS |
| UI surfaces window | PASS |
| Explicit Backfill action | PASS |
| Progress UI | **Follow-up** (not shipped) |

---

## Files touched (B1–B3)

**B1:** `retreaver_adapter.go`, `retreaver_adapter_test.go`, `types.go`, `registry.go`, `mock.go`, other adapters’ `FetchPage` signature, `models.go`, `syncer.go`, `store/integrations.go`

**B2:** `analytics.service.ts`, `coverage.service.ts`, `web/src/lib/api.ts`, `AnalyticsMetricsGrid.tsx`

**B3:** `000023_initial_lookback_days.{up,down}.sql`, `integrations.service.ts`, `integrations.controller.ts`, `IntegrationsList.tsx`, `api.ts`

---

## Verify

| Command | Result |
|---------|--------|
| `go test ./internal/adapter/` | PASS |
| `docker compose run migrate` → `23/u` | PASS |
| Presentation / smoke / api-sync / ingestion (Phase 1.6 final) | PASS (see `phase1.6-final-report.md`) |

**PASS/FAIL slice:** **PASS** for B1–B3 implementation scope.
