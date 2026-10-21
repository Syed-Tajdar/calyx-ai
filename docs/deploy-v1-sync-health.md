# Calyx — deploy pack: v1 sync health (Part 1)

**Server:** `calyx.ridgerisemedia.com` · `~/deployments/calyx-ai` · compose project **`deploy`**
**Code:** target = **the new `main` hash after Syed merges `v1/sync-health`** (Syed sends it). It contains
the last code commit `2b65785` of `v1/sync-health`. Until Syed merges it, do nothing.
**People:** **Aryan** (server terminal) · **Syed** (browser, platform reports)
**Timezone for every comparison:** Eastern, `America/New_York`

**Why:** from 2026-10-18 15:03 UTC to 2026-10-21 14:22 UTC every sync failed at the redis sync lock, and
Settings kept showing `ok` with no error. The owner could not see the outage. After this deploy:

- every sync failure (lock, redis, DNS, network, auth, API, incomplete window) writes `degraded`, and
  3 in a row write `failed`, with a short reason in `last_sync_error` (no secrets);
- Settings → Integrations shows **"Last successful sync: <time, Eastern>"** and turns the badge **red** when
  that is older than 30 minutes, whatever the status says;
- ingestion and platform-sync no longer crash-loop or flood the log when redis is gone: one clear error,
  retries with backoff, and they recover by themselves when redis is back;
- Calyx publishes **only** host ports `127.0.0.1:3100` (api) and `127.0.0.1:3101` (web) — what nginx
  needs. postgres 5432, redpanda 9092, clickhouse 8123/9100, ingestion 8080, consumer 8081 and
  platform-sync 8088 are reachable only inside the compose network, so they can never collide with
  Plane (or anything else) after a reboot.

**What changes on the server:** new images for platform-sync, ingestion, api and web; migration
**`000034_sync_health`** (additive: 2 columns, a widened status check, 1 replaced + 1 new function, no
existing row is changed); postgres, redpanda, clickhouse and consumer are recreated once (same images,
same volumes) to drop their host ports. **No env change.** (Optional: `SYNC_FAILED_AFTER`, default 3.)

**Migration numbering:** before 2D is merged, v2/intelligence's 000034_workflows must be renumbered to
000035 when rebasing on main.

Rules: never `docker compose down` (at all), never `-v`, always `-p deploy`, never print secrets (no secret in
chat either). On anything unexpected stop and send the output. Do **not** touch Plane's containers.
Not between 08:30 and 10:00 PKT (the recon before/after must be on the same Eastern day).

Use **one terminal** for the whole pack. If the connection drops, re-run Step 0.1 first.

---

## Step 0 — Before you touch anything (📤 send output)

### 0.1 Check — changes nothing

```bash
cd ~/deployments/calyx-ai
DC="docker compose -p deploy -f deploy/docker-compose.yml"
ENVF=.env   # the real env file; deploy/.env is a symlink to it — never write to deploy/.env directly
[ -f .env ] && [ "$(readlink -f deploy/.env)" = "$(readlink -f .env)" ] && echo "env file: OK (deploy/.env -> .env)" || echo "env file: CHECK - stop"
git status --short                       # must be empty
git fetch origin
git log --oneline -1                     # what runs now (expected: d7efa16, Part 0)
git log --oneline -1 origin/main         # the target: the hash Syed sent
git merge-base --is-ancestor 2b65785 origin/main && echo "target has v1/sync-health: OK" || echo "target is missing v1/sync-health - stop"
$DC exec -T postgres psql -U callloom -d callloom -c "SELECT version, dirty FROM schema_migrations;"   # 33 | f

# Part 0 is complete: no override file, redis has no host port
ls -l ~/calyx-overrides/redis-no-host-port.yml 2>/dev/null && echo "override file STILL THERE - stop" || echo "override file: gone (OK)"
echo "redis host ports: [$(docker port deploy-redis-1)]"        # must be []
docker inspect deploy-redis-1 --format '{{range $n,$v := .NetworkSettings.Networks}}network={{$n}} aliases={{$v.Aliases}}{{end}}'
# expect: network=deploy_default aliases=[... redis ...]
```

