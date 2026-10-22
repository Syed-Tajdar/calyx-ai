# Calyx — deploy pack: v2 2D (workflows / alerts) + 2B (transcription + recording archive) — flags OFF, then on one by one

**Server:** `calyx.ridgerisemedia.com` · `~/deployments/calyx-ai` · compose project **`deploy`**
**Code:** target = **the `main` hash Syed sends** (the merge of `v2/2b-transcription`, which contains `v2/2d-on-main`,
plus the `TRANSCRIBE_AUTO` / `TRANSCRIBE_BACKFILL` switches). The server now runs `5cdb46e` (or `4181c4a`), migration 34.
**People:** **Aryan** (server terminal) · **Syed** (browser, platform reports, secrets at hidden prompts)
**Timezone for every comparison:** Eastern, `America/New_York`

**What this deploy changes:**
- **Images:** `api`, `web`, `platform-sync`. ingestion, consumer, postgres, redis, redpanda and clickhouse are **not**
  rebuilt or recreated.
- **Migrations:** `000035_workflows`, then `000036_transcription`. Both are additive: new tables and functions, plus two
  nullable columns on `integrations`. No existing row is changed.
- **The transcriber** is a goroutine **inside `platform-sync`**. There is **no new container, no compose profile and no
  new host port**. The compose file only gains env lines for `api` and `platform-sync`. The `intelligence` profile stays
  off (`COMPOSE_PROFILES` empty). Only `127.0.0.1:3100` (api) and `127.0.0.1:3101` (web) stay published.
- **Flags:** every flag ships **off**. Step 6 turns them on one at a time, with a check after each.
- **Not in this deploy:** migration `000037` (function grants). It ships tomorrow, after 35/36.

## Rules for everyone

- 🔴 No API key, password, bucket key or `.env` value in chat, WhatsApp or email. Secrets are typed only at a **hidden
  prompt** (`read -rs`). Nothing in this pack prints one.
- **Never** `docker compose down` (at all), never `-v`, never delete volumes, always `-p deploy`. Never
  `docker desktop restart`.
- **Do not touch** Plane's or trackitin's containers, files or ports.
- **Not between 08:30 and 10:00 PKT**: the recon before and after must fall on the same Eastern day.
- Write-back (`WRITEBACK_ENABLED`) is not changed. `WORKFLOW_ACTIONS_ENABLED=false` and
  `WORKFLOW_RINGBA_PAUSE_VERIFIED=false` the whole night: **no publisher is paused**.
- On anything unexpected: **stop**, send the output, do not retry on your own.

Use **one terminal** for the whole pack. Every block still starts by setting its own variables (last time `$BK` was empty
in a new terminal), so a block can be re-run after a dropped connection. If the connection drops, re-run Step 0.1 first.

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
git log --oneline -1                     # what runs now: 5cdb46e (or 4181c4a)
git log --oneline -1 origin/main         # the target: the hash Syed sent
git merge-base --is-ancestor HEAD origin/main && echo "target is ahead of the server: OK" || echo "target does not contain the server commit - stop"
$DC exec -T postgres psql -U callloom -d callloom -c "SELECT version, dirty FROM schema_migrations;"   # 34 | f

# Flags and switches (none of these is a secret; keys are only reported as set / empty)
grep -E '^(APP_ENV|V1_ONLY|WRITEBACK_ENABLED|FEATURE_[A-Z_]+|NEXT_PUBLIC_FEATURE_[A-Z_]+|WORKFLOW_[A-Z_]+|TRANSCRIBE_[A-Z_]+|RECORDING_RETENTION_MONTHS|DEEPGRAM_BASE_URL|ARCHIVE_S3_REGION|COMPOSE_PROFILES)=' "$ENVF" | sort
for k in OPENAI_API_KEY DEEPGRAM_API_KEY ARCHIVE_S3_ENDPOINT ARCHIVE_S3_BUCKET ARCHIVE_S3_KEY_ID ARCHIVE_S3_SECRET_KEY; do
  grep -q "^$k=." "$ENVF" && echo "$k: set" || echo "$k: empty"
done
echo "shell COMPOSE_PROFILES=[${COMPOSE_PROFILES:-}]"     # must be []
docker ps --format '{{.Names}}' | grep -E '^deploy-' | sort | paste -sd ' ' -
# expect the 9 v1 services: deploy-api-1 deploy-clickhouse-1 deploy-consumer-1 deploy-ingestion-1
#   deploy-platform-sync-1 deploy-postgres-1 deploy-redis-1 deploy-redpanda-1 deploy-web-1  (no ask-ai, scoring, …)
```

**Continue only if:**
- `env file: OK (deploy/.env -> .env)`, `git status` empty, `target is ahead of the server: OK`, migration `34 | f`;
- `APP_ENV=production`;
- every `FEATURE_*` and `NEXT_PUBLIC_FEATURE_*` line that exists is `false` (a missing line also means off);
- no `COMPOSE_PROFILES` line or an empty one, and `shell COMPOSE_PROFILES=[]`;
- **no `DEEPGRAM_BASE_URL` line** (it is for the laptop fake only, and production refuses to start with it);
- the 9 v1 containers and nothing else `deploy-*`.

---

## Step 1 — Backup and the 7-day recon BEFORE (📤 send the counts and the recon)

Run this as **one block**. Every variable it uses is set inside it.

```bash
cd ~/deployments/calyx-ai
DC="docker compose -p deploy -f deploy/docker-compose.yml"
CH='clickhouse-client --password "$CLICKHOUSE_PASSWORD" --database default --multiquery'
BK=~/calyx-backups/deploy-v2-2d-2b
GL=~/calyx-golive
mkdir -p "$BK" "$GL" && chmod 700 "$BK"
$DC exec -T postgres pg_dump -U callloom -d callloom -Fc > "$BK/callloom-before.dump"
git rev-parse HEAD > "$BK/commit-before.txt"
ls -lh "$BK"      # the dump must be larger than 0; commit-before.txt holds 5cdb46e… (or 4181c4a…)

