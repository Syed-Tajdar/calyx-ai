# Phase 2A-fix2c report

Phase 2A-fix2c only. **No Phase 2B. No deletions. Not pushed.**
Spec: `docs/writeback-phase2a-fix2c-spec.md`.

Images rebuilt: `deploy-api`, `deploy-platform-sync`.

---

## Part 1 â€” Consumer health must not take down the API

### 1.1 â€” Separate signal `[PROVEN]`

- **`GET /live-status`**: `group_id`, `required`, `is_group_member`, `last_event_at`, `last_join_at`, `last_error`, `recreate_count`, `lag` (null in-process â€” admin lag not wired).
- **`GET /readyz`**: Postgres + ClickHouse only. Live consumer membership removed.

### 1.2 â€” Gate `[PROVEN]`

`verify-system.ps1` gained a **Live feed status** section that fails when `required` and not a member. Healthz remains **11/11** (10Ã— healthz + readyz). LiveStatus is a separate hard-fail line.

### 1.3 â€” Watchdog recreates consumer `[PROVEN]`

`resilient-consumer.ts`: on sustained non-membership (â‰¥60s), disconnect + recreate consumer with exponential backoff (2sâ†’30s). **No `process.exit` for consumer-only failures** (live feed + alert-engine).

**Process restart still not used** for consumer failures. Justification: exiting the process is what took the whole API out of Kubernetes readiness; recreating the consumer is enough. Unrecoverable **process** faults (OOM, fatal Node errors) remain the container runtimeâ€™s job via existing `restart: unless-stopped`.

### 1.4 â€” Proofs `[PROVEN]`

| Path | Result |
|---|---|
| **Short** (~25s broker stop) | `/live-status` downâ†’up; `recreate_count` stayed 0 (KafkaJS rejoined). API restart count **0**. One transient `/readyz` failure coincident with membership drop; subsequent probes while live was still down returned **200** for readyz/login/metrics. ws-smoke **PASS**. Recover ~73s from stop. |
| **Long** (~100s broker stop) | Watchdog **recreate_count max=2**. API restart count **0â†’0**. `loginFails=0`. `readyzFails=3` during reconnect storm (transient). After broker return, live member again; recover ~152s from stop. ws-smoke **PASS**. |

---

## Part 2 â€” Probe traffic (report only â€” **no deletion**)

### How messages were built `[PROVEN]`

From Phase 2A-fix2 lag tests (session logs):

| Batch | IDs | Fields |
|---|---|---|
| `lagprobe-*` | `lagprobe-{0..1999}` | `tenant_id=11111111-1111-1111-1111-111111111111`, `platform=ringba`, `ani_hash=abc`, `sync_path=webhook`, produced via `rpk topic produce` |
| `lagprobe2-*` | `lagprobe2-{0..2999}` while API paused | same shape |

### ClickHouse `pings` `[PROVEN]`

| Check | Result |
|---|---|
| `countIf(external_call_id LIKE 'lagprobe%')` | **0** |
| `countIf(ani_hash = 'abc')` | **0** |

`ani_hash` column is **`FixedString(64)`**. Inserting `"abc"` fails consumer write â€” probes stayed on Kafka (enrichment saw them) but **never landed in `pings`**.

### Test-data exclusion `[PROVEN]`

Prefixes covered: `apisync-` / `demo-` / `smoke-`. **`lagprobe-` is not excluded** â€” but with **0 rows** there is nothing uncaught in CH.

### KPI vs Ringba `[PROVEN]`

| Metric | Value |
|---|---|
| API default `analytics.total_pings` | **4586** |
| CH `uniqExact(external_call_id)` ringba excl. apisync/demo/smoke | **4586** |

They match. Probe traffic did not inflate CH KPIs.

### Proposed delete

**None.** No probe rows in `pings`. (If they had landed: exact-ID `ALTER TABLE pings DELETE WHERE external_call_id IN (...)` â€” **not run**.)

