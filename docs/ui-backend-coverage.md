# UI ↔ Backend Coverage Report

**Date:** 2026-06-23  
**Sprint:** Teal Glass UI + Part 4 wiring  
**Sign-off:** 100% NestJS user-facing API surface covered

---

## NestJS HTTP routes (35)

| Backend endpoint | UI surface | Status |
|------------------|------------|--------|
| `GET /healthz` | Ops only (no UI) | N/A |
| `GET /readyz` | Ops only (no UI) | N/A |
| `POST /auth/login` | `/login` | ✅ |
| `GET /auth/me` | Sidebar footer, TopBar dropdown | ✅ |
| `GET /pings` | `/dashboard`, `/pings` (hybrid WS + poll) | ✅ |
| `GET /pings/export` | `/pings` Export CSV button | ✅ |
| `GET /analytics/metrics` | `/dashboard`, `/reports` | ✅ |
| `GET /analytics/pnl` | `/reports` P&L | ✅ |
| `GET /attribution/events` | `/attribution` events table | ✅ |
| `GET /attribution/status` | `/attribution` platform grid | ✅ |
| `GET /coverage/heatmap` | `/coverage` heatmap | ✅ |
| `GET /enrichment/:aniHash` | `/calls/[callId]` enrichment panel | ✅ |
| `POST /ask` | `/ask` chat | ✅ |
| `GET /ask/history` | `/ask` history sidebar | ✅ |
| `GET /compliance/risk-scores` | `/compliance`, `/dashboard` ring | ✅ |
| `GET /compliance/violations` | `/compliance` violations table | ✅ |
| `GET /compliance/exposure` | `/compliance`, `/dashboard` | ✅ |
| `GET /alerts/rules` | `/settings` Alerts tab | ✅ |
| `POST /alerts/rules` | `/settings` create rule form | ✅ |
| `DELETE /alerts/rules/:id` | `/settings` delete per rule | ✅ |
| `GET /integrations` | `/settings` Integrations tab | ✅ |
| `DELETE /integrations/:id` | `/settings` delete per integration | ✅ |
| `GET /providers/catalog` | `/settings` Providers tab | ✅ |
| `GET /providers/bindings` | `/settings` Providers list | ✅ |
| `POST /providers/bindings` | `/settings` add binding form | ✅ |
| `PATCH /providers/bindings/:id` | `/settings` edit priority | ✅ |
| `DELETE /providers/bindings/:id` | `/settings` delete binding | ✅ |
| `POST /providers/bindings/:id/test` | `/settings` test button + toast | ✅ |
| `POST /providers/resolve` | Diagnostic via test/resolve path | ✅ (via test) |
| `GET /scorecards` | `/scoring/scorecards` list | ✅ |
| `GET /scorecards/:id` | Editor modal (edit load) | ✅ |
| `POST /scorecards` | `/scoring/scorecards` create | ✅ |
| `PATCH /scorecards/:id` | `/scoring/scorecards` edit | ✅ |
| `DELETE /scorecards/:id` | `/scoring/scorecards` delete | ✅ |
| `GET /scoring/scores` | `/scoring`, `/dashboard`, intelligence hub | ✅ |
| `GET /scoring/transcript` | `/calls/[callId]` transcript panel | ✅ |
| `POST /scoring/deepgram/callback` | Webhook only (no UI) | N/A |
| `GET /native/call-deep-view` | `/calls/[callId]` deep view panel | ✅ |

## WebSocket (1)

| Capability | UI surface | Status |
|------------|------------|--------|
| `WS /live` → `live_event` | `use-hybrid-ping-feed` on dashboard + pings; `LiveActivityStrip` on dashboard | ✅ |

---

## Part 4 (Backend-First) — previously flagged gaps

| Item | Status |
|------|--------|
| Scorecard CRUD UI | ✅ `/scoring/scorecards` |
| Live feed WebSocket primary | ✅ hybrid feed; poll fallback when disconnected |
| DELETE alert rules / integrations / bindings | ✅ Settings with confirm dialogs |
| Provider bindings admin (edit/test/delete) | ✅ Settings Providers tab |

---

## Mock data

| Source | Purpose |
|--------|---------|
| `db/clickhouse/migrations/000006_demo_fixture.up.sql` | Cold-start populated dashboard (50 pings, scores, compliance, attribution) |
| `db/postgres/migrations/000015_demo_alert_rules.up.sql` | 3 demo alert rules |
| `scripts/demo-seed.ps1` | Full pipeline demo (live Kafka path) |

`verify-presentation.ps1` asserts `GET /pings` total > 0 after fixtures.

---

## Visual / typography acceptance

| Check | Status |
|-------|--------|
| Teal gradient CTAs (not Google blue) | ✅ `--calyx-gradient` on `.calyx-btn-primary` |
| Teal-led aurora (max 1 purple orb) | ✅ `DesignBackground` |
| Geist headings + Inter body | ✅ `layout.tsx` + `font-heading` / `font-body` |
| Tabular numerals on metrics | ✅ `.calyx-metric-value`, `.tabular`, table `.numeric` |
| Light theme disabled | ✅ toggle removed; `ThemeProvider` forces dark |
| Mini sparklines on metric cards | ✅ `MetricCard` + dashboard timeline data |
| Glass: no nested blur on rows | ✅ container glass; `hover:bg-white/[0.03]` on rows |
| `-webkit-backdrop-filter` | ✅ `.calyx-glass` in globals |
| `prefers-reduced-motion` aurora | ✅ `.calyx-aurora-orb` animation gated |
| `@supports` glass fallback | ✅ solid background fallback |

---

## Remaining (non-blocking, future sprints)

- Full Teal Glass pass on every chart/card wrapper (core shell + metric cards done)
- `POST /providers/resolve` dedicated dev diagnostic button (optional)
- Light theme parity (deferred)

**Ready for production demo: YES**
