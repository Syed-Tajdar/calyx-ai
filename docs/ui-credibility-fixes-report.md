# UI Credibility Fixes Report (C1 / C2 / C3)

**Date:** 2026-07-23  
**Scope:** Part 1 only — credibility fixes. Part 2 (Ringba parity) **not started** (gate).  
**Proof surface:** rebuilt Docker images `deploy-api` / `deploy-web` on `:3000` / `:3001` (not `npm run dev`).

---

## Summary scoreboard

| Item | Result |
|------|--------|
| C1 — Enrichment attribute count | **PASS** |
| C2 — Duplicate CALL QUALITY card | **PASS** |
| C3.1 — Demo-row audit (with/without) | **PASS** |
| C3.2 — Shared predicate + consistent toggle | **PASS** |
| C3.3 — Anti-drift test | **PASS** |
| Gate — rebuild + verify scripts | **PASS** (see gate notes: presentation 20/20; system pipeline 4/4; local web build flake cleared) |

---

## C1 — Enrichment attribute count disagrees with itself

### Root cause

| UI location | Number shown | Exact source |
|-------------|--------------|--------------|
| Left column **Enrichment** | **520** | `CallDeepViewPanel` ← `deepView.enrichment.attribute_count` ← `COUNT(*)` in `services/api/src/native-integration/deep-query.service.ts` (`loadEnrichmentSummary`) |
| Right column **Audience Enrichment** | **500** | `EnrichmentPanel` ← `attributes.length` after `GET /enrichment/:aniHash?limit=500` |

Confirmed mechanisms (not assumptions):

1. **Payload LIMIT / hard cap at 500** — `EnrichmentApiService` clamped `safeLimit` to max 500.
2. **Wrong `total`** — controller returned `total: data.length` (capped length), not the true ClickHouse count.
3. **Not** a stored count column vs array length on the left — left used a real `COUNT(*)`. Right used capped array length.
4. **Not** key de-duplication — CH `count()` and `uniqExact(attribute_key)` both = **520**.

### True count (ClickHouse proof)

Call: `/calls/944e4ef2-859b-4d14-a24c-ccde6788cd54`  
`ani_hash=f6615ea25fcc38028b945dd9e68a1f48ab2e2ccc021334770828f7c2e6191731`

```
SELECT count() AS true_count, uniqExact(attribute_key) AS uniq_keys
FROM enrichment_attrs
WHERE ani_hash='f6615ea25fcc38028b945dd9e68a1f48ab2e2ccc021334770828f7c2e6191731'
  AND tenant_id='11111111-1111-1111-1111-111111111111'
→ 520	520
```

**Correct number: 520.**

### What changed

| File | Change |
|------|--------|
| `services/api/src/enrichment/enrichment.service.ts` | Separate `COUNT(*)` → `total`; return `{ data, total, limit }` |
| `services/api/src/enrichment/enrichment.controller.ts` | Returns full `EnrichmentListResult` |
| `web/src/components/enrichment/EnrichmentPanel.tsx` | Renders `520 attributes (showing 500 of 520)` when capped |
| `web/src/components/native/CallDeepViewPanel.tsx` | Removed left **Enrichment** mini-card (duplicate count) |
| `web/src/lib/api.ts` | Types include `limit` |

### Proof (rebuilt Docker API)

```
GET /enrichment/{aniHash}?limit=500
→ data=500  total=520  limit=500
```

Expected UI label (same formula as panel):  
`520 attributes (showing 500 of 520)`

Deployed web bundle (`deploy-web` `/app/.next/.../calls/[callId]/page.js`) contains:

- `Audience Enrichment`
- `` showing ${attributes.length} of ${total} ``
- `Table lists up to … attributes from the full set of …`
- Deep-view sections: **Ping / RTB**, **TCPA Risk**, **Ad Attribution** only — **no Enrichment section**

### DoD C1

- [x] Root cause with file references  
- [x] CH query proving 520  
- [x] Single attribute count source (Audience Enrichment panel + disclosed cap)  
- [x] Cap disclosed in UI text  
- [ ] Pixel screenshot of hard-refreshed `:3001` — **open item** (authenticated headless capture blocked in this environment; login smoke PNG only). **Operator:** hard-refresh `http://localhost:3001/calls/944e4ef2-859b-4d14-a24c-ccde6788cd54` and confirm one count label.

---

## C2 — Duplicate CALL QUALITY card

### Cause

Two **different** components on the call-detail page:

| Card | Component | File | Fields |
|------|-----------|------|--------|
| A (lean) | `CallDeepViewPanel` section `Call Quality` | `web/src/components/native/CallDeepViewPanel.tsx` | `overall_score`, `sentiment` only |
| B (rich) | `ScoreCard` | `web/src/components/scoring/ScoreCard.tsx` | score, sentiment, **platform badge**, **rules_passed / rules_failed** |

Mounted together from `web/src/app/(app)/calls/[callId]/page.tsx` (`CallDeepViewPanel` + `ScoreCard`).

### What changed

Removed the lean **Call Quality** section from `CallDeepViewPanel`. Kept `ScoreCard`.

### Fields before → after

| Field | Before (lean) | Before (ScoreCard) | After |
|-------|---------------|--------------------|-------|
| overall_score | ✓ | ✓ | ✓ (ScoreCard) |
| sentiment | ✓ | ✓ | ✓ |
| platform badge | ✗ | ✓ | ✓ |
| rules_passed / failed | ✗ | ✓ | ✓ |

**No unique field lost.**

### Proof (deployed bundle)

Call-detail SSR/client chunk shows a single `Call Quality` title inside `ScoreCard` (`rules_passed` / `rules_failed` / `PlatformBadge`). `CallDeepViewPanel` no longer emits a Call Quality section.

### DoD C2

- [x] Exactly one CALL QUALITY card in source + deployed call page bundle  
- [x] No field lost  
- [x] Cause named  
- [ ] Pixel screenshot — same open item as C1 (operator hard-refresh)

---

## C3 — Demo-row hiding (`apisync-%`)

### C3.1 — AUDIT FIRST (pre-fix, deployed API + ClickHouse)

Tenant `11111111-1111-1111-1111-111111111111`. Numbers at audit time (data grows with sync; treat as snapshot).

#### Headline KPIs — did they include `apisync-%`?

| KPI (dashboard) | Included demo? | Proof |
|-----------------|----------------|-------|
| **TOTAL REVENUE** | **YES** | CH `rev_with=82509.02` vs `rev_without=48837.77`; API metrics `82509.02` matched **with** |
| **TOTAL PINGS** | **YES** | CH `with_demo=9837` / `without=6844`; API metrics `9837` |
| **AVG CALL SCORE** | **YES** | `GET /scoring/scores?limit=10` → **10/10** `apisync-*` rows; avg derived from that list |
| **COMPLIANCE EXPOSURE** | **YES** | CH `exp_with=27505.15` vs `exp_without=16633.71`; API `27505.15` |

#### Surface audit table (pre-fix)

| Surface / endpoint | Excluded `apisync-%`? | Value WITH demo | Value WITHOUT demo |
|--------------------|----------------------|-----------------|--------------------|
| `GET /pings` `total` / `distinct_total` | **YES** (default) | distinct **9837** (`show_demo=true`) | distinct **6844** |
| `GET /analytics/metrics` Total Revenue / RPC / Total Pings | **NO** | pings **9837**, rev **82509.02** | pings **6844**, rev **48837.77** (CH) |
| `GET /analytics/platform-mix` | **NO** | sum counts **9837** | (same as metrics distinct) |
| `GET /analytics/pnl` / `/reports` | **NO** | rev **82509.02** | rev **48837.77** |
| `GET /coverage/heatmap` | **NO** | total_pings **9826** | **6833** (state≠'') |
| `GET /scoring/scores` Top Scores / Avg Call Score | **NO** | top10 all demo | (post-fix: 0 demo) |
| `GET /compliance/exposure` | **NO** | **27505.15** | **16633.71** |
| Dashboard ping timeline (`fetchPings` 7d) | **YES** via `/pings` default | — | inherits ping filter |

#### Diagnostic only — Coverage vs dashboard totals

| Metric | Value (pre-fix, includes demo) |
|--------|----------------------------------|
| Dashboard Total Pings (metrics) | **9837** (later grew slightly) |
| Coverage `total_pings` | **9826** |
| Gap | **~11** — Coverage subquery requires `state != ''`; metrics counts all distinct calls. **Not reconciled in this slice.** |

User-cited 9,022 / 9,778 were earlier snapshots; current CH/API numbers above supersede them for this report.

---

### C3.2 — FIX

**Single helper:** `services/api/src/common/demo-rows.ts` → `demoExclusionSql(column, showDemo?)`.

Wired into:

- `pings/pings.filters.ts` (`external_call_id`)
- `analytics/analytics.service.ts` (`external_call_id`)
- `coverage/coverage.service.ts` (`external_call_id`)
- `scoring/scoring.service.ts` (`call_id` on list)
- `compliance/compliance.service.ts` (`call_id` on list + exposure)