# ClickHouse pings rows per platform (must be the same or higher after the deploy)
echo "SELECT platform, count() AS rows FROM pings GROUP BY platform ORDER BY platform FORMAT TSV;" \
  | $DC exec -T clickhouse sh -c "$CH" | tee "$BK/pings-rows-before.tsv"

# 7-day recon BEFORE (same Eastern day as Step 5)
[ -s "$GL/recon-7days.sql" ] && echo "recon file: OK" || echo "recon file: MISSING - stop"
$DC exec -T clickhouse sh -c "$CH" < "$GL/recon-7days.sql" | tee "$GL/before-2d2b-7days.txt"
```

If `recon file: MISSING`, write it from `docs/deploy-v1-hotfix-and-2a.md` Step 0.2 first, then run this block again.

---

## Step 2 — Pull and build (the site stays up)

```bash
cd ~/deployments/calyx-ai
DC="docker compose -p deploy -f deploy/docker-compose.yml"
git pull --ff-only origin main
git log --oneline -1                       # the hash Syed sent
ls db/postgres/migrations | tail -4        # ends with 000035_workflows.* and 000036_transcription.* — no 000037

# Which images changed since the server's commit (expect services/api, services/platform-sync and web; no
# services/ingestion, no services/consumer):
git diff --name-only "$(cat ~/calyx-backups/deploy-v2-2d-2b/commit-before.txt)" HEAD -- services web packages \
  | awk -F/ '{ print ($1=="web" ? "web" : $1"/"$2) }' | sort -u

# Only api and web publish host ports. Prints only service names, never the config (it holds secrets):
$DC config 2>/dev/null | awk '/^services:/{s=1;next} /^[^ ]/{s=0} s&&/^  [a-z0-9-]+:$/{svc=$1} s&&/^    ports:$/{print svc}' | tr -d ':' | sort | paste -sd ' ' -
# must print exactly: api web

$DC build api web platform-sync
```

- If the ports check prints an empty line, the config could not be read (env file): stop.
- The repo also holds `deploy/docker-compose.fake-providers.yml` and `services/platform-sync/cmd/fake-providers`. They are
  **laptop only**: they are never used here and are not in any image.
- The web image is built with every `NEXT_PUBLIC_FEATURE_*` off, because the web flags are build-time. Step 6 rebuilds
  web each time a web flag changes.
- Nothing is restarted by Step 2.

---

## Step 3 — Migrate to 35 and 36, then recreate in a safe order (📤 send output)

**Order and why:**
- The migration runs first, while the old code still runs. The migrations are additive, so the old code is unaffected.
- Then `platform-sync`, `api`, `web`, one at a time.
- postgres is **not** recreated, so the API's Postgres IP does not change (the `/readyz` pinning note in
  `docs/deploy-v1-sync-health.md` Step 3 does not apply).
- **Expected downtime:** a few seconds per service. For api and web the site is down for those seconds.

```bash
cd ~/deployments/calyx-ai
DC="docker compose -p deploy -f deploy/docker-compose.yml"
wait_up() {   # wait_up <service> — up to 3 minutes for running + healthy (or running, no healthcheck)
  local c="deploy-$1-1" s i
  for i in $(seq 1 90); do
    s=$(docker inspect -f '{{.State.Status}} {{if .State.Health}}{{.State.Health.Status}}{{else}}none{{end}} {{.State.Restarting}}' "$c" 2>/dev/null)
    case "$s" in "running healthy false"|"running none false") echo "$1: up ($s)"; return 0;; esac
    sleep 2
  done
  echo "$1: NOT UP ($s) - STOP"; return 1
}
probe() {     # probe <service> <url inside the container> — must answer 2xx
  local i
  for i in $(seq 1 30); do
    docker exec "deploy-$1-1" wget -qO- "$2" >/dev/null 2>&1 && { echo "$1: $2 OK"; return 0; }
    sleep 2
  done
  echo "$1: $2 FAILED - STOP"; return 1
}
```

Then, **one line at a time**. After each line, continue only if it ends with `up` / `OK` / `200` / the expected value:

```bash
# 1. migrations 35 and 36 (one run applies them in order; each is its own transaction)
$DC up -d --no-deps migrate && echo "migrate exit: $(docker wait deploy-migrate-1)"     # migrate exit: 0
$DC logs migrate 2>&1 | grep -E '/u '                                                   # 35/u workflows, 36/u transcription
$DC exec -T postgres psql -U callloom -d callloom -c "SELECT version, dirty FROM schema_migrations;"   # 36 | f

# 2. platform-sync (new image; the transcriber inside it stays off)
$DC up -d --no-deps --force-recreate platform-sync && wait_up platform-sync && probe platform-sync http://127.0.0.1:8088/readyz
$DC logs platform-sync 2>&1 | grep -E '"msg":"transcription"|publisher pause routes' | tail -2
# expect: "enabled":false ... and "workflow_actions_enabled":false

# 3. api (new image)
$DC up -d --no-deps --force-recreate api && wait_up api && curl -s -o /dev/null -w 'api /healthz %{http_code}\n' http://127.0.0.1:3100/healthz
curl -s -o /dev/null -w 'api /readyz %{http_code}\n' http://127.0.0.1:3100/readyz        # 200

# 4. web (new image)
$DC up -d --no-deps --force-recreate web && wait_up web && curl -s -o /dev/null -w 'web /login %{http_code}\n' http://127.0.0.1:3101/login

