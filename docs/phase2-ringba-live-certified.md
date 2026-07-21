# Phase 2 — Ringba live certification

**Date:** 2026-07-20  
**Integration:** `ae17d28e-3f98-4288-abf9-ef1bfa978776` (“Real R. Test”)  
**Credentials:** UI-only — never printed. ANI redacted in all probes.  
**End state:** flipped back to **Test** (`sync_mode=mock`).

**Verdict: YES (live certified)** — D8 A1 / D9 / D10 / D5 / D8b all passed on the live account.  
Note: `POST|GET …/v2/{accountId}/calllogs/details` returns **HTTP 404** on this API host; A1 was proven via official **Get Call Log** (`POST …/calllogs`) exact-record match (same migration class as retired `CallLogs/Date`).

---

## Live window summary

| Item | Value |
|------|--------|
| Adapter | `POST https://api.ringba.com/v2/{accountId}/calllogs`, `Authorization: Token …`, body `reportStart`/`reportEnd`/`filters`/`size`≤1000, response `report.records[]` |
| Sync during cert | `sync_mode=real`, `sync_status=ok`, ~2 min tick, ~100–120 calls/page, published deltas small (content-hash) |
| CH after live | ~378 rows / ~330 uniq (all-time ringba on this stack at close) |
| Deep-view | Stable 4/4 (attributed / unattributed / transcript / Retreaver) before cert — see prior verify |

---

## D8 — A1 id proof

**Proof call:** `RGB6044D6A54617A46BFB9709D08C9B2899736A94C2RGB9B02`

| Step | Result |
|------|--------|
| `…/calllogs/details` GET/POST + `InboundCallIds` | **HTTP 404** HTML “resource cannot be found” (all variants tried) |
| `POST …/calllogs` 24h window, exact `inboundCallId` in `report.records[]` | **found=true**, `isSuccessful=true` |
| Ringba fields | `conversionAmount=34.75`, `tag:InboundNumber:State=LA`, campaign/publisher/target present |
| Calyx CH `argMax` | `revenue=34.75`, `state=LA`, same campaign/publisher/target |

**D8 A1: PASS** (id round-trip + field match via Get Call Log).  
**Details path: unavailable (404)** — do not block A1; track as Ringba surface gap.

---

## D9 — value-change (0 → nonzero)

Natural conversion observed on the same proof call (no hand-insert):

| received_at (UTC) | revenue | payout | state | is_duplicate |
|-------------------|---------|--------|-------|--------------|
| 2026-07-20 18:06:22 | **0** | 0 | LA | 0 |
| 2026-07-20 18:14:22 | **34.75** | 27.8 | LA | 0 |

```
argMax(revenue)=34.75  rows=2  uniqExact(external_call_id)=1
```

`/pings?search=<id>` returns both rows; `max_rev_in_list=34.75`.

**D9 API-level: YES** — new row on conversion; `argMax` picks new revenue; counted once.

---

## D10 — volume sanity (bounded window)

| Source | Window | Distinct calls |
|--------|--------|----------------|
| Ringba Get Call Log API | 2026-07-19T18:59Z → 2026-07-20T18:59Z (24h) | **298** (1 page, `partialResult=false`) |
| Calyx ClickHouse | `occurred_at >= now()-24h` | **262** (310 rows) |

**Gap:** 36 (~12%). Explanation:

1. This cert stack’s Ringba CH history for the trailing day effectively starts mid-window (~14:05 UTC), not a full prior-day backfill to `windowStart`.
2. Tip lag: API `reportEnd` ≈ 18:59 vs CH `max(occurred_at)` ≈ 18:54.
3. Content-hash correctly suppresses unchanged republishes; does not inflate distinct.

Order-of-magnitude match; gap explained — **D10: PASS**.

---

## D5 — coverage map (2-letter NA states)

`GET /coverage/heatmap` (tenant default):

| State | ping_count | country |
|-------|------------|---------|
| WA | 661 | US |
| TX | 1023 | US |
| LA | 105 | US |

Ringba-only CH confirm: `WA=1`, `TX=31`, `LA=14` distinct calls. Codes are **2-letter**, not full names.

**D5: PASS**.

---

## D8b — two-way id + deep link

| Check | Result |
|-------|--------|
| Calyx exact search `?search=<inboundCallId>` | **total=2**, id match, states/revenues include LA / 34.75 |
| Calyx prefix search (12-char) | **total=2**, same call family |
| Ringba → Calyx (operator) | Copied **Inbound Call ID** from Ringba Call Details **Record** tab → Calyx search → **matched**. Revenue **34.75** / payout **27.80** / campaign+publisher+target IDs match UI ↔ stored |
| Calyx → Ringba | Same id exact-found in Get Call Log records (API) |
| Per-call deep link | **Does not exist** — `app.ringba.com/#/call/{id}` redirects to the dashboard. Calls open inside the Call Logs **report** table (`#/dashboard/call-logs/report/new`); no documented query param to pre-filter by `inboundCallId` (filters are UI `+Filter` only) |
| Catalog / “Open in Ringba” | Report-level URL + **COPY+OPEN**: clipboard gets `inboundCallId`, then opens report, toast instructs paste into Inbound Call ID filter |

**D8b: PASS** — two-way ID match **operator-confirmed**; deep link is **report-level**, not call-level (Ringba limitation).

---

## Throttle (live window)

| Metric | Value |
|--------|--------|
| Ringba sync completions / 60m | ~31 (~1 / 120s tick) |
| Estimated Get Logs req / 60m | ~31 (`size=1000` ⇒ ≤1 page/sync) |
| Rolling-60s peak (est.) | **1–2** (≪ 80) |
| True `HTTP 429` / `rate limited` in 120m logs | **0** |

Dual-window limiter (5/sec ∧ 80/min per `account_id`) held under load of this cert window.

---

## Close-out

```text
PATCH /integrations/ae17d28e-…/sync-mode  {"sync_mode":"mock"}
→ sync_mode=mock  name=Real R. Test  sync_status=ok
```

Smoke-test / ws-smoke **not** run during the cert window.

---

## GATE

```
Ringba D8 A1: PASS (Get Call Log exact id + fields; details endpoint 404)
Ringba D9 live value-change: YES (0 → 34.75; argMax; distinct=1)
Ringba D10: PASS (API 298 vs CH 262 / 24h; gap explained)
Ringba D5: PASS (WA/TX/LA 2-letter on heatmap)
Ringba D8b: PASS (two-way ID operator-confirmed; deep-link report-level only — no per-call URL)
Throttle ≤80/min, 429=0: PASS
Ringba CERTIFIED: YES
Ready for Phase 3 (TrackDrive): YES (Ringba live DoD closed)
```