**Continue only if:** `env file: OK (deploy/.env -> .env)`, `git status` empty, `origin/main` is the hash Syed sent,
`target has v1/sync-health: OK`, migration `33 | f`, `override file: gone (OK)`, `redis host ports: []`.

### 0.2 Host ports: who listens, who uses them (📤 send output)

This deploy removes the host ports 5432, 9092, 8123, 9100, 8080, 8081 and 8088. Before that, prove that only
Calyx containers hold them and nothing on the host uses them.

```bash
# What listens on those ports now. Expect only docker-proxy lines (one per Calyx port), nothing else:
sudo ss -ltnp | awk 'NR==1 || /:(5432|9092|8123|9100|8080|8081|8088)[[:space:]]/'
# Which containers publish them (expect only deploy-* names):
docker ps --format '{{.Names}}  {{.Ports}}' | grep -E ':(5432|9092|8123|9100|8080|8081|8088)->' | sort
# Open connections from the host to those ports right now (must print nothing):
ss -tn state established '( dport = :5432 or dport = :9092 or dport = :8123 or dport = :9100 or dport = :8080 or dport = :8081 or dport = :8088 )' | tail -n +2
# Host jobs that use them (each must print nothing):
crontab -l 2>/dev/null | grep -nE ':(5432|9092|8123|9100|8080|8081|8088)\b|psql|clickhouse-client'
sudo crontab -l 2>/dev/null | grep -nE ':(5432|9092|8123|9100|8080|8081|8088)\b|psql|clickhouse-client'
sudo grep -rnE ':(5432|9092|8123|9100|8080|8081|8088)\b' /etc/cron.d /etc/cron.daily /etc/cron.hourly 2>/dev/null
# Plane's (and anything else's) published host ports — Syed records these in the repo:
docker ps --format '{{.Names}}  {{.Ports}}' | grep -v '^deploy-' | sort
```

**Stop** if a non-docker-proxy process listens on one of the ports, a non-`deploy-` container publishes one, the
connections line prints anything, or a cron job matches: something on the host still uses a port this deploy
removes. Send the output to Syed.

(The repo side is already checked: nginx proxies only to `127.0.0.1:3100` and `127.0.0.1:3101`, and every
command in the deploy packs runs `psql` / `clickhouse-client` through `docker exec` / `$DC exec`.)

---

## Step 1 — Backups and counts BEFORE (📤 send the counts) — before anything is recreated

```bash
BK=~/calyx-backups/deploy-v1-sync-health
mkdir -p $BK && chmod 700 $BK
$DC exec -T postgres pg_dump -U callloom -d callloom -Fc > $BK/callloom-before.dump
git rev-parse HEAD > $BK/commit-before.txt
ls -lh $BK      # the dump must be larger than 0

# ClickHouse pings rows per platform (raw rows; must be the same or higher after the deploy)
CH='clickhouse-client --password "$CLICKHOUSE_PASSWORD" --database default --multiquery'
echo "SELECT platform, count() AS rows FROM pings GROUP BY platform ORDER BY platform FORMAT TSV;" \
  | $DC exec -T clickhouse sh -c "$CH" | tee $BK/pings-rows-before.tsv

# Then the 7-day and 30-day recon BEFORE (same Eastern day as Step 6)
$DC exec -T clickhouse sh -c "$CH" < ~/calyx-golive/recon-7days.sql  | tee ~/calyx-golive/before-synchealth-7days.txt
$DC exec -T clickhouse sh -c "$CH" < ~/calyx-golive/recon-30days.sql | tee ~/calyx-golive/before-synchealth-30days.txt
```

The recon files are the ones from `docs/deploy-v1-hotfix-and-2a.md` Step 0.2; if they are missing, write them
from that Step first. **Backfill proof (📤):** in the 7-day output, 2026-10-18, 19 and 20 have rows for both
platforms (filled by the 7-day sweep after Part 0). Syed compares them with the Ringba/Retreaver reports. If a
day is still empty, stop and send the output.

## Step 2 — Pull and build (the site stays up)