docker ps -a --filter name=deploy- --format '{{.Names}}  {{.Status}}  {{.Ports}}' | sort
```

**Expect at the end:**
- the 9 v1 services **Up**, none `Restarting`;
- **only** `deploy-api-1` (`127.0.0.1:3100->3000`) and `deploy-web-1` (`127.0.0.1:3101->3000`) with a host port;
- `api /healthz 200`, `api /readyz 200`, `web /login 200`, migration `36 | f`.

If a line prints `STOP` (or not 200), or migrate does not exit 0, or the version is dirty: do not continue. Send
`$DC logs <service> --tail 50` and the output so far to Syed.

---

## Step 4 — Secrets and settings, every flag still OFF (📤 send the "set / empty" lines only)

**Before you start (Syed):** the B2 bucket is **private**, with default encryption (SSE-B2) on. The application key is
restricted to **this one bucket** with read, write and delete. For the 24-month deletion to really free space, set the
bucket lifecycle to **"Keep only the last version of the file"**: B2 keeps old versions by default [INFERRED from B2
defaults; Syed to confirm in the B2 console].

Syed types each value at a hidden prompt on Aryan's terminal (or reads it to nobody: Syed sits at the terminal for this
step). Nothing is echoed, stored in the shell history or printed.

```bash
cd ~/deployments/calyx-ai
ENVF=.env
[ -f .env ] && [ "$(readlink -f deploy/.env)" = "$(readlink -f .env)" ] && echo "env file: OK (deploy/.env -> .env)" || echo "env file: CHECK - stop"
umask 077
set_secret() {   # set_secret <KEY> — hidden prompt; replaces the line; prints only "set" or why not
  local k="$1" v
  read -rs -p "$k (hidden): " v; echo
  case "$v" in ""|*[[:space:]]*|*'$'*|*'"'*|*"'"*|*'#'*) echo "$k: NOT set (empty, or has a space, \$, quote or #) - ask Syed"; unset v; return 1;; esac
  grep -v "^$k=" "$ENVF" > "$ENVF.new" && printf '%s=%s\n' "$k" "$v" >> "$ENVF.new" && mv "$ENVF.new" "$ENVF" && chmod 600 "$ENVF"
  unset v; echo "$k: set"
}
set_env() {      # set_env <KEY> <value> — for non-secret values (printed)
  grep -v "^$1=" "$ENVF" > "$ENVF.new" && printf '%s=%s\n' "$1" "$2" >> "$ENVF.new" && mv "$ENVF.new" "$ENVF" && chmod 600 "$ENVF"
  echo "$1=$2"
}

# Secrets, one by one (Syed types each):
set_secret DEEPGRAM_API_KEY
set_secret ARCHIVE_S3_ENDPOINT      # e.g. https://s3.<region>.backblazeb2.com  (with https://)
set_secret ARCHIVE_S3_BUCKET
set_secret ARCHIVE_S3_KEY_ID        # B2 keyID
set_secret ARCHIVE_S3_SECRET_KEY    # B2 applicationKey
grep -q '^OPENAI_API_KEY=.\{20,\}' "$ENVF" && echo "OPENAI_API_KEY: already set" || set_secret OPENAI_API_KEY

# The region is part of the endpoint; B2 signs requests per region. Only the region is printed:
R=$(grep '^ARCHIVE_S3_ENDPOINT=' "$ENVF" | sed -E 's#^ARCHIVE_S3_ENDPOINT=https://s3\.([a-z0-9-]+)\.backblazeb2\.com/?$#\1#')
case "$R" in ARCHIVE_S3_ENDPOINT=*|"") echo "endpoint is not https://s3.<region>.backblazeb2.com - stop";; *) set_env ARCHIVE_S3_REGION "$R";; esac; unset R

# Non-secret settings:
set_env RECORDING_RETENTION_MONTHS 24
set_env TRANSCRIBE_DAILY_MINUTES_CAP 800
set_env TRANSCRIBE_MIN_SECONDS 20
set_env TRANSCRIBE_SHOULD_CONVERT false
set_env TRANSCRIBE_AUTO false
set_env TRANSCRIBE_BACKFILL false
set_env WORKFLOW_ACTIONS_ENABLED false
set_env WORKFLOW_RINGBA_PAUSE_VERIFIED false
# Every flag explicitly off:
for f in FEATURE_ASK_AI NEXT_PUBLIC_FEATURE_ASK_AI FEATURE_WORKFLOWS NEXT_PUBLIC_FEATURE_WORKFLOWS FEATURE_TRANSCRIPTION NEXT_PUBLIC_FEATURE_TRANSCRIPTION; do set_env "$f" false; done

# Check (no values of secrets):
for k in DEEPGRAM_API_KEY ARCHIVE_S3_ENDPOINT ARCHIVE_S3_BUCKET ARCHIVE_S3_KEY_ID ARCHIVE_S3_SECRET_KEY OPENAI_API_KEY; do
  grep -q "^$k=." "$ENVF" && echo "$k: set" || echo "$k: EMPTY - stop"
done
grep -c '^DEEPGRAM_BASE_URL=' "$ENVF"     # 0
ls -l "$ENVF" | cut -c1-10                # -rw-------
```

The containers only read the env file when they are created. Nothing changes until Step 6 recreates them, so Step 5
checks still run the flags-off state.

---

## Step 5 — Checks with every flag off (📤 send output)

```bash
cd ~/deployments/calyx-ai
DC="docker compose -p deploy -f deploy/docker-compose.yml"
CH='clickhouse-client --password "$CLICKHOUSE_PASSWORD" --database default --multiquery'
BK=~/calyx-backups/deploy-v2-2d-2b
GL=~/calyx-golive

