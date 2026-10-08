# Calyx — v1 hardening report

**Branch:** `v1/hardening` from `main` @ `7dc3a66`  
**Login rule:** production startup refuses placeholder service secrets. It does not read, reset, or re-hash `admin@calyx.ai`. The app password stays as it is.

Values below are masked. Nothing in this file is a usable secret.

## B1 — Secret inventory on `main` @ `7dc3a66`

Every value below was a **literal** in `deploy/docker-compose.yml`. Redis had no password.

| Secret | Services | Form on `7dc3a66` | What breaks if it changes |
|---|---|---|---|
| Postgres superuser `callloom` | `postgres`, `migrate` | literal `POSTGRES_PASSWORD` and the migrate URL | `POSTGRES_PASSWORD` applies only to a **new** volume. On this volume the role password stays until `ALTER ROLE`. Migrate then fails until that matches. |
| `callloom_api` | api, providerconfig, compliance-intelligence, ask-ai, ad-attribution | literal inside each `DATABASE_URL` | Those services cannot connect until `ALTER ROLE callloom_api`. Rows stay. |
| `callloom_ingestion` | ingestion | literal inside `DATABASE_URL` | Ingestion cannot connect until `ALTER ROLE`. |
| `callloom_sync` | platform-sync | literal inside `DATABASE_URL` | platform-sync cannot connect until `ALTER ROLE`. |
| `CLICKHOUSE_PASSWORD` | clickhouse, clickhouse-init (every `clickhouse-client --password`), api `CLICKHOUSE_URL`, consumer, scoring, enrichment, compliance-intelligence, ask-ai, ad-attribution | literal | The ClickHouse user password on an existing volume does not follow the env var. Clients fail until `ALTER USER`. |
| `JWT_SECRET` | api | literal | Existing session cookies stop validating. The user row and its password hash are unchanged, so the same login still works and issues a new cookie. |
| `CREDENTIAL_ENCRYPTION_KEY` | api, providerconfig, platform-sync | literal 64-hex dev key | Old `integrations.api_credentials_blob` values do not decrypt. New saves use the new key. The laptop stack must keep the current key. |
| `ANI_ENCRYPTION_KEY` | ingestion, platform-sync | literal, same 64-hex dev key | Old caller-number ciphertext and `call_key` ciphertext do not decrypt. Dashboard reads do not decrypt those columns, so they do not crash. New writes use the new key. |
| `WRITEBACK_INTERNAL_TOKEN` | api, platform-sync | literal dev token | Convert/void forwarding fails if the two services disagree. This is not the app login. |
| `DEEPGRAM_CALLBACK_SECRET` | api | literal dev token | A real Deepgram callback would fail the check. `MOCK_MODE=true` does not require it at boot today. |
| Redis | api, ingestion, platform-sync, providerconfig, compliance-intelligence, ad-attribution, scoring, enrichment | `redis://redis:6379`, no password | Nothing stored is encrypted with this. Adding `requirepass` without the same password in every `REDIS_URL` makes those clients fail. |

`WRITEBACK_ENABLED` is already `${WRITEBACK_ENABLED:-false}`. It is a switch, not a secret.

## B5 — What a new encryption key does to old rows

| Key | Stored where | After the key changes |
|---|---|---|
| `CREDENTIAL_ENCRYPTION_KEY` | Postgres `integrations.api_credentials_blob` | The row stays. List and Settings do not decrypt it. Test connection returns "Re-enter credentials" and does not 500. platform-sync records the same text on `last_sync_error` and continues to the next integration. Convert/void for that row is rejected with the same text. |
| `ANI_ENCRYPTION_KEY` | ClickHouse `pings.call_key` only (hex ciphertext). Caller numbers are stored as `pings.ani_hash`, which is a hash, not ciphertext. The encrypted caller blob is not written to ClickHouse. | Dashboard, pings, and call detail do not decrypt `call_key`, so old rows do not crash. A `call_key` written with the old key cannot be used for a later Retreaver update. |

No migration re-encrypts old blobs. On the server the owner is deleting the 4 dev webhook rows. Those are the only rows whose `call_key` could have been encrypted with the dev key. After they are deleted, nothing on the server needs re-encryption. Any integration saved with the dev key must be entered again. The laptop key is not rotated.
