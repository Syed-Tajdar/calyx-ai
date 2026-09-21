# Ringba Write-Back — Product Slice Report (`hasConverted` + allowlisted convert)

**Date:** 2026-09-18 → 2026-09-19  
**Scope:** Ringba only. **No TrackDrive / CallGrid.** No write-back **UI**.  
**Prior gate:** Phase G convert loop **PASS** — [`ringba-writeback-phase-g-report.md`](ringba-writeback-phase-g-report.md)  
**Evidence:** [`docs/evidence/ringba-writeback-product-slice/slice.txt`](evidence/ringba-writeback-product-slice/slice.txt)  
**Roadmap:** enterprise-roadmap **§J** — mapping + allowlisted API shipped; write UI still deferred.

This file is the hand-off for an independent review (Claude / human). Confirm or reject each scoreboard row against the code and evidence. Do **not** treat known gaps in §8 as new bugs unless the code contradicts this report.

---

## 1. Scoreboard (reviewer fills CONFIRMED / REJECTED)

| ID | Claim | Author | Reviewer |
|----|--------|--------|----------|
| **S1** | Adapter requests `hasConverted` on Get Call Log and maps **absent → false**, **present true → true** | PASS | |
| **S2** | Flag flows adapter → Kafka `has_converted` → consumer → ClickHouse `pings.has_converted` | PASS | |
| **S3** | Content-hash includes `HasConverted` (false vs true hashes differ) | PASS | |
| **S4** | Deep-view reads `argMax(has_converted, received_at)` and UI shows **Converted Yes/No** | PASS | |
| **S5** | Convert writes use Phase G path only: inbound **GET** `webhook.ringba.com/hook/{id}?call_id=&call_revenue=&converted=yes` | PASS | |
| **S6** | Campaign allowlist is **fail-closed** (`RINGBA_WRITEBACK_ALLOWED_CAMPAIGN_IDS`; empty = refuse all) | PASS | |
| **S7** | Compose allowlist is sandbox campaign `CA98abf9d92be446e786f298680a3bbee6` only | PASS | |
| **S8** | Convert requires `platform=ringba` + `sync_mode=real`; paused ticker (`is_active=false`) is allowed via `get_api_sync_integration_any` | PASS | |
| **S9** | No Settings convert/write button shipped | PASS | |
| **S10** | This slice did **not** fire the product convert API against Ringba (Phase G call already converted) | PASS | |
| **S11** | Sync ticker stayed off after ship (`Ringba Live` `is_active=false`) | PASS | |

**Author verdict:** slice **SHIPPED (API + read display)**. Convert **loop in product** is coded but **not live-proven on a new clean call** in this slice — Phase G remains the live convert proof.

---

## 2. Why this slice existed

Phase G proved convert + read-back on a **clean** sandbox call. Product was not shipped because:

1. Sync adapter **silent-omitted** `hasConverted` unless requested (B1 leftover).
2. Three write paths exist (`payments` relative, `payments/override` absolute, inbound webhook). Product must pick one.
3. Writes must be **campaign-allowlisted**.

This slice did (1) + (3) and locked convert to the **proven inbound GET webhook**. Payments / override / void stay research-only.

---

## 3. Decisions (up front)

| Decision | Choice | Why |
|----------|--------|-----|
| Convert write path | Inbound GET webhook (Phase G) | Only path with `hasConverted=true` + `$1.00` on a clean call |
| Webhook pick | Prefer enabled conversion hook named **My Webhook**; else first enabled conversion hook | Do not auto-create webhooks |
| Allowlist | Fail-closed CSV of campaign IDs | Empty env must refuse, not “allow all” |
| Amount | Default `$1`, max **$25** | Sandbox cap; avoids large accidental writes |
| Already converted | Refuse | Phase G / dirty-call lesson — HTTP 200 empty body is not a convert signal; do not reconvert |
| Paused credentials | `GetAPISyncAny` (no `is_active` filter) | Convert must not require turning the 120s pull back on |
| Live gate | `sync_mode=real` still required | Operator must opt into Live before Ringba writes |
| UI | Read **Converted** only | No write button this slice |
| Dedup | Append `HasConverted` to content-hash | Converted-state change must republish |

---

## 4. Mapping pipeline

