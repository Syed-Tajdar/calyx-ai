# Calyx — deploy pack: migration 000037 (function grants) — after the 2D + 2B deploy

**Server:** `calyx.ridgerisemedia.com` · `~/deployments/calyx-ai` · compose project **`deploy`**
**Code:** target = **`6a34fd1`, pinned** (the final main: 000037 plus `TRANSCRIBE_AUTO_SINCE`). The server must
already run **`6a8ad8a`** (2D + 2B, `docs/deploy-v2-2d-2b.md` Steps 0–6(c)) at migration **`36 | f`**. Do not start
before that deploy is finished and checked. Every later step works from this `6a34fd1` checkout **without another
pull** (`docs/aryan-run-order-2026-10-23.md`).
**People:** **Aryan** (server terminal) · **Syed** (browser)

**What this deploy changes:**
- **One SQL migration, `000037_function_grants`:** `REVOKE ... FROM PUBLIC` on 13 older `SECURITY DEFINER` functions,
  then `GRANT EXECUTE` to the one role that calls each. It changes no data and no function body.
- **Effect:** the API and ingestion can no longer call the functions that return the encrypted credential blob
  (`get_api_sync_integration`, `_any`, `get_publisher_pause_integration`). They never did call them; only
  platform-sync does.
- **No image rebuild and no restart.** The `migrate` service applies the SQL file straight from the repo, and the
  grants apply to the next call.
  - `6a34fd1` also holds two code changes that this pack does **not** build:
    - `services/platform-sync/cmd/fake-providers/main.go`: laptop only, and not built into the platform-sync image
      (which builds `./cmd/server` only);
    - the `TRANSCRIBE_AUTO_SINCE` change in `services/api` and `web`: built later by `docs/deploy-v2-2d-2b.md` 6(d),
      the only step that needs it.
  - So `api`, `web` and `platform-sync` keep running the `6a8ad8a` images.
- **Rollback:** 37's own down file restores the old grants exactly; version back to 36.

## Rules

- 🔴 No key, password or `.env` value in chat. Nothing here prints one.
- **Never** `docker compose down`, never `-v`, always `-p deploy`. Do not touch Plane's or trackitin's containers.
- Not between 08:30 and 10:00 PKT.
- On anything unexpected: **stop**, send the output, do not retry on your own.

Use **one terminal**. Every block sets its own variables. If the connection drops, re-run Step 0.

---

## Step 0 — Checks (changes nothing; 📤 send output)

```bash
cd ~/deployments/calyx-ai
DC="docker compose -p deploy -f deploy/docker-compose.yml"
[ -f .env ] && [ "$(readlink -f deploy/.env)" = "$(readlink -f .env)" ] && echo "env file: OK (deploy/.env -> .env)" || echo "env file: CHECK - stop"
git status --short                       # must be empty
git log --oneline -1                     # 6a8ad8a (the 2D + 2B deploy)
git fetch origin
git log --oneline -1 6a34fd1         # the pinned target
git merge-base --is-ancestor HEAD 6a34fd1 && echo "target is ahead: OK" || echo "target does not contain the server commit - stop"
$DC exec -T postgres psql -U callloom -d callloom -c "SELECT version, dirty FROM schema_migrations;"   # 36 | f
docker ps -a --filter name=deploy- --format '{{.Names}}  {{.Status}}' | sort                       # the 9 v1 services Up
# Functions that PUBLIC may run today (expect 13: the older ones this migration fixes)
$DC exec -T postgres psql -U callloom -d callloom -tAc "SELECT count(*) FROM pg_proc p WHERE p.prosecdef AND p.pronamespace = 'public'::regnamespace AND EXISTS (SELECT 1 FROM aclexplode(p.proacl) a WHERE a.grantee = 0);"
```

**Continue only if:** `env file: OK`, `git status` empty, the server is on `6a8ad8a`, `target is ahead: OK`, `36 | f`,
the 9 services are Up, and the count is `13`.

## Step 1 — Backup first

```bash
cd ~/deployments/calyx-ai
DC="docker compose -p deploy -f deploy/docker-compose.yml"
BK=~/calyx-backups/deploy-37-grants
mkdir -p "$BK" && chmod 700 "$BK"
$DC exec -T postgres pg_dump -U callloom -d callloom -Fc > "$BK/callloom-before.dump"
git rev-parse HEAD > "$BK/commit-before.txt"
ls -lh "$BK"      # the dump must be larger than 0; commit-before.txt holds 6a8ad8a…
```

## Step 2 — Pull and migrate to 37 (📤 send output)

