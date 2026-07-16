# Phase 2.0 — Deploy verification report (close D11)

**Date:** 2026-07-15  
**Status:** **PASS**

## 1.1 Rebuild

- Dockerfiles for ingestion + platform-sync use **repo-root context** + `packages/pingdedup`.
- Root [`.dockerignore`](../.dockerignore) keeps API builds lean.
- Images rebuilt and recreated: ingestion, platform-sync, api (deep-view fix).

## 1.2 Deployed binary proof

| Service | Proof |
|---------|--------|
| platform-sync | Startup `dedup_mode=content_hash`; strings markers present |
| ingestion | Startup `dedup_mode=content_hash`; strings markers present |

## 1.3 ClickHouse row-level content-hash (Retreaver Test + `MOCK_DEDUP_PROOF`)

| Step | rows | calls | Notes |
|------|------|-------|-------|
| Sync A | 5 | 5 | Initial |
| Sync B identical | **5** | **5** | **ZERO growth** |
| Revenue path | ≤12 | 5 | `argMax` on `proof-retreaver-0` = **99.99** |

Log: `calls=8 published=0 unchanged=8 page1_refresh=true`

Ingestion slice on running stack: identical webhook resend → **0 new rows**; changed revenue → new row `is_duplicate=0`.

## 1.4 page=1 refresh

- Unit `TestFetchAllPagesPage1Refresh` PASS  
- Running log `page1_refresh=true` PASS  

## 1.5 Five verify scripts (rebuilt stack)

| Script | Result |
|--------|--------|
| verify-presentation | **20/20 PASS** (deep-view fixed: formatDateTime inside inner aggregate) |
| smoke-test-all-platforms | **4/4 PASS** |
| verify-api-sync | **PASS** |
| verify-ingestion-slice | **PASS** (new content-hash contract) |
| ws-smoke-test | **PASS** |

## 1.6 R2 Retreaver

- Test sync + smoke Retreaver E2E PASS  
- `api_key=` leak count in platform-sync logs: **0**  
- CERTIFIED remains YES with running-system evidence  

## GATE (PART 1)

```
Content-hash deployed + row-level proven: YES
Five verify scripts on rebuilt stack: PASS
Retreaver R2 on running system: CERTIFIED YES
```
