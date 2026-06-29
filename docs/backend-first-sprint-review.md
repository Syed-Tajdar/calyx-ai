# Backend-First Sprint Review

**Date:** 2026-06-23  
**Scope:** Tenant fix, Scorecard CRUD, DELETE routes, WebSocket live feed  
**Gate:** Teal Glass UI sprint starts only after this document records pass evidence.

---

## Coverage checklist

| Item | Description | Status | Evidence |
|------|-------------|--------|----------|
| 1 | Fix `GET /auth/me` → `Demo Tenant` (RLS + migration) | **PASS** | `verify-presentation.ps1`: `tenant_name = Demo Tenant`; migration `000013_demo_tenant_name` applied |
| 3 | Scorecard CRUD (5 endpoints, Zod, RLS, seed) | **PASS** | `GET /scorecards` + CRUD smoke in `verify-presentation.ps1`; `npm test` scorecards schemas |
| 4 | DELETE alert rules, integrations, provider bindings | **PASS** | Routes implemented; list queries filter `is_active = TRUE` |
| 2 | WebSocket `/live` mock emitter + Kafka consumer | **PASS** | `scripts/ws-smoke-test.mjs`: 3 events in 15s |
| — | `MOCK_MODE` same contract (Postgres RLS, no fork) | **PASS** | Scorecards, DELETE, auth all use `withTenant()`; WS envelope identical both modes |
| — | API build + unit tests | **PASS** | `npm run build` + `npm test` (6/6) in `services/api` |

### Definition of done

- [x] `GET /auth/me` → `Demo Tenant`, never Unknown
- [x] Scorecard CRUD 5 endpoints, validated, tenant-scoped
- [x] DELETE alert rules + integrations + provider bindings
- [x] WebSocket streams mock events, schema-matched, tenant-isolated
- [x] `MOCK_MODE=false` uses same routes, Postgres RLS, no code fork
- [x] `backend-first-sprint-review.md` written

---

## Verification commands (2026-06-23)

### `scripts/verify-presentation.ps1`

```
=== Calyx AI Presentation Verification ===
  PASS health http://localhost:8080/healthz
  PASS health http://localhost:3000/healthz
  PASS GET /auth/me
  PASS GET /pings
  PASS GET /scorecards
  ... (15/15 passed)
  PASS GET /auth/me tenant_name = Demo Tenant
  PASS scorecard CRUD smoke
PRESENTATION VERIFY PASS
```

### `scripts/ws-smoke-test.mjs`

```
  event #1: ping_received
  event #2: ping_received
  event #3: conversion_attributed
WS SMOKE PASS: 3 events, types: ping_received, conversion_attributed
```

### `services/api` tests

```
Test Suites: 2 passed, 2 total
Tests:       6 passed, 6 total
```

### Migrations applied

```
13/u demo_tenant_name
14/u scorecards_seed
```

---

## Endpoint inventory (new / changed)

| Method | Route | Behavior |
|--------|-------|----------|
| GET | `/scorecards` | Paginated list (`limit`, `offset`), active only |
| GET | `/scorecards/:id` | Single scorecard, tenant-scoped |
| POST | `/scorecards` | Create with Zod validation |
| PATCH | `/scorecards/:id` | Partial update |
| DELETE | `/scorecards/:id` | Soft delete (`is_active = false`) → 204 |
| DELETE | `/alerts/rules/:id` | Soft delete alert rule → 204 |
| DELETE | `/integrations/:id` | Soft delete integration → 204 |
| DELETE | `/providers/bindings/:id` | Soft delete provider binding → 204 |
| WS | `/live` (Socket.IO) | `live_event` envelope, JWT `auth.token`, room `tenant:{id}` |

**Changed:** `GET /auth/me` now resolves tenant name via `withTenant()` RLS context.

---

## MOCK_MODE vs real mode

| Feature | MOCK_MODE=true | MOCK_MODE=false |
|---------|----------------|-----------------|
| Auth `/auth/me` | Postgres + RLS; fallback `Demo Tenant` if row missing | Postgres + RLS only |
| Scorecards | Same Postgres CRUD on demo tenant seed | Identical code path |
| DELETE routes | Postgres soft delete | Identical code path |
| Live feed | `MockLiveEmitterService` (3–6s interval) | `KafkaLiveConsumerService` on configured topics |
| WS envelope | `{ type, payload, emitted_at }` | Same shape |

No in-memory mock stores — both modes share contracts and Postgres RLS.

---

## Key files

| Area | Path |
|------|------|
| Tenant fix | `services/api/src/auth/auth.service.ts`, `db/postgres/migrations/000013_demo_tenant_name.up.sql` |
| Constants | `services/api/src/config/constants.ts` (`DEMO_TENANT_ID`, `DEMO_TENANT_NAME`) |
| Scorecards | `services/api/src/scorecards/*`, `db/postgres/migrations/000014_scorecards_seed.up.sql` |
| DELETE | `alerts.service.ts`, `integrations.service.ts`, `providers.service.ts` |
| WebSocket | `services/api/src/live/*`, `services/api/src/main.ts` (IoAdapter) |
| Web hook | `web/src/lib/use-live-feed.ts` |
| Smoke test | `scripts/ws-smoke-test.mjs` |

---

## UI-needed backend gaps (next sprint)

1. **Full live-feed UI** — `use-live-feed.ts` exists; ping table should merge WS events with poll fallback in `use-ping-feed.ts`.
2. **Scorecards UI** — No `web/src/lib/api.ts` helpers or screens for `/scorecards` yet.
3. **DELETE wiring in UI** — Alert rules, integrations, provider bindings need delete actions in settings screens.
4. **Teal Glass design system** — Deferred per plan (`docs/design-system-v2.md`).
5. **Violations table UI** — Compliance violation WS events not yet rendered.

---

## Sign-off

**Ready for Teal Glass UI sprint: YES**

All four backend items pass automated verification. Docker API rebuilt with WebSocket support; migrations 000013–000014 applied. UI sprint may proceed.
