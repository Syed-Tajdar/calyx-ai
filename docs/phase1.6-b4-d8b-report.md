# Phase 1.6 B4 — D8b Call ID / deep-link (implementation report)

**Date:** 2026-07-13  
**Slice:** Retreaver Call Logs identity ↔ Calyx Ping Feed / Call detail  
**Credentials:** never printed.

---

## Verdict

| Case | Decision |
|------|----------|
| **Case A** | **SELECTED** — Call Logs UI shows **UUID** = our `external_call_id` (API `uuid` → `call_uuid` → `id`). |
| **Case B** | **SKIPPED** — no `platform_display_id` column/Kafka path. |

**Pending:** Syed Call Logs screenshot + exact `{id}` URL confirmation.

### Evidence for Case A

| Source | Finding |
|--------|---------|
| Fixture [`retreaver_calls.json`](../services/platform-sync/internal/adapter/testdata/retreaver_calls.json) | Keys: `uuid`, `call_uuid`, `id` (empty-id case); **no `cid`** |
| Mapper [`retreaver_adapter.go`](../services/platform-sync/internal/adapter/retreaver_adapter.go) | Precedence `uuid` → `call_uuid` → `id` → `ExternalCallID` |
| Dedup / aggregates | Identity stays `(platform, external_call_id)` — unchanged |

---

## Implementation (what to look at)

### 1. Deep-link config (not hardcoded only in table UI)

| File | Role |
|------|------|
| [`shared/providers/platform-deep-links.json`](../shared/providers/platform-deep-links.json) | Catalog pattern + notes |
| [`web/src/lib/platform-deep-links.ts`](../web/src/lib/platform-deep-links.ts) | `buildPlatformCallUrl`, `hasPlatformCallDeepLink`; env override |
| [`.env.example`](../.env.example) | `NEXT_PUBLIC_RETREAVER_CALL_URL_PATTERN` |

Default pattern:

```text
https://retreaver.com/calls/{id}
```

`{id}` → `encodeURIComponent(external_call_id)`.

### 2. Ping Feed CALL ID

[`web/src/components/dashboard/CallIdCell.tsx`](../web/src/components/dashboard/CallIdCell.tsx)

- Display truncate ~20 chars (`formatCallIdDisplay`).
- `title={full id}` on hover.
- Copy button → `navigator.clipboard.writeText(callId)`.
- Wired from [`PingsTable.tsx`](../web/src/components/dashboard/PingsTable.tsx).

### 3. Search

Unchanged — still `external_call_id` via [`pings.filters.ts`](../services/api/src/pings/pings.filters.ts) + hybrid feed client filter. Case A needs no second-id search.

### 4. Call detail

[`web/src/app/(app)/calls/[callId]/page.tsx`](../web/src/app/(app)/calls/[callId]/page.tsx)

- Labelled Call ID + copy (`CallIdCell` / same helpers).
- **Open in Retreaver** when `buildPlatformCallUrl(platform, callId)` non-null (`platform === 'retreaver'`).
- Also surfaced in [`CallDeepViewPanel.tsx`](../web/src/components/native/CallDeepViewPanel.tsx).

### 5. CSV export

`external_call_id` already in [`pings.service.ts`](../services/api/src/pings/pings.service.ts) export columns — no change required for Case A.

---

## Case B (deferred — when to add)

If screenshot proves Call Logs show a **short id ≠ UUID**:

1. CH migration `platform_display_id` (nullable) — same path as `occurred_at`.
2. Map field in Retreaver adapter; Kafka → consumer → API.
3. Feed display prefers `platform_display_id`; identity/dedup stays `external_call_id`.
4. Search matches either id; CSV both columns.

---

## Checklist

| Item | Status |
|------|--------|
| Case A selected | ✅ |
| Case B column not added | ✅ (intentional) |
| Deep-link in catalog + env helper | ✅ |
| Ping Feed: fuller ID + hover + copy | ✅ |
| Search on `external_call_id` | ✅ |
| Call detail + Open in Retreaver | ✅ |
| CSV has `external_call_id` | ✅ |
| Syed screenshot | ⬜ pending |

**PASS/FAIL slice:** **PASS** (Case A UX + config). Full D8b “both copy directions + deep link proven in browser” still wants Syed URL/screenshot confirmation.
