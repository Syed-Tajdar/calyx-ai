# Write-back implementation audit

**Date:** 2026-09-19  
**Mode:** Report only. No code, migration, UI, or endpoint changes in this pass.  
**Purpose:** Inventory unplanned product write-back work so stay / revert / premature can be decided next.

This is **not** a ship report. It does not recommend a path. It records what exists and what a user can reach **today**.

Related (not substitutes): [`ringba-writeback-phase-g-report.md`](ringba-writeback-phase-g-report.md) (research convert proof), [`ringba-writeback-product-slice-report.md`](ringba-writeback-product-slice-report.md) (author scoreboard of the unplanned slice).

---

## 0. Split: research vs product

Two layers exist. Do not mix them when deciding revert.

| Layer | What it is | In product HTTP path? |
|-------|------------|------------------------|
| **Research harness** | `services/platform-sync/cmd/probe-ringba-*` (probe, sandbox, Phase G, etc.) | **No.** Run as one-shot binaries. Predates this slice. |
| **Unplanned product slice** (2026-09-18/19) | Mapping `hasConverted` through the ping pipeline + allowlisted convert APIs | **Yes.** Nest + platform-sync + consumer + deep-view. |

This audit inventories the **product slice** and the two migrations it added. Research probes are listed only so they are not forgotten.

---

## 1. Migrations

Exactly **two** schema files were added for this slice. No other write-back tables or columns.

### 1.1 Postgres `000024_writeback_integration_any`

| | |
|--|--|
| Up | [`db/postgres/migrations/000024_writeback_integration_any.up.sql`](../db/postgres/migrations/000024_writeback_integration_any.up.sql) |
| Down | [`db/postgres/migrations/000024_writeback_integration_any.down.sql`](../db/postgres/migrations/000024_writeback_integration_any.down.sql) |
| Adds columns/tables? | **No.** |
| Adds | `SECURITY DEFINER` function `get_api_sync_integration_any(UUID, UUID)` + `GRANT EXECUTE` to `callloom_sync` |
| Diff vs existing `get_api_sync_integration` | Same return shape. **Omits** `is_active = TRUE`. Still requires `connection_mode = 'api_sync'` and `api_credentials_blob IS NOT NULL`. |

**Applied on this stack (2026-09-19):**

```
schema_migrations.version = 24, dirty = f
pg_proc includes get_api_sync_integration_any
```

**Reversible?** Yes, in-repo: down revokes + `DROP FUNCTION`. Does not touch `integrations` rows. Revert would break `store.GetAPISyncAny` until that Go call is removed.

Ticker list is unchanged: `list_api_sync_integrations()` still filters `is_active = TRUE` ([`000023_initial_lookback_days.up.sql`](../db/postgres/migrations/000023_initial_lookback_days.up.sql)).

### 1.2 ClickHouse `000012_pings_has_converted`

| | |
|--|--|
| Up | [`db/clickhouse/migrations/000012_pings_has_converted.up.sql`](../db/clickhouse/migrations/000012_pings_has_converted.up.sql) |
| Down | **None** (unlike `000011_pings_ringba_parity_b1.down.sql`, which drops B1 columns) |
| SQL | `ALTER TABLE pings ADD COLUMN IF NOT EXISTS has_converted UInt8 DEFAULT 0;` |
| Compose | [`deploy/docker-compose.yml`](../deploy/docker-compose.yml) `clickhouse-init` mounts it as `/init-phase12.sql` and runs it (`\|\| true`) |

**Applied on this stack (2026-09-19):**

```
system.columns: pings.has_converted  type=UInt8  default=0
GROUP BY: has_converted=0 → 4154 rows; no rows with 1
```

**Reversible?** Not via golang-migrate / this repo. Revert would be a new CH `DROP COLUMN` (or leave the default-0 column unused). Existing 4154 rows were ingested **before** adapter mapping; they are all `0` even when Ringba revenue > 0.

### 1.3 Not migrations (ops state)

SQL `UPDATE` on paused **Ringba Live** (not a migration file):

| id | name | connection_mode | is_active | sync_mode |
|----|------|-----------------|-----------|-----------|
| `ce6fd338-af48-47d8-850a-bd10b855e2f4` | Ringba Live | `api_sync` | **false** | **real** |
| `db273021-f9da-444e-89c9-8f037c57f3ad` | Ringba Dev Pixel | `webhook` | true | mock |

Paused + Live is what lets convert use credentials without the 120s pull.

---

## 2. `pings.has_converted` (and other columns)

