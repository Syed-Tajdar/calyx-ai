# Calyx — Convert / Cancel Screens (Ringba only)

**Step:** 3 of 7 in the v1 remaining-work list.
**Scope:** the **UI** for a write-back path that is already built and proven on the backend.
**Platform:** **Ringba only.** Retreaver is explicitly out of scope (see `docs/retreaver-parity-plan.md` when written).
**Deliverable:** `docs/convert-cancel-screens-report.md`

---

## ⛔ RULE

- **The backend is done. This slice must not change write-back behaviour**, only expose it.
- **No Ringba writes against real customer calls during development.** Test against a call in the
  disposable sandbox campaign, or with `sync_mode=mock`.
- Label claims `[PROVEN]` / `[INFERRED]` / `[UNKNOWN]`. Proven means seen in a real browser.
- **No push.** Operator reviews first.
- **No new npm dependency** — the confirm dialog is written in-repo (no `@radix-ui/react-dialog`).

---

## Goal

Give an operator three things:

1. See at a glance whether a call converted.
2. Find the calls that **should** have converted but did not — the revenue leak.
3. Convert or cancel one of those calls, deliberately, with confirmation.

---

## Current state (verified 2026-09-30, live local stack)

| Piece | State |
|---|---|
| `POST /integrations/:id/writeback/convert` | ✅ exists — body `{inbound_call_id, amount}`, amount must be positive |
| `POST /integrations/:id/writeback/void` | ✅ exists — body `{inbound_call_id}` |
| Routes registered only when `WRITEBACK_ENABLED=true` | ✅ `integrations.controller.ts:59-88` |
| Per-tenant gate `integrations.writeback_enabled` (default **false**) | ✅ `integrations.service.ts:269` → 403 |
| Convert → Ringba | `GET webhook.ringba.com/hook/{id}?call_id&call_revenue&converted=yes` |
| Cancel → Ringba | `POST /v2/{account}/calls/void`, reversible |
| Settle check (re-reads the call to confirm) | ✅ `SettleWindow = 12s` |
| Immutable audit row per action | ✅ PG migration `000026` |
| **`has_converted` on `GET /pings`** | ❌ **absent** — `PING_SELECT` omits it (`pings.service.ts:41-47`) |
| **Conversion filter on `GET /pings`** | ❌ **absent** — `pings.filters.ts` has platform/from/to/search/show_demo only |
| **`writeback_enabled` on `GET /integrations`** | ❌ **absent** from `IntegrationRecord` |
| **Status column in the call list** | ❌ columns are Time/Platform/Call ID/State/Event/Revenue/Dup |
| **Convert / Cancel buttons** | ❌ none anywhere |
| Dialog / modal primitive | ❌ none in `web/src/components/ui/` |

ClickHouse columns available: `has_converted UInt8`, `duration_seconds UInt32`,
`required_duration_seconds Nullable(UInt32)`, `revenue`, `payout`,
`was_conversion_adjusted`, `was_payout_adjusted`.

---

## Locked decisions

### D1 — "Should have converted" rule (operator chose: **Either condition**)

```sql
has_converted = 0
AND ( revenue > 0
   OR ( required_duration_seconds IS NOT NULL
        AND duration_seconds >= required_duration_seconds ) )
```

Catches both *earned money but unconverted* and *talked long enough but unconverted*.
Wider net, more false positives — accepted deliberately.

### D2 — Convert amount

- Pre-fill the call's **revenue when it is > 0**; otherwise leave **blank and required**.
- Reason: under D1 the list contains calls with `revenue = 0`, and the API rejects a non-positive
  amount. Calyx does not store a per-campaign conversion price, so there is nothing else to default to.
- Existing **$25 cap** (`ValidateAmount`) is left unchanged in this slice. Flagged to management as a
  question — real conversions above $25 are refused today.
- Confirm dialog on every action, showing exactly what will be written to Ringba.

### D3 — Which integration to write through

The endpoint is per-integration. The UI resolves it by fetching `GET /integrations` and selecting:
`platform = 'ringba'` **and** `connection_mode = 'api_sync'` **and** `sync_mode = 'real'`.

- 0 matches → buttons hidden, reason shown.
- >1 match → use the first and log; multi-Ringba-account tenants are out of scope for v1.

### D4 — Amount semantics must be visible to the operator

The dialog states plainly that the amount **sets the call's value in Ringba** and will sync back as
that call's revenue. This is not a label; it is money.

---

## Build order

### B1 — `has_converted` on the ping list (backend)

- `services/api/src/pings/pings.service.ts`
  - add to `PING_SELECT`: `has_converted`, `duration_seconds`, `required_duration_seconds`
  - extend `PingRecord`; map `has_converted` UInt8 → boolean in `mapPingRow`
- `services/api/src/pings/pings.filters.ts` — no change here
- **Proof:** `GET /pings?limit=1&show_demo=true` returns `has_converted`.