```bash
cd ~/deployments/calyx-ai
DC="docker compose -p deploy -f deploy/docker-compose.yml"
BK=~/calyx-backups/deploy-37-grants
git fetch origin && git merge --ff-only 6a34fd1     # pinned; NOT a plain pull
git log --oneline -1                                   # 6a34fd1
ls db/postgres/migrations | tail -2                    # 000037_function_grants.down.sql / .up.sql
git diff --name-only "$(cat $BK/commit-before.txt)" HEAD -- services web packages
# expect: services/api, services/platform-sync (cmd/fake-providers only) and web — none is built in this pack:
#   fake-providers is laptop only; the api/web TRANSCRIBE_AUTO_SINCE change is built by the v2 pack's 6(d)
date -u +%Y-%m-%dT%H:%M:%SZ | tee "$BK/migrated-at.txt"   # the start of the "permission denied" window
$DC up -d --no-deps migrate && echo "migrate exit: $(docker wait deploy-migrate-1)"     # migrate exit: 0
$DC logs migrate 2>&1 | grep '/u '                                                      # 37/u function_grants
$DC exec -T postgres psql -U callloom -d callloom -c "SELECT version, dirty FROM schema_migrations;"   # 37 | f
$DC exec -T postgres psql -U callloom -d callloom -tAc "SELECT count(*) FROM pg_proc p WHERE p.prosecdef AND p.pronamespace = 'public'::regnamespace AND EXISTS (SELECT 1 FROM aclexplode(p.proacl) a WHERE a.grantee = 0);"   # 0
```

The role table (each function runs for exactly one app role; 📤 send it):

```bash
cd ~/deployments/calyx-ai
DC="docker compose -p deploy -f deploy/docker-compose.yml"
$DC exec -T postgres psql -U callloom -d callloom -c "SELECT p.proname AS function,
  has_function_privilege('callloom_api', p.oid, 'EXECUTE') AS api,
  has_function_privilege('callloom_ingestion', p.oid, 'EXECUTE') AS ingestion,
  has_function_privilege('callloom_sync', p.oid, 'EXECUTE') AS sync
  FROM pg_proc p WHERE p.prosecdef AND p.pronamespace = 'public'::regnamespace ORDER BY 1;"
```

**Expect 26 rows, each with exactly one `t`:**
- `api`: `authenticate_user`, `list_transcription_tenants`, `list_workflow_tenants`;
- `ingestion`: `lookup_integration_by_token`;
- `sync`: every other function.

## Step 3 — Checks (📤 send output) — nobody lost access

Wait for one sync tick (2 minutes), then:

```bash
cd ~/deployments/calyx-ai
DC="docker compose -p deploy -f deploy/docker-compose.yml"
BK=~/calyx-backups/deploy-37-grants
T=$(cat "$BK/migrated-at.txt"); echo "since $T"
# 1. Sync tick ok, last_success_at fresh (platform-sync: list / get / update_integration_sync_state)
$DC logs --since "$T" platform-sync | grep -c '"api sync completed"'          # at least 1 (wait up to 5 min)
$DC exec -T postgres psql -U callloom -d callloom -c "SELECT platform, name, sync_status, last_success_at, consecutive_failures FROM integrations WHERE deleted_at IS NULL AND connection_mode = 'api_sync' AND is_active ORDER BY platform;"
# expect: ok, last_success_at within the last few minutes, consecutive_failures 0
# 2. API (login function) and web
curl -s -o /dev/null -w 'api /readyz %{http_code}\n' http://127.0.0.1:3100/readyz                     # 200
for p in /login /settings /alerts; do printf '%-10s ' "web $p"; curl -s -o /dev/null -w '%{http_code}\n' "http://127.0.0.1:3101$p"; done   # 200 each
# 3. Ingestion (token lookup function)
docker exec deploy-ingestion-1 wget -qO- http://127.0.0.1:8080/readyz >/dev/null && echo "ingestion /readyz OK"
```

**Syed, in the browser (Incognito, admin):**
1. **Log in.** That calls `authenticate_user`.
2. **Settings** loads: Integrations, Alerts and Providers.
3. **Ask AI** (`/ask`): ask *"Calls by platform in the last 7 days"*. It must answer as before (the 2D + 2B pack's
   question 2).
4. **Alerts:** the bell and `/alerts` load.
5. **Transcribe:** one converted call that is not transcribed yet → **Transcribe** → Ready. This calls platform-sync's
   claim, recording and transcript functions.

Then the Postgres log since the migration:

```bash
cd ~/deployments/calyx-ai
DC="docker compose -p deploy -f deploy/docker-compose.yml"
BK=~/calyx-backups/deploy-37-grants
T=$(cat "$BK/migrated-at.txt")
$DC logs --since "$T" postgres 2>&1 | grep -ci 'permission denied'            # must be 0
$DC logs --since "$T" postgres 2>&1 | grep -i 'permission denied' | head -5    # must print nothing
$DC exec -T postgres psql -U callloom -d callloom -c "SELECT state, count(*) FROM transcription_jobs WHERE updated_at > '$T' GROUP BY 1;"   # Syed's job: done
```

