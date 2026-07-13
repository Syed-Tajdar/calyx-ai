# Dedup Path Investigation — API-sync vs Webhook

**Date:** 2026-07-09  
**Scope:** Report only (no code changes). Reconciles Retreaver `is_duplicate=1` rows with ingestion-slice findings.

---

## One-line verdict

| Path | Same Redis dedup as webhook? | Duplicate semantics |
|------|------------------------------|---------------------|
| **Webhook** (ingestion HTTP) | — | **Stored-and-flagged** (`is_duplicate=1` in Kafka → ClickHouse). HTTP **200**, not 409. |
| **API-sync** (platform-sync) | **YES** (same Redis instance, same key format, same SET NX logic) | **Stored-and-flagged** (identical). Does **not** call ingestion HTTP; publishes to Kafka directly after its own Redis check. |

**API-sync uses the same dedup as webhook: YES** (shared Redis namespace; bypasses ingestion *service*, not Redis dedup).

---

## 1. Publish paths (file:line)

### Webhook path

```
platform HTTP POST
  → services/ingestion/internal/handlers/webhook.go:125-141
       dedup.IsDuplicate (Redis SET NX)
       if isDup → ping.IsDuplicate = true
       producer.Publish (always, including duplicates)
  → services/ingestion/internal/producer/producer.go:47-80
       Kafka topic callloom.pings.v1, JSON includes is_duplicate
  → services/consumer/internal/writer/writer.go:29-46
       passes IsDuplicate → ClickHouse UInt8
```

Ringba legacy handler mirrors webhook: `services/ingestion/internal/handlers/ringba.go:126-141`.

Redis dedup implementation:

```22:29:services/ingestion/internal/dedup/dedup.go
func (c *Checker) IsDuplicate(ctx context.Context, tenantID, platform, externalCallID string) (bool, error) {
	key := fmt.Sprintf("%s:dedup:%s:%s", tenantID, platform, externalCallID)
	set, err := c.client.SetNX(ctx, key, "1", c.ttl).Result()
	// ...
	return !set, nil
}
```

TTL: `DEDUP_TTL_HOURS` env (default **24h**), wired in `services/ingestion/cmd/server/main.go:50`.

**No 409 anywhere in ingestion handlers** — grep for `409` / `StatusConflict` under `services/` returns zero matches. Duplicates are never dropped before Kafka.

### API-sync path

```
platform-sync poll / manual sync
  → services/platform-sync/internal/syncer/syncer.go:199-209
       dedup.IsDuplicate (Redis SET NX, same key format)
       producer.Publish(..., isDup, ...)  — always publishes
  → services/platform-sync/internal/producer/producer.go:25-62
       Kafka topic callloom.pings.v1, JSON includes is_duplicate
  → services/consumer/... (same consumer as webhook)
```

Redis dedup (identical logic):

```20:26:services/platform-sync/internal/dedup/dedup.go
func (c *Checker) IsDuplicate(ctx context.Context, tenantID, platform, externalCallID string) (bool, error) {
	key := fmt.Sprintf("%s:dedup:%s:%s", tenantID, platform, externalCallID)
	set, err := c.client.SetNX(ctx, key, "1", c.ttl).Result()
	// ...
	return !set, nil
}
```

Wired in `services/platform-sync/cmd/server/main.go:129` with **24h** TTL.

Both services use `REDIS_URL: redis://redis:6379` and `KAFKA_TOPIC_PINGS: callloom.pings.v1` in `deploy/docker-compose.yml` (ingestion ~L135-137, platform-sync ~L361-363).

**API-sync does not call the ingestion HTTP service.** It has a parallel copy of the dedup checker and its own Kafka producer. Because the Redis key is `{tenant_id}:dedup:{platform}:{external_call_id}`, a call ingested via webhook and re-seen via API-sync within TTL is flagged duplicate (and vice versa).

---

## 2. Where `is_duplicate` comes from

### Column

Added in the **initial** ClickHouse migration (not a later add):

```13:13:db/clickhouse/migrations/000001_pings_table.up.sql
    is_duplicate     UInt8 DEFAULT 0,
```

**Live schema check (2026-07-09):**

```powershell
docker exec deploy-clickhouse-1 clickhouse-client --query "DESCRIBE pings" | findstr /i duplicate
# is_duplicate	UInt8	DEFAULT	0
```

### Who sets it to 1

| Stage | Role |
|-------|------|
| **Ingestion / platform-sync** | Redis SET NX returns `false` (key already exists) → `isDup=true` → set on Kafka JSON `is_duplicate: true` |
| **Consumer** | **Pass-through only** — reads `is_duplicate` from Kafka JSON, writes UInt8 to ClickHouse (`writer.go:30-46`, `clickhouse.go:64,84`). No second dedup check in consumer. |

Platform connectors may also set `IsDuplicate` from webhook payload fields (`is_duplicate`, `duplicate`) before the Redis check, but the Retreaver API-sync bad-mapping case was driven by **Redis** on the shared empty key (see §4).

