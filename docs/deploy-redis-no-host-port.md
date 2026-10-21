# Calyx — deploy note: redis without a host port (hotfix)

**Server:** `calyx.ridgerisemedia.com` · `~/deployments/calyx-ai` · compose project **`deploy`**
**Code:** target = **the new `main` hash after the docs merge `v1-hotfix: deploy note — redis without a host port`**
(Syed sends it). It contains the compose fix `b077a82` (`merge: v1 hotfix — redis has no host port`).
**Why:** after the 2026-10-18 reboot, Plane's `plane-redis` took host port 6379 first; `deploy-redis-1` could not bind
`127.0.0.1:6379` and came up **without a network**, so `redis` did not resolve and every sync failed. The repo no
longer publishes a redis host port; every service reaches it on the compose network as `redis:6379`.

**Current state:** until this deploy is done, the override file `~/calyx-overrides/redis-no-host-port.yml` is **not**
in use, because it was never applied. Sync has been down since **2026-10-18 15:03 UTC** (last successful sync).

Rules: never `docker compose down` (at all), never `-v`, always `-p deploy`, never print secrets (no secret in chat
either). On anything unexpected stop and send the output. Do **not** touch Plane's containers.

## 1. Check — changes nothing (📤 send output)

```bash
cd ~/deployments/calyx-ai
ENVF=.env   # the real env file; deploy/.env is a symlink to it — never write to deploy/.env directly
[ -f .env ] && [ "$(readlink -f deploy/.env)" = "$(readlink -f .env)" ] && echo "env file: OK (deploy/.env -> .env)" || echo "env file: CHECK - stop"
git status --short                       # must be empty
git fetch origin && git log --oneline -1 origin/main   # the new main hash Syed sent (after the docs merge)
git merge-base --is-ancestor b077a82 origin/main && echo "target has the redis fix b077a82: OK" || echo "target is missing b077a82 - stop"
ls -l ~/calyx-overrides/redis-no-host-port.yml 2>/dev/null || echo "no override file (expected: it was never applied)"
```

Continue only if `env file: OK (deploy/.env -> .env)`, `git status` is empty, `origin/main` is the hash Syed sent and
`target has the redis fix b077a82: OK`.

## 2. Pull

```bash
git pull --ff-only origin main
git log --oneline -1                     # the new main hash Syed sent
DC="docker compose -p deploy -f deploy/docker-compose.yml"
# redis must have no host port. This looks ONLY at the redis service of the resolved compose config,
# so comments and the REDIS_URL lines of other services do not matter. It prints only the two verdict lines.
# Never run "$DC config" without the pipe: the resolved config contains secrets.
RB=$($DC config 2>/dev/null | awk '/^  redis:$/{f=1;next} f && !/^   /{f=0} f')
echo "$RB" | grep -q '^    image:' && echo "redis service found" || echo "redis service NOT found - stop"
echo "$RB" | grep -q '^    ports:' && echo "redis host ports: FOUND - stop" || echo "redis host ports: none (OK)"
unset RB
```

Continue only if `redis service found` and `redis host ports: none (OK)`.

## 3. Recreate redis, then the services that use it — WITHOUT any override file

From here on, use the normal `$DC` (no `-f ~/calyx-overrides/...`):

```bash
DC="docker compose -p deploy -f deploy/docker-compose.yml"
$DC up -d --no-deps --force-recreate redis
sleep 15
$DC up -d --no-deps --force-recreate platform-sync ingestion
sleep 150
date -u                                  # note this time: the "After 60 minutes" section counts from here
```

The redis data volume is kept (no `-v`), so dedup state survives.

## 4. Prove (📤 send output)

```bash
# redis: on deploy_default with alias "redis", healthy, NO host port
docker inspect deploy-redis-1 --format '{{range $n,$v := .NetworkSettings.Networks}}network={{$n}} aliases={{$v.Aliases}}{{end}}'
echo "redis host ports: [$(docker port deploy-redis-1)]"        # must be []
docker ps -a --filter name=deploy- --format '{{.Names}}  {{.Status}}  {{.Ports}}'
# expect: deploy-redis-1 Up (healthy) with no port column entry; deploy-ingestion-1 Up (NOT Restarting);
# deploy-platform-sync-1 Up; nothing Exited or Restarting

# platform-sync reaches redis by name
$DC exec -T platform-sync sh -c 'nc -z redis 6379 && echo "platform-sync -> redis:6379 reachable"'

# no more DNS failures, and a successful sync since the recreate
$DC logs --since 10m platform-sync | grep -ci 'lookup redis'        # must be 0
$DC logs --since 10m platform-sync | grep -E '"api sync completed"|"sync lock released"' | tail -5   # at least one "api sync completed"

# ingestion: Up, not Restarting, and no redis errors in its log
docker ps -a --filter name=deploy-ingestion --format '{{.Names}}  {{.Status}}'   # must say Up, not Restarting
$DC logs ingestion --tail 30

# sync status is fresh (no secrets)
$DC exec -T postgres psql -U callloom -d callloom -c "SELECT platform, name, sync_status, last_sync_at, left(COALESCE(last_sync_error,''),120) AS last_sync_error FROM integrations WHERE deleted_at IS NULL AND connection_mode = 'api_sync' ORDER BY platform;"
# expect: both rows ok, last_sync_at within the last few minutes, last_sync_error empty
```

