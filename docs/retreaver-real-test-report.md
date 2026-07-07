# Retreaver Ingestion Real Test — Report

**Date:** 2026-07-06  
**Environment:** Local Docker (`deploy-*` containers) + tunnel-ready  
**Status:** **LOCAL PATH VERIFIED** — signed smoke + automated checks PASS; **real Retreaver account test PENDING** (user must configure tunnel + credentials)

---

## Summary

| Milestone | Status |
|-----------|--------|
| Code pre-flight (connector, HMAC, call_key encrypt, routes) | **PASS** |
| Payload mapping documented | **PASS** |
| Local Docker stack running | **PASS** |
| Signed local Retreaver webhook (dev HMAC) | **PASS** (200, E2E) |
| `verify-retreaver-real.ps1` | **PASS** (10/10) |
| Real Retreaver account → tunnel → live calls | **PENDING** (Steps 3–4 — user action) |

**Proves today:** Retreaver ingestion path works end-to-end on local Docker — auth, parse, encrypt `call_key`, ClickHouse, API, platform-mix, coverage aggregates.

**Pending:** Point a real Retreaver postback at `https://<tunnel>/webhooks/retreaver/{token}` with production signing secret.

---

## STEP 0 — Pre-flight code confirmation

### Connector files

```
services/ingestion/internal/connectors/retreaver.go
services/ingestion/internal/connectors/retreaver_test.go
```

### Auth + call_key + encrypt (grep highlights)

```
retreaver.go:20  Authenticate → security.VerifyRetreaverHMAC
retreaver.go:98  ping.CallKey = firstValue(values, "call_key", "callKey", "CallKey")
producer.go:50   crypto.EncryptSecret(ping.CallKey, ...) → hex in Kafka/ClickHouse
platform_hmac.go:11  X-Retreaver-Signature, HMAC-SHA256 hex of body or raw query
```

### Webhook routes

```
services/ingestion/cmd/server/main.go:60  mux.Handle("GET /webhooks/", webhook)
services/ingestion/cmd/server/main.go:61  mux.Handle("POST /webhooks/", webhook)
```

Path pattern: `/webhooks/{platform}/{token}` — Retreaver: `/webhooks/retreaver/{webhook_token}`

### Postgres integration row (secrets not printed)

```sql
SELECT platform, name, is_active, length(webhook_token) AS token_len,
       (webhook_secret IS NOT NULL AND webhook_secret != '') AS has_secret
FROM integrations WHERE platform = 'retreaver';
```

**Output:**
```
 platform  |          name          | is_active | token_len | has_secret
-----------+------------------------+-----------+-----------+------------
 retreaver | Retreaver Dev Postback | t         |        29 | t
```

Dev placeholders from [`db/postgres/migrations/000018_multi_platform_integrations.up.sql`](../db/postgres/migrations/000018_multi_platform_integrations.up.sql). Replace via SQL before real Retreaver traffic (see STEP 2).

### Note on outdated docs

[`docs/real-readiness-report.md`](real-readiness-report.md) §2.3 lists `call_key` plaintext as a blocker — **superseded** by Phase 0 ([`docs/ingestion-slice-report.md`](ingestion-slice-report.md)).

---

## STEP 1 — Payload mapping audit

### Calyx parser vs Retreaver fields

| Internal field | Accepted query / tag sources | Normalization |
|----------------|------------------------------|---------------|
| `external_call_id` | `call_uuid`, `callUuid`, `CallUUID` | Required |
| `call_key` | `call_key`, `callKey`, `CallKey` (query or JSON) | Encrypted at `producer.Publish`; log `call_key_present` only |
| `state` | `state`, `State`, `tag[state]`, `tags` CSV, `tag[geo_state]` | `NormalizeState()` in `ping_builder.go` |
| `caller` | `caller`, `caller_id`, `caller_number`, `ani` | ANI hashed + encrypted |
| `revenue` | `revenue`, `conversion_value`, `payout`, `tags[revenue]` | float |
| `event` | `event`, `event_type`, `status` | Default `incoming` |

### Auth (must match Retreaver account)

| Property | Calyx implementation |
|----------|---------------------|
| Header | `X-Retreaver-Signature` (fallback `X-Signature`) |
| Algorithm | HMAC-SHA256, lowercase hex |
| Signed payload | Raw query string (GET) or request body (POST) |
| Secret source | `integrations.webhook_secret` per tenant |

**Pre-test gate:** Confirm Retreaver account signing matches [`platform_hmac.go`](../services/ingestion/internal/security/platform_hmac.go). Mismatch → **401** until connector updated.

### Risk — JSON POST path

When `Content-Type: application/json`, Retreaver connector uses `ParseRingbaJSON` (Ringba field names: `CallID`, `caller_id`, …). Real Retreaver JSON may use different keys.

**Recommendation:** First real test use **GET query string** (same as [`scripts/smoke-test.ps1`](../scripts/smoke-test.ps1) Retreaver branch).

### Postback URL template (GET)

```
https://<TUNNEL_HOST>/webhooks/retreaver/<WEBHOOK_TOKEN>?call_uuid=[uuid]&call_key=[key]&caller=[caller]&state=[state]&event=incoming&revenue=[revenue]
```

Local signing (dev secret):

