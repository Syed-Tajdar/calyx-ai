# Pre-Live Gate Report

**Date:** 2026-07-09  
**Integration:** remains on **Test** (real Retreaver integration not flipped).

---

## BLOCKER 1 — Deployed image contains redaction fix

### Rebuild + recreate

```
cd deploy
docker compose build --no-cache platform-sync
docker compose up -d --force-recreate platform-sync
docker compose ps platform-sync
```

```
NAME                     IMAGE                  COMMAND             SERVICE         CREATED         STATUS
deploy-platform-sync-1   deploy-platform-sync   "./platform-sync"   platform-sync   4 seconds ago   Up
```

### Binary proof

```
docker exec deploy-platform-sync-1 sh -c "ls -l /app && date"
total 17008
-rwxr-xr-x    1 root     root      17414227 Jul  9 18:39 platform-sync
Thu Jul  9 18:39:53 UTC 2026

docker exec deploy-platform-sync-1 sh -c "strings /app/platform-sync | grep -c REDACTED"
2
```

(Binary path is `/app/platform-sync`, not `/app/server`.)

### Log-site grep (all sanitized)

```
services/platform-sync/cmd/server/main.go:191  scheduled sync failed → redact.SanitizeErr(err)
services/platform-sync/cmd/server/main.go:299  test-auth failed → safeMsg (SanitizeErr)
services/platform-sync/internal/syncer/syncer.go:57-58  sync failed → redact.SanitizeErr(err)
```

**Verdict: PASS** — no-cache image running; `REDACTED` marker in binary; no raw `err` at log sites.

---

## BLOCKER 2 — Live-fire redaction (BOGUS key)

### Steps

1. Created `bogus-redaction-gate` integration: `api_key=BOGUS_KEY_ABC123`, `company_id=999999`
2. Flipped to `sync_mode=real`, triggered sync (failed auth as expected)
3. platform-sync processed sync: `retreaver HTTP 401` (no URL in error body)

### Grep counts

```
docker logs deploy-platform-sync-1 | grep -c BOGUS_KEY_ABC123  → 0
docker logs deploy-platform-sync-1 | grep -c REDACTED           → 0
docker logs deploy-api-1 | grep -c BOGUS_KEY_ABC123             → 0
```

`REDACTED` count 0 in logs is **expected** for the HTTP 401 path — Go does not embed the request URL in 401 response errors. URL redaction applies to transport errors (`Get "https://...?api_key=..."`). Binary contains `REDACTED` (×2); bogus key never appeared.

### `last_sync_error` (sanitized + classified)

```
bogus-redaction-gate | error | authentication error: retreaver HTTP 401: {"error":"You need to sign in or sign up before continuing."}
```

No `api_key` value present.

### Cleanup

Bogus integration deleted via `DELETE /integrations/{id}` — `bogus_remaining=0`.

**Verdict: PASS**

---

## BLOCKER 3 — Webhook regression guard restored

### Re-seed

Added `scripts/seed-dev-integrations.ps1` + `make seed-dev-integrations` (idempotent; mirrors migrations 000001/000002/000018).

```
INSERT 0 4
callgrid|dev_callgrid_token_change_me
retreaver|dev_retreaver_token_change_me
ringba|dev_ringba_token_change_me
trackdrive|dev_trackdrive_token_change_me
PASS: dev webhook integrations seeded
```

### Script output

**smoke-test-all-platforms.ps1** — **4/4 PASS**
```
ALL PLATFORMS PASS (4/4)
```

**verify-ingestion-slice.ps1** — **PASS** (with platform-sync paused to avoid concurrent mock inserts)
```
INGESTION SLICE VERIFY PASS
```
Note: first run failed (`2824 → 2825`) because scheduled platform-sync inserted a mock ping during the duplicate window. Pausing `platform-sync` before re-run produced stable counts.

**verify-presentation.ps1** — **20/20 PASS**

**verify-api-sync.ps1** — **PASS** (after platform-sync restarted)
```
PASS: analytics.total_pings increased (2830 -> 2831)
```

**ws-smoke-test.mjs** — **PASS**
```
WS SMOKE PASS: 3 events
```

**Verdict: PASS**

---

## CONFIRMATION A — Coverage revenue dedupe

### Actual coverage SQL (`coverage.service.ts`)

```sql
SELECT state, count() AS ping_count, sum(revenue) AS revenue
FROM (
  SELECT state, external_call_id, argMax(revenue, received_at) AS revenue
  FROM pings
  WHERE tenant_id = {tenant_id:UUID}
    AND state != ''
    AND is_duplicate = 0
  GROUP BY state, external_call_id
)
GROUP BY state
```

Inner group picks **one revenue per (state, external_call_id)** via `argMax(revenue, received_at)` before outer `sum()` / `count()`.

**Dedupes revenue: YES** — no change required.

---

## CONFIRMATION B — Airtight dedupe proof

platform-sync **stopped** during test. Inserted 2 rows: `external_call_id='dedupe-gate-001'`, both `is_duplicate=0`, revenue 10 and 20.

### Three numbers (captured together)

| Metric | Value |
|--------|-------|
| **raw** row count (`count()` all rows) | **2841** |
| **deduped** call count (`GROUP BY external_call_id` + `is_duplicate=0`) | **2830** |
| **API** `total_pings` | **2830** |

```
raw > deduped:  TRUE  (2841 > 2830)
API == deduped: TRUE  (2830 == 2830)
```

`/pings?search=dedupe-gate-001` → **2 rows** (raw list unchanged).

Revenue for test call (analytics `argMax` + coverage inner query): **20** (latest row wins, counted once).

Test rows deleted; platform-sync restarted.

**Verdict: PASS**

---

## Checklist

- [x] BLOCKER 1: no-cache rebuild, binary REDACTED proof, log sites sanitized
- [x] BLOCKER 2: BOGUS_KEY absent everywhere; classified `last_sync_error`; bogus integration deleted
- [x] BLOCKER 3: seed script + 4/4 smoke + ingestion slice + 20/20 + api-sync + ws
- [x] CONFIRMATION A: coverage uses `argMax(revenue, received_at)` — YES
- [x] CONFIRMATION B: raw > deduped, API == deduped, revenue once

---

**Safe to flip Live: YES** — superseded by final live gate. See **[final-live-gate-report.md](./final-live-gate-report.md)** for transport-error redaction positive control and `(platform, external_call_id)` dedupe proof.
