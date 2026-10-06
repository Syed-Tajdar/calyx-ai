# Calyx — Convert / Cancel Screens Report (Ringba)

**Plan:** `docs/convert-cancel-screens-plan.md`
**Date:** 2026-09-30
**Step:** 3 of 7 · **Platform:** Ringba only · **Constraint honoured:** no push, no commit, no Ringba writes.

---

## 1. Scoreboard

| Item | Result |
|---|---|
| B1 `has_converted` on the call list | **PASS** |
| B2 Conversion filter (incl. "should have converted") | **PASS** |
| B3 `writeback_enabled` exposed on `/integrations` | **PASS** |
| B4 API client (`convertCall` / `voidCall`) | **BUILT** — not exercised against Ringba |
| B5 Confirm dialog | **BUILT** — not seen on screen |
| B6 Status column | **PASS** |
| B7 Filter control | **PASS** |
| B8 Convert / Cancel buttons | **BUILT** — unavailable-path PASS, action path **UNPROVEN** |

**Builds:** API image 0 · web image 0 (`✓ Compiled successfully`, 16/16 pages).
**Tests:** API Jest **15/15** · platform-sync adapter `ok`.

---

## 2. Locked decisions as built

| # | Decision | Built as |
|---|---|---|
| D1 | "Should have converted" = **either condition** (operator's choice) | `has_converted = 0 AND (revenue > 0 OR (required_duration_seconds IS NOT NULL AND duration_seconds >= required_duration_seconds))` |
| D2 | Amount pre-fill | Pre-fills the call's revenue when `> 0`; blank and required when `0`, because the API rejects a non-positive amount and Calyx stores no per-campaign conversion price |
| D3 | Which integration | First `platform=ringba` + `connection_mode=api_sync` + `sync_mode=real` |
| D4 | Amount semantics shown | Dialog states the amount **sets the call's value in Ringba** and syncs back as its revenue |

The **$25 cap** (`writeback.ValidateAmount`) was left unchanged, and is raised to management as an
open question — real conversions above $25 are refused today.

---

## 3. Proof

### B1 — conversion state on the call list `[PROVEN]`

`GET /pings` previously omitted it (`PING_SELECT`). Now returns `has_converted` as a boolean plus
`duration_seconds` and `required_duration_seconds`:

```
external_call_id             'demo-call-1'
revenue                      8.5
has_converted                False
duration_seconds             0
required_duration_seconds    None
```

### B2 — conversion filter `[PROVEN]`

| `conversion=` | calls |
|---|---|
| *(absent)* | 53 |
| `converted` | **0** |
| `unconverted` | **53** |
| `should_convert` | **53** |

Correct for this dataset: all 53 demo rows are unconverted with `revenue > 0` and no required
duration, so `should_convert` legitimately matches all of them.

Invalid input is rejected cleanly, not a 500:

```json
{"statusCode":422,"message":"Validation failed",
 "errors":{"fieldErrors":{"conversion":[
   "Invalid enum value. Expected 'converted' | 'unconverted' | 'should_convert', received 'bogus'"]}}}
```

(HTTP status is **400** — `BadRequestException` in the shared `parseZod` helper always sends 400
while carrying `422` in the body. Left as-is; changing it would affect every endpoint using it.)

Applied to `GET /pings/export` too, so the CSV matches the screen.

### B3 — `writeback_enabled` on `/integrations` `[PROVEN]`

All four integrations return `writeback_enabled: false` (the correct per-tenant default). Exposed so
the UI can explain unavailability instead of surfacing a 403 after the operator clicks.

### B6 / B7 — Status column and filter `[PROVEN in browser]`

Header row is now `Time · Platform · Call ID · State · Event · Revenue · Status · Syncs`, and
`demo-call-1` renders **Review** (amber).

Clicking each filter in a real browser:

| Filter | Rows rendered |
|---|---|
| Converted | **0**, empty state shown |
| Not converted | 25 (page size; 53 total) |
| Should have converted | 25 |
| All | 25 |

Matches the API exactly. The badge and the filter share one predicate (`shouldHaveConverted` in
`web/src/lib/api.ts`), so they cannot drift apart.

### B8 — unavailable path `[PROVEN in browser]`

With no live Ringba API connection the call detail page renders:

```
CONVERSION
Not converted in Ringba
No live Ringba API connection — add one in Settings and set it to Live.
```

No button is offered. This is the designed behaviour: explain, don't fail on click.

---

## 4. NOT PROVEN — read this before trusting the feature

**The Convert button, the Cancel button and the confirm dialog have never been seen working.**

They compile and the code paths are wired, but every integration on this machine is `webhook` mode,
so `resolveWritebackIntegration` correctly finds nothing and the controls never render. Proving them
requires:

1. A Ringba **API-key** integration in Settings, set to **Live**
2. `integrations.writeback_enabled = true` for that tenant
3. `WRITEBACK_ENABLED=true` (already set in local compose)
4. A **sandbox** call to act on — never a customer call

Deliberately not faked. Creating a "Live" integration with invented credentials is exactly the kind
of shortcut the project's own process warns against, and a convert writes real money into Ringba.

Also unproven: the 403 / 409 / 400 error paths, and the settle-window outcome text
(`settled` vs pending).

---

## 5. Notes and limitations

1. **Demo data shows no variety.** All 53 rows are unconverted with revenue > 0, so every row reads
   **Review**. The filter is correct; the fixture simply has no converted calls and no
   `required_duration_seconds`. Real Ringba data will show a mix. No fake conversions were seeded to
   make the screenshot look better — that is how the `csp-live-` contamination started.

2. **`has_converted` cannot express "unknown"** — it is `UInt8 DEFAULT 0` in ClickHouse (audit
   F038 / K8). A call the platform never reported on is indistinguishable from one reported as not
   converted. Treated as not-converted here, matching the deep view. Not fixed in this slice.

3. **Error messages now surface the API's own text.** `fetchWithAuth` discarded the response body
   and threw `Request failed: /path`, which would have shown an operator that string instead of
   "call already converted". It now reads Nest's `message` field, falling back to the old text.
   This affects every call in the app, not just write-back.

4. **Multi-account tenants out of scope** — `resolveWritebackIntegration` takes the first matching
   Ringba integration.

5. **Gates not run:** `verify-system`, `verify-ingestion-slice`, `verify-presentation` are
   PowerShell and `pwsh` is not installed on this macOS host (the Homebrew cask needs an admin
   password). The Node gates that could run — `check-smoke-call-id-prefix`, `check-ws-origin` —
   both pass.

---

## 6. Files changed

**Backend**
- `services/api/src/pings/pings.service.ts` — `has_converted` / durations on the list; `sync_count`; CSV column
- `services/api/src/pings/pings.filters.ts` — `conversion` filter + D1 predicate
- `services/api/src/pings/pings.controller.ts` — `conversion` param, `parseZod`
- `services/api/src/integrations/integrations.service.ts` — `writeback_enabled` on the record

**Frontend**
- `web/src/lib/api.ts` — `convertCall`, `voidCall`, `WritebackResult`, `shouldHaveConverted`, `resolveWritebackIntegration`, error-message extraction
- `web/src/lib/date-filter.tsx` — `conversion` in shared filter state
- `web/src/components/ui/FilterBar.tsx` — Status segmented control
- `web/src/components/ui/ConfirmDialog.tsx` — **new**, in-repo (no new dependency)
- `web/src/components/native/ConversionActions.tsx` — **new**
- `web/src/components/dashboard/PingsTable.tsx` — Status column
- `web/src/app/(app)/pings/page.tsx`, `web/src/app/(app)/calls/[callId]/page.tsx`

---

## STOP

No push. No commit. Convert / Cancel remain **API-only in practice** until the live path is proven.
