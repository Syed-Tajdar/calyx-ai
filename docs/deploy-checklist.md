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
      testing; the code default is `false`. Do not copy the compose value.
- [ ] **Per-tenant `writeback_enabled`** — only tenants deliberately opened. Default `false`.

## Network exposure

- [ ] **Port `8088` (platform-sync) must not be publicly reachable.** Write-back routes
      require a bearer token, but `/sync` and `/test-auth` are still **unauthenticated**.
      Internal network only.

## Data

- [ ] **Retreaver re-sync** — local ClickHouse currently has 0 real Retreaver calls (lost in
      the machine reset). Re-sync before relying on Retreaver data.
- [ ] Confirm headline KPIs exclude test data (`apisync-`, `demo-`, `smoke-`) on the
      production database.

## Migrations

- [ ] **ClickHouse `000012_pings_has_converted`** has no `down` migration. Add one, or
      document the manual rollback, before production.

## Accounts

- [ ] **Dev login `admin@calyx.ai` / `calyx_dev`** — disable or replace. It is documented in
      the README.

## Verification

- [ ] `verify-system.ps1`, `verify-ingestion-slice.ps1`, `verify-presentation.ps1` all
      exit 0 against the production-configured build.