```
Ringba POST /v2/{accountId}/calllogs  valueColumns includes hasConverted
  → mapRingbaCallLogEntry: parseRingbaBool (absent/empty → false)
  → RemoteCall.HasConverted
  → Kafka PingMessage.has_converted
  → consumer writer → ClickHouse pings.has_converted UInt8
  → deep-query argMax(has_converted, received_at)
  → CallDeepViewPanel "Converted" Yes/No
```

**Semantics (unchanged from Phase G):**

| Get Call Log | Calyx `has_converted` |
|--------------|------------------------|
| key absent / empty | `false` / `0` |
| present `true` | `true` / `1` |
| present `false` | `false` / `0` |

Do **not** infer converted from `conversionAmount` / revenue alone. A `$13` Ringba call ingested **before** this mapping still stores `has_converted=0`.

---

## 5. Convert API (no UI)

### Nest (auth JWT)

`POST /integrations/:id/writeback/convert`

```json
{ "inbound_call_id": "<Ringba inboundCallId>", "amount": 1 }
```

`amount` optional, positive, max 25.

Looks up the integration **including paused rows**. Proxies to platform-sync.

### platform-sync

`POST /writeback/convert`

```json
{
  "integration_id": "<uuid>",
  "tenant_id": "<uuid>",
  "inbound_call_id": "<id>",
  "amount": 1
}
```

Flow:

1. Decrypt credentials via `get_api_sync_integration_any`
2. Require ringba + `sync_mode=real`
3. `LookupCall` in a **7-day** Get Call Log window (paginate, match `inboundCallId`)
4. Refuse if already `HasConverted`
5. `Allows(campaignId)` — fail-closed allowlist
6. List enabled conversion webhooks; pick **My Webhook** else first
7. GET fire (20s timeout)
8. Poll read-back up to ~12s until `hasConverted` or revenue ≥ amount
9. Else error: fire returned ok but call did not convert on read-back

HTTP mapping (platform-sync): allowlist → 403; not found → 404; validation / already converted / not Live → 400; webhook/read-back → 502.

### Env

| Key | Dev compose | Empty means |
|-----|-------------|-------------|
| `RINGBA_WRITEBACK_ALLOWED_CAMPAIGN_IDS` | `CA98abf9d92be446e786f298680a3bbee6` | **Refuse all writes** |

---

## 6. Files touched (review these)

### Mapping + hash + store

- `services/platform-sync/internal/adapter/ringba_adapter.go` — request column, map, `LookupCall`, `ListConversionWebhooks`, `FireInboundConversion`
- `services/platform-sync/internal/adapter/ringba_adapter_test.go`
- `services/platform-sync/internal/models/models.go`
- `services/platform-sync/internal/producer/producer.go`
- `services/platform-sync/internal/syncer/syncer.go`
- `packages/pingdedup/content.go` + `content_test.go`
- `services/consumer/internal/models/ping.go`
- `services/consumer/internal/writer/writer.go`
- `services/consumer/pkg/clickhouse/clickhouse.go`

### Write-back service

- `services/platform-sync/internal/writeback/allowlist.go` + `_test.go`
- `services/platform-sync/internal/writeback/convert.go` + `_test.go` (webhook pick)
- `services/platform-sync/internal/store/integrations.go` — `GetAPISyncAny`
- `services/platform-sync/internal/config/config.go` — `WritebackAllowCSV`
- `services/platform-sync/cmd/server/main.go` — `POST /writeback/convert`

### API / UI (read + API write, no button)

- `services/api/src/integrations/integrations.controller.ts`
- `services/api/src/integrations/integrations.service.ts` — `convertWriteback` + `getRowAny`
- `services/api/src/native-integration/deep-query.service.ts`
- `web/src/lib/api.ts` — `has_converted?: boolean`
- `web/src/components/native/CallDeepViewPanel.tsx` — Converted Yes/No

### Migrations / ops

- `db/clickhouse/migrations/000012_pings_has_converted.up.sql`
- `db/postgres/migrations/000024_writeback_integration_any.up.sql` + `.down.sql`
- `deploy/docker-compose.yml` — clickhouse-init 000012 + allowlist env
- `.env.example`

---

## 7. Tests + deploy evidence