**No other write-back columns** were added (`conversion_amount`, webhook ids, etc. do not exist on `pings`).

### Populated from

```
Ringba Get Call Log valueColumns includes "hasConverted"
  → mapRingbaCallLogEntry / parseRingbaBool
      absent or empty → false
      present true/1/yes → true
  → RemoteCall.HasConverted
  → Kafka PingMessage.json "has_converted"
  → consumer writer UInt8
  → INSERT pings.has_converted
```

Files: [`ringba_adapter.go`](../services/platform-sync/internal/adapter/ringba_adapter.go) (request + map), [`producer.go`](../services/platform-sync/internal/producer/producer.go), [`syncer.go`](../services/platform-sync/internal/syncer/syncer.go), [`packages/pingdedup/content.go`](../packages/pingdedup/content.go) (hash append), [`consumer/.../ping.go`](../services/consumer/internal/models/ping.go), [`writer.go`](../services/consumer/internal/writer/writer.go), [`clickhouse.go`](../services/consumer/pkg/clickhouse/clickhouse.go).

Content-hash **includes** `HasConverted`. A later live sync that maps `true` would republish even if money fields are unchanged.

### Read today

| Reader | Uses `has_converted`? |
|--------|------------------------|
| [`deep-query.service.ts`](../services/api/src/native-integration/deep-query.service.ts) | **Yes** — `argMax(has_converted, received_at)` + `Number(...) === 1` |
| [`CallDeepViewPanel.tsx`](../web/src/components/native/CallDeepViewPanel.tsx) | **Yes** — label **Converted** Yes/No |
| [`web/src/lib/api.ts`](../web/src/lib/api.ts) | Type only: `has_converted?: boolean` |
| Analytics / pings feed / coverage | **No** grep hits |
| Convert write path | Reads Ringba live `HasConverted` via `LookupCall`, **not** ClickHouse |

Because ticker is paused and pre-mapping rows are all `0`, the UI field is present but **not** reflecting Ringba converted-state on existing calls.

---

## 3. UI changes

**No Settings convert control.** No hidden/disabled convert button. Grep of `web/src` for `writeback` / convert API client: **only** the two files below.

### 3.1 Call deep-view — **Converted** (read)

[`web/src/components/native/CallDeepViewPanel.tsx`](../web/src/components/native/CallDeepViewPanel.tsx) ~205–207, in **Ping / RTB**, after Total call length, before Recording:

```
Converted
Yes | No     (boolean from ping.has_converted; missing → No)
```

**Observed 2026-09-19** on `http://localhost:3001/calls/RGBAA6C57E14B992E47B5DFD1595EC13B4C411F3644RGB7202`:

- Native Deep View → Ping / RTB
- Revenue **$13.00**, Payout **$10.66**, **Converted No**
- No convert / write button on this page

Phase G sandbox `…V3NYI01`: **No data found for this call ID** (not in ClickHouse).

### 3.2 Settings → Integrations

[`web/src/app/(app)/settings/page.tsx`](../web/src/app/(app)/settings/page.tsx) + [`IntegrationsList.tsx`](../web/src/components/settings/IntegrationsList.tsx): **unchanged for write-back** (no new controls).

**Observed 2026-09-19:**

- Tabs: Integrations / Alerts / Providers
- Add-integration form only (Test/Live radios for **new** connects)
- Connected Platforms: Retreaver / TrackDrive / CallGrid webhooks + **Ringba Dev Pixel** (webhook)
- **Ringba Live absent** — `GET /integrations` filters `is_active = TRUE`
- Trash delete on listed rows; **no Convert**

### 3.3 Dashboard

Unrelated to this slice except it still shows the earlier accidental Ringba pull (~4.1k pings). No convert UI.

---

## 4. Endpoints and service code

### 4.1 Nest — JWT product API

[`integrations.controller.ts`](../services/api/src/integrations/integrations.controller.ts) class `@UseGuards(AuthGuard('jwt'))`.

| Route | Handler | Guards (in order) |
|-------|---------|-------------------|
| `POST /integrations/:id/writeback/convert` | `convertWriteback` | JWT; Zod `inbound_call_id` 8–128 chars, `amount` optional positive ≤ 25; then service |

[`integrations.service.ts`](../services/api/src/integrations/integrations.service.ts) `convertWriteback`:

1. `getRowAny` — **no** `is_active` filter (paused OK)
2. `connection_mode === 'api_sync'`
3. `platform === 'ringba'`
4. `sync_mode === 'real'`
5. `POST {PLATFORM_SYNC_URL}/writeback/convert` with `integration_id`, `tenant_id`, `inbound_call_id`, `amount` default 1