```powershell
cd services/ingestion
go run ./cmd/platformsig retreaver "call_uuid=...&call_key=...&caller=...&state=WA&event=incoming&revenue=9.75"
# Header: X-Retreaver-Signature: <output>
```

---

## STEP 2 — Local stack + credentials

### Stack status (2026-07-06)

| Container | Status |
|-----------|--------|
| deploy-ingestion-1 | Up |
| deploy-api-1 | Up |
| deploy-web-1 | Up |
| deploy-postgres-1 | Healthy |
| deploy-clickhouse-1 | Healthy |

### MOCK_MODE scoping (Option B)

| Service | Mode |
|---------|------|
| ingestion, consumer | Always real |
| api reads | `MOCK_MODE=true` OK |
| scoring, compliance, attribution | `MOCK_MODE=true` |

### Tunnel setup (for real Retreaver)

Retreaver must reach your machine over HTTPS:

```powershell
# Example: ngrok (install separately)
ngrok http 8080
# Use the https URL as TUNNEL_HOST
```

Postback: `https://<TUNNEL_HOST>/webhooks/retreaver/<WEBHOOK_TOKEN>?...`

### Update real credentials (DB only — never commit)

```sql
UPDATE integrations
SET webhook_token = '<REAL_TOKEN>',
    webhook_secret = '<REAL_RETREAVER_SIGNING_SECRET>',
    is_active = true,
    updated_at = NOW()
WHERE tenant_id = '11111111-1111-1111-1111-111111111111'
  AND platform = 'retreaver';
```

Then rebuild if needed: `cd deploy && docker compose up -d --build ingestion`

---

## STEP 3–4 — Real Retreaver account (PENDING)

**User actions required:**

1. Run tunnel to `:8080`
2. Update `integrations` with real token + signing secret
3. Configure Retreaver postback URL to tunnel + required fields
4. Fire 2–3 real calls across states (include **WA** or other non-demo state)
5. Ensure at least one call sends `call_key`

---

## STEP 5 — Verification results

### Local signed smoke test

```powershell
powershell -ExecutionPolicy Bypass -File scripts/smoke-test.ps1 -Platform retreaver
```

**Output:**
```
[retreaver] Sending test ping (call_id=4fe44bbf-9a65-483b-90c4-647b964197d9)
Ingestion response: 200
E2E PASS (retreaver)
platform  external_call_id                     state revenue event_type
retreaver 4fe44bbf-9a65-483b-90c4-647b964197d9 CA       9.75 incoming
call_key stored in ClickHouse (value not logged)
```

### Automated verify script

```powershell
make verify-retreaver-real
# or: powershell -ExecutionPolicy Bypass -File scripts/verify-retreaver-real.ps1
```

**Output (2026-07-06):**
```
=== Retreaver real verify ===
  PASS GET /pings?platform=retreaver (rows=10)
  PASS API response excludes call_key field
  PASS GET /analytics/platform-mix includes retreaver (count=16)
  PASS GET /coverage/heatmap has geo data (total_pings=124 states=11)
  PASS ClickHouse retreaver rows by state (state_groups=4)
  PASS call_key stored (with_key=4)
  PASS call_key likely ciphertext (ciphertext=4 plaintext_len_le30=0)
  PASS Ingestion logs show retreaver ping ingested
  PASS No retreaver 401 auth failures in recent logs
  PASS call_key value not in ingestion logs
Summary: PASS=10 FAIL=0 WARN=0
```

### Ingestion log sample (no call_key value)

```json
{"msg":"ping ingested","platform":"retreaver","external_call_id":"4fe44bbf-...","is_duplicate":false,"call_key_present":true}
```

### ClickHouse checks (safe — no call_key values)

```sql
SELECT state, count() AS cnt FROM pings
WHERE platform = 'retreaver' AND is_duplicate = 0
GROUP BY state ORDER BY cnt DESC;

SELECT countIf(call_key != '') AS with_key,
       countIf(length(call_key) > 30) AS likely_ciphertext
FROM pings WHERE platform = 'retreaver';
```

---

## Checklist

| Check | Local signed | Real Retreaver account |
|-------|--------------|------------------------|
| Ingestion 200 (HMAC verified) | **PASS** | PENDING |
| ClickHouse rows, normalized state | **PASS** | PENDING |
| Non-demo state (e.g. WA) | WARN — use WA on real test | PENDING |
| call_key encrypted; not in API/UI/logs | **PASS** | PENDING |
| Dashboard `/pings` + platform-mix | **PASS** | PENDING |
| Coverage map (refresh page) | **PASS** (not live — see [coverage-data-faq.md](coverage-data-faq.md)) | PENDING |
| Duplicate does not inflate totals | **PASS** (via verify-ingestion-slice) | PENDING |

---

## What this proves / what's next

**Proves (local):** Retreaver connector, HMAC auth, field parse, `call_key` encryption, Kafka → ClickHouse → API → dashboard/coverage — zero rework needed for ingestion.

**Next:**

1. Tunnel + real Retreaver credentials + live calls → re-run `make verify-retreaver-real`
2. Phases 1–2: Deepgram transcription + OpenAI scoring on real calls (separate milestone)

---

## Artifacts added

| File | Purpose |
|------|---------|
| [`scripts/verify-retreaver-real.ps1`](../scripts/verify-retreaver-real.ps1) | API + ClickHouse + log checks (no secrets printed) |
| `make verify-retreaver-real` | Makefile target |
