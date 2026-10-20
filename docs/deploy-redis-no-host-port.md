# Calyx — deploy note: redis without a host port (hotfix)

**Server:** `calyx.ridgerisemedia.com` · `~/deployments/calyx-ai` · compose project **`deploy`**
**Code:** `main` with `merge: v1 hotfix — redis has no host port` (check the hash Syed sends).
**Why:** after the 2026-10-18 reboot, Plane's `plane-redis` took host port 6379 first; `deploy-redis-1` could not bind
`127.0.0.1:6379` and came up **without a network**, so `redis` did not resolve and every sync failed. The repo no
longer publishes a redis host port; every service reaches it on the compose network as `redis:6379`.

Rules: no secret in chat; never `down -v`; always `-p deploy`; on anything unexpected stop and send the output.
Do **not** touch Plane's containers.

## 1. Check — changes nothing (📤 send output)

```bash
cd ~/deployments/calyx-ai
ENVF=.env   # the real env file; deploy/.env is a symlink to it — never write to deploy/.env directly
[ -f .env ] && [ "$(readlink -f deploy/.env)" = "$(readlink -f .env)" ] && echo "env file: OK (deploy/.env -> .env)" || echo "env file: CHECK - stop"
git status --short                       # must be empty
git fetch origin && git log --oneline -1 origin/main   # the hash Syed sent
ls -l ~/calyx-overrides/redis-no-host-port.yml          # the temporary override (still in use until step 4)
```

Continue only if `env file: OK (deploy/.env -> .env)` and `git status` is empty.

## 2. Pull

```bash
git pull --ff-only origin main
git log --oneline -1                     # the hash Syed sent
grep -n '6379' deploy/docker-compose.yml # must print nothing: redis has no host port any more
```

## 3. Recreate redis, then the services that use it — WITHOUT the override file

From here on, use the normal `$DC` (no `-f ~/calyx-overrides/...`):

```bash
DC="docker compose -p deploy -f deploy/docker-compose.yml"
$DC up -d --no-deps --force-recreate redis
sleep 10
$DC up -d --no-deps --force-recreate platform-sync ingestion
sleep 30
```

The redis data volume is kept (no `-v`), so dedup state survives.

## 4. Prove (📤 send output)

```bash
# redis: on deploy_default with alias "redis", healthy, NO host port
docker inspect deploy-redis-1 --format '{{range $n,$v := .NetworkSettings.Networks}}network={{$n}} aliases={{$v.Aliases}}{{end}}'
echo "redis host ports: [$(docker port deploy-redis-1)]"        # must be []
docker ps --filter name=deploy- --format '{{.Names}}  {{.Status}}  {{.Ports}}'
# expect: deploy-redis-1 Up (healthy) with no port column entry; deploy-ingestion-1 Up (NOT Restarting); platform-sync Up

# platform-sync reaches redis by name
$DC exec -T platform-sync sh -c 'nc -z redis 6379 && echo "platform-sync -> redis:6379 reachable"'

# no more DNS failures, and a successful sync since the recreate (wait up to 5 minutes)
$DC logs --since 10m platform-sync | grep -ci 'lookup redis'        # must be 0
$DC logs --since 10m platform-sync | grep -E '"api sync completed"|"sync lock released"' | tail -5   # at least one "api sync completed"

# sync status is fresh (no secrets)
$DC exec -T postgres psql -U callloom -d callloom -c "SELECT platform, name, sync_status, last_sync_at, left(COALESCE(last_sync_error,''),120) AS last_error FROM integrations WHERE deleted_at IS NULL AND connection_mode = 'api_sync' ORDER BY platform;"
# expect: both rows ok, last_sync_at within the last few minutes
```

## 5. Remove the temporary override

Only after step 4 passed:

```bash
rm ~/calyx-overrides/redis-no-host-port.yml
rmdir ~/calyx-overrides 2>/dev/null || true
```

From now on the normal `$DC` command is enough for every deploy.

## Rollback (only if step 4 fails)

```bash
cd ~/deployments/calyx-ai
DC="docker compose -p deploy -f deploy/docker-compose.yml"
mkdir -p ~/calyx-overrides
printf 'services:\n  redis:\n    ports: !reset []\n' > ~/calyx-overrides/redis-no-host-port.yml
$DC -f ~/calyx-overrides/redis-no-host-port.yml up -d --no-deps --force-recreate redis
sleep 10
$DC -f ~/calyx-overrides/redis-no-host-port.yml up -d --no-deps --force-recreate platform-sync ingestion
```

(The override and the new compose file do the same thing — no redis host port — so rollback only matters if the
pull itself went wrong. Send the output first.)

## What was proven on the laptop before this was merged

- A stand-in container held **host 127.0.0.1:6379** (like Plane's redis). With it running, `deploy-redis-1` was recreated
  from this compose file: Up (healthy), **no host port**, on `deploy_default` with aliases `[deploy-redis-1 redis]`.
- platform-sync and ingestion recreated: health 200, `platform-sync -> redis:6379 reachable`, **0** "lookup redis" lines.
- Full `pwsh-preview -V1` gates with the stand-in still holding 6379: SYSTEM VERIFY PASS (Docker 9/9, Healthz 5/5, API,
  Web routes 13/13, WebSocket, BrowserCSP, unit, web build, pipeline 4/4), PRESENTATION 20/20, INGESTION PASS — including
  "identical resend added ZERO new rows", the content-hash dedup that lives in redis.
