# Calyx — S1B Follow-up Report

**Date:** 2026-09-25  
**Scope:** Poison retry safety, BATCH_SIZE restore, B4 size-path live re-proof, lock renewal live. **No S2.**  
**Evidence:** `docs/evidence/s1b-followup/`

---

## Scoreboard

| Item | Result |
|---|---|
| F1 poison backoff (≥ few minutes for N=5) | **PASS** `[PROVEN]` |
| F1 CH transient codes never count toward N | **PASS** `[PROVEN]` |
| F2 BATCH_SIZE=500 compose + printenv | **PASS** `[PROVEN]` |
| F3 B4 live size-path outage | **PASS** `[PROVEN]` |
| F4 lock renewal live (≥2 renew logs + PTTL refresh) | **PASS** `[PROVEN]` |
| Gates verify-system / ingestion-slice / presentation | **0 / 0 / 0** `[PROVEN]` |

---

## 1. Poison retry safety (B4.2)

### (a) Backoff between N=5 attempts

- `MaxUnknownAttempts = 5`
- `UnknownAttemptMinInterval = 45s`
- Minimum wall time first counted failure → poison: `(5−1)×45s = **3 minutes**` via `UnknownAttemptMinSpan()`

`bumpAttempts` only increments when `now.Sub(st.lastAt) ≥ UnknownAttemptMinInterval`, so near-immediate retries do not burn the budget. A brief ClickHouse blip cannot dead-letter valid messages within seconds.

Unit: `TestUnknownAttemptBackoffRequiresMinInterval` — asserts span ≥ `UnknownAttemptMinSpan()` before DLQ.

### (b) ClickHouse transient classification

`isTransientClickHouseError` never counts toward N (even when Ping succeeds). Codes via clickhouse-go `Exception.Code`:

| Code | Name |
|---|---|
| 159 | TIMEOUT_EXCEEDED |
| 160 | TOO_SLOW |
| 209 | SOCKET_TIMEOUT |
| 210 | NETWORK_ERROR |
| 241 | MEMORY_LIMIT_EXCEEDED |
| 252 | TOO_MANY_PARTS |

Plus name/message fallbacks and `net.Error` timeout/temporary.

Units: `TestTransientCHErrorDoesNotCountTowardPoison`, `TestIsTransientClickHouseErrorByCode` — **ok** (`unit_poison.txt`).

---

## 2. Config

| Source | BATCH_SIZE | BATCH_FLUSH_MS |
|---|---|---|
| `deploy/docker-compose.yml` | **500** | **5000** |
| `deploy-consumer-1` printenv | **500** | **5000** |

Evidence: `batch_size.txt`, `batch_size_final.txt`.

**Test overrides restored:**
- Temporary `BATCH_FLUSH_MS=600000` (size-path proof aid) → restored **5000**
- Temporary `MOCK_MODE=false` via `docker-compose.s1bf-sync.yml` (live Ringba backfill) → override removed; printenv **true**
- No other durable compose env changes left from tests

**Also:** Redpanda healthcheck `timeout` raised 5s→15s + `start_period: 20s` — under load `rpk cluster health` exceeded 5s and falsely marked unhealthy (Docker 14/15). Cluster was Healthy; Docker stale. After recreate: `redpanda=healthy` (`rp_health.txt`).

---

## 3. B4 live evidence (BATCH_SIZE=500)

Size-triggered flush:

```
msg":"size-triggered flush" ... "batch_size":500
```

Evidence: `b4_size_logs.txt`. Summary (`b4_summary2.txt`):

| Metric | Value |
|---|---|
| smoke rows after recovery (prefix_count) | **884** |
| poison_total before→after | **0→0** unchanged |
| dlq_total before→after | **0→0** unchanged |
| max buffered_messages | **500** (within bound) |
| consumer PID unchanged | **True** |

---

## 4. Lock renewal live

Full-history `/sync/window` backfill with lock held >90s. Renew every 30s.

**≥2 renew log lines** (`f4_renew_logs_raw.txt`), e.g.:

```
2026-09-25T06:09:03Z sync lock renewed integration_id=ce6fd338… ttl=1m30s
2026-09-25T06:09:33Z sync lock renewed integration_id=ce6fd338… ttl=1m30s
```

(12 renew lines during the held run.)

**Redis PTTL refresh** (`f4_pttl_samples2.txt`): `t=79s pttl=62659` → `t=87s pttl=85276`.

**Fix:** log field `pttl_ms` was inflated by double-converting go-redis `PTTL` (already a `time.Duration`). Fixed in `sync_lock.go`. Real TTL refresh proven via `redis-cli PTTL`.

---

## 5. Gates

| Gate | Exit | Notes |
|---|---|---|
| verify-system | **0** | Docker **15/15**, Healthz **11/11**, LiveStatus, WS, Pipeline 4/4, units, web build |
| verify-ingestion-slice | **0** | |
| verify-presentation | **0** | 20/20 |

Evidence: `gate_system.txt`, `gate_ingestion.txt`, `gate_presentation.txt`, `gate_exits.txt`.

---

## 6. Commit / scan / push

*(filled after commit)*

S2 not started.