```bash
git pull --ff-only origin main
git log --oneline -1               # the hash Syed sent
# Only api and web publish host ports now. Prints only service names, never the config (it holds secrets):
$DC config 2>/dev/null | awk '/^services:/{s=1;next} /^[^ ]/{s=0} s&&/^  [a-z0-9-]+:$/{svc=$1} s&&/^    ports:$/{print svc}' | tr -d ':' | sort | tr '\n' ' '; echo
# must print exactly: api web
$DC build api web platform-sync ingestion
```

Nothing is restarted by Step 2.

## Step 3 — Recreate, one service at a time (📤 send output)

**What to expect:** each container is replaced once. The named volumes (Postgres, ClickHouse, Redpanda, redis
data) are **kept** — `up --force-recreate` never touches volumes; never `down`, never `-v`. Expected brief
downtime, about 5–10 minutes in total, a few seconds to a minute per service:

- postgres (~10–30 s): the API, ingestion and platform-sync cannot reach the database; a sync tick in that
  window is recorded as a failure and cleared by the next one;
- clickhouse (~10–30 s): dashboards and Pings answer errors; the consumer waits, nothing is lost (rows stay in
  Redpanda until written);
- redpanda (~20–40 s): publishing pauses; platform-sync retries on its next tick (the cursor only moves on
  success), the API's live feed reconnects;
- consumer, ingestion, platform-sync, api, web: a few seconds each; for api and web the site is down for those
  seconds.

Run this block once. It defines two helpers and does nothing else:

```bash
wait_up() {   # wait_up <service> — up to 3 minutes for running + healthy (or running, no healthcheck)
  local c="deploy-$1-1" s i
  for i in $(seq 1 90); do
    s=$(docker inspect -f '{{.State.Status}} {{if .State.Health}}{{.State.Health.Status}}{{else}}none{{end}} {{.State.Restarting}}' "$c" 2>/dev/null)
    case "$s" in "running healthy false"|"running none false") echo "$1: up ($s)"; return 0;; esac
    sleep 2
  done
  echo "$1: NOT UP ($s) - STOP"; return 1
}
probe() {     # probe <service> <url inside the container> — must answer 200
  local i
  for i in $(seq 1 30); do
    docker exec "deploy-$1-1" wget -qO- "$2" >/dev/null 2>&1 && { echo "$1: $2 OK"; return 0; }
    sleep 2
  done
  echo "$1: $2 FAILED - STOP"; return 1
}
```

Then, **one line at a time**, in this order. After each line, continue only if it ends with `up` / `OK` / `200`:

```bash
# 1. postgres (stateful) — then the migration, before any new code starts
$DC up -d --no-deps --force-recreate postgres && wait_up postgres
$DC exec -T postgres psql -U callloom -d callloom -tAc "SELECT 1"                       # 1
$DC up -d --no-deps migrate && echo "migrate exit: $(docker wait deploy-migrate-1)"     # migrate exit: 0
$DC exec -T postgres psql -U callloom -d callloom -c "SELECT version, dirty FROM schema_migrations;"   # 34 | f

# 2. clickhouse (stateful)
$DC up -d --no-deps --force-recreate clickhouse && wait_up clickhouse
echo "SELECT count() FROM pings FORMAT TSV;" | $DC exec -T clickhouse sh -c "$CH"        # a number

# 3. redpanda (stateful)
$DC up -d --no-deps --force-recreate redpanda && wait_up redpanda

# 4. consumer
$DC up -d --no-deps --force-recreate consumer && wait_up consumer && probe consumer http://127.0.0.1:8081/healthz

# 5. ingestion (new image)
$DC up -d --no-deps --force-recreate ingestion && wait_up ingestion && probe ingestion http://127.0.0.1:8080/readyz

# 6. platform-sync (new image)
$DC up -d --no-deps --force-recreate platform-sync && wait_up platform-sync && probe platform-sync http://127.0.0.1:8088/readyz

# 7. api (new image)
$DC up -d --no-deps --force-recreate api && wait_up api && curl -s -o /dev/null -w 'api /healthz %{http_code}\n' http://127.0.0.1:3100/healthz

# 8. web (new image)
$DC up -d --no-deps --force-recreate web && wait_up web && curl -s -o /dev/null -w 'web /login %{http_code}\n' http://127.0.0.1:3101/login

docker ps -a --filter name=deploy- --format '{{.Names}}  {{.Status}}  {{.Ports}}' | sort
```

