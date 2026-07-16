# Ping Feed vs Coverage counts + 56-states bug

**Date:** 2026-07-14  
**Scope:** Read-only. New-account Retreaver backfill. No credentials / ANI.

---

## Final line

**Feed vs Coverage explained: YES; extra-states cause: Canadian provinces (BC/MB/NS/ON) via permissive platform-sync NormalizeState + PR territory; header fix recommended: B (distinct calls).**

---

## Verdict

| Item | Result |
|------|--------|
| PART 1 — Feed raw vs Coverage distinct | **Explained** — Coverage `5,382` = distinct calls with `state != ''`; Feed header = raw `count()` (snapshot 20,577; live API later ~27k as backfill continues) |
| PART 2 — “56 states” | **Bug / product gap** — 51 US (50+DC) + **PR** + **4 Canadian provinces** (`BC`, `MB`, `NS`, `ON`) |
| PART 3 — Feed header semantics | **Recommend B** — headline = distinct calls; table stays raw + DUP |

Duplicate example: `78b653b3-…` has **multiple raw rows** (`is_duplicate` 0 then 1…) from scheduled re-sync — why Feed ≫ Coverage. Content-hash dedup (Phase 2.0) stops *new* writes; header semantics are independent.

---

## PART 1 — Reconcile totals

Tenant: `11111111-1111-1111-1111-111111111111` (all platforms unless noted).

| Metric | Value (CH at check time) | Role |
|--------|--------------------------|------|
| `count()` raw rows | **27,025** (+ climbing; UI snapshot was **20,577** mid-backfill) | Ping Feed `total` / `displayTotal` ≈ API `GET /pings` `total` |
| `uniqExact(external_call_id)` | **5,392** | Distinct calls (all states incl. empty) |
| `countIf(state='')` empty-state rows | **57** | Raw rows excluded from Coverage filter |
| `countIf(state!='')` | **26,968** | |
| `uniqExact(external_call_id)` where `state != ''` | **5,382** | **Matches Coverage `total_pings`** |

Live API check (same window):

| Endpoint | Result |
|----------|--------|
| `GET /pings?limit=1` → `total` | ~27,165 (raw) |
| `GET /coverage/heatmap` → `total_pings` / `data.length` | **5,382** / **56** |

### Why the gap

**Coverage** (`coverage.service.ts`):

1. Dedupes to one row per `(platform, external_call_id)` via `argMax(..., received_at)`.
2. Drops empty `state`.
3. Sums `ping_count` per state → `total_pings` = **distinct calls with a non-empty state**.

**Ping Feed**:

- Header uses poll `total` = ClickHouse **raw row count** for the filter (no identity dedupe).
- Table shows every ingested row including Redis-dedup *marked* duplicates (`is_duplicate=1`) from backfill/re-sync.

So: **Feed ≫ Coverage is expected** while re-syncs write new rows for the same uuid. Example `78b653b3-04c2-4119-94d5-70d63589df23`: multiple rows over successive syncs (first `is_duplicate=0`, later `1`).

Retreaver-only slice at check: raw **26,057** · distinct **4,437** · distinct-with-state **4,427** (other platforms add the rest of Coverage’s 5,382).

---

## PART 2 — The 56-states bug

### Distinct `state` values: **56**

All observed codes are **2-letter uppercase** (no lowercase split, no full names, no whitespace junk). Length = 2 for every group.

### US mainland + DC (51) — present

`AK AL AR AZ CA CO CT DC DE FL GA HI IA ID IL IN KS KY LA MA MD ME MI MN MO MS MT NC ND NE NH NJ NM NV NY OH OK OR PA RI SC SD TN TX UT VA VT WA WI WV WY`  
(= 50 states + DC).

### Exact extras beyond 51

| Code | Calls (distinct) | Classification |
|------|------------------|----------------|
| **PR** | 10 | US territory — allowlisted in **ingestion** `validCodes`; map “US” products often treat separately from 50+DC |
| **BC** | 2 | Canada — British Columbia |
| **MB** | 1 | Canada — Manitoba |
| **NS** | 1 | Canada — Nova Scotia |
| **ON** | 1 | Canada — Ontario |

**51 + PR + BC + MB + NS + ON = 56.**

### Root cause (not a Retreaver “normalize miss” of lowercase/full names)

Two `NormalizeState` implementations diverge:

| Path | Behavior for unknown 2-letter (e.g. `ON`, `BC`) |
|------|--------------------------------------------------|
| **ingestion** `services/ingestion/internal/state` | Only codes in `validCodes` (US + DC + PR/GU/VI). Unknown → **`""`** |
| **platform-sync** `services/platform-sync/internal/state` | **Any** `len==2` alpha → **pass through** (`return upper`) |

Retreaver Live pulls go through **platform-sync**, so Canadian `caller_state` values are stored as-is. They then appear on the US Coverage heatmap and inflate “states” to 56.

This is **genuine non-US geo on the wire**, plus a **normalization policy bug** (sync too permissive vs ingest allowlist).

### Recommendation (report only — no silent change)

1. **Align** platform-sync `NormalizeState` with ingestion: allowlist US (+ decide territories PR/GU/VI); unknown codes → `''`. That alone would drop Coverage regions toward ≤51 (+PR if kept).
2. **US heatmap product decision:**
   - **(a) Prefer for current UI:** exclude non-US from the US map / “states” label (and optionally a future “international” bucket).
   - **(b)** Keep Canada visible but **relabel** (“regions” / “geos”), not “56 states”.
3. **PR:** either count as US territory in map/legend, or exclude from “states” copy so the headline says ≤51 (+territory note).

---

## PART 3 — Ping Feed header: raw vs calls

| Option | Behavior | Pros / cons |
|--------|----------|-------------|
| **A** | Keep raw “stream events”; add separate “distinct calls” | Honest event volume; still easy to misread as call volume if raw stays primary |
| **B (recommend)** | Header = **distinct calls** (same identity as Coverage/dashboard); table still lists raw rows + DUP | Headline numbers agree; operators stop comparing 20k vs 5k as a data bug |

**Recommend B.** Content-hash dedup (Phase 2.0) reduces *future* duplicate writes; **header semantics should still be “calls”** even after that, so analytics language stays consistent.

Implementation sketch (not this pass): `GET /pings` total as `uniqExact(external_call_id)` for the filter, or a sibling `distinct_total` field and UI uses that for the header.

---

## Code refs

- Coverage dedupe + empty-state filter: `services/api/src/coverage/coverage.service.ts`
- Coverage copy “X pings across Y states”: `web/src/app/(app)/coverage/page.tsx`
- Feed header `displayTotal`: `web/src/lib/use-hybrid-ping-feed.ts` + `web/src/app/(app)/pings/page.tsx`
- Permissive sync normalizer: `services/platform-sync/internal/state/state.go`
- Strict ingest allowlist: `services/ingestion/internal/state/state.go`

---

## One-line

Feed (**raw rows**, e.g. 20,577→27k) vs Coverage (**5,382** distinct non-empty-state calls) is expected; “56 states” = 50+DC + PR + Canadian **BC/MB/NS/ON** from permissive platform-sync NormalizeState — recommend US allowlist + Feed header = distinct calls (**B**).
