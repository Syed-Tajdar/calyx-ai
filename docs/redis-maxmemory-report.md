# Redis maxmemory + volatile-ttl

**Date:** 2026-07-21  
**Scope:** Bound Redis memory in `deploy/docker-compose.yml` without breaking content-hash dedup.

## Change

`redis` service command now includes:

```yaml
command:
  - redis-server
  - --appendonly
  - "yes"
  - --appendfsync
  - everysec
  - --maxmemory
  - 1gb
  - --maxmemory-policy
  - volatile-ttl
```

`redis_data` volume unchanged.

### Why `volatile-ttl` (not the alternatives)

| Policy | Rejected because |
|--------|------------------|
| `allkeys-lru` | Can evict live dedup hashes mid-TTL → needless republishes |
| `noeviction` + bound | Writes fail at the ceiling → duplicate balloon |
| **`volatile-ttl`** | All our dedup keys have TTL; evicts only keys with expiry, nearest-expiry first. Early-evicted dedup → at most one republish; no data loss; `argMax` stays correct |

## Proof after `docker compose up -d --force-recreate redis`

| Check | Result |
|-------|--------|
| `CONFIG GET maxmemory` | `1073741824` (1gb) |
| `CONFIG GET maxmemory-policy` | `volatile-ttl` |
| `CONFIG GET appendonly` | `yes` |
| `DBSIZE` before → after | 6376 → 6375 |
| `*:dedup:*` key count | **2230 → 2230** |
| Sample key `EXISTS` | **1** (AOF + volume survived recreate) |

## Regression — Retreaver (Test) unchanged re-sync

Integration: `269ba4dd-…` (“Test 1”), `sync_mode=mock`.  
For a stable content-hash proof under Test, `MOCK_DEDUP_PROOF=1` was set only for this window (stable `proof-retreaver-*` payloads), then restored to `0`.

| Moment | `proof-retreaver-*` rows | All `platform=retreaver` rows | Sync log |
|--------|--------------------------|-------------------------------|----------|
| Before sync 1 | 12 | 67884 | — |
| After sync 1 | **17** | **67889** | `calls=5 published=5 unchanged=0` |
| After sync 2 (unchanged) | **17** | **67889** | `calls=8 published=0 unchanged=8 page1_refresh=true` |

**Row count stayed flat** on the second sync — content-hash still suppressing under bounded Redis + `volatile-ttl`.

## GATE

```
Redis maxmemory + volatile-ttl + AOF: PASS
Dedup flat on unchanged re-sync: PASS
```
