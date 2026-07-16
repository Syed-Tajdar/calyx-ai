# Investigate-only (PART 3) — findings + recommendations. No implementation without Syed approval.

**Date:** 2026-07-15

## 3.1 Redis durability

Commands (dev compose Redis):

Observed on `deploy-redis-1` (2026-07-15):

```
maxmemory-policy → noeviction
appendonly       → yes
save             → 3600 1 300 100 60 10000
```

Compose still declares Redis **without** a named volume — AOF on by image default still loses durability across `docker compose down -v` / recreate without bind.

**Impact:** Content-hash keys live only in Redis (TTL 168h). Flush/restart/eviction → next overlap sync sees "key absent" → republishes every call in the window → row explosion under Ringba overlap.

**Recommendations (do not implement yet):**
1. Enable AOF + named volume for Redis (ops).
2. Prefer `maxmemory-policy volatile-ttl` (never `allkeys-*` for dedup).
3. **Durable path:** on Redis miss, compare mapped hash to last CH row for `(tenant, platform, external_call_id)` — Redis becomes a cache; CH is source of truth.

## 3.2 Canonical hash vs `DESCRIBE pings`

| Column | In hash? | Mutable after first sight? | Notes |
|--------|----------|----------------------------|-------|
| tenant_id | n/a (key scope) | no | |
| platform | n/a (key) | no | |
| external_call_id | YES | no (identity) | |
| ani_hash | **NO** | yes (re-hash if ANI encrypt changes rare) | update-blind |
| campaign_id | YES | yes | |
| publisher_id | YES | yes | |
| target_id | YES | yes | |
| state | YES | yes | |
| event_type | YES | yes | |
| revenue | YES | yes | |
| payout | YES | yes | |
| is_duplicate | n/a | ingest flag | not content |
| received_at | n/a | ingest clock | not content |
| call_key | **NO** | yes (Retreaver) | update-blind if call_key arrives later |
| occurred_at | YES | usually stable | |
| platform_duplicate | YES | yes | |

**Verdict: canonical hash covers all mutable stored fields: NO**

Unhashed mutable fields: **`ani_hash`**, **`call_key`**. Recommend add to hash (Syed approval) or document blindness in cert.

## 3.3 Cursor safety on partial failure

Code path ([`syncer.go`](../services/platform-sync/internal/syncer/syncer.go)):
- Start: status `syncing`, **cursor unchanged**
- Fetch/publish error: `markError` with **old** cursor
- Success only: persist `nextCursor`

Unit: `TestPublishSyncCallsStopsMidBatchOnError` — publish stops mid-batch; cursor advance is syncer's responsibility (not advanced on error).

**Verdict: Cursor safe on partial failure: YES** (by design). Note: Evaluate may SET Redis hash before Publish succeeds — next sync skips republish of successfully hashed-but-unpushed rows (Kafka gap risk). Separate durability item.

## 3.4 Credential encryption key rotation

Single env `CREDENTIAL_ENCRYPTION_KEY` (API encrypt / platform-sync decrypt). **No** key version column, **no** dual-key re-encrypt path.

Buyer answer today: rotate key ⇒ all tenants must re-enter API credentials (or offline decrypt-with-old / encrypt-with-new script ops).

Recommend: `credential_key_version` + dual-decrypt + background re-encrypt.

## 3.5 Pre-existing ringba rows

Observed on this stack (2026-07-15): `platform='ringba'` count = **1** (not ~912). Still wipe demos before Live if accumulation returns:

```sql
ALTER TABLE pings DELETE WHERE platform = 'ringba' AND external_call_id LIKE 'apisync-%';
-- or full platform wipe for a clean Live window (ops approval)
```

## Recommendations priority

1. CH fallback when Redis miss (3.1.3) — correctness
2. Add `ani_hash` + `call_key` to hash (3.2) — correctness  
3. Redis AOF/volume (3.1.1) — ops
4. Key versioning (3.4) — enterprise process
