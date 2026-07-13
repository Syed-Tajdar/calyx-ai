# Phase 1.6 — Evidence Report (Part A)

**Date:** 2026-07-13  
**Scope:** Evidence only. No aggregate product changes in this document’s capture window (Part B followed after Decision Gate match).  
**Credentials:** never printed.

---

## A1 — Official last-modified sources

| Platform | Last-modified fetch? | Param(s) | Official doc URL | Confidence |
|----------|----------------------|----------|------------------|------------|
| Retreaver | **YES** | `updated_at_start`, `updated_at_end`; `sort_by=updated_at` | https://retreaver.github.io/core-api-docs/ | **HIGH** (docs + live A2) |
| Ringba | **UNKNOWN / likely NO as incremental last-mod** | Call Logs use date-range reporting; filters are attribute filters (buyer, campaign, …), not a documented “modified since” cursor | https://support.ringba.com/hc/en-us/articles/17994137377815-Reporting-Filters ; Call Details API notes on https://updates.ringba.com/ ; rate limits https://support.ringba.com/hc/en-us/articles/23787558523159 | **LOW — confirm by live probe in Ringba phase** (third-party PHP SDK alone is not used for NO) |
| TrackDrive | **NO as filter** | `created_at_from`/`to`, `ended_at_from`/`to`; `updated_at` is **order**/column only | https://trackdrive.com/api/docs/1.0/calls/index.html | **HIGH** |
| CallGrid | **NO as documented last-mod filter** | Official download examples: `startDate` / `endDate` only; optional tag `filters` JSON — no `updated_at_*` param documented | https://callgrid.com/api | **MEDIUM** (vendor page; no last-mod param named) |

Also updated in `docs/apisync-platform-research.md`.

---

## A2 — Live probe `updated_at_*`

**Tool:** `services/platform-sync/cmd/probe-retreaver-updated-at`  
**Call:** `ffb60c8b-d4f7-4ec7-9287-c0c44b60fe32` (timestamps/revenue only)

```
created_at=2026-07-06T16:00:13.475Z
updated_at=2026-07-06T16:18:01.106Z
revenue=21.544
probe_updated_at_start_before=2026-07-06T16:16:01Z end_bound=2026-07-06T16:48:01Z appears=true
probe_updated_at_start_after=2026-07-06T16:20:01Z end_bound=2026-07-06T16:48:01Z appears=false
probe_updated_at_end_includes appears=true
probe_updated_at_end_before_update appears=false (expect false)
sort_by_updated_at_page1_first=064d9085-… stable_repeat=true
updated_at_cursor_VIABLE=true
```

**Semantics:** Match docs — `updated_at_start` / `updated_at_end` filter; `sort_by=updated_at` stable across repeat page-1.  
**Pagination hazard:** Open-ended `updated_at_start` + `order=desc` can bury older matches past `maxPagesPerSync`; safety delta + bounded progress required (Decision 1).  
**Revenue → `updated_at`:** Official docs do **not** state that a revenue/conversion change bumps `updated_at`. Samples show `conversions_determined_at` near `updated_at` but that is **not** an explicit guarantee → `docs: UNVERIFIED` for B5.

**Verdict: `updated_at cursor VIABLE: YES`**

---

## A3 — `platform_duplicate` populate test

Webhook Retreaver (dev token) with `is_duplicate=1`:

```
ingest=200
phase16-dup-fc58eb79	0	1
```

EXPECT met: `platform_duplicate=1`, `is_duplicate=0` (first sighting).  
**Wiring: ALIVE** (not dead). Test row deleted via `ALTER TABLE pings DELETE …`.

| Platform | Payload flags read | Meaning (citation) |
|----------|--------------------|--------------------|
| Retreaver | `is_duplicate`, `duplicate` | Connector treats as boolean platform flag; Retreaver postback docs do not fully define billable vs caller-dupe in our repo — treat as **platform-declared duplicate lead/call** for aggregates |
| Ringba | `is_duplicate`, `IsDuplicate`, `duplicate` | Same boolean mapping in connector |
| TrackDrive / CallGrid | `is_duplicate`, `duplicate` | Same helper |

---

## A4 — Current backfill / empty-cursor behaviour (**before B3**)

Grep (non-test):

```
retreaver_adapter.go:21  const retreaverInitialLookback = 30 * 24 * time.Hour
retreaver_adapter.go:77  empty → updated_at_start = now − 30d
registry.go:9             maxPagesPerSync = 10
ringba/trackdrive/callgrid still use 24h empty windows (out of Retreaver scope)
```

| Item | Behaviour (pre-B3) |
|------|--------------------|
| Constant | Hardcoded Retreaver-only **30 days** (not 24h) |
| Empty / reset cursor | Silent **30-day** `updated_at` flood |
| Ongoing | `hasMore=false` → was `now−5m` (pre-B1); mid-window continues via `page` across syncs (10 pages/run) |
| Rate limits | Retreaver docs: consult team for concurrency; our HTTPClient retries **429** with quadratic 500ms backoff (3 attempts). Busy 30d × 10 pages/sync is **resumable** via page cursor but can take many sync ticks — respect 429 |
| UI | Was **not** surfaced |

**Checklist:** (1) A2 YES → cursor on `updated_at` viable; advance should be max-seen − delta (B1). (2) Rate limits: backoff present; no Retreaver published RPM — observe 429. (3) Configurable + UI → B3.

---

## A5 — D8b identifiers

Fixture `services/platform-sync/internal/adapter/testdata/retreaver_calls.json` id-shaped keys:

| Key | Type | Mapped? |
|-----|------|---------|
| `uuid` | string | yes → `external_call_id` |
| `call_uuid` | string | fallback |
| `id` | string | fallback |
| campaign/target ints, `afid` | int/string | yes (not call display id) |
| `cid` | — | **not in fixture / not coded** |

Syed screenshot / call-detail URL pattern: **not provided** this session.  
**Conclusion: Case A** (UI Call ID = our `uuid` / `external_call_id`), **pending Syed screenshot confirmation**. Deep-link pattern stored in config (`platform-deep-links` / env), not hardcoded in shared table logic.

---

## Decision Gate

| # | Condition | Result |
|---|-----------|--------|
| 1 | A2 VIABLE YES | **MATCH** → `updated_at` + 15m safety delta |
| 2 | A3 populates | **MATCH** → rule **(b)** |
| 3 | Backfill explicit / configurable | Proceed B3 (replace silent 30d const with per-integration + UI) |

**No stop — Part B proceeds.**