# Closed routes (anonymous; 404 = closed before auth)
for p in /ask/status /workflows/rules /workflows/alerts /transcription/usage /scoring/deepgram/callback; do
  printf '%-30s ' "api $p"; curl -s -o /dev/null -w '%{http_code}\n' "http://127.0.0.1:3100$p"; done      # 404 each
for p in /ask /alerts; do printf '%-30s ' "web $p"; curl -s -o /dev/null -w '%{http_code}\n' "http://127.0.0.1:3101$p"; done   # 404 each
for p in /dashboard /login /settings; do printf '%-30s ' "web $p"; curl -s -o /dev/null -w '%{http_code}\n' "http://127.0.0.1:3101$p"; done  # 200 each
printf '%-30s ' "public /login"; curl -s -o /dev/null -w '%{http_code}\n' https://calyx.ridgerisemedia.com/login           # 200

# Sync: a successful sync since the restart, fresh last_success_at
$DC logs --since 10m platform-sync | grep -c '"api sync completed"'          # at least 1 (wait up to 5 min)
$DC exec -T postgres psql -U callloom -d callloom -c "SELECT platform, name, sync_status, last_success_at, consecutive_failures, left(COALESCE(last_sync_error,''),120) AS last_sync_error FROM integrations WHERE deleted_at IS NULL AND connection_mode = 'api_sync' ORDER BY platform;"
# expect: Live rows ok, last_success_at within the last few minutes, consecutive_failures 0

# No transcription and no workflow work happened
$DC exec -T postgres psql -U callloom -d callloom -tAc "SELECT (SELECT count(*) FROM transcription_jobs) AS jobs, (SELECT count(*) FROM workflow_rules) AS rules;"   # 0|0

# pings rows: same or higher than Step 1
echo "SELECT platform, count() AS rows FROM pings GROUP BY platform ORDER BY platform FORMAT TSV;" \
  | $DC exec -T clickhouse sh -c "$CH" | tee "$BK/pings-rows-after.tsv"
join -t $'\t' "$BK/pings-rows-before.tsv" "$BK/pings-rows-after.tsv" | awk -F'\t' '{print $1, $2, "->", $3, ($3 >= $2 ? "OK" : "LOWER - STOP")}'
```

**Recon AFTER:** wait **30 minutes** after Step 3, then on the **same Eastern day** as Step 1:

```bash
cd ~/deployments/calyx-ai
DC="docker compose -p deploy -f deploy/docker-compose.yml"
CH='clickhouse-client --password "$CLICKHOUSE_PASSWORD" --database default --multiquery'
GL=~/calyx-golive
$DC exec -T clickhouse sh -c "$CH" < "$GL/recon-7days.sql" | tee "$GL/after-2d2b-7days.txt"
diff "$GL/before-2d2b-7days.txt" "$GL/after-2d2b-7days.txt" && echo "7-day: IDENTICAL"
```

- **Pass:** `7-day: IDENTICAL`. This deploy does not change what is synced or how it is stored.
- A difference is allowed only if a platform itself changed a past call in those 30 minutes. Then compare that day
  with the Ringba/Retreaver report.
- Syed (browser, Incognito, admin): **Settings → Integrations** shows a green **ok** and a fresh "Last successful sync".
  There is no Providers tab and no bell yet, and Dashboard, Pings and Reports look as before.

**Do not start Step 6 until Step 5 passes.**

---

## Step 6 — Turn on, one at a time, with a check after each

Each block below starts with the same variables and the `set_env` helper. A web flag is build-time, so each one
rebuilds `web`.

### 6(a) Ask AI (📤 send the 5 answers + the 5 query outputs)

```bash
cd ~/deployments/calyx-ai
DC="docker compose -p deploy -f deploy/docker-compose.yml"; ENVF=.env; umask 077
set_env() { grep -v "^$1=" "$ENVF" > "$ENVF.new" && printf '%s=%s\n' "$1" "$2" >> "$ENVF.new" && mv "$ENVF.new" "$ENVF" && chmod 600 "$ENVF"; echo "$1=$2"; }
grep -q '^OPENAI_API_KEY=.\{20,\}' "$ENVF" && echo "openai key: set" || echo "openai key: MISSING - stop"
set_env FEATURE_ASK_AI true; set_env NEXT_PUBLIC_FEATURE_ASK_AI true
$DC build web
$DC up -d --no-deps --force-recreate api && sleep 5 && curl -s -o /dev/null -w 'api /readyz %{http_code}\n' http://127.0.0.1:3100/readyz
$DC up -d --no-deps --force-recreate web && sleep 5 && curl -s -o /dev/null -w 'web /login %{http_code}\n' http://127.0.0.1:3101/login
$DC exec -T api printenv FEATURE_ASK_AI                     # true
```

Syed: open **`https://calyx.ridgerisemedia.com/ask`** (Ask AI). Ask the 5 questions. For each, write down the answer and its **"Numbers used"**. Aryan runs
the matching query in the same terminal; they must give the **same numbers**. Ask AI uses Eastern time, and "last 7/30
days" end **yesterday**.