If there is no `"api sync completed"` yet: wait 2 minutes and re-run the log and SELECT lines (retry for up to
5 minutes in total). After that, stop and send the output.

## 5. Remove the temporary override (if it exists)

Only after step 4 passed:

```bash
rm -f ~/calyx-overrides/redis-no-host-port.yml
rmdir ~/calyx-overrides 2>/dev/null || true
```

From now on the normal `$DC` command is enough for every deploy.

## 6. After 60 minutes — backfill proof (📤 send output)

The **7-day sweep** (Ringba and Retreaver, 7 days back) runs on an hourly timer that starts when platform-sync
starts. It does **not** run at startup: the first sweep begins **60 minutes after step 3** and can take up to
30 minutes. It re-fetches the missed days 2026-10-18, 19 and 20.

```bash
cd ~/deployments/calyx-ai
DC="docker compose -p deploy -f deploy/docker-compose.yml"
# the sweep finished: one line per integration (2), and no sweep failure
$DC logs --since 100m platform-sync | grep '"sync lock released"' | grep -c '"reason":"sweep"'   # expect at least 2
$DC logs --since 100m platform-sync | grep -c '"ringba sweep failed"'                            # must be 0
```

If the first count is still below 2, wait 15 minutes and re-run it (up to 90 minutes after step 3).

Then run the 7-day recon. The query is the one from `docs/deploy-v1-hotfix-and-2a.md` Step 0.2. If
`~/calyx-golive/recon-7days.sql` is already there, skip the `cat` block; otherwise this writes it identically:

```bash
mkdir -p ~/calyx-golive && cd ~/calyx-golive
[ -f recon-7days.sql ] || cat > recon-7days.sql <<'SQL'
SELECT platform,
       toDate(toTimeZone(event_at, 'America/New_York')) AS day_et,
       count() AS calls,
       round(sumIf(last_revenue, last_dup = 0), 2) AS revenue,
       countIf(last_conv = 1) AS converted_flag,
       countIf(last_conv = 1 AND last_revenue > 0) AS converted_paid
FROM (
  SELECT platform, external_call_id,
         argMax(revenue, received_at) AS last_revenue,
         argMax(platform_duplicate, received_at) AS last_dup,
         argMax(has_converted, received_at) AS last_conv,
         max(if(occurred_at > toDateTime(0), occurred_at, received_at)) AS event_at
  FROM pings
  WHERE tenant_id = '11111111-1111-1111-1111-111111111111'
    AND external_call_id != ''
    AND external_call_id NOT LIKE 'apisync-%'
    AND external_call_id NOT LIKE 'demo-%'
    AND external_call_id NOT LIKE 'smoke-%'
  GROUP BY platform, external_call_id
)
WHERE toDate(toTimeZone(event_at, 'America/New_York')) >= toDate(now(), 'America/New_York') - 7
  AND toDate(toTimeZone(event_at, 'America/New_York')) <  toDate(now(), 'America/New_York')
GROUP BY platform, day_et
ORDER BY platform, day_et
FORMAT PrettyCompactMonoBlock;
SQL
cd ~/deployments/calyx-ai
CH='clickhouse-client --password "$CLICKHOUSE_PASSWORD" --database default --multiquery'
$DC exec -T clickhouse sh -c "$CH" < ~/calyx-golive/recon-7days.sql | tee ~/calyx-golive/after-redis-fix-7days.txt
```

**Expected:** 2026-10-18, 2026-10-19 and 2026-10-20 now have rows for **both** platforms. Syed compares those days
(Eastern) with the Ringba and Retreaver reports.

**If those days are still empty after the first sweep finished:** there is no manual trigger in this note. Wait for
the next hourly 7-day sweep (it runs every 60 minutes after step 3: at +60, +120, +180 minutes, each taking up to
30 minutes), then re-run the two log counts with `--since 200m` (expect at least 4 sweep lines and 0 failures) and
the recon above. If 2026-10-18, 19 and 20 are still empty after the
**second** sweep finished (about 150 minutes after step 3), stop and send all outputs to Syed. Do not restart
platform-sync to "speed it up": a restart resets the hourly timer.

## Rollback (only if step 4 fails)

```bash
cd ~/deployments/calyx-ai
DC="docker compose -p deploy -f deploy/docker-compose.yml"
mkdir -p ~/calyx-overrides
printf 'services:\n  redis:\n    ports: !reset []\n' > ~/calyx-overrides/redis-no-host-port.yml
$DC -f ~/calyx-overrides/redis-no-host-port.yml up -d --no-deps --force-recreate redis
sleep 15
$DC -f ~/calyx-overrides/redis-no-host-port.yml up -d --no-deps --force-recreate platform-sync ingestion
sleep 150
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
- 2026-10-21 (docs only, no code or compose change): the step 2 redis-ports check was tested against the resolved
  compose config: `none (OK)` on `b077a82`, `FOUND - stop` on the file before the fix, `NOT found - stop` when the
  config cannot be read. The pwsh-preview gates were not re-run for this docs change; the server deploy is the proof.
