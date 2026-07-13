# Phase 1.5 — Close gaps before Ringba (research / recommend only)

**Date:** 2026-07-10  
**Scope:** RESEARCH + RECOMMEND. **No** update-mechanism implementation. **No** `platform_duplicate` aggregate change.  
**Retreaver:** stayed on **Test** except historical Phase 1 Live sync already completed; this phase did not flip Live for a D9 API proof (mechanism not chosen yet).

---

## 1 — D9 gap: updates are never re-fetched

### Problem (confirmed)

Retreaver adapter pages with `created_at_start = sync_cursor`. Revenue often posts **after** `created_at`, so a later sync with `created_at_start > T` never re-fetches call T. Aggregate `argMax` is correct but **inert** without a second row.

Also consistent with research: `call_key` is **not** in the pull API — conversions are designed for **webhook** postbacks.

### Per-platform: fetch by last-modified?

| Platform | Last-modified fetch? | Doc link |
|----------|----------------------|----------|
| **Retreaver** | **YES** — docs sample notes `updated_at_start` / `updated_at_end`; V2 also has `sort_by=updated_at` | https://retreaver.github.io/core-api-docs/ |
| **Ringba** | **NO** — `CallLogs/Date` uses `reportStart` / `reportEnd` (call/report time) | https://github.com/diablomedia/ringba-php-sdk/blob/HEAD/docs/Api/CallLogApi.md |
| **TrackDrive** | **NO** as filter — `created_at_from`/`to`, `ended_at_from`/`to`; `updated_at` is sort/field only | https://trackdrive.com/api/docs/1.0/calls/index.html |
| **CallGrid** | **NO** — `startDate` / `endDate` only in published download flow | https://callgrid.com/api |

Full write-up: `docs/apisync-platform-research.md` (Phase 1.5 sections).

### Options (do not implement until Syed picks)

| Option | What it costs | What it guarantees |
|--------|---------------|--------------------|
| **A. `updated_at` cursor** (Retreaver-capable) | Low extra volume; must live-probe that `updated_at_*` filters behave as documented | Re-pulls calls whose fields changed after create |
| **B. Overlap window** (`created_at_start = cursor − N days`) | Re-fetch ~N days every sync (API cost + Redis DUP rows) | Works on Ringba/TrackDrive/CallGrid too; relies on existing argMax |
| **C. Webhook for conversions** | Dual connection (API sync + webhook); ops complexity | Real-time revenue; matches Retreaver’s `call_key` design |
| **D. Both** (webhook + A or B) | Highest cost / complexity | Best completeness; safety net if one path misses |

Must be **per-adapter configurable** (Ringba/CallGrid/TrackDrive lack last-modified filters → B and/or C).

### Recommendation (for Syed)

**Retreaver: D (Both) — primary A (`updated_at` cursor) + webhook conversions; optional short overlap as safety net.**

- Retreaver is the only platform with documented last-modified filters.
- Webhook is what Retreaver itself designed for conversion postbacks (`call_key`).
- Overlap alone works but is wasteful if `updated_at_*` works.

**Ringba / TrackDrive / CallGrid (when their phases start): B (overlap) + webhook if available.**

### API-level D9 proof

**Not run.** Requires chosen mechanism + a real revenue change between two Live syncs.  
Phase 1 “0→46.5” evidence was a **hand-inserted** pair → **SQL-level only**.

**Retreaver D9: SQL-level YES / API-level PENDING**

---

## 2 — `platform_duplicate` business rule

### Raw counts

```
=== platform_duplicate counts ===
callgrid	0	20
callloom-native	0	8
retreaver	0	2396
ringba	0	894
trackdrive	0	20
```

### Code refs (non-test)

```
services/ingestion/internal/producer/producer.go — Kafka field
services/ingestion/internal/handlers/webhook.go — log field
services/ingestion/internal/handlers/ringba.go — log field
services/consumer/pkg/clickhouse/clickhouse.go — INSERT column
services/consumer/internal/models/ping.go — JSON field
```

No analytics/coverage/web filter on `platform_duplicate` yet.  
`is_duplicate` was removed from aggregate INNER queries in Phase 0.

### Current behaviour

**(a)-like:** platform-declared duplicates are **included** in counts and revenue if present (same as any other row; `argMax` still applies). Today **zero** rows have `platform_duplicate=1`, so dashboards are unaffected.

### Options (unchanged until Syed decides)

| Option | Behaviour |
|--------|-----------|
| **(a)** | Include in counts and revenue (current) |
| **(b)** | Exclude from revenue; include in ping counts |
| **(c)** | Exclude from both; separate “duplicate leads” metric |
| **(d)** | `/pings` display only (badge); tenant setting later |