```bash
cd ~/deployments/calyx-ai
DC="docker compose -p deploy -f deploy/docker-compose.yml"
CH='clickhouse-client --password "$CLICKHOUSE_PASSWORD" --database default --multiquery'
cat > ~/calyx-golive/askai-base.sql <<'SQL'
  SELECT platform, external_call_id,
         argMax(revenue, received_at) AS rev,
         argMax(payout, received_at) AS pay,
         argMax(platform_duplicate, received_at) AS dup,
         argMax(has_converted, received_at) AS conv,
         argMax(campaign_id, received_at) AS campaign_id,
         argMax(campaign_name, received_at) AS campaign_name,
         argMax(publisher_id, received_at) AS publisher_id,
         argMax(publisher_name, received_at) AS publisher_name,
         max(if(occurred_at > toDateTime(0), occurred_at, received_at)) AS event_at
  FROM pings
  WHERE tenant_id = '11111111-1111-1111-1111-111111111111'
    AND external_call_id != ''
    AND external_call_id NOT LIKE 'apisync-%' AND external_call_id NOT LIKE 'demo-%' AND external_call_id NOT LIKE 'smoke-%'
    AND platform IN ('ringba', 'retreaver')
  GROUP BY platform, external_call_id
SQL
askq() {  # usage: askq "<select ... FROM (BASE) ...>"  — BASE is replaced by the subquery above
  local base; base=$(cat ~/calyx-golive/askai-base.sql)
  printf '%s\nFORMAT PrettyCompactMonoBlock;\n' "${1//BASE/$base}" | $DC exec -T clickhouse sh -c "$CH"
}
ET="toDate(toTimeZone(event_at, 'America/New_York'))"
TODAY="toDate(now(), 'America/New_York')"
```

| # | Question to type | Query that must give the same numbers |
|---|---|---|
| 1 | *Revenue by platform yesterday* | `askq "SELECT platform, count() calls, round(toFloat64(sumIf(rev, dup = 0)), 2) revenue FROM (BASE) WHERE $ET = $TODAY - 1 GROUP BY platform ORDER BY revenue DESC"` |
| 2 | *Calls by platform in the last 7 days* | `askq "SELECT platform, count() calls FROM (BASE) WHERE $ET >= $TODAY - 7 AND $ET < $TODAY GROUP BY platform ORDER BY calls DESC"` |
| 3 | *Which publisher had the lowest conversion in the last 30 days? Only publishers with at least 20 calls.* | `askq "SELECT if(publisher_name != '', publisher_name, publisher_id) publisher, count() calls, countIf(conv = 1 AND rev > 0) converted, round(100 * converted / calls, 1) conv_pct FROM (BASE) WHERE $ET >= $TODAY - 30 AND $ET < $TODAY GROUP BY publisher HAVING calls >= 20 ORDER BY conv_pct ASC, publisher ASC LIMIT 5"` |
| 4 | *How many converted calls did each platform have in the last 7 days?* | `askq "SELECT platform, countIf(conv = 1 AND rev > 0) converted FROM (BASE) WHERE $ET >= $TODAY - 7 AND $ET < $TODAY GROUP BY platform ORDER BY converted DESC"` |
| 5 | *Top 5 campaigns by revenue in the last 7 days* | `askq "SELECT if(campaign_name != '', campaign_name, campaign_id) campaign, count() calls, round(toFloat64(sumIf(rev, dup = 0)), 2) revenue FROM (BASE) WHERE $ET >= $TODAY - 7 AND $ET < $TODAY GROUP BY campaign ORDER BY revenue DESC, campaign ASC LIMIT 5"` |

- "Converted" means the flag **and** revenue > 0. Revenue excludes platform duplicates. Ringba and Retreaver only, test
  rows excluded.
- If Ask AI chose a different range (it states the dates), re-run the query with those dates.
- Send the token use shown on the page ("Today: N of 300,000 AI tokens used").
- **Pass:** all 5 match. **Any mismatch:** Ask AI off (Rollback 1), send both outputs, and do not continue to 6(b).

### 6(b) Workflows: alerts only, publisher actions stay OFF (📤 send output)

```bash
cd ~/deployments/calyx-ai
DC="docker compose -p deploy -f deploy/docker-compose.yml"; ENVF=.env; umask 077
set_env() { grep -v "^$1=" "$ENVF" > "$ENVF.new" && printf '%s=%s\n' "$1" "$2" >> "$ENVF.new" && mv "$ENVF.new" "$ENVF" && chmod 600 "$ENVF"; echo "$1=$2"; }
grep -E '^(WORKFLOW_ACTIONS_ENABLED|WORKFLOW_RINGBA_PAUSE_VERIFIED)=' "$ENVF"     # both =false — else stop
set_env FEATURE_WORKFLOWS true; set_env NEXT_PUBLIC_FEATURE_WORKFLOWS true
$DC build web
$DC up -d --no-deps --force-recreate api && sleep 5 && curl -s -o /dev/null -w 'api /readyz %{http_code}\n' http://127.0.0.1:3100/readyz
$DC up -d --no-deps --force-recreate web && sleep 5 && curl -s -o /dev/null -w 'web /alerts %{http_code}\n' http://127.0.0.1:3101/alerts   # 200
$DC exec -T api printenv FEATURE_WORKFLOWS WORKFLOW_ACTIONS_ENABLED          # true, false
```

**Syed: Settings → Alerts.**
1. **Status line:** it must read "actions **off**", "Ringba not verified" and "0/3 auto pauses".
2. **New rule:**
   - trigger "conversion rate below" 10%, window 168 h, minimum 20 calls;
   - scope: publishers, both platforms;
   - mode **Notify**.
3. **Dry run before saving:** note the publishers it lists with calls / converted / %.
4. **Aryan checks the numbers** with the same window (last 168 h, Eastern; run in the 6(a) terminal, or re-run the
   6(a) query block first):

   ```bash
   askq "SELECT if(publisher_name != '', publisher_name, publisher_id) publisher, platform, count() calls, countIf(conv = 1 AND rev > 0) converted, round(100 * converted / calls, 1) conv_pct FROM (BASE) WHERE event_at >= now() - INTERVAL 168 HOUR GROUP BY publisher, platform HAVING calls >= 20 AND conv_pct < 10 ORDER BY conv_pct ASC"
   ```

