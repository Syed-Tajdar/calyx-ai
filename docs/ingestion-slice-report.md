# Ingestion Slice — Phase 0 Report

**Date:** 2026-07-03  
**Status:** IMPLEMENTED — local verification PASS

---

## PRE-BUILD CONFIRMATION CHECKS

### CONFIRM 1 — `is_duplicate` exists and where dedup happens

**Command:**
```powershell
docker exec deploy-clickhouse-1 clickhouse-client --query "DESCRIBE pings" | Select-String -Pattern "duplicat"
```

**Raw output:**
```
is_duplicate	UInt8	DEFAULT	0
```

**Dedup code path:**
```
services/ingestion/internal/dedup/dedup.go     — Redis SetNX {tenant}:dedup:{platform}:{external_call_id}
services/ingestion/internal/handlers/webhook.go — isDup → ping.IsDuplicate = true; still Publish()
services/ingestion/internal/handlers/ringba.go — same pattern
services/consumer/internal/writer/writer.go    — writes IsDuplicate → UInt8 is_duplicate column
services/consumer/pkg/clickhouse/clickhouse.go — INSERT includes is_duplicate
```

**Duplicate row counts in ClickHouse (pre-build):**
```
dup_rows (is_duplicate=1): 3
unique_rows (is_duplicate=0): 123
```

| Verdict | **CASE A** |
|---------|------------|
| Meaning | `is_duplicate` column exists; consumer writes it; duplicates **are stored** with flag `1` |
| FIX 2 action | **Add `WHERE is_duplicate = 0`** to analytics metrics, platform-mix, and coverage heatmap aggregates |

---

### CONFIRM 2 — Plaintext `call_key` rows before encryption

**Pre-build counts:**
```
total=126  nonempty_call_key=6
len=23, count=6 (all retreaver — plaintext smoke tests)
```

**Cleanup SQL (run post-deploy):**
```sql
ALTER TABLE pings DELETE
WHERE platform = 'retreaver' AND call_key != '' AND length(call_key) = 23
```

**Post-cleanup verification:**
```
plaintext_len (len=23): 0
ciphertext_len (len>30): 1
```

---

## IMPLEMENTATION SUMMARY

### FIX 1 — `call_key` encrypt at ingestion producer

| File | Change |
|------|--------|
| `services/ingestion/internal/crypto/ani.go` | Added `DecryptANI`, `EncryptSecret`, `DecryptSecret` |
| `services/ingestion/internal/producer/producer.go` | Encrypt `call_key` → hex before Kafka JSON; `encryptionKey` on `Producer` |
| `services/ingestion/cmd/server/main.go` | Pass `cfg.EncryptionKey` to `NewProducer` |
| `services/ingestion/internal/models/ping.go` | Document in-memory plaintext vs at-rest ciphertext |
| `services/ingestion/internal/crypto/ani_test.go` | `EncryptSecret` round-trip test |
| `services/ingestion/internal/producer/producer_test.go` | Kafka JSON must not contain plaintext substring |

### FIX 2 — Dedup aggregates (CASE A)

| File | Change |
|------|--------|
| `services/api/src/analytics/analytics.service.ts` | `nonDuplicateClause = 'AND is_duplicate = 0'` on `getMetrics` + `getPlatformMix` |
| `services/api/src/coverage/coverage.service.ts` | `AND is_duplicate = 0` on heatmap query |

**Not changed:** `/pings` list/export pagination counts (raw rows may include duplicates).

### FIX 3 — Mode scoping (Option B)

| Service | Phase 0 mode |
|---------|----------------|
| ingestion, consumer | Always real (no MOCK_MODE gate) |
| api (reads) | `MOCK_MODE=true` OK — reads ClickHouse |
| api WS | Mock emitter — verify via poll/refresh |
| scoring, compliance, attribution | `MOCK_MODE=true` |

### Exposure audit

- API `PING_SELECT` — does **not** include `call_key` (grep: no `call_key` in `services/api/src`)
- Ingestion logs `call_key_present` bool only — value never logged

---

## VERIFICATION RESULTS

### Unit tests

```powershell
cd services\ingestion
go test ./internal/crypto/... ./internal/producer/... ./...
```
```
ok  	github.com/Syed-Tajdar/calyx-ai/services/ingestion/internal/crypto	0.946s
ok  	github.com/Syed-Tajdar/calyx-ai/services/ingestion/internal/producer	1.074s
ok  	github.com/Syed-Tajdar/calyx-ai/services/ingestion/internal/connectors	0.600s
...
```

```powershell
cd services\consumer
go test ./...
```
```
(no test failures — all packages compile)
```

### Builds

```powershell
cd services\api && npm run build   # PASS
cd web && npm run build            # PASS
```

### `verify-presentation.ps1`

```
API checks: 20 / 20 passed
PRESENTATION VERIFY PASS
```

### `verify-ingestion-slice.ps1`

```
Before: analytics.total_pings=119 coverage.total_pings=119
Sending unique Ringba ping (... state=WA)...
PASS: unique ping incremented analytics.total_pings (119 -> 120)
PASS: unique ping incremented coverage.total_pings (119 -> 120)
Resending duplicate Ringba ping (same CallID)...
PASS: duplicate did not inflate analytics.total_pings (120)
PASS: duplicate did not inflate coverage.total_pings (120)
PASS: /pings shows is_duplicate=1 on resend (1 flagged row(s))
Retreaver call_key ciphertext check...
PASS: call_key stored as ciphertext (length=94, plaintext not in DB)

INGESTION SLICE VERIFY PASS
```

### Implementation checklist

- [x] FIX 1 code + unit tests
- [x] FIX 2 aggregate filters
- [x] ClickHouse plaintext `call_key` cleanup (6 rows deleted)
- [x] `scripts/verify-ingestion-slice.ps1` + `make verify-ingestion-slice`
- [x] `go test ./...` (ingestion + consumer)
- [x] `npm run build` (api + web)
- [x] `verify-presentation.ps1`
- [ ] Staging Ringba pixel test (pending — local signed webhook + WA state used as proof)

| Check | Result |
|-------|--------|
| call_key encrypt/decrypt unit test | **PASS** |
| call_key never in logs/API/UI | **PASS** |
| Aggregates exclude is_duplicate=1 | **PASS** |
| Duplicate does not inflate totals | **PASS** |
| Mode scoping documented | **PASS** |
| Real Ringba ping end-to-end (local signed) | **PASS** (WA state) |
| Non-demo state on map | **PASS** (WA ping in verify script) |
| go test / npm build / verify-presentation | **PASS** |

---

## STAGING CHECKLIST (manual — pending)

1. `make dev-up` + migrations
2. Update `integrations` row: real Ringba `webhook_token` + `webhook_secret`
3. Ringba pixel → `https://<staging>/webhooks/ringba/{token}` with **Geo:State** macro
4. Fire 3–5 test calls across states (include non-demo state)
5. Confirm: `200` ingestion, ClickHouse normalized state, `/pings` + `/coverage/heatmap`, map colors

---

## WHAT THIS PROVES

**Proves:** Real webhook auth → normalized state → dedup-correct aggregates → encrypted `call_key` at rest → dashboard/coverage on refresh.

**Does not prove:** Deepgram, OpenAI scoring, DNC, Meta attribution (later phases).