Web client: **does not call this route.**

### 4.2 platform-sync — **no auth**

Host bind: `0.0.0.0:8088` ([compose](../deploy/docker-compose.yml)). Same pattern as existing `POST /sync` and `POST /test-auth` (also unauthenticated).

| Route | Auth | Body |
|-------|------|------|
| `POST /writeback/convert` | **None** | `integration_id`, `tenant_id`, `inbound_call_id` required; `amount` optional |

Then [`writeback.Service.Convert`](../services/platform-sync/internal/writeback/convert.go):

| Guard | Effect if fail |
|-------|----------------|
| inbound_call_id empty | error |
| amount ≤ 0 → 1; amount > 25 | refuse |
| `GetAPISyncAny` (paused OK) | not found |
| platform ringba | refuse |
| `sync_mode == real` | refuse |
| decrypt blob | error |
| `LookupCall` 7-day Get Call Log scan (up to 8×1000) | not found / Ringba error |
| already `HasConverted` | refuse |
| allowlist on **call.CampaignID** | refuse |
| enabled conversion webhook exists; prefer name `My Webhook` | refuse |
| `FireInboundConversion` GET `webhook.ringba.com/hook/{id}?call_id=&call_revenue=&converted=yes` | write |
| poll LookupCall ~12s for converted or revenue ≥ amount | error if fire OK but no read-back |

`MOCK_MODE` on platform-sync **does not gate convert**. Per-row `sync_mode` does.

### 4.3 Adapter methods added for write-back

Same file as sync: [`ringba_adapter.go`](../services/platform-sync/internal/adapter/ringba_adapter.go)

| Method | Side effect |
|--------|-------------|
| `LookupCall` | **Read** Get Call Log |
| `ListConversionWebhooks` | **Read** `GET …/WebHooks` |
| `FireInboundConversion` | **Write** inbound conversion GET |

### 4.4 New package

```
services/platform-sync/internal/writeback/allowlist.go
services/platform-sync/internal/writeback/allowlist_test.go
services/platform-sync/internal/writeback/convert.go
services/platform-sync/internal/writeback/convert_test.go   # webhook pick only
```

Config: `WritebackAllowCSV` ← `RINGBA_WRITEBACK_ALLOWED_CAMPAIGN_IDS` ([`config.go`](../services/platform-sync/internal/config/config.go)).

Store: `GetAPISyncAny` ([`integrations.go`](../services/platform-sync/internal/store/integrations.go)).

Wiring: [`cmd/server/main.go`](../services/platform-sync/cmd/server/main.go) parse allowlist, `writeback.NewService`, route, log key `writeback_allowlist_configured`.

### 4.5 Not in product HTTP

- `payments` / `payments/override` / `void` / annotate
- Webhook create/delete
- Retreaver write-back
- Research `cmd/probe-ringba-*`

---

## 5. Allowlist

**Name in logs:** `writeback_allowlist_configured` = `!allow.Empty()` at process start. It is **not** a feature flag for the HTTP route existing. Empty CSV still serves `/writeback/convert` and then **refuses every campaign**.

**Env:** `RINGBA_WRITEBACK_ALLOWED_CAMPAIGN_IDS`  
**Parse:** comma-separated, trim, **case-insensitive** campaign IDs ([`allowlist.go`](../services/platform-sync/internal/writeback/allowlist.go)).

**What it gates:** only `Convert` step `Allows(call.CampaignID)` **after** LookupCall. It does **not** gate sync pull, Settings, or deep-view.

**On this stack:**

| Source | Value |
|--------|--------|
| compose `platform-sync` environment | `CA98abf9d92be446e786f298680a3bbee6` (sandbox `Calyx-Sandbox-DO-NOT-USE`) |
| `.env.example` | commented, same ID |
| live log | `"writeback_allowlist_configured":true` (boot 2026-09-18T18:49:33Z) |

Final Expense / other live campaign IDs are **not** on the list. A convert of a non-sandbox call should 403 after a successful lookup.

Webhook object is **not** allowlisted. If “My Webhook” is missing, code uses the first enabled conversion webhook; campaign allowlist still applies to the **call**.

---

## 6. What is reachable **right now**

### 6.1 From the Calyx UI

**No convert/write action.** No `fetch` to `/writeback/convert` in `web/`. Settings has no control. Deep-view Converted is display-only.

### 6.2 Smokes (2026-09-19, this machine — no Ringba write fired)

