# Phase 2A-fix2e report

Phase 2A-fix2e only. **No Phase 2B. No deletions. Not pushed.**
Spec: `docs/writeback-phase2a-fix2e-spec.md`.

Images rebuilt: `deploy-api`, `deploy-consumer`.

Commit hash: recorded in the operator summary for this commit (no follow-up hash-only commit).

---

## Part 1 - ClickHouse outage must not be classified as poison

### 1.1 - Classifier `[PROVEN]`

Poison (skip + commit offset):

1. Pre-insert validation `ValidatePingForInsert`: invalid `tenant_id` UUID; `ani_hash` length > 64.
2. Insert error substring match in `isPoisonInsertError`: `AppendRow`, `invalid UUID`, `FixedString`, `batch is invalid`, `invalid size`.

Everything else (DNS, connection refused, i/o timeout, …) is **transient**: kept in the in-memory batch, **not** counted as poison, offset **not** committed until a later successful write.

```203:234:services/consumer/internal/consumer/consumer.go
func isPoisonInsertError(err error) bool {
	// ...
	needles := []string{"AppendRow", "invalid UUID", "FixedString", "batch is invalid", "invalid size"}
	// ...
}
```

Unit test `TestIsPoisonInsertError` asserts connection/DNS strings are transient. **PASS**.

### 1.2 - Real ClickHouse outage proof `[PROVEN]`

| Step | Result |
|---|---|
| `poison_total` before | **2** (from prior fix2d poison probes) |
| Stop `deploy-clickhouse-1`; send **20** `smoke-fix2e-chout-034324-*` via ingestion | webhooks 200 |
| Wait ≥60s with CH down | Logs: `per-message insert transient failure` / `retry=N` / `poison_skipped=0` / `poison_total=2` — **no** `poison ping skipped` for these ids |
| Start CH | `batch flushed count=21` (20 smoke + 1 concurrent real call) |
| CH `count()` for prefix | **20 / 20** |
| `poison_total` after | **2** (unchanged) |

Evidence: `docs/evidence/phase2a-fix2e-ch-outage.txt`, `phase2a-fix2e-ch-outage-count.txt`.

**No classifier change required for outage safety** — the existing deny-list already treated CH-down errors as transient. Comment clarified in code.

### 1.3 - Poison log recoverability `[PROVEN]`

| Field | Before fix2e | After |
|---|---|---|
| `external_call_id` | yes | yes |
| `tenant_id` | yes | yes |
| `platform` | **missing** | **added** |
| `kafka_topic` | **missing** | **added** |
| `kafka_partition` | **missing** | **added** |
| `kafka_offset` | **missing** | **added** |
| `reason` | yes | yes |
| `poison_total` | yes | yes |

---

## Part 2 - Postgres timeout scope

### 2.1 - Shared pool? `[PROVEN]`

Yes. `PostgresService.pool` is used by `withTenant` across auth, scorecards, integrations, analytics, alerts, providers — not only `/readyz`.

### 2.2 - Split pools + proof `[PROVEN]`

| Pool | Timeouts | Purpose |
|---|---|---|
| App `pool` | connect **10s**, query/statement **30s** | Real tenant workloads (joins, scorecard CRUD) |
| Ready `readyPool` | connect/query/statement **1.5s**, `max=2` | `/readyz` only |
| ClickHouse `readyClient` | `request_timeout` **1.5s** | `/readyz` only |

**Additional finding during Part 2 proof `[PROVEN]`:** with a shared DNS path, stopping Redpanda caused KafkaJS `getaddrinfo ENOTFOUND redpanda` storms that saturated Node's DNS threadpool so Nest PG/CH readiness queries timed out even while `wget`/`nc` from the same container to those hosts succeeded (`event_loop_max_lag_ms` rose to ~697). Mitigations shipped:

- `UV_THREADPOOL_SIZE=64` (compose + Dockerfile)
- Ready PG/CH clients pin IPv4 via `getent hosts` at boot
- KafkaJS `connectionTimeout` / retry caps reduced

**≥100s broker-outage `/readyz` proof (rebuilt API):** **0 failures**. Evidence in session log `READYZ_FAILS=0`.

---

## Part 3 - Label correction + checklist

### 3.1 - fix2d report `[PROVEN]`

`docs/writeback-phase2a-fix2d-report.md`: the Docker-network / pool-pin mechanism for `/readyz` 503s is now labeled **`[INFERRED]`**; the fix and 0-failure proof remain `[PROVEN]`. (fix2e further proved a DNS-threadpool mechanism under broker stop.)

### 3.2 - Checklist `[PROVEN]`

`docs/deploy-checklist.md` **Data** section: TTL retention item added (`received_at + 24 months` vs `occurred_at`).

---

## Part 4 - Gates, commit, scan

| Script | Exit | Evidence |
|---|---|---|
| `verify-system.ps1` | **0** — Docker 15/15, Healthz 11/11, LiveStatus PASS, WS PASS | `docs/evidence/phase2a-fix2e-gates.txt` |
| `verify-ingestion-slice.ps1` | **0** — INGESTION SLICE VERIFY PASS | `docs/evidence/phase2a-fix2e-ingestion.txt` |
| `verify-presentation.ps1` | **0** — PRESENTATION VERIFY PASS | `docs/evidence/phase2a-fix2e-presentation.txt` |

### Secret rescan since `b47335e` `[PROVEN]`

Patterns (same as fix2c):

| Pattern | Purpose | Result |
|---|---|---|
| `\bRA[0-9a-fA-F]{20,}\b` | Ringba account IDs | **0** |
| `eyJ…` JWT shape | JWTs | **0** |
| `CREDENTIAL_ENCRYPTION_KEY` / `ANI_ENCRYPTION_KEY` ≠ `0123456789abcdef`×4 | non-dev keys | **0** |
| `\b1?[2-9][0-9]{9}\b` in changed `docs/evidence/` excl. `15551234567` | phones | **0** |

Dev placeholders: compose `CREDENTIAL_ENCRYPTION_KEY` / `ANI_ENCRYPTION_KEY` = `0123456789abcdef`×4.

---

## Stop

Phase 2B not started. **Do not push** until operator approves.