5. **Pass:** the same publishers and numbers (the dry run and the query use the dashboard's per-call view).
6. **Then:** save the rule as **Notify** only if Syed wants alerts in the bell for the demo. **Do not** use Suggest or
   Auto, and **do not** switch on "Allow Calyx to pause publishers".

### Owner UI step — Ringba Recording Access (Syed only, before 6(c))

**Settings → Integrations →** the Live Ringba row → **Recording access: Set** → the Ringba Recording Access
username and password → **Save**.
- The values are stored encrypted and are never shown again; the row then says "set".
- **Never** put them in the env file, the terminal or chat.

### 6(c) Transcription with the button only (automatic and backfill OFF) (📤 send output)

```bash
cd ~/deployments/calyx-ai
DC="docker compose -p deploy -f deploy/docker-compose.yml"; ENVF=.env; umask 077
set_env() { grep -v "^$1=" "$ENVF" > "$ENVF.new" && printf '%s=%s\n' "$1" "$2" >> "$ENVF.new" && mv "$ENVF.new" "$ENVF" && chmod 600 "$ENVF"; echo "$1=$2"; }
grep -E '^(TRANSCRIBE_AUTO|TRANSCRIBE_BACKFILL|TRANSCRIBE_SHOULD_CONVERT|TRANSCRIBE_DAILY_MINUTES_CAP)=' "$ENVF" | sort
# expect: TRANSCRIBE_AUTO=false, TRANSCRIBE_BACKFILL=false, TRANSCRIBE_DAILY_MINUTES_CAP=800, TRANSCRIBE_SHOULD_CONVERT=false
set_env FEATURE_TRANSCRIPTION true; set_env NEXT_PUBLIC_FEATURE_TRANSCRIPTION true
$DC build web
# platform-sync first: with the flag on in production it REFUSES to start without the Deepgram key and the bucket
$DC up -d --no-deps --force-recreate platform-sync && sleep 10 && docker inspect -f '{{.State.Status}} restarting={{.State.Restarting}}' deploy-platform-sync-1
$DC logs platform-sync 2>&1 | grep -E '"msg":"transcription( worker started)?"' | tail -2
# expect: "enabled":true,"archive_configured":true,"deepgram_key_set":true,"daily_cap_minutes":800 and "mip_opt_out":true
$DC up -d --no-deps --force-recreate api && sleep 5 && curl -s -o /dev/null -w 'api /readyz %{http_code}\n' http://127.0.0.1:3100/readyz
$DC up -d --no-deps --force-recreate web && sleep 5 && curl -s -o /dev/null -w 'web /login %{http_code}\n' http://127.0.0.1:3101/login
```

- If platform-sync is `restarting=true`: `$DC logs platform-sync --tail 20`. A line "refuses to start with
  missing/placeholder settings: [...]" names the setting (never its value). Turn transcription off (Rollback 3) and send
  the line.
- **Syed: Settings → Providers.** Deepgram **Configured**, Worker **Running**, archive **Configured**, today
  0 / 800 min, "**Transcribe button only (automatic off)**", and "Backfill is switched off on the server".

**Pick the calls:** 3 converted Retreaver calls and 3 converted Ringba calls from the last 7 days, plus one Ringba call
20–30 days old for the permanent-link test:

```bash
cd ~/deployments/calyx-ai
DC="docker compose -p deploy -f deploy/docker-compose.yml"
CH='clickhouse-client --password "$CLICKHOUSE_PASSWORD" --database default --multiquery'
cat > ~/calyx-golive/transcribe-pick.sql <<'SQL'
SELECT platform, external_call_id, toTimeZone(event_at, 'America/New_York') AS call_time_et, dur AS seconds,
       campaign_name, hr AS has_recording, age_days
FROM (
  SELECT platform, external_call_id,
         argMax(has_converted, received_at) AS conv, argMax(revenue, received_at) AS rev,
         argMax(platform_duplicate, received_at) AS dup, argMax(duration_seconds, received_at) AS dur,
         argMax(has_recording, received_at) AS hr, argMax(recording_known, received_at) AS rk,
         argMax(campaign_name, received_at) AS campaign_name,
         max(if(occurred_at > toDateTime(0), occurred_at, received_at)) AS event_at,
         dateDiff('day', event_at, now()) AS age_days
  FROM pings
  WHERE tenant_id = '11111111-1111-1111-1111-111111111111' AND platform IN ('ringba', 'retreaver')
    AND external_call_id != '' AND external_call_id NOT LIKE 'apisync-%' AND external_call_id NOT LIKE 'demo-%' AND external_call_id NOT LIKE 'smoke-%'
  GROUP BY platform, external_call_id
)
WHERE conv = 1 AND rev > 0 AND dup = 0 AND dur >= 20 AND NOT (rk = 1 AND hr = 0)
  AND (age_days <= 7 OR (platform = 'ringba' AND age_days BETWEEN 20 AND 30))
ORDER BY platform, age_days > 7, event_at DESC
LIMIT 5 BY platform, age_days > 7
FORMAT PrettyCompactMonoBlock;
SQL
$DC exec -T clickhouse sh -c "$CH" < ~/calyx-golive/transcribe-pick.sql
```

**Syed transcribes the calls.** Open `https://calyx.ridgerisemedia.com/calls/<external_call_id>?platform=<platform>` →
**Transcript → Transcribe**:
- 3 Retreaver calls and 3 Ringba calls from the last 7 days (prefer a Spanish campaign if one is listed);
- **plus** the one 20–30-day-old Ringba call.

Each takes about 10–60 s. Reload the page.

**Checks (Aryan), after the 7 transcriptions:**

```bash
cd ~/deployments/calyx-ai
DC="docker compose -p deploy -f deploy/docker-compose.yml"
PSQL="$DC exec -T postgres psql -U callloom -d callloom"
$PSQL -c "SELECT platform, external_call_id, reason, state, attempts, no_recording_reason, left(COALESCE(last_error,''),100) AS last_error FROM transcription_jobs ORDER BY created_at;"
# expect 7 rows, reason on_demand, state done (a no_recording row must give a reason — send it)
$PSQL -c "SELECT platform, external_call_id, archive_state, bytes, left(sha256,16) AS sha256, content_type, round(duration_seconds) AS secs FROM call_recordings ORDER BY created_at;"
# expect archive_state archived for each (not_archived = bucket problem: send $DC logs platform-sync --tail 30)
$PSQL -c "SELECT platform, external_call_id, transcript_version, language, model, redaction, scrubbed_count FROM call_transcripts ORDER BY created_at;"
# English: redaction deepgram:pci,ssn+calyx; Spanish campaign: deepgram:numbers+calyx
$PSQL -c "SELECT * FROM transcription_usage_daily;"
$PSQL -c "SELECT count(*) FILTER (WHERE outcome='served') AS served, count(*) FILTER (WHERE outcome<>'served') AS other FROM recording_access_audit;"
# no recording URL in the logs (must print 0):
$DC logs --since 2h platform-sync 2>&1 | grep -E 'transcription|recording' | grep -cE 'https?://'
```

**Syed checks in the browser on 2 of the calls (1 Retreaver, 1 Ringba):**
- **Transcript:** speakers and times; any card number or SSN shows `[REDACTED]`.
- **Play:** the recording plays from the archive (Calyx, not the platform). Each play adds one `served` audit row.
- **Evidence pack:** download the zip and unzip it. On Windows PowerShell, in the unzipped folder:

  ```powershell
  (Get-FileHash .\recording.* -Algorithm SHA256).Hash.ToLower()
  (Get-Content .\metadata.json | ConvertFrom-Json).recording_sha256
  # the two lines must be identical
  ```

- **Settings → Providers:** this month's minutes · calls · ≈ $ and today against 800. Compare the minutes with the Deepgram
  console's usage for today (they should be close).
- **B2 console:** the bucket has objects under `recordings/<tenant>/<platform>/<yyyy>/<mm>/<sha256>.<ext>`.

**Q6, Retreaver stereo (Syed):**
1. Open one Retreaver recording in Audacity: **Tracks → Split Stereo to Mono**.
2. Listen to each track. If the left track is only one party and the right is only the other, the legs are separate
   (then `multichannel=true` can replace diarization for Retreaver later).
3. Note the result and the transcript's speaker labels for that call.

**Ringba permanent link:** the 20–30-day-old Ringba call must be `done`.
- `no_recording` / `no_access`: the Recording Access user or password is wrong.
- `fetch_failed`: the link did not work for that old call.

Send the row.

**Pass for 6(c):**
- the 7 jobs are `done` (or a clear `no_recording` reason Syed accepts);
- the recordings are `archived`;
- redaction is visible, playback and the evidence-pack hash match, and the audit rows exist;
- the minutes appear in Providers;
- 0 URL lines.

### 6(d) Automatic transcription of converted calls (only after 6(c) passes; Syed says go)

```bash
cd ~/deployments/calyx-ai
DC="docker compose -p deploy -f deploy/docker-compose.yml"; ENVF=.env; umask 077
set_env() { grep -v "^$1=" "$ENVF" > "$ENVF.new" && printf '%s=%s\n' "$1" "$2" >> "$ENVF.new" && mv "$ENVF.new" "$ENVF" && chmod 600 "$ENVF"; echo "$1=$2"; }
grep -E '^(TRANSCRIBE_DAILY_MINUTES_CAP|TRANSCRIBE_SHOULD_CONVERT|TRANSCRIBE_BACKFILL)=' "$ENVF" | sort   # 800, false, false
set_env TRANSCRIBE_AUTO true
$DC up -d --no-deps --force-recreate api && sleep 5 && curl -s -o /dev/null -w 'api /readyz %{http_code}\n' http://127.0.0.1:3100/readyz
$DC exec -T api printenv TRANSCRIBE_AUTO                     # true
```

(The API reads `TRANSCRIBE_AUTO`, so only `api` is recreated. platform-sync and web are unchanged.)

After **20 minutes** (the first run starts 90 s after the API starts, then every 15 minutes):

```bash
cd ~/deployments/calyx-ai
DC="docker compose -p deploy -f deploy/docker-compose.yml"
$DC exec -T postgres psql -U callloom -d callloom -c "SELECT reason, state, count(*) FROM transcription_jobs GROUP BY 1,2 ORDER BY 1,2;"
# expect reason converted rows appearing (queued → done); no should_convert, no backfill
$DC exec -T postgres psql -U callloom -d callloom -c "SELECT usage_date, minutes, calls FROM transcription_usage_daily ORDER BY usage_date DESC LIMIT 2;"
# minutes must stay at or below 800 per Eastern day; at the cap new jobs wait as 'capped' until midnight ET
```

Syed: **Settings → Providers** now says "automatic: converted · ≥ 20s". Keep **backfill off** tonight.

**STOP.** Send 6(a)–6(d) to Syed.

---

## Rollback (only if Syed says so)

Each block starts with the same variables and helper:

```bash
cd ~/deployments/calyx-ai
DC="docker compose -p deploy -f deploy/docker-compose.yml"; ENVF=.env; umask 077
[ -f .env ] && [ "$(readlink -f deploy/.env)" = "$(readlink -f .env)" ] && echo "env file: OK (deploy/.env -> .env)" || echo "env file: CHECK - stop"
set_env() { grep -v "^$1=" "$ENVF" > "$ENVF.new" && printf '%s=%s\n' "$1" "$2" >> "$ENVF.new" && mv "$ENVF.new" "$ENVF" && chmod 600 "$ENVF"; echo "$1=$2"; }
```

**Each feature off on its own.** The other features keep running.

1. **Ask AI off:** `set_env FEATURE_ASK_AI false; set_env NEXT_PUBLIC_FEATURE_ASK_AI false; $DC build web; $DC up -d --no-deps --force-recreate api web`.
   Then `/ask` answers 404.
2. **Workflows off:** `set_env FEATURE_WORKFLOWS false; set_env NEXT_PUBLIC_FEATURE_WORKFLOWS false; $DC build web; $DC up -d --no-deps --force-recreate api web`.
   The rules and alerts stay in the database, unused.
3. **Transcription off:**
   `set_env FEATURE_TRANSCRIPTION false; set_env NEXT_PUBLIC_FEATURE_TRANSCRIPTION false; $DC build web; $DC up -d --no-deps --force-recreate platform-sync api web`.
   - The worker stops. Queued jobs stay queued.
   - Transcripts, recordings and the archive stay.
4. **Automatic off only:** `set_env TRANSCRIBE_AUTO false; $DC up -d --no-deps --force-recreate api`. The button keeps
   working.
5. **Spend brake without turning anything off:** `set_env TRANSCRIBE_DAILY_MINUTES_CAP 0; $DC up -d --no-deps --force-recreate platform-sync`.
   Every new job waits as `capped`.

**Full rollback (the database back to 34 and the previous code):**

```bash
BK=~/calyx-backups/deploy-v2-2d-2b
# 1. Database 36 -> 35 -> 34 with the migrations' own down files (they exist only in the NEW checkout, so this runs first).
#    000036 down drops the transcription tables (transcripts, recording rows, audit, usage) and the two Recording Access
#    columns. The audio objects in B2 stay; delete them only with Syed. 000035 down drops the workflow rules, alerts and
#    the action audit.
$DC exec -T postgres psql -U callloom -d callloom -v ON_ERROR_STOP=1 -1 < db/postgres/migrations/000036_transcription.down.sql
$DC exec -T postgres psql -U callloom -d callloom -c "UPDATE schema_migrations SET version = 35, dirty = false;"
$DC exec -T postgres psql -U callloom -d callloom -v ON_ERROR_STOP=1 -1 < db/postgres/migrations/000035_workflows.down.sql
$DC exec -T postgres psql -U callloom -d callloom -c "UPDATE schema_migrations SET version = 34, dirty = false;"
# 2. Every flag off in the env file (the old code ignores the new settings; the secrets may stay)
for f in FEATURE_ASK_AI NEXT_PUBLIC_FEATURE_ASK_AI FEATURE_WORKFLOWS NEXT_PUBLIC_FEATURE_WORKFLOWS FEATURE_TRANSCRIPTION NEXT_PUBLIC_FEATURE_TRANSCRIPTION TRANSCRIBE_AUTO; do set_env "$f" false; done
# 3. The previous code
git checkout "$(cat $BK/commit-before.txt)"
$DC build api web platform-sync
$DC up -d --no-deps --force-recreate platform-sync && sleep 10
$DC up -d --no-deps --force-recreate api && sleep 5
$DC up -d --no-deps --force-recreate web && sleep 5
$DC exec -T postgres psql -U callloom -d callloom -c "SELECT version, dirty FROM schema_migrations;"   # 34 | f
docker ps -a --filter name=deploy- --format '{{.Names}}  {{.Status}}  {{.Ports}}' | sort      # all Up; only 3100 / 3101
git status -sb | head -1     # "## HEAD (no branch)": the checkout is detached
```

The Postgres dump from Step 1 is the last resort. Restore it only with Syed.

**After a rollback the server checkout is on a detached HEAD** (the old commit, not a branch). Before the next deploy,
put it back on `main` first. This changes only files; nothing is restarted:

```bash
cd ~/deployments/calyx-ai
git checkout main
git status --short           # must be empty
git log --oneline -1         # then continue with the next pack's Step 0 (git pull --ff-only origin main)
```

---

## What was proven on the laptop before this was pushed

Laptop: Windows, Docker Desktop 4.88.1 (engine 29.7.2), dev `.env`, Test-mode (mock) integrations only. There was **no
Live integration, no platform API call, no real Deepgram key and no real bucket on the laptop**.

- **Migrations:**
  - `000035` up / down / up (report §2D.3);
  - `000036` up / down / up on a throwaway Postgres, with role, RLS and function checks
    (`scripts/verify-migration-36-transcription.sh`);
  - the laptop stack is at `36 | f`.
- **End to end with fake Deepgram and a fake S3** (report §2B.5):
  - one fetch, the same bytes sent to the archive and to Deepgram, `mip_opt_out=true`;
  - English `redact=pci,ssn`, Spanish `language=multi` + `redact=numbers`, plus our scrub;
  - playback and evidence pack hash-checked, every access audited, 0 URL log lines.
- **Main (this merge), every flag off, laptop stack rebuilt from main at `36 | f`:**
  - `verify-system.ps1 -V1`: **SYSTEM VERIFY PASS** — Docker 9/9, Healthz 5/5, LiveStatus, API v1, presentation 20/20,
    Web routes 13/13, WebSocket, SmokePrefix, BrowserCSP, unit tests 356/356, web build, pipeline 4/4;
  - `verify-ingestion-slice.ps1`: **INGESTION SLICE VERIFY PASS**;
  - `verify-api-sync.ps1`: **PASS**;
  - Go platform-sync `vet` + `test`: ok; web 54/54.
- **This pack's new ClickHouse queries** (the call picker and the workflows dry-run check) ran on the laptop
  ClickHouse without errors. The picker returned 5 calls per platform and age group once the laptop's test-prefix
  filter was lifted (on the server it stays).
- **Switches:** `TRANSCRIBE_AUTO` and `TRANSCRIBE_BACKFILL` (API tests: with the flag on and auto off, the job queues
  nothing and the button works; backfill gets 409 while off, and the estimate works).