| Call | Result | Meaning |
|------|--------|---------|
| `POST :3000/integrations/{Ringba Live id}/writeback/convert` **no JWT** | **401** | Nest route is live; JWT required |
| `POST :3000/.../writeback/convert` JWT + `inbound_call_id: "short"` | **500** | Zod min(8) throws unhandled (not 400). **No** platform-sync / Ringba |
| `POST :3000/integrations/00000000-…/writeback/convert` JWT + fake 19-char id | **404** | Authenticated path reaches `getRowAny` |
| `POST :8088/writeback/convert` `{}` | **400** | platform-sync route is live; no auth |
| `POST :8088/writeback/convert` fake integration UUID + tenant + call id | **404** | Hits `GetAPISyncAny` only (Postgres). **No** webhook fire |

`401`/`400` prove the routes **exist**. They do **not** prove convert cannot fire.

### 6.3 Can a convert **write** fire today?

**Yes, if someone calls the API with the right IDs. No, not from clicking the UI.**

Conditions that are **already true** on this stack:

1. Nest convert exists and accepts JWT (demo admin can obtain a token).
2. platform-sync convert exists on **`:8088` with no auth**.
3. Ringba Live is `sync_mode=real` and `GetAPISyncAny` **ignores pause**.
4. Allowlist contains the sandbox campaign.
5. `FireInboundConversion` is wired.

A write would still need a **clean** sandbox inbound call in the **7-day** lookup window (Phase G `…V3NYI01` is already converted → product would refuse after lookup, and it is not in CH). This audit **did not** send that request.

**Highest-risk reachability:** anyone who can hit `localhost:8088` (compose publishes it) can invoke convert **without JWT**, using known `tenant_id` + `ce6fd338-…` + a sandbox call id. Nest JWT is not on that hop.

Sync ticker: still off (`is_active=false` + `list_api_sync_integrations` filter). Pull is not how convert fires.

---

## 7. File inventory (product slice)

**Write path**

- `services/platform-sync/internal/writeback/*`
- `services/platform-sync/cmd/server/main.go` (route + wiring)
- `services/platform-sync/internal/config/config.go` (`WritebackAllowCSV`)
- `services/platform-sync/internal/store/integrations.go` (`GetAPISyncAny`)
- `services/platform-sync/internal/adapter/ringba_adapter.go` (`LookupCall`, webhooks, fire)
- `services/api/src/integrations/integrations.controller.ts` (convert route)
- `services/api/src/integrations/integrations.service.ts` (`convertWriteback`, `getRowAny`)
- `db/postgres/migrations/000024_writeback_integration_any.{up,down}.sql`
- `deploy/docker-compose.yml` (`RINGBA_WRITEBACK_ALLOWED_CAMPAIGN_IDS`)
- `.env.example` (commented allowlist)

**Read/mapping path** (also used by convert lookup)

- Adapter `hasConverted` column + `HasConverted` on models
- producer / syncer / pingdedup / consumer / CH insert
- `db/clickhouse/migrations/000012_pings_has_converted.up.sql`
- compose `clickhouse-init` phase 12
- `deep-query.service.ts` + `CallDeepViewPanel.tsx` + `web/src/lib/api.ts`

**Docs written during the slice** (not runtime)

- `docs/ringba-writeback-product-slice-report.md`
- `docs/evidence/ringba-writeback-product-slice/slice.txt`
- edits to `README.md`, `docs/README.md`, `docs/enterprise-roadmap.md` §J

**Research only (older)**

- `services/platform-sync/cmd/probe-ringba-*`
- `docs/ringba-writeback-{probe,sandbox,phase-g}-report.md` + evidence dirs

---

## 8. Decision inputs (not decisions)

Facts a stay/revert/premature call can use:

| Item | If reverted, what breaks | If kept |
|------|--------------------------|---------|
| CH `has_converted` + mapping + deep-view field | Converted column / hash field go away; B1 “silent omit” returns | Read path exists; values still all `0` until a live sync |
| Convert APIs + writeback package + 000024 | No product write; probes still can write if run | Writes reachable via API (esp. unauth `:8088`) |
| Allowlist env | N/A if convert gone | Only sandbox campaign ID |
| `sync_mode=real` on paused Ringba Live | Convert would 400 “must be Live” even if APIs stay | Convert unblocked without ticker |
| Docs claiming “shipped” | Misleading if APIs revert | Matches current code, not a live convert re-proof |

**Not proven in product:** end-to-end `POST /writeback/convert` → Ringba `hasConverted=true`. That proof remains Phase G probe on `…V3NYI01`.