Expect at the end: the 9 v1 services **Up** (none `Restarting`), **only** `deploy-api-1` (`127.0.0.1:3100->3000`)
and `deploy-web-1` (`127.0.0.1:3101->3000`) with a host port, `api /healthz 200`, `web /login 200`, migration
`34 | f`. redis is not recreated (Part 0 already removed its host port).

Why api comes after postgres and clickhouse: the API's `/readyz` pins the Postgres IP when the API starts
(`services/api/src/database/postgres.service.ts`). A recreated postgres can get a new IP, and an API that was
not recreated afterwards answers `/readyz` 503 while the site itself keeps working. This order recreates the
API last of the backends, so it starts with the new IPs. (Seen on the laptop on 2026-10-21; it was already so
before this deploy.)

If any line prints `STOP` (or not 200): do not continue with the next service. Send `$DC logs <service> --tail 50`
for that service and the output so far to Syed. Everything already recreated keeps running; the rest still runs
the old containers.

## Step 4 — Checks (📤 send output)

```bash
# restart policy: every v1 service unless-stopped
for c in postgres redis redpanda clickhouse ingestion consumer api platform-sync web; do
  printf '%-16s %s\n' "$c" "$(docker inspect -f '{{.HostConfig.RestartPolicy.Name}}' deploy-$c-1)"
done
# redis: still on deploy_default with alias redis, no host port
docker inspect deploy-redis-1 --format '{{range $n,$v := .NetworkSettings.Networks}}network={{$n}} aliases={{$v.Aliases}}{{end}}'
echo "redis host ports: [$(docker port deploy-redis-1)]"        # must be []

# the new columns and function are there
$DC exec -T postgres psql -U callloom -d callloom -c "\df record_integration_sync_failure"
# sync: a successful sync since the restart, nothing about redis being unavailable
$DC logs --since 10m platform-sync | grep -c '"api sync completed"'                     # at least 1 (wait up to 5 min)
$DC logs --since 10m platform-sync | grep -cE 'lookup redis|"redis unavailable'          # must be 0
$DC logs ingestion --tail 30
$DC exec -T postgres psql -U callloom -d callloom -c "SELECT platform, name, sync_status, last_sync_at, last_success_at, consecutive_failures, left(COALESCE(last_sync_error,''),120) AS last_sync_error FROM integrations WHERE deleted_at IS NULL AND connection_mode = 'api_sync' ORDER BY platform;"
# expect: both rows ok, last_success_at within the last few minutes, consecutive_failures 0, last_sync_error empty

# pings rows per platform: the same or higher than Step 1 (nothing was lost by the recreate)
echo "SELECT platform, count() AS rows FROM pings GROUP BY platform ORDER BY platform FORMAT TSV;" \
  | $DC exec -T clickhouse sh -c "$CH" | tee $BK/pings-rows-after.tsv
join -t $'\t' $BK/pings-rows-before.tsv $BK/pings-rows-after.tsv | awk -F'\t' '{print $1, $2, "->", $3, ($3 >= $2 ? "OK" : "LOWER - STOP")}'

# the site and login (login through the API, a GET only: no form is sent)
for p in /dashboard /login /settings; do printf '%-12s ' "$p"; curl -s -o /dev/null -w '%{http_code}\n' "http://127.0.0.1:3101$p"; done
printf '%-12s ' "api /healthz"; curl -s -o /dev/null -w '%{http_code}\n' http://127.0.0.1:3100/healthz
printf '%-12s ' "public /login"; curl -s -o /dev/null -w '%{http_code}\n' https://calyx.ridgerisemedia.com/login
# expect: 200 for each
```

`last_success_at` is empty until the first successful sync after the deploy (one 2-minute tick): the badge is
red for that first tick, then green. If there is no `"api sync completed"` yet: wait 2 minutes and re-run the
log and SELECT lines (up to 5 minutes in total). After that, stop and send the output.

