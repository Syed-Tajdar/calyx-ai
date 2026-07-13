# Final Live Gate Report

**Date:** 2026-07-09  
**Integration:** real Retreaver stays on **Test** (not flipped in this pass).

---

## ITEM 1 — Transport-error redaction (positive control)

### 1a — Go unit test (unresolvable host)

```
cd services/platform-sync
go test ./internal/adapter/... ./internal/redact/... -count=1 -v -run "TestHTTPClientDoRedacts|TestSanitize|TestClassify|TestRedact"
```

```
=== RUN   TestHTTPClientDoRedactsTransportErrorURL
--- PASS: TestHTTPClientDoRedactsTransportErrorURL (3.00s)
PASS ok .../internal/adapter 3.630s
=== RUN   TestSanitizeErrRedactsAPIKey
--- PASS: TestSanitizeErrRedactsAPIKey (0.00s)
=== RUN   TestRedactURL
--- PASS: TestRedactURL (0.00s)
=== RUN   TestClassifySyncErrorNetwork
--- PASS: TestClassifySyncErrorNetwork (0.00s)
=== RUN   TestClassifySyncErrorAuth
--- PASS: TestClassifySyncErrorAuth (0.00s)
PASS ok .../internal/redact 0.282s
```

Test file: `services/platform-sync/internal/adapter/httpclient_test.go`  
Asserts: no `BOGUS_KEY_ABC123`, contains `[REDACTED]`, host preserved, `ClassifySyncError` → `network error (transient, will retry):`.

### 1b — E2E bogus key + unreachable host

- Added `RETREAVER_BASE_URL` env override in `retreaver_adapter.go` (defaults to production URL).
- Set override on platform-sync container: `https://this-host-does-not-exist-xyz.invalid/api/v2/calls.json`
- Created `bogus-transport-gate` with `api_key=BOGUS_KEY_ABC123`, `company_id=999999`, flipped to Live, triggered sync → DNS transport error.

**Four assertions:**

| Check | Result |
|-------|--------|
| `docker logs deploy-platform-sync-1 \| grep -c BOGUS_KEY_ABC123` | **0** |
| `docker logs deploy-platform-sync-1 \| grep -c REDACTED` | **1** ✅ positive control |
| `docker logs deploy-api-1 \| grep -c BOGUS_KEY_ABC123` | **0** |

**Log excerpt (sanitized URL in transport error):**
```
{"msg":"request failed","error":"Get \"https://this-host-does-not-exist-xyz.invalid/api/v2/calls.json?api_key=[REDACTED]&company_id=[REDACTED]&...\": dial tcp: lookup this-host-does-not-exist-xyz.invalid ... no such host"}
```

**`last_sync_error`:**
```
bogus-transport-gate | error | network error (transient, will retry): Get "https://this-host-does-not-exist-xyz.invalid/api/v2/calls.json?api_key=[REDACTED]&company_id=[REDACTED]&...": dial tcp: lookup this-host-does-not-exist-xyz.invalid on 127.0.0.11:53: no such host
```

**Cleanup:** `RETREAVER_BASE_URL` removed from `deploy/docker-compose.yml`; bogus integration deleted (`bogus_remaining=0`); platform-sync recreated without override.

**Verdict: PASS** — redaction fired on the actual leak path (`REDACTED >= 1`, key count 0).

---

## ITEM 2 — Dedupe identity `(platform, external_call_id)`

### Final SQL

**Analytics** (`analytics.service.ts`):
```sql
SELECT count() AS total_pings, sum(revenue), sum(payout)
FROM (
  SELECT platform, external_call_id,
         argMax(revenue, received_at) AS revenue,
         argMax(payout, received_at) AS payout
  FROM pings
  WHERE tenant_id = {tenant_id:UUID}
    AND is_duplicate = 0
    AND external_call_id != ''
  GROUP BY platform, external_call_id
)
```

**Coverage** (`coverage.service.ts`):
```sql
SELECT state, count() AS ping_count, sum(revenue) AS revenue
FROM (
  SELECT platform, external_call_id,
         argMax(state, received_at) AS state,
         argMax(revenue, received_at) AS revenue
  FROM pings
  WHERE tenant_id = {tenant_id:UUID}
    AND is_duplicate = 0
    AND external_call_id != ''
  GROUP BY platform, external_call_id
)
WHERE state != ''
GROUP BY state
```

`state != ''` filter moved to **outer** query (after `argMax(state, received_at)`).

### Verification (platform-sync paused)

**Case 1 — same `external_call_id`, different states (TX then CA):**
```
INSERT dedupe-key-001: TX/revenue=10, then CA/revenue=20 (later received_at)
ClickHouse inner query → CA | 1 | 20
```
Call counted **once under CA** (latest state), revenue **20** — not split across TX and CA.

**Case 2 — same `external_call_id`, different platforms:**
```
INSERT shared-id-002 on retreaver + ringba
GROUP BY platform, external_call_id → count = 2
```
Two platforms = **two distinct calls** (not collapsed).

**Case 3 — raw / deduped / API triple (captured together):**

| Metric | Value |
|--------|-------|
| raw rows | **2856** |
| deduped `(platform, external_call_id)` | **2845** |
| API `total_pings` | **2845** |

`raw > deduped` ✅ | `API == deduped` ✅

Test rows deleted; platform-sync restarted.

**Verdict: PASS**

---

## Post-change verify scripts (all green)

| Script | Result |
|--------|--------|
| `verify-presentation.ps1` | **20/20 PASS** |
| `smoke-test-all-platforms.ps1` | **4/4 PASS** |
| `verify-ingestion-slice.ps1` | **PASS** |
| `verify-api-sync.ps1` | **PASS** |
| `ws-smoke-test.mjs` | **PASS** |

---

## Files changed

| File | Change |
|------|--------|
| `services/platform-sync/internal/adapter/httpclient_test.go` | Transport-error redaction test |
| `services/platform-sync/internal/adapter/retreaver_adapter.go` | `RETREAVER_BASE_URL` override |
| `services/api/src/analytics/analytics.service.ts` | `GROUP BY platform, external_call_id` |
| `services/api/src/coverage/coverage.service.ts` | `argMax(state)` + platform identity |
| `.env.example` | Documented optional `RETREAVER_BASE_URL` |

---

**Safe to flip Live: YES** — transport-error redaction proven with positive control (`REDACTED >= 1`), dedupe identity aligned to `(platform, external_call_id)`, all verify scripts green. Flip manually per go-live checklist; paste fresh credentials in Settings UI only.
