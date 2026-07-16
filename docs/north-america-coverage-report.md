# North America coverage — NormalizeState + Feed header

**Date:** 2026-07-14  
**Decision:** Keep **US + Canada** (all four platforms support Canada).  
**Scope:** Region allowlist + Coverage country tags + Ping Feed headline = distinct calls.  
**UI map details:** [`docs/coverage-north-america-ui-report.md`](coverage-north-america-ui-report.md).  
Credentials / ANI: never printed.

---

## Final line

**Coverage = US + Canada: YES; junk regions eliminated: YES; Feed header = distinct calls: YES.**

---

## PART A1 — Region set + country tag

Shared helper logic (identical allowlist) lives in:

| Layer | Path |
|-------|------|
| Ingestion | `services/ingestion/internal/state/state.go` |
| platform-sync | `services/platform-sync/internal/state/state.go` |
| API (country tag) | `services/api/src/common/na-regions.ts` |
| Web (display) | `web/src/lib/na-regions.ts` |

### Allowed codes

| Group | Codes |
|-------|--------|
| US states (50) + DC | `AL`…`WY`, `DC` |
| US territories | `PR`, `GU`, `VI`, `AS`, `MP` |
| Canadian provinces/territories | `ON QC BC AB MB SK NS NB NL PE NT YT NU` |

- **Anything else → `''` (unknown)** — no more “any 2-letter passes” on platform-sync.
- Full names + lowercase still map (`Ontario`→`ON`, `British Columbia`→`BC`, `Texas`→`TX`, …).
- **`CA` = California (US).** Canada is **never** bare `CA` — only province codes.

### Country tagging

```go
CountryFor(code) → US | CA | ""
```

Coverage API adds `country` per row and filters unknown codes from the heatmap. Identity / `argMax` unchanged:

```text
GROUP BY platform, external_call_id
argMax(state|revenue|platform_duplicate, received_at)
```

---

## PART A2 — Coverage display (summary)

- Header: **`N pings across M regions · US + Canada`**
- Subtitle split: **`52 US · 4 CA`** (live example)
- Map / tooltip / detail: see UI report (shared graded teal scale; All · US · Canada filter).

---

## PART A4 — Region breakdown (verify)

At check time (tenant demo):

| Metric | Value |
|--------|-------|
| Distinct known regions in Coverage | **56** (= 52 US incl. PR/GU/VI + **4** CA: `BC ON MB NS`) |
| Coverage `total_pings` | ~5,456 (distinct calls with known region) |
| `CA` country | **US** (California) — no Canada collision |
| `ON` country | **CA** |

CH sample Canadian present: `BC`, `ON`, `MB`, `NS`. No junk / 3-letter / full-name states in the live set. Future non-NA codes blank at ingest/sync via allowlist.

---

## PART B — Ping Feed header = distinct calls

| Field | Meaning |
|-------|---------|
| `GET /pings` → `total` | **Raw row count** (pagination through the event stream; unchanged) |
| `GET /pings` → `distinct_total` | **`uniqExactIf(external_call_id, …)`** — headline |
| UI badge | `{distinct_total} calls` |

`/pings` listing still returns raw rows with DUP badges. Dedup identity + analytics `argMax` unchanged.

Live check example:

| Source | Value |
|--------|--------|
| `pings.distinct_total` | ~5,466 |
| `analytics.total_pings` | ~5,466 (distinct calls) |
| `coverage.total_pings` | ~5,456 (distinct **with known region**; empty-state excluded) |
| `pings.total` (raw) | ~66,747 |

Feed ↔ dashboard agree on distinct calls. Coverage is slightly lower when some calls lack a known region — expected.

---

## Verifies

| Check | Result |
|-------|--------|
| `verify-presentation.ps1` | **20/20 PASS** |
| `smoke-test-all-platforms.ps1` | **4/4 PASS** |
| `verify-api-sync.ps1` | **PASS** |
| `verify-ingestion-slice.ps1` | **PASS** |
| `ws-smoke-test.mjs` | **PASS** |
| Go `NormalizeState` / `CountryFor` tests | **PASS** (ingestion + platform-sync) |

---

## One-line

US+CA allowlist (junk→`''`) with country tags; Coverage labelled North America; Ping Feed headline = distinct calls while raw `/pings` rows stay unchanged.