Browser (Incognito, `https://calyx.ridgerisemedia.com`, admin login): **Settings → Integrations** — each API
integration's Sync column shows a green **ok** badge and **"Last successful sync: Oct 2x, h:mm AM/PM EDT"** a few
minutes old. Dashboard, Pings and Reports open as before.

## Step 5 — Post-reboot check (📤 send output) — run after every server reboot

Not part of today unless the server reboots; keep it for the next one. Within 5 minutes of boot:

```bash
cd ~/deployments/calyx-ai
DC="docker compose -p deploy -f deploy/docker-compose.yml"
docker ps -a --filter name=deploy- --format '{{.Names}}  {{.Status}}  {{.Ports}}' | sort
# expect: all 9 Up (none Restarting); only api 3100 and web 3101 have ports
for c in postgres redis redpanda clickhouse ingestion consumer api platform-sync web; do
  printf '%-16s %s\n' "$c" "$(docker inspect -f '{{.HostConfig.RestartPolicy.Name}}' deploy-$c-1)"
done   # every one unless-stopped
docker inspect deploy-redis-1 --format '{{range $n,$v := .NetworkSettings.Networks}}network={{$n}} aliases={{$v.Aliases}}{{end}}'
$DC logs --since 10m platform-sync | grep -c '"api sync completed"'                     # at least 1 within 5 minutes
$DC exec -T postgres psql -U callloom -d callloom -c "SELECT platform, sync_status, last_success_at FROM integrations WHERE deleted_at IS NULL AND connection_mode = 'api_sync' ORDER BY platform;"
# expect: ok, last_success_at within the last few minutes
curl -s -o /dev/null -w 'api /readyz %{http_code}\n' http://127.0.0.1:3100/readyz        # 200
```

If `api /readyz` is 503 while the site works, postgres came back with a new IP after the API had started
(see Step 3): `$DC up -d --no-deps --force-recreate api`, then check again.

## Step 6 — Reconciliation AFTER (📤 send output) — must match Step 1

