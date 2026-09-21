# Enterprise roadmap — deferred gaps

**Date:** 2026-07-21  
**Scope:** Documentation only. No implementation in this slice.

Two gaps are explicitly **deferred** with build-triggers. They are not ignored — they are gated until a concrete customer/security event makes the cost worth it.

---

## A — Quota / export-as-primary architecture (Ringba)

### State today

- Live sync uses paginated **Get Call Logs**: `POST /v2/{accountId}/calllogs`
- Page `size` = **1000** (official max)
- Dual-window throttle: **5/sec ∧ 80/min** scoped per Ringba `account_id` (matches the account-wide Reporting / Get Logs bucket)
- Cert window proved fine: ~31 Get Logs req/hour at 120s tick, **0** true 429s, rolling-60s peak ≪ 80  
  See [`docs/phase2-ringba-live-certified.md`](phase2-ringba-live-certified.md)

### Risk at scale

The **5/sec · 80/min** bucket is **account-wide** — shared with the customer’s own Reporting UI and other API tokens on the same account. A high-volume Calyx backfill can starve (or be starved by) human Reporting usage.

CSV **export** is a separate path (`…/calllogs/export/…`) with stricter **per-user** caps (5/min · 20/hr · 200/day). Using export-as-primary for large backfills could reduce pressure on the shared Get Logs bucket — **if** export does not also consume that bucket.

### Open question (blocking)

Ringba support was emailed (Q1–Q3 in [`docs/ringba-quota-architecture-research.md`](ringba-quota-architecture-research.md)):

> Does an export request (and its progress GET) also count against the account-wide 5/sec·80/min Reporting/Get Logs bucket, or **ONLY** against the per-user export bucket?

**Not yet confirmed.** Until answered, we will not switch architecture to export-as-primary.

### Build trigger

Implement export-as-primary (or a hybrid) **before**:

- onboarding a **high-volume** Ringba customer, or
- **GA** of Ringba API sync as a default product path  

— and only after the support answer lands.

---

## B — Credential encryption key rotation

### State today

- Single env/Secrets Manager value: `CREDENTIAL_ENCRYPTION_KEY`
- Platform API credentials stored as encrypted blobs in Postgres
- No key version column; no dual-key decrypt window; rotation today = re-encrypt everything under a cutover (operationally risky)

### Fix shape (when built)

1. Add `credential_key_version` (or equivalent) on credential rows  
2. Support a **dual-key decrypt window** (current + previous) during rotation  
3. Background re-encrypt to the new version; then retire the old key  

### Build trigger

- First **enterprise security review**, or  
- **Suspected key exposure** / forced rotation event  

---

## Already-tracked items (not new work)

| Item | Status | Notes |
|------|--------|-------|
| Ringba `…/calllogs/details` **404** | Tracked | A1 proven via Get Call Log exact-record match; details path unavailable on current host ([`phase2-ringba-live-certified.md`](phase2-ringba-live-certified.md)) |
| Redis-as-cache with **ClickHouse fallback on key miss** | Long-term | Durable fix beyond TTL + `volatile-ttl`; content-hash keys remain a cache, not the source of truth. `argMax` in CH already tolerates republish. Build when Redis eviction/republish cost or multi-region cache coherency demands it |
| Redis `maxmemory 1gb` + `volatile-ttl` | **Done** | [`docs/redis-maxmemory-report.md`](redis-maxmemory-report.md) |

---

## Summary

| Gap | Action now | Trigger |
|-----|------------|---------|
| A Quota / export-as-primary | Stay on paginated Get Logs + 80/min | High-volume customer or GA **and** support Q3 answered |
| B Credential key rotation | Single key remains | Enterprise security review or key exposure |
| details 404 / Redis→CH fallback | Tracked only | As product/ops demand |
| C Dashboard vs Coverage totals | Document period mismatch; no code | Before any customer-facing demo |
| D Anti-drift behavioural assertion | Enumeration-based test remains | Next new analytics endpoint |
| E Top Scores label vs ordering | Confirm semantics | Before customer-facing demo |
| F AVG CALL SCORE sample disclosure | Disclose N or suppress thin trend | Before customer-facing demo |
| G Recording playback (Ringba B2) | `hasRecording` yes; `recordingUrl` obtainable — **PII/policy** is the blocker | Customer requirement OR GA — whichever first |
| J Ringba converted-state | **Shipped 2026-09-18** — `hasConverted` mapped + campaign-allowlisted convert API; no write UI | Write UI / payments-override later |
| H `verify-api-sync.ps1` post-C3 | Script asserts metrics growth from `apisync-%` | Before next pre-push relying on that script |
| I WS live smoke 0-events | **Fixed (Delta 2)** | KafkaJS lacked Snappy; D1 rpk produce crashed live consumer — see `part1-delta2-report.md` |

---

## C — Dashboard vs Coverage totals

### State today