---

## 3. Reconcile with “CASE B / dead code / 409 drop”

**That conclusion does not match this repo’s code or docs.**

| Source | What it actually says |
|--------|----------------------|
| `docs/ingestion-slice-report.md` | Pre-build verdict is **CASE A**: column exists, duplicates **are stored** with flag `1`; FIX 2 adds `WHERE is_duplicate = 0` to aggregates. Verify script expects `/pings` to show `is_duplicate=1` on resend. |
| Git history (`dc019ab` Phase 1 → `f8f215e` → today) | Webhook/ringba handlers always: `if isDup { ping.IsDuplicate = true }` then `producer.Publish` — never 409, never skip Kafka. |
| Repo grep | No `CASE B`, no `dead code`, no `409` in `docs/` or ingestion handlers. |

The **409 / dropped-before-Kafka / 60s TTL** narrative is **not evidenced** in current or historical handler code. Actual TTL is **24 hours** (`DEDUP_TTL_HOURS`, platform-sync hardcoded `24*time.Hour`).

Commit **`8532dc9`** (2026-07-03, *feat: Phase 0 real ingestion slice*) added aggregate filters **because** duplicates are stored-and-flagged, not dropped:

- `services/api/src/analytics/analytics.service.ts:32` — `nonDuplicateClause = 'AND is_duplicate = 0'`
- `services/api/src/coverage/coverage.service.ts:29` — `AND is_duplicate = 0`

The `WHERE is_duplicate = 0` filter is **not dead code**; it is required for correct totals.

---

## 4. Do the two paths differ in duplicate semantics?

**No — both are stored-and-flagged.** Neither path drops duplicates before Kafka.

### Retreaver bad-mapping window (empty `external_call_id`)

Before the mapper fix, platform-sync published calls with `external_call_id=''`. All such calls shared one Redis key:

```
{tenant_id}:dedup:retreaver:
```

**Evidence — ClickHouse (2026-07-09):**

```sql
SELECT external_call_id = '' AS empty_id, is_duplicate, count() AS cnt
FROM pings WHERE platform='retreaver'
GROUP BY empty_id, is_duplicate ORDER BY empty_id, is_duplicate;
```

| empty_id | is_duplicate | cnt |
|----------|--------------|-----|
| 0 | 0 | 1886 |
| 1 | 0 | **1** |
| 1 | 1 | **370** |

First empty-id row won SET NX → `is_duplicate=0`; the next 370 lost → `is_duplicate=1`. Flagging came from **platform-sync Redis dedup** at publish time, not from a consumer-side lookup.

**Redis key still present:**

```
11111111-1111-1111-1111-111111111111:dedup:retreaver:
```

(Empty suffix after final `:` — matches empty `external_call_id`.)

---

## 5. Consequence check (no fix)

### Overlapping API-sync re-sync

| Window | Behavior |
|--------|----------|
| **Within 24h TTL** | Redis key exists → `is_duplicate=1` → **second row stored** in ClickHouse but excluded from analytics/coverage aggregates. |
| **After TTL expires** | SET NX succeeds again → `is_duplicate=0` → **true duplicate row** in ClickHouse (MergeTree has no unique constraint on `external_call_id`). |

Current data: zero non-empty `external_call_id` values with `count() > 1` for `platform='retreaver'` (query returned `dup_groups=0`), so re-sync duplicates have not manifested for well-keyed calls yet.

### Aggregate protection

| Surface | Filters `is_duplicate=0`? |
|---------|---------------------------|
| `/analytics` metrics, platform mix | **Yes** (`analytics.service.ts:32`) |
| `/coverage/heatmap` | **Yes** (`coverage.service.ts:29`) |
| `/pings` list/export | **No** — raw rows include duplicates (`pings.filters.ts` has no `is_duplicate` clause; `ingestion-slice-report.md` documents this intentionally) |

---

## Query reference (reproducible)

```powershell
# Schema
docker exec deploy-clickhouse-1 clickhouse-client --query "DESCRIBE pings" | findstr /i duplicate

# Retreaver duplicate breakdown
docker exec deploy-clickhouse-1 clickhouse-client --query "
  SELECT is_duplicate, count() FROM pings WHERE platform='retreaver' GROUP BY is_duplicate"

# Empty-id vs duplicate flag
docker exec deploy-clickhouse-1 clickhouse-client --query "
  SELECT external_call_id='', is_duplicate, count()
  FROM pings WHERE platform='retreaver'
  GROUP BY external_call_id='', is_duplicate"

# Redis empty-key evidence
docker exec deploy-redis-1 redis-cli KEYS "*dedup*retreaver:*"
```

---

## Related

- Retreaver mapping root cause: `docs/retreaver-real-pull-diagnosis.md`
- Phase 0 CASE A + verify output: `docs/ingestion-slice-report.md`
