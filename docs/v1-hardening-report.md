# Calyx — v1 hardening report

**Branch:** `v1/hardening` from `main` @ `7dc3a66`  
**Login rule:** production startup refuses placeholder service secrets. It does not read, reset, or re-hash `admin@calyx.ai`. The app password stays as it is.

Values below are masked. Nothing in this file is a usable secret.

## Scoreboard

| Item | Result | Proof |
|---|---|---|
| A1 | DONE | With show-demo and the v1 platform flag, platform mix is Ringba 25 and Retreaver 10. The metric total is 35. Call Loom, CallGrid, and TrackDrive stay in the table and drop out of All. |
| A2 | DONE | `web/src/lib/ping-series.test.ts`: 7 days and 30 days fill through today, empty days are 0, and the point total equals the source count. |
| A3 | DONE in the repo | `Feature-Policy` is removed. `Permissions-Policy` keeps recognised features. CSP is unchanged. Local Login, Dashboard, Pings, and Settings responses have no `Feature-Policy` header. The browser gate reported zero console errors. The live nginx file still needs the Part D reload. |
| A4 | DONE | With the v1 flag on, Dashboard and call detail do not call scoring, ask-ai, compliance, enrichment, or attribution. |
| A5 | DONE | Dashboard subtitle no longer says "and intelligence". The sidebar group label is Analytics. The logo tagline is unchanged. |
| B1 | DONE | Inventory below. |
| B2 | DONE | Compose secrets are `${VAR:-dev default}`. `deploy/.env.example` is tracked. `.env` stays ignored. |
| B3 | DONE | Redis `requirepass` and every `REDIS_URL` use the same variable. Go `ParseURL` and the Node client accept a password in the URL. |
| B4 | DONE | Each B1 variable has a test that fails the start check and names the variable. API cookies are `Secure` when `APP_ENV=production`. The admin password is not part of the check. |
| B5 | DONE | Findings below. Rehearsal dummy integration returned "Re-enter credentials" after the key change and did not 500. |
| B6 | DONE | Profile `intelligence`. Without it, `docker compose ps` shows api, web, ingestion, consumer, platform-sync, postgres, redis, clickhouse, redpanda. Login, metrics, pings, call detail, and settings returned 200. |
| C | DONE | Rehearsal log below. Laptop Retreaver all-time after the gates: 3,652 calls / $33,143.85. |

## Rehearsal log

The laptop project `deploy` was stopped with `stop`, never `down`. `calyx-rehearsal` used its own volumes.

1. Started `7dc3a66` with the dev literals. Migration 31, dirty false. Demo pings were present. Login returned 201. A dummy Retreaver integration was saved with fake credentials (`sync_mode=mock`).
2. Switched that project to `v1/hardening` with a new git-ignored env file (`chmod 600`). `ALTER ROLE` for `callloom`, `callloom_api`, `callloom_ingestion`, and `callloom_sync`. ClickHouse picked up the new password when its container was recreated. Redis `requirepass` was set, then the container was recreated. `APP_ENV=production`. Intelligence containers were stopped. Images for api, web, platform-sync, ingestion, and consumer were rebuilt.
3. Postgres was not wiped: 5 integration rows, including the dummy, and 53 pings remained. Schema stayed 31.
4. Login with the same admin account returned 201. Both session cookies were `Secure`.
5. From another container, each old dev password was rejected and each new password was accepted. Redis without a password returned `NOAUTH`. ClickHouse with the old dev password was rejected.
6. The dummy integration's test returned `ok=false` and "Re-enter credentials".
7. `docker compose ps` after the intelligence stop listed only the v1 services. Every published port was `127.0.0.1`.
8. `calyx-rehearsal` was stopped. Its volumes were kept.
9. The laptop project was started again with its existing env file. The encryption key in that file was not changed. Retreaver all-time was 3,652 / $33,143.85 before and after the gates. "My test" is still `api_sync`, `mock`, `is_active=false`, and the credential blob is still 84 bytes.

Gates on the restored laptop stack, new images, same data: `verify-system.ps1 -V1` exit 0, `verify-presentation.ps1 -V1` exit 0 (19/19, POST /ask skipped), `verify-ingestion-slice.ps1` exit 0. That script has no `-V1` switch. The default view stayed 3,652 through the ingestion gate.