Wait **30 minutes** after Step 3, then — **still on the same Eastern day as Step 1** (if midnight ET has passed,
run Step 1's recon again for the new day and compare only days both outputs cover):

```bash
cd ~/deployments/calyx-ai
$DC exec -T clickhouse sh -c "$CH" < ~/calyx-golive/recon-7days.sql  | tee ~/calyx-golive/after-synchealth-7days.txt
$DC exec -T clickhouse sh -c "$CH" < ~/calyx-golive/recon-30days.sql | tee ~/calyx-golive/after-synchealth-30days.txt
diff ~/calyx-golive/before-synchealth-7days.txt ~/calyx-golive/after-synchealth-7days.txt && echo "7-day: IDENTICAL"
```

- **Pass:** every full past day has the same `calls` and `revenue` per platform as before. This deploy does not
  change what is synced or how it is stored. (`diff` may only differ if a platform itself changed a past call
  in those 30 minutes — then compare that day with the Ringba/Retreaver report.)
- 2026-10-18, 19 and 20 still have rows for both platforms.

**STOP.** Send Steps 0, 4 and 6 to Syed.

---

## Rollback (only if Syed says so)

```bash
cd ~/deployments/calyx-ai
DC="docker compose -p deploy -f deploy/docker-compose.yml"
ENVF=.env   # the real env file; deploy/.env is a symlink to it — never write to deploy/.env directly
[ -f .env ] && [ "$(readlink -f deploy/.env)" = "$(readlink -f .env)" ] && echo "env file: OK (deploy/.env -> .env)" || echo "env file: CHECK - stop"
```

Only after `env file: OK (deploy/.env -> .env)`:

```bash
BK=~/calyx-backups/deploy-v1-sync-health
# 1. Step the database back to 33 with the migration's own down file (drops the 2 columns and the new
#    function, restores the old update function and status check, maps any 'failed' status to 'error').
$DC exec -T postgres psql -U callloom -d callloom -v ON_ERROR_STOP=1 -1 < db/postgres/migrations/000034_sync_health.down.sql
$DC exec -T postgres psql -U callloom -d callloom -c "UPDATE schema_migrations SET version = 33, dirty = false;"
# 2. The old code (it publishes the old host ports again, as before this deploy)
git checkout "$(cat $BK/commit-before.txt)"
$DC build api web platform-sync ingestion
$DC up -d --remove-orphans
$DC exec -T postgres psql -U callloom -d callloom -c "SELECT version, dirty FROM schema_migrations;"   # 33 | f
```

The down migration must run **before** the checkout: the old checkout does not have the file. Only the two new
columns are dropped; no other data is touched. The Postgres dump from Step 1 is the last resort (restore only
with Syed).

---

## What was proven on the laptop before this was pushed

Laptop: Windows, Docker Desktop 29.7.2, dev `.env` from `.env.example` (`MOCK_MODE=true`), Test-mode (mock)
integrations only — **no Live integration and no platform API call on the laptop**. No certified Retreaver data
on this laptop (fresh database), so the "3,652 calls / $33,143.85" all-time check could not be run here; the
gates' own checks and the server recon (Steps 1 and 6) cover it.

- **Migration 000034** up → down → up on a throwaway Postgres 16: degraded, degraded, failed after 3 failures;
  one success → ok, error cleared, count 0, `last_success_at` set; down maps `failed` to `error` and drops the
  columns and the function. Applied on the laptop stack: `34 | f`.
- **Unit tests** (all 10 Go modules build, vet, test OK; api jest 221/221). New: redis down → degraded with the
  2026-10-18 error text; 3 in a row → failed; recovery → ok and cleared; `last_success_at` moves only on success;
  no secret in `last_sync_error` (redis/postgres URL passwords, api_key, Bearer token), at most 300 characters.
- **Redis outage** (`scripts/verify-redis-outage.ps1`, 18/18 PASS): redis disconnected from `deploy_default`
  (the name `redis` does not resolve, exactly as on 2026-10-18) → integration `failed` with
  `redis unavailable: dial tcp: lookup redis on 127.0.0.11:53: no such host`, one log line, ingestion Up (also
  after its own restart) and `/readyz` 503; redis reconnected only → `ok`, error cleared, `last_success_at`
  moved, ingestion `/readyz` 200, nothing restarted.
- **Reboot, engine restart (2026-10-21 ~18:40 UTC):** `docker desktop restart` crashed Docker Desktop on the
  laptop; the owner quit and started it again. All 9 Calyx services came back **by their restart policy,
  without a recreate** (created 18:39:4x, started 18:40:17 UTC, restart count 0); kafka-init, migrate and
  clickhouse-init stayed exited (one-shot jobs); after that a Test-mode sync completed and `last_success_at` was
  set, no "lookup redis". In that run the owner saw the stand-in **stopped** after the engine came back (the
  script's own check had reported it running), so that run does **not** prove a race against a running Plane.
- **Reboot, Plane racing (`scripts/verify-reboot-resilience.ps1`, 27/27 PASS):** server compose file only; a
  stand-in with `restart: unless-stopped` held host 6379, 5432, 9092, 8123, 9100, 8080, 8081 and 8088; the whole
  Calyx stack was stopped and started again while the stand-in stayed up: the same 9 containers came back (none
  recreated), each `unless-stopped`, redis on `deploy_default` with aliases `[deploy-redis-1 redis]` and no host
  port, Calyx published only 3100 and 3101, a Test-mode sync completed, ingestion ready. (Docker was not
  restarted for this proof.)
- **Settings:** "Last successful sync: Oct 21, 3:04 PM EDT" under each API integration; an active integration
  whose last success was 45 minutes old showed a **red "Not syncing"** badge while `sync_status` was still `ok`,
  and turned green on the next successful tick; paused integrations are never red.
- **Gates** (Windows PowerShell 5.1 — `pwsh-preview` is the macOS name; `COMPOSE_ENV_FILES=../.env` because the
  `deploy/.env` symlink is a plain file on a Windows checkout): `verify-system.ps1 -V1` **SYSTEM VERIFY PASS**
  (Docker 9/9, Healthz 5/5, LiveStatus, API v1, Web routes 13/13, WebSocket, SmokePrefix, BrowserCSP, unit tests,
  web build, Pipeline 4/4), `verify-ingestion-slice.ps1` **INGESTION SLICE VERIFY PASS** (identical resend added
  zero rows), `verify-api-sync.ps1` **complete**.