Controllers accept `show_demo`. Web: `DateFilterProvider.showDemo` + `FilterBar` toggle (“Demo included” badge) on dashboard, pings, reports, coverage, scoring, compliance.

Default: **excluded**. Demo rows **not deleted**.

### Proof after rebuild (Docker API)

```
metrics default:     pings=6844  rev=48837.7722
metrics show_demo:   pings=9852  rev=82677.7722
coverage default:    6833
coverage show_demo:  9841
exposure default:    16633.71
exposure show_demo:  27528.21
pings distinct:      6844 / +demo 9852
scores top10 demo:   0/10 default; 10/10 with show_demo=true
```

Sample default score IDs (UUID, not `apisync-`):

```
cdb56507-6c8f-48e4-8b6d-…  83.5
f70ab5c5-19c0-46cd-aac8-…  73.5
…
```

### C3.3 — Anti-drift

`services/api/src/common/demo-rows.spec.ts`:

- Asserts every required read path imports/`demoExclusionSql(` and has **no** inline `NOT LIKE 'apisync-%'`.
- Asserts analytics wires `demoExclusionSql('external_call_id'…)`.

**FAIL when predicate removed** (temporary strip of analytics helper):

```
Test Suites: 1 failed
Tests: 2 failed, 3 passed
> expect(analytics).toMatch(/demoExclusionSql\(\s*['"]external_call_id['"]/);
```

**PASS after restore:** `5 passed`.

### Grep — no duplicated inline SQL predicates

```
demoExclusionSql — only in demo-rows.ts + the five read-path files above
NOT LIKE 'apisync-%' — only inside demo-rows.ts (and the spec string)
```

### Demo rows still in ClickHouse

| When | `count()` where `external_call_id LIKE 'apisync-%'` |
|------|------------------------------------------------------|
| Before rebuild | **3005** |
| After verify | **3008** |

Count **increased** slightly (ongoing mock sync), not decreased — filter is read-time only.

### DoD C3

- [x] Full audit table  
- [x] Yes/no per headline KPI with query proof  
- [x] Single shared predicate  
- [x] Top Scores zero `apisync-%` by default (API + sample IDs)  
- [x] Toggle ON/OFF changes counts (metrics/coverage/exposure/scores)  
- [x] Anti-drift fail-then-pass demonstrated  
- [x] Demo rows still present  
- [ ] Dashboard Top Scores pixel screenshot — open item (operator hard-refresh `/dashboard`)

---

## Gate commands

```powershell
cd deploy
docker compose build api web
docker compose up -d --force-recreate api web
```

`scripts/verify-presentation.ps1` (standalone): **PRESENTATION VERIFY PASS** — API checks **20 / 20**.

`scripts/verify-system.ps1` (full run): Docker 14/14, Healthz 10/10, API 20/20, Web routes 13/13, WS PASS, Unit tests PASS, Pipeline **4/4 PASS**. First pass marked **SYSTEM VERIFY FAIL** solely because local `next build` hit a Windows `.next` rename race (`ENOENT` on `export/500.html`). Clean `Remove-Item .next; npm run build` afterward **exit 0**. Deployed `deploy-web` image was already rebuilt successfully before this flake.

---

## Honest limitations / open items

1. **Authenticated UI screenshots** of call detail + Top Scores not captured in this session (sandbox blocked token→browser automation). Deployed bundle + Docker API proofs stand; operator should hard-refresh `:3001` for visual confirmation.
2. **Avg Call Score** still averages the **top N scores fetched** (5), not a tenant-wide `avg(overall_score)` endpoint — pre-existing; demo exclusion fixed the wrong sample set, not the aggregation design.
3. **Coverage 6833 vs metrics 6844** (without demo) — empty-state calls; not fixed (out of scope).
4. **Ask AI / alerts** do not query `pings` aggregates in the same path set; not in the anti-drift file list. If they later add ping aggregates, they must call `demoExclusionSql`.

---

## Found but not fixed (out of scope)

- Dashboard **9,022 vs Coverage 9,778** (and current 6844 vs 6833) reconciliation  
- Ping chart period / `occurred_at` bucketing  
- Currency formatting / indigo→teal design pass  
- Publishers card real-data wiring  
- Part 2 Ringba parity (A + B1) — **blocked until this report is accepted**  
- Recording URL (B2)

---

## STOP — Part 1 complete

Do **not** begin Part 2 until this report is reviewed. Next deliverable when unblocked: `docs/ringba-parity-a-b1-report.md`.
