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