| Check | Result |
|-------|--------|
| `go test ./internal/writeback ./internal/adapter` (platform-sync, docker golang 1.22) | **ok** (adapter ~81s) |
| `go build ./cmd/server` (platform-sync) | **ok** |
| `go test ./...` (`packages/pingdedup`) | **ok** |
| Postgres migrate `000024_writeback_integration_any` | **applied** |
| ClickHouse `pings.has_converted` | **UInt8** present |
| Rebuild + recreate `api` `platform-sync` `web` `consumer` | **ok** |
| `GET :8088/healthz` | **ok** |
| platform-sync log `writeback_allowlist_configured=true` | **yes** |
| `POST :3000/integrations/:id/writeback/convert` unauthenticated | **401** |
| `POST :8088/writeback/convert` empty body | **400** |
| Browser Settings → Integrations | **no convert button**; **Ringba Live absent** (paused) |
| Browser call deep-view | **Converted** field present |

Unit coverage is **mapping + allowlist + webhook pick**. There is **no** integration test that fires Ringba convert through the product HTTP path.

---

## 8. Runtime state after ship (do not “fix” unless asked)

| Item | State |
|------|--------|
| Integration `ce6fd338-af48-47d8-850a-bd10b855e2f4` (“Ringba Live”) | `is_active=false`, **`sync_mode=real`** |
| `list_api_sync_integrations()` | still `is_active=TRUE` only — ticker **off** |
| Nest `GET /integrations` | paused rows hidden — Ringba Live **not in Settings list** |
| ClickHouse | **4154** ping rows, **all** `has_converted=0` (ingested **before** mapping) |
| UI sample `…RGB7202` | Revenue **$13**, **Converted No** — expected until next live sync republishes mapped flag |
| Phase G sandbox `…V3NYI01` | **Not in ClickHouse**; already converted on Ringba; product convert **must refuse** |

`sync_mode=real` was set by SQL on the paused row so convert can use Live credentials without unpausing. Nest `PATCH /integrations/:id/sync-mode` still requires `is_active=TRUE`, so the UI cannot flip a paused row to Live.

---

## 9. Known gaps (not in this slice)

1. **Product convert not live-fired.** Need a **new clean** sandbox call (never converted / never adjusted) on campaign `CA98abf9…`, then `POST /integrations/:id/writeback/convert`. Do not reuse `…V3NYI01`.
2. **`LookupCall` is a 7-day scan**, not an inboundCallId filter. Calls older than 7 days or beyond 8×1000 pages will 404.
3. **No backfill** of `has_converted` for rows already in ClickHouse.
4. **`writebackHTTPStatus` uses substring matching** (`required`, `webhook`, …) — brittle.
5. Fire uses a dedicated `http.Client` (20s); does **not** share the Ringba dual-window limiter (single convert is fine; do not batch-fire).
6. Fallback “first enabled conversion webhook” could fire a non-sandbox hook if **My Webhook** is missing — allowlist still blocks non-sandbox **calls**, not the webhook object.
7. `payments` / `payments/override` / void **not** in product.
8. Write UI **not** shipped.

---

## 10. Safety held

| Rule | Outcome |
|------|---------|
| Research-only writes on Phase G call this slice | **Held** — product API not fired |
| No payments / void / annotate product path | **Held** |
| No webhook create/delete | **Held** |
| Campaign allowlist fail-closed | **Held** (unit + compose sandbox ID) |
| Tokens / ANI not printed | **Held** |
| Sync ticker left paused | **Held** |

---

## 11. Review brief (paste this to Claude)

You are reviewing Calyx AI Ringba write-back **product slice** dated 2026-09-18/19.

**Read first:** `docs/ringba-writeback-product-slice-report.md` (this file) and Phase G `docs/ringba-writeback-phase-g-report.md`.

**Then review the file list in §6.** Confirm or reject scoreboard **S1–S11**. For each REJECTED row, cite file + reason.

**Confirm these invariants:**

1. Convert cannot run if allowlist env is empty.
2. Convert cannot run if `sync_mode != real`.
3. Convert **can** run when `is_active=false` (paused).
4. Convert refuses already-converted calls.
5. Amount > 25 is refused.
6. Adapter does not treat missing `hasConverted` as true.
7. Content-hash changes when `HasConverted` flips.
8. No Settings UI control calls `/writeback/convert`.
9. `list_api_sync_integrations` still filters `is_active=TRUE`.

**Do not demand** a live convert fire on `…V3NYI01`. **Do not** treat ClickHouse all-zeros `has_converted` as a mapping bug — those rows predate the column mapping.

**Out of scope:** TrackDrive, CallGrid, Deepgram, write UI, payments/override/void.

Return: scoreboard table with CONFIRMED/REJECTED, any real defects, and whether the slice matches the report.
