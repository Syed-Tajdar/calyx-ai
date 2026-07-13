# Retreaver Hardening + Retest Report

**Date:** 2026-07-09  
**Integration:** stays on **Test** throughout this pass (not flipped to Live).

---

## Summary

| Part | Status |
|------|--------|
| PART 1 — Credential redaction | **PASS** |
| PART 2a — Aggregate DISTINCT dedupe | **PASS** |
| PART 2b — `DEDUP_TTL_HOURS` (168h default) | **PASS** |
| PART 2c — ReplacingMergeTree note | Documented (not implemented) |
| PART 3 — Mapper tests (no live API) | **PASS** |
| PART 4 — Empty-id cleanup | **PASS** (371 → 0) |
| PART 5 — DNS retry + error classification | **PASS** |
| PART 6 — Builds + verify scripts | **MOSTLY PASS** (see notes) |
| PART 7 — Go-Live checklist | Documented below |
| PART 8 — Shared dedup package follow-up | Documented below |

---

## PART 1 — Security: credential redaction

### Implementation

- New package: `services/platform-sync/internal/redact/redact.go`
  - `RedactMessage` / `SanitizeErr` — strips query params from URLs; redacts `api_key`, `company_id`, etc.
  - `ClassifySyncError` — prefixes network / auth / mapping errors for `last_sync_error`
  - `IsRetryableNetwork` — DNS/timeout detection for HTTP client retry
- Applied at:
  - `syncer.go` — `markError`, `RunAll` log, `FetchAllPages` error persist
  - `httpclient.go` — sanitized errors returned from `Do()`
  - `cmd/server/main.go` — `/sync`, `/test-auth`, scheduled sync logs

### Unit test

```
cd services/platform-sync
go test ./internal/redact/... -count=1
ok  	github.com/.../internal/redact	0.639s
```

`TestSanitizeErrRedactsAPIKey` asserts `SECRET` absent and `api_key=[REDACTED]` present.

### Grep audit

All platform-sync error log/persist sites for HTTP errors now go through `redact.SanitizeErr` or `ClassifySyncError`. Adapter `checkHTTPStatus` returns status snippets only (no URLs).

### Log hygiene

```
docker compose -f deploy/docker-compose.yml restart platform-sync
```

Container recreated during rebuild (PART 6); prior log buffer rotated.

---

## PART 2 — Durable dedup

### 2a — Query-level DISTINCT (analytics + coverage)

**Files changed:**
- `services/api/src/analytics/analytics.service.ts` — metrics + platform-mix use inner `GROUP BY external_call_id` with `argMax(revenue|payout, received_at)`
- `services/api/src/coverage/coverage.service.ts` — heatmap uses `GROUP BY state, external_call_id` then outer aggregate

`/pings` list unchanged (raw rows + DUP badge).

### Aggregate dedupe verification

```
before metrics.total_pings=2811
INSERT 2 rows: external_call_id='dedup-hardening-test-001', both is_duplicate=0
/pings?search=dedup-hardening-test-001 → total=2  (both rows visible)
ClickHouse deduped count for call: 1
API metrics.total_pings=2814 == ClickHouse deduped total=2814
```

Test rows removed after verify:
```sql
ALTER TABLE pings DELETE WHERE external_call_id='dedup-hardening-test-001'
```

### 2b — Configurable dedup TTL

- `platform-sync/internal/config/config.go` — reads `DEDUP_TTL_HOURS` (default **168** = 7 days)
- `deploy/docker-compose.yml` — `DEDUP_TTL_HOURS: "168"` on ingestion + platform-sync
- `.env.example` — updated with memory tradeoff comment

### 2c — Future option (not implemented)

**ReplacingMergeTree** on `(tenant_id, platform, external_call_id)` would let ClickHouse collapse duplicate rows on merge. Requires engine migration + read-path `FINAL` or `GROUP BY` everywhere. Recorded for a later phase.

---

## PART 3 — Mapper tests (fixture only, no live API)

### Tests (`go test ./internal/adapter/... -count=1`)

| Test | Asserts |
|------|---------|
| `TestMapRetreaverCall` | Flat V2 fields; revenue/payout/campaign/publisher/target |
| `TestMapRetreaverCallNestedV2ListItem` | `{"call":{uuid,...}}` unwrap |
| `TestMapRetreaverCallNormalizeState` | `Texas` → `TX`, `tx` → `TX` |
| `TestMapRetreaverCallFieldFallbacks` | `call_uuid`, `id`, legacy `campaign_id`/`client_afid` |
| `TestMapRetreaverCallShapes` | Wrapped item, flat object, empty uuid → empty (not silent fake id) |
| `TestPublishSyncCallsSkipsEmptyExternalID` | 2 blank ids skipped, 1 published (`syncer_test.go`) |