A browser login against the rehearsal while `WEB_ORIGINS` was only the public https host failed CORS from `http://localhost:3101`. The API login still returned 201. On the server the browser origin is that https host, so CORS matches. Secure cookies are stored because the site is HTTPS.

## Part D — Server deploy note

Run this on the server, from the repo root. Do not paste any generated value into chat. Give Syed a copy of the new `.env`. If that file is lost, stored platform credentials cannot be decrypted.

```bash
DC="docker compose -p deploy -f deploy/docker-compose.yml"
cd /path/to/calyx-ai

git log -1 --oneline
git status

mkdir -p "$HOME/calyx-backups"
$DC exec -T postgres pg_dump -U callloom -d callloom > "$HOME/calyx-backups/callloom-$(date +%Y%m%d).sql"
cp .env "$HOME/calyx-backups/env-$(date +%Y%m%d).bak"
chmod 600 "$HOME/calyx-backups/env-$(date +%Y%m%d).bak"
```

Generate each value on the server. `openssl rand -hex 32` is 64 hex characters, which is what the encryption keys require.

```bash
umask 077
cp .env .env.new
# For each name below, run: openssl rand -hex 32
# Write the result as NAME=value in .env.new. Leave no blanks.
# POSTGRES_PASSWORD POSTGRES_API_PASSWORD POSTGRES_INGESTION_PASSWORD POSTGRES_SYNC_PASSWORD
# CLICKHOUSE_PASSWORD JWT_SECRET CREDENTIAL_ENCRYPTION_KEY ANI_ENCRYPTION_KEY
# WRITEBACK_INTERNAL_TOKEN DEEPGRAM_CALLBACK_SECRET REDIS_PASSWORD
# Then set:
# APP_ENV=production
# WEB_ORIGINS=https://calyx.ridgerisemedia.com
# COOKIE_DOMAIN=
# WRITEBACK_ENABLED=false
# COMPOSE_PROFILES=
# TRUSTED_PROXIES=$(docker network inspect deploy_default -f '{{(index .IPAM.Config 0).Gateway}}')
mv .env.new .env
chmod 600 .env
```

`NEXT_PUBLIC_API_URL` and `NEXT_PUBLIC_WS_URL` stay the public URLs already used by the web build. Do not point them at localhost.

```bash
git fetch origin
git checkout v1/hardening
git pull --ff-only

set -a
source .env
set +a

$DC exec -T postgres psql -U callloom -d callloom -c "ALTER ROLE callloom PASSWORD '${POSTGRES_PASSWORD}'"
$DC exec -T postgres psql -U callloom -d callloom -c "ALTER ROLE callloom_api PASSWORD '${POSTGRES_API_PASSWORD}'"
$DC exec -T postgres psql -U callloom -d callloom -c "ALTER ROLE callloom_ingestion PASSWORD '${POSTGRES_INGESTION_PASSWORD}'"
$DC exec -T postgres psql -U callloom -d callloom -c "ALTER ROLE callloom_sync PASSWORD '${POSTGRES_SYNC_PASSWORD}'"
```

Do not run `ALTER USER` on ClickHouse. The user is XML-backed and the statement fails. The new password is applied when the clickhouse container is recreated below.

```bash
$DC exec -T redis redis-cli CONFIG SET requirepass "${REDIS_PASSWORD}"

$DC build api web platform-sync ingestion consumer
$DC up -d --remove-orphans
$DC stop scoring ask-ai compliance-intelligence enrichment ad-attribution providerconfig
```

`--remove-orphans` does not stop a container that is still named in the compose file under a profile. The `stop` line is what leaves only the v1 services running. It does not delete volumes.

Nginx still sends the old `Feature-Policy` header until this file is installed. Copy `deploy/nginx/calyx.ridgerisemedia.com.conf` over the live site file. Delete any `add_header Feature-Policy` line. The `Permissions-Policy` line is now only recognised features. Do not edit the `Content-Security-Policy` line. Then:

```bash
sudo nginx -t && sudo systemctl reload nginx
```

Verify:

```bash
$DC exec -T api printenv APP_ENV
$DC exec -T api printenv WRITEBACK_ENABLED
$DC exec -T postgres psql -U callloom -d callloom -c "SELECT version, dirty FROM schema_migrations;"
$DC ps
docker ps --filter name=deploy- --format '{{.Names}} {{.Ports}}'
$DC exec -T redis redis-cli PING
```

`APP_ENV` is `production`. `WRITEBACK_ENABLED` is `false`. Migration is 31 and dirty is false. `ps` shows api, web, ingestion, consumer, platform-sync, postgres, redis, clickhouse, and redpanda. Every published port starts with `127.0.0.1`. Redis `PING` without a password returns `NOAUTH`.

Confirm one old dev password fails from the docker network, then log in as `admin@calyx.ai` in the browser at `https://calyx.ridgerisemedia.com`. The password is unchanged.

Rollback:

```bash
cp "$HOME/calyx-backups/env-$(date +%Y%m%d).bak" .env
chmod 600 .env
git checkout 7dc3a66
set -a
source .env
set +a
$DC exec -T postgres psql -U callloom -d callloom -c "ALTER ROLE callloom PASSWORD '${POSTGRES_PASSWORD}'"
$DC exec -T postgres psql -U callloom -d callloom -c "ALTER ROLE callloom_api PASSWORD '${POSTGRES_API_PASSWORD}'"
$DC exec -T postgres psql -U callloom -d callloom -c "ALTER ROLE callloom_ingestion PASSWORD '${POSTGRES_INGESTION_PASSWORD}'"
$DC exec -T postgres psql -U callloom -d callloom -c "ALTER ROLE callloom_sync PASSWORD '${POSTGRES_SYNC_PASSWORD}'"
$DC up -d
```

Use the backup `.env` from before this deploy, so the `ALTER ROLE` passwords match that file. ClickHouse and Redis follow the same backup file on recreate. If the app still cannot connect, restore the SQL backup into a new volume only after Syed agrees. Do not `down -v` the current volume to try again.

## Found, not fixed

- The live server nginx is unchanged until Part D is applied. The repo file is updated.
- `verify-ingestion-slice.ps1` has no `-V1` parameter. It was run as written.
- A production rehearsal on `http://localhost` cannot complete a browser login when `WEB_ORIGINS` is only the https site. The API login still works. The server site is https.
- ClickHouse `ALTER USER` cannot change the `default` user. Part D recreates the container instead.
- Compose does not stop profile containers that are already running. Part D stops those six by name.
- The top search box still has no `id` or `name`. CSP still blocks `eval`. Both were left as they are.
- `stage4/r3` is still `f1215f5`. `main` is still `7dc3a66`. Rehearsal volumes were not deleted.


## B1 — Secret inventory on `main` @ `7dc3a66`

Every value below was a **literal** in `deploy/docker-compose.yml`. Redis had no password.

| Secret | Services | Form on `7dc3a66` | What breaks if it changes |
|---|---|---|---|
| Postgres superuser `callloom` | `postgres`, `migrate` | literal `POSTGRES_PASSWORD` and the migrate URL | `POSTGRES_PASSWORD` applies only to a **new** volume. On this volume the role password stays until `ALTER ROLE`. Migrate then fails until that matches. |
| `callloom_api` | api, providerconfig, compliance-intelligence, ask-ai, ad-attribution | literal inside each `DATABASE_URL` | Those services cannot connect until `ALTER ROLE callloom_api`. Rows stay. |
| `callloom_ingestion` | ingestion | literal inside `DATABASE_URL` | Ingestion cannot connect until `ALTER ROLE`. |
| `callloom_sync` | platform-sync | literal inside `DATABASE_URL` | platform-sync cannot connect until `ALTER ROLE`. |
| `CLICKHOUSE_PASSWORD` | clickhouse, clickhouse-init (every `clickhouse-client --password`), api `CLICKHOUSE_URL`, consumer, scoring, enrichment, compliance-intelligence, ask-ai, ad-attribution | literal | The user `default` is defined in XML. `ALTER USER` fails with `ACCESS_STORAGE_READONLY`. Recreating the container rewrites `/etc/clickhouse-server/users.d/default-user.xml` from this env var. The data volume stays. |
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