- Dashboard Total Pings uses `/analytics/metrics` with the global date filter (often **30d**).
- Coverage uses `/coverage/heatmap` with **no period filter** (all-time distinct calls with `state != ''`).
- Part 1 also measured an **11-row** gap on the same all-time window (`state != ''` filter only).
- The originally observed ~756 gap mixed a 30-day dashboard with all-time Coverage — **not yet explained as a single bug**.

### Fix shape

- Add a period filter to Coverage **or** state the period explicitly in the Coverage header (e.g. “All time”).
- Optionally align Coverage with metrics’ distinct-call definition (include empty-state calls or disclose exclusion).

### Build trigger

**Before any customer-facing demo.**

---

## D — Anti-drift coverage is enumeration-based

### State today

- `demo-rows.spec.ts` / `apiSyncExclusionSql` anti-drift asserts a fixed list of five service files call the helper.
- Drift that created C3 happened because a path was **not on the list**. A sixth read path would still pass silently.

### Fix shape

- Behavioural assertion: insert (or fixture) an `apisync-%` row, hit every aggregate endpoint, assert exclusion.
- Optionally scan `services/api/src/**/*.service.ts` for `FROM pings` / `call_scores` without the helper.

### Build trigger

**Next new analytics / scoring / coverage / compliance read endpoint.**

---

## E — Top Scores label vs ordering

### State today

- UI label: **TOP SCORES**.
- `GET /scoring/scores` orders by `created_at DESC` (recency), not `overall_score DESC`.
- Dashboard AVG CALL SCORE averages those N rows — so a “top scores” sample can under/overstate quality.

### Fix shape

- Confirm intended semantics (recent vs highest).
- If highest: `ORDER BY overall_score DESC`.
- Consider a tenant-wide `avg(overall_score)` endpoint for the KPI.

### Build trigger

**Before any customer-facing demo.**

---

## F — AVG CALL SCORE sample disclosure

### State today

- KPI can show e.g. 81.5 from **5** scored rows out of thousands, with a sparkline on those 5 points.

### Fix shape

- Show sample coverage inline (`5 of N scored`), or suppress the trend when N is below a threshold.

### Build trigger

**Before any customer-facing demo.**

---

## G — Recording playback (Ringba parity Bucket B2)

### State today

- Calyx captures **`hasRecording`** (boolean via API `hasRecording`).
- The live Get Call Log API **accepts a `recordingUrl` value column** — the URL is obtainable and is **not** the blocker. (Earlier deferral text wrongly reasoned from the report Column Dictionary UI play-control into “no URL column”; P-B1.1 live evidence corrected that.)
- **Blocker:** recordings contain **PII**. Retention policy, access control, encryption at rest, presigned-URL TTL, and player UX must be decided before implementation.

### Fix shape

- Map `recordingUrl` from Get Call Log.
- Decide link-out vs store; if store: S3 SSE-KMS + short-lived **presigned** access with TTL.
- Retention aligned to compliance policy; wire player only after access-control policy is approved.

### Build trigger

**Customer requirement OR GA — whichever comes first.**

---

## J — Ringba converted-state (write-back gate)

### State after product slice (2026-09-18)

- **Inbound convert + read-back `[PROVEN]`** on a clean sandbox call (Phase G). Report: [`ringba-writeback-phase-g-report.md`](ringba-writeback-phase-g-report.md).
- Adapter **requests** `hasConverted` and maps it (`absent`/`false` → not converted; present `true` → converted) through Kafka → ClickHouse `pings.has_converted` → deep-view **Converted**. Report: [`ringba-writeback-product-slice-report.md`](ringba-writeback-product-slice-report.md).
- Convert writes: inbound GET webhook, fail-closed `RINGBA_WRITEBACK_ALLOWED_CAMPAIGN_IDS`, Live credentials (paused ticker allowed). No Settings write button.

### Remaining

Write UI; `payments` / `payments/override` / void product paths; Retreaver write-back.

### Build trigger (UI)

Customer asks to convert from Calyx UI **and** allowlist is confirmed for that account.

---

## H — `verify-api-sync.ps1` asserts metrics inflation from mock sync

### State today

- Script creates a mock `api_sync` integration, triggers sync, asserts `/analytics/metrics` `total_pings` increases.
- After C3, `apisync-%` rows are **excluded** from metrics by default — assertion fails even when sync works.
- Script does **not** soft-delete the integration it creates (leftover `api-sync-verify-*` rows).

### Fix shape

- Assert growth with `?show_demo=true` **or** assert `apisync-%` distinct count via `/pings?show_demo=true`.
- Soft-delete (or deactivate) the created integration in teardown.

### Build trigger

**Before next pre-push / CI gate that requires `verify-api-sync.ps1`.**

---

## I — WebSocket live smoke receives 0 events

### State today

- **Fixed in Part 1 Delta 2** (`docs/part1-delta2-report.md`).
- Root cause: KafkaJS core has no Snappy codec; a Snappy-compressed message on `callloom.pings.v1` (from `rpk topic produce` default during D1 evidence) crashed consumer group `calyx-api-live-feed` (`MEMBERS=0`).
- Fix: register `kafkajs-snappy` via `createKafkaClient()` for all API Kafka clients.

### Build trigger

N/A — closed.