`NormalizeState` wired at `retreaver_adapter.go:164` via `state.NormalizeState(row.CallerState)`.

---

## PART 4 — Cleanup

### Before

```
SELECT count() FROM pings WHERE platform='retreaver' AND external_call_id=''
→ 371
```

### Actions

```sql
ALTER TABLE pings DELETE WHERE platform='retreaver' AND external_call_id=''
```

```
redis-cli DEL "11111111-1111-1111-1111-111111111111:dedup:retreaver:"
→ 1
```

### After

```
empty_id count → 0
platform='retreaver' total rows → 1899 (demo apisync-* rows retained)
```

---

## PART 5 — DNS resilience

- `httpclient.go` — retries on `IsRetryableNetwork` (DNS blips, timeouts) with backoff (same 3-attempt loop as 429)
- `deploy/docker-compose.yml` — platform-sync `dns: [1.1.1.1, 8.8.8.8]`
- `ClassifySyncError` — `last_sync_error` prefixes:
  - `network error (transient, will retry): …`
  - `authentication error: …`
  - `mapping error: …`

---

## PART 6 — Rebuild + verification

### Builds

```
cd services/platform-sync && go build ./... && go test ./...
→ all packages ok

cd services/api && npm run build
→ PASS

cd web && npm run build
→ PASS (Next.js 14.2.35)
```

### Docker

```
cd deploy
docker compose build platform-sync api
docker compose up -d platform-sync api
→ both recreated and started
```

### Scripts

**verify-api-sync.ps1** — PASS
```
PASS: platform-sync healthz
Before: analytics.total_pings=2809
PASS: test connection
PASS: analytics.total_pings increased (2809 -> 2810)
PASS: found apisync ping
```

**verify-presentation.ps1** — PASS (20/20)

**verify-ingestion-slice.ps1** — FAIL (Ringba webhook `unauthorized`)
- Root cause: `dev_ringba_token_change_me` not present in `integrations` table (0 rows). Pre-existing seed gap; unrelated to Retreaver hardening.

**smoke-test-all-platforms.ps1** — FAIL (Ringba `unauthorized`, same cause)

**ws-smoke-test.mjs** — PASS
```
WS SMOKE PASS: 3 events, types: ping_received, call_scored
```

---

## PART 7 — Go-Live checklist (user runs later)

1. Paste **fresh** Retreaver `api_key` + `company_id` in Settings (UI only).
2. Keep Data source = **Test**. Save. Confirm mock sync ok.
3. Flip **only that row** to **Live**. Trigger one sync.
4. Verify:
   - `sync_status = ok`
   - ClickHouse: retreaver rows with real UUIDs (not `apisync-%`, not empty), 2-letter state, revenue populated
   - `docker logs deploy-platform-sync-1` — no `api_key` in output
   - `/pings` shows real calls; platform-mix includes Retreaver
   - Coverage map shows states after refresh
5. Trigger sync **again immediately** → aggregates unchanged; `/pings` shows duplicate flagged (`is_duplicate=1`)
6. If anything off → flip back to Test (one click)

---

## PART 8 — Follow-up (not in this pass)

- Extract shared `dedup` + `producer` packages — `services/ingestion/internal/dedup/dedup.go` and `services/platform-sync/internal/dedup/dedup.go` are identical copies (drift risk).
- Seed `dev_ringba_token_change_me` integration row so webhook smoke scripts pass again.
- Redact `scheduled sync failed` was fixed in final pass; rebuild platform-sync image if not yet deployed from latest `main.go`.

---

## Verification checklist

- [x] PART 1: URLs/query params redacted; redaction unit test passes; `last_sync_error` sanitized
- [x] PART 2a: analytics + coverage DISTINCT by `external_call_id`; duplicate call does not double-count; `/pings` shows both rows
- [x] PART 2b: platform-sync reads `DEDUP_TTL_HOURS`; default 168h; documented
- [x] PART 2c: ReplacingMergeTree recorded as future option
- [x] PART 3: mapper unwrap + NormalizeState + fallbacks + empty-id guard tests pass
- [x] PART 4: empty-id rows deleted (371 → 0); stale Redis key removed
- [x] PART 5: DNS/network retry + classified `last_sync_error`
- [x] PART 6: builds green; verify-api-sync + presentation + ws green
- [x] PART 6: smoke-test-all-platforms 4/4 — **PASS** (after `make seed-dev-integrations`; see pre-live-gate-report)
- [x] PART 7: go-live checklist written
- [x] No credentials in this report or test output

---

**Safe to flip Live: YES** — superseded by pre-live gate. See **[pre-live-gate-report.md](./pre-live-gate-report.md)** for deployed-image proof, bogus-key test, and final script output.