**Note:** future probe traffic must use the **`smoke-`** prefix so exclusion applies even if CH accepts the row.

---

## Part 3 â€” Lock release safety

### Before `[PROVEN]`

`RedisSyncLocker.Release` was plain `DEL` of `platform-sync:lock:{id}` â€” a late release after TTL expiry could delete another runâ€™s lock.

### After `[PROVEN]`

- `TryAcquire` stores a **unique 16-byte hex token** via `SET NX`.
- `Release` runs Lua compare-and-delete: delete only if `GET` equals the callerâ€™s token.
- `MemorySyncLocker` mirrors the same semantics.
- Unit test `TestReleaseOnlyDeletesOwnToken` **PASS**.

---

## Part 4 â€” Gates, commit, rescan

### Gates (sync live) `[PROVEN]`

| Script | Exit | Evidence |
|---|---|---|
| `verify-system.ps1` | **0** â€” Docker **15/15**, Healthz **11/11**, LiveStatus **PASS**, WS **PASS**, API 20/20 | `docs/evidence/phase2a-fix2c-gates.txt` |
| `verify-ingestion-slice.ps1` | **0** â€” INGESTION SLICE VERIFY PASS | `docs/evidence/phase2a-fix2c-ingestion.txt` |
| `verify-presentation.ps1` | **0** â€” PRESENTATION VERIFY PASS (API 20/20) | `docs/evidence/phase2a-fix2c-presentation.txt` |

### Commit

| Field | Value |
|---|---|
| Hash | cfe2c46ec05b588afa2d80f4dc9ce8d05a5d6904 |
| Branch | `main` (ahead of `origin/main`; **not pushed**) |
| `git status` | clean after commit |

### Secret rescan since `8f4c8f7` `[PROVEN]`

Scope: commits `288f0ff`â€¦`ca211e8` plus this working tree (fix2c). Patterns used:

| Pattern | Purpose | Result |
|---|---|---|
| `\bRA[0-9a-fA-F]{20,}\b` | Ringba account IDs (`RA` + hex) | **0 hits** |
| `eyJ[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+` | JWTs | **0 hits** |
| `CREDENTIAL_ENCRYPTION_KEY` / `ANI_ENCRYPTION_KEY` values â‰  `0123456789abcdef`Ã—4 | non-placeholder encryption keys | **0 non-placeholder** |
| `[A-Za-z0-9_\-]{32,}` outside known placeholders | long token-like strings | see below |
| `\b1?[2-9][0-9]{9}\b` in `docs/evidence/` excl. `15551234567` | phones | **0** in evidence files added/changed since `8f4c8f7` |

**Long â‰¥32 candidates that are not secrets** (listed for transparency):

- Ringba **call** IDs in `docs/evidence/phase2a-api-ids.txt` (`RGBâ€¦`) â€” not `RA` account IDs.
- Git SHAs / PowerShell `ps-script-â€¦` UUIDs in gate logs.
- Env **names** (`KAFKA_TOPIC_*`, migration filenames).

**Dev placeholders (separate list):**

| Location | Value |
|---|---|
| `deploy/docker-compose.yml` | `ANI_ENCRYPTION_KEY` / `CREDENTIAL_ENCRYPTION_KEY` = `0123456789abcdef`Ã—4 |
| `deploy/docker-compose.yml` | `WRITEBACK_INTERNAL_TOKEN=dev_writeback_internal_token_change_me` |
| `deploy/docker-compose.yml` | `JWT` / auth secret `callloom_dev_jwt_secret_min_32_chars_long` |

**Pre-existing (outside commit range; not introduced since `8f4c8f7`):** `docs/evidence/ringba-writeback-probe/postman-collection.json` contains many 10â€“11 digit example numbers from Ringbaâ€™s public Postman export (first-seen commit `9a9da2a`). Not new in this rescan window.

---

## Stop

Phase 2B not started. **Do not push** until operator approves.