### Recommendation

**(c)** for pay-per-call: platform-declared duplicate leads are usually **not billable** — counting them overstates volume and money. Surface a separate metric so ops can still see them.

Tradeoff: needs a small metrics/UI addition. Until then, leaving **(a)** is honest (and currently a no-op given zero rows). **Do not change aggregates until Syed picks.**

---

## 3 — Confirmations

### 3.1 D10 — volume sanity

Our clean-sync window (CH `occurred_at > 0`, excl apisync/phase):

```
min=2026-07-09 15:18:05  max=2026-07-10 15:19:02
rows=230  distinct=226
```

Same window from Retreaver API (`created_at_start`/`end`, paginated, credentials never printed):

```
page=1 page_count=100 running_total=100
page=2 page_count=100 running_total=200
page=3 page_count=25 running_total=225
api_window_count=225 window=2026-07-09T15:18:05Z..2026-07-10T15:19:02Z
```

| Source | Count |
|--------|-------|
| Retreaver API (window) | **225** |
| Calyx distinct `occurred_at>0` | **226** |
| Phase 1 immediate post-sync rows | **227** |

**Gap:** 1–2 calls — same order of magnitude; explainable by boundary inclusivity / concurrent webhook smoke / timing. **D10: PASS.**

### 3.2 241 → 227 explained

Prior verification (`docs/retreaver-real-verification-report.md`): `calls(uniq)=241` over a **wide** history (`received_at` from 2026-06-25 …).  
Phase 1 clean re-sync used empty cursor → adapter default **~24h** `created_at_start` → **227** rows in that window.

```
=== 3.2 occurred_at split ===
no_time=27  with_time=232  total=259

=== 3.2 no_time received_at range ===
min=2026-06-25 05:27:03  max=2026-07-10 15:20:06  count=27
```

**no_time leftovers are not stale API pulls:**

- 10× `demo-call-*` fixtures  
- 17× UUID rows, all `event_type=incoming`, revenues 5.5/9.75 — **webhook smoke** (delete was `received_at >= 2026-07-10` for real API cleanup; older webhooks kept)

```
aeaf37a2-…	incoming	9.75	2026-06-29 …
…
81560ccb-…	incoming	9.75	2026-07-10 15:19:11   # smoke during D11
eed10fa6-…	incoming	5.5	2026-07-10 15:20:06
```

**Time display fallback (confirmed):**

- API: `EVENT_AT_EXPR = if(occurred_at > toDateTime(0), occurred_at, received_at)`
- Web: `pingEventAt()` prefers `occurred_at` when valid, else `received_at`

**No delete of leftovers** — they are webhook/demo, not stale API rows.

**14-call gap (241→227):** prior distinct set spanned **multi-day** history; clean sync only re-pulled **~24h**. Calls with `created_at` outside that window were not re-fetched (by design of empty-cursor default). Not data loss of the account — window mismatch.

### 3.3 Fixture provenance

```
=== fixture header ===
// source: real-account (scrubbed)
// doc: https://retreaver.docs.apiary.io/#reference/calls
// Captured shape: V2 nested list item {"call":{...}}. ANI/revenue ids scrubbed.

=== fixture secrets grep ===
clean
```

Scrub check: uuid `75f1e19d-aaaa-bbbb-cccc-ddddeeeeffff` (obviously fake), caller `+15550001111`, afid `aff-scrubbed`. **PASS.**

---

## 4 — Certification table

Updated `docs/platform-certification.md`:

- D9 split: **SQL-level ✅ / API-level ⬜** for Retreaver  
- New **Update mechanism** column (pending Syed)  
- Certified footnote: mapping/conformance/A1/SQL D9 only  

---

## Verification (no code changes to aggregates / adapters)

```
API checks: 20 / 20 passed
PRESENTATION VERIFY PASS

ALL PLATFORMS PASS (4/4)

=== API Sync verify complete ===
PASS: analytics.total_pings increased (3343 -> 3344)

INGESTION SLICE VERIFY PASS

WS SMOKE PASS: 3 events, types: ping_received, compliance_violation
```

No regressions.

## Final lines

**Update mechanism recommended:** Retreaver = Both (`updated_at` cursor + webhook); other platforms = overlap window (± webhook). Per-adapter configurable. **Not implemented.**

**platform_duplicate rule recommended:** **(c)** exclude from revenue and ping counts; separate “duplicate leads” metric. **Not changed.**

**Retreaver D9: SQL-level YES / API-level PENDING**

**Ready to start Phase 2 (Ringba): NO** — wait for Syed on update mechanism (shared design) and preferably `platform_duplicate` rule so Ringba does not re-litigate mid-phase.