**Pass:**
- a sync completed since the migration, and `last_success_at` is fresh;
- every page and check is `200` / `OK`;
- Syed's 5 steps work;
- **0** `permission denied`.

**On any `permission denied` line:** run the rollback right away, then send the line to Syed. It names the function that
an unknown caller needed.

**STOP.** Send Steps 0, 2 and 3 to Syed.

---

## Rollback (only if Syed says so, or on any `permission denied`)

```bash
cd ~/deployments/calyx-ai
DC="docker compose -p deploy -f deploy/docker-compose.yml"
BK=~/calyx-backups/deploy-37-grants
# 37's own down file: gives the 13 functions back to PUBLIC exactly as before (nothing else changes)
$DC exec -T postgres psql -U callloom -d callloom -v ON_ERROR_STOP=1 -1 < db/postgres/migrations/000037_function_grants.down.sql
$DC exec -T postgres psql -U callloom -d callloom -c "UPDATE schema_migrations SET version = 36, dirty = false;"
$DC exec -T postgres psql -U callloom -d callloom -c "SELECT version, dirty FROM schema_migrations;"   # 36 | f
$DC exec -T postgres psql -U callloom -d callloom -tAc "SELECT count(*) FROM pg_proc p WHERE p.prosecdef AND p.pronamespace = 'public'::regnamespace AND EXISTS (SELECT 1 FROM aclexplode(p.proacl) a WHERE a.grantee = 0);"   # 13
# Back to the previous checkout: otherwise the next "up" of the migrate service would apply 37 again
git checkout "$(cat $BK/commit-before.txt)"
git status -sb | head -1     # "## HEAD (no branch)": detached on 6a8ad8a
```

- No container is recreated: the code did not change.
- **Detached HEAD:** before the next deploy, run `git checkout main` (it changes files only), then that pack's
  Step 0.

---

## What was proven on the laptop before this was pushed

Laptop: Windows, Docker Desktop 4.88.1, dev `.env`, Test-mode integrations only. There was **no Live integration, no
platform API call and no real key**. The laptop stack ran `main` `6a8ad8a` images with 000037 applied.

- **Throwaway Postgres** (`bash scripts/verify-migration-37-function-grants.sh`):
  - every migration up to 36 → **37** → **down** (grants identical to 36) → **up**;
  - no `SECURITY DEFINER` function executable by PUBLIC;
  - the exact role matrix for **all 26** `SECURITY DEFINER` functions on main (2D's and 2B's included), with a count
    check so an unlisted new function fails;
  - real calls allowed and refused per role.
- **Laptop stack at 37, every flag off:**
  - `verify-system.ps1 -V1`: **SYSTEM VERIFY PASS** (Docker 9/9, Healthz 5/5, LiveStatus, API v1, presentation
    20/20, Web routes 13/13, WebSocket, SmokePrefix, BrowserCSP, unit tests 356/356, web build, pipeline 4/4).
    LiveStatus failed once in a run started 15 s after an API restart: the live-feed Kafka consumer took 27 s to
    rejoin its group. A rerun passed; Postgres is not involved;
  - `verify-ingestion-slice.ps1`: **INGESTION SLICE VERIFY PASS**;
  - `verify-api-sync.ps1`: **PASS**.
- **Admin login** in the browser; **Settings → Integrations → Save integration** (Test-mode Retreaver) → 201.
- **Sync button** → a Test-mode sync tick through `list_api_sync_integrations` / `update_integration_sync_state`:
  `sync_status ok`, fresh `last_success_at`, 0 failures. That integration was paused again afterwards.
- **Flags on, against the laptop fakes** (`deploy/docker-compose.fake-providers.yml`, now also a fake OpenAI):
  - **Ask AI:** a fake OpenAI tool call → `metrics` on ClickHouse → an answer (HTTP 201; `no_data`, because every
    laptop call has a test prefix);
  - **Alerts:** a dry run in Settings → Alerts (200; publisher actions off) and `/alerts` (200);
  - **Settings → Providers:** "Transcribe button only (automatic off)";
  - **Transcribe button:** HTTP 202 → worker `claim` → Test-mode recording → fake S3 (SSE) + the same 8,096 bytes to
    the fake Deepgram (`redact=pci,ssn`) → transcript **Ready** with `[REDACTED]`; then **Play** (200) and the
    **evidence pack** (4 files), each with an audit row `served`.
- **The detector works:** one deliberate call as `callloom_api` to `get_api_sync_integration` logged
  `ERROR: permission denied for function get_api_sync_integration`.
- **Postgres log** from just after that deliberate call (14:07:21 UTC) through every check above and the down/up
  (17:42 UTC): **0 `permission denied`, 0 ERROR lines**.
- **Up / down / up on the real laptop stack:** 37 (0 PUBLIC-executable) → down → `36 | f` (13, the exact old state) →
  up → `37 | f` (0).