### B2 — Conversion filter (backend)

- `pings.filters.ts` — add `conversion?: 'converted' | 'unconverted' | 'should_convert'`
  - `converted` → `AND has_converted = 1`
  - `unconverted` → `AND has_converted = 0`
  - `should_convert` → the D1 clause
- `pings.controller.ts` — accept + validate the query param (Zod, reject unknown values)
- Apply to `GET /pings` **and** `GET /pings/export` so CSV matches the screen.
- **Proof:** each value returns a different, correct count; an invalid value is rejected cleanly,
  not a 500. (Observed: HTTP **400** with body `statusCode: 422` + field error — `BadRequestException`
  in the shared `parseZod` helper always sends 400. Left as-is; changing it would affect every
  endpoint using that helper.)

### B3 — `writeback_enabled` on `GET /integrations` (backend)

- `integrations.service.ts` list query already reads the column — add it to the returned shape.
- `web/src/lib/api.ts` — add to `IntegrationRecord`.
- **Why:** lets the UI explain *why* buttons are unavailable instead of failing with a 403 on click.

### B4 — API client (frontend)

- `web/src/lib/api.ts`
  - extend `PingRecord` with the three new fields
  - `convertCall(token, integrationId, { inbound_call_id, amount })`
  - `voidCall(token, integrationId, { inbound_call_id })`
  - both via `fetchWithAuth` (CSRF header is already automatic for non-GET)
  - a `WritebackResult` type mirroring the Go `Result` struct

### B5 — Confirm dialog (frontend, new component)

- `web/src/components/ui/ConfirmDialog.tsx` — focus trap, Escape to close, backdrop click,
  `prefers-reduced-motion` respected, no new dependency.

### B6 — Status column (frontend)

- `web/src/components/dashboard/PingsTable.tsx` — add **Status**:
  - `has_converted = 1` → **Converted** (teal)
  - matches D1 → **Review** (amber) — the actionable state
  - otherwise → **—** (muted)

### B7 — Filter control (frontend)

- `web/src/components/ui/FilterBar.tsx` — optional `showConversion` segmented control:
  All · Converted · Not converted · **Should have converted**
- `web/src/lib/date-filter.tsx` — carry `conversion` in the existing context alongside `showDemo`
- `web/src/app/(app)/pings/page.tsx` — pass it through; reset pagination on change (same as `showDemo`)

### B8 — Convert / Cancel buttons (frontend)

- `web/src/app/(app)/calls/[callId]/page.tsx`
  - Convert button when `has_converted` is false; Cancel when true
  - resolve integration per D3; hide with a reason when unavailable
  - on success: toast with the settle outcome, refresh the deep view
  - on 403 / 409 / 400: show the API's message, never a raw stack

---

## Verification

```bash
# backend
curl -s "$API/pings?limit=1&show_demo=true" -H "$AUTH" | grep has_converted
curl -s "$API/pings?conversion=should_convert&show_demo=true" -H "$AUTH"
curl -s "$API/pings?conversion=bogus" -H "$AUTH" -o /dev/null -w '%{http_code}'   # expect 422

cd services/api && npm test
cd web && npm run build
```

Browser (required — a 200 is not proof):

1. `/pings` → Status column renders; each filter value changes the rows.
2. Filter to **Should have converted** → every row shows **Review**.
3. Open a Review call → Convert dialog pre-fills revenue when > 0, blank when 0.
4. Amount `0` and `26` → both rejected client-side with a readable message.
5. Convert a **sandbox** call → success toast, status flips to Converted, audit row exists.
6. Cancel it → status flips back, `void_reversible` true.
7. Zero CSP violations and zero console errors throughout.

Then the existing gates: `verify-system`, `verify-ingestion-slice`, `verify-presentation`.

---

## Out of scope

- **Retreaver** convert/cancel — different mechanism, `call_key` blocker, not researched.
- Payments / payout override — deliberately not exposed by the backend.
- Changing the **$25 cap** or the settle window.
- Bulk / multi-select convert.
- The step-2 UI clean-up (duplicate rows, coverage-vs-dashboard mismatch, Publishers card).
- Any change to write-back behaviour, gating, or audit.

---

## Risks

| Risk | Handling |
|---|---|
| Operator converts at the wrong amount | Dialog states the amount sets the call's value in Ringba; confirm required |
| `has_converted` is `UInt8 DEFAULT 0` — cannot express "unknown" (audit F038 / K8) | Treat 0 as not-converted, as the deep view already does. Documented, not fixed here |
| Ping list shows raw rows, so a re-synced call can appear twice with different statuses | Known step-2 issue; note it in the report, do not fix here |
| `writeback_enabled` is false by default per tenant | Buttons hidden with a reason; no silent 403 |
| Convert against a real customer call by accident | Sandbox campaign or `sync_mode=mock` only during development |
