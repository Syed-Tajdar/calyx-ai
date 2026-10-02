# Calyx — Production Deploy Checklist

Items that are correct for local development but **must change before production**.
Each was found during write-back and pre-Phase-2 work. Do not deploy with any unchecked.

## Secrets and keys

- [ ] **`CREDENTIAL_ENCRYPTION_KEY`** — generate a real random key; load from the secrets
      manager. The committed value (`0123456789abcdef` ×4) is a public dev key.
- [ ] **`ANI_ENCRYPTION_KEY`** — same as above.
- [ ] **`WRITEBACK_INTERNAL_TOKEN`** — real random value; never
      `dev_writeback_internal_token_change_me`.
- [ ] **Re-enter platform credentials in production.** Credentials stored locally are
      encrypted with the dev key. They cannot be decrypted with the production key and must
      not be migrated — each tenant re-saves them in Settings. (Related: enterprise roadmap
      Gap B, key rotation.)

## Feature flags

- [ ] **`WRITEBACK_ENABLED`** — set explicitly in production. Local compose sets `true` for
      testing; the code default is `false`. Do not copy the compose value. Leave it `false`
      on the server until a live write is approved.
- [ ] **`WRITEBACK_SETTLE_SECONDS`** — seconds platform-sync waits for Ringba to confirm a
      convert or cancel. Default **20** when unset. 12 was tight against an observed ~10.4s
      read-back.
- [ ] **`RINGBA_RATE_PER_MIN`** — per-account Ringba budget. Default **75**. Ringba's ceiling
      is 80/min; the gap is for wall-clock jitter.
- [ ] **Before migration `000029`**, run this read-only query and decide which
      `is_active = false` rows are old deletes. Do not guess in a migration. It returns
      tenant, platform, name, and `updated_at` only.

      ```sql
      SELECT t.name AS tenant, i.platform, i.name, i.updated_at
      FROM integrations i
      JOIN tenants t ON t.id = i.tenant_id
      WHERE NOT i.is_active
      ORDER BY i.updated_at;
      ```
- [ ] **Per-tenant `writeback_enabled`** — only tenants deliberately opened. Default `false`.

## Network exposure

- [ ] **Port `8088` (platform-sync) must not be publicly reachable.** All mutating routes
      (`/sync`, `/test-auth`, `/sync/window`, write-back) require the internal bearer token.
      Internal network only.

## Infrastructure

- [ ] Webhook ingestion (`ingestion`, :8080) is NOT exposed publicly in v1 — internal network
      only. v1 connects platforms by API key.
- [ ] Set `COOKIE_DOMAIN` to the parent domain, `WEB_ORIGINS` to the web origin(s),
      `APP_ENV=production`, and the trusted-proxy setting for client IPs.
- [ ] HTTPS everywhere — session cookie is `Secure` in production; HSTS enabled.
- [ ] **Kubernetes probes** — readiness (`/readyz`) must depend only on what is needed
      to serve requests (Postgres, ClickHouse). Kafka / live-feed health is monitored via
      `/live-status`, never wired into readiness or liveness.
- [ ] **Kubernetes `terminationGracePeriodSeconds` ≥ 30s for api, consumer, platform-sync —
      shutdown handlers are bounded to 25s.**
- [ ] **DLQ replay (procedure only)** — poison pings land on `callloom.pings.v1.dlq`
      (retention ≥ 30d). To replay one message: (1) `rpk topic consume callloom.pings.v1.dlq`
      and identify the record; (2) fix the root cause (schema / producer); (3) re-produce the
      original value to `callloom.pings.v1` with the original key (headers
      `x-source-*` / `x-poison-reason` are metadata only). Do not commit past DLQ until the
      produce to the primary topic succeeds. A dedicated replay tool is out of scope (S1B).

## Data

- [ ] **Data retention (TTL)** — `pings` rows expire at `received_at + 24 months`.
      Decide the retention period with management (billing disputes, customer history),
      and whether it should key on `occurred_at` (call time) instead of `received_at`
      (last sync time — every republish resets it).
- [ ] **Retreaver re-sync** — local ClickHouse currently has 0 real Retreaver calls (lost in
      the machine reset). Re-sync before relying on Retreaver data.
- [ ] **Retreaver pagination** — the Retreaver adapter pages with `page`/`per_page`
      over a changing `updated_at` window: the same pattern that silently skipped 431
      Ringba calls. Apply the windowed approach before re-enabling Retreaver sync.
- [ ] **Ringba deep-sweep range (default 30 days)** — confirm against the dispute window
      in buyer contracts; raise `RINGBA_DEEP_SWEEP_DAYS` if contracts allow longer.
- [ ] Confirm headline KPIs exclude test data (`apisync-`, `demo-`, `smoke-`) on the
      production database.

## Migrations

- [x] **ClickHouse `000012_pings_has_converted`** down migration added
      (`000012_pings_has_converted.down.sql`).

## Accounts

- [ ] **Dev login `admin@calyx.ai` / `calyx_dev`** — disable or replace. It is documented in
      the README.

## Verification

- [ ] `verify-system.ps1`, `verify-ingestion-slice.ps1`, `verify-presentation.ps1` all
      exit 0 against the production-configured build.
