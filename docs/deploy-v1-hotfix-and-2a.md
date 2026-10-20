# Calyx — deploy pack: v1 hotfix + Retreaver mapping + Phase 2A (all flags OFF)

**Server:** `calyx.ridgerisemedia.com` · `~/deployments/calyx-ai` · compose project **`deploy`**
**Code:** `main` after the three merges of 2026-10-20 (v1 hide-intel-routes → v1 Retreaver mapping →
v2 Phase 2A). Check the exact hash in Step 0 against the one Syed sends.
**People:** **Aryan** (server terminal) · **Syed** (browser, platform reports)
**Timezone for every comparison:** Eastern, `America/New_York`

What this deploy changes for users: **nothing visible** except that the out-of-scope pages now answer 404
and Retreaver calls start to show campaign name, buyer (target) and recording yes/no. Ask AI ships **off**.

## Rules for everyone

- 🔴 No API key, password or `.env` value in chat, WhatsApp or email. Nothing in this pack prints one.
- **Never** `docker compose down -v`, never delete volumes, always `-p deploy`.
- Write-back stays **off** (`WRITEBACK_ENABLED=false`). Ask AI stays **off**.
- On anything unexpected: **stop**, send the output, do not retry on your own.

Use **one terminal** for the whole pack. If the connection drops, re-run Step 0.1 first.

---

## Step 0 — Before you touch anything (📤 send output)

### 0.1 Check — changes nothing

```bash
cd ~/deployments/calyx-ai
DC="docker compose -p deploy -f deploy/docker-compose.yml"
ENVF=$(grep -l '^NEXT_PUBLIC_API_URL=' .env deploy/.env 2>/dev/null); echo "env file: [$ENVF]"
[ "$(printf '%s\n' $ENVF | wc -l)" -eq 1 ] && [ -f "$ENVF" ] && echo "env file: OK" || echo "env file: NOT EXACTLY ONE - stop"
git fetch origin
git log --oneline -1            # what runs now (expected: 99618e0, Stage 4b, migration 32)
git merge-base --is-ancestor 99618e0 HEAD && echo "server has 99618e0: OK" || echo "server is OLDER than 99618e0 - stop and tell Syed"
git log --oneline -1 origin/main   # the target: must match the hash Syed sent
git status --short              # must be empty
grep -E '^(APP_ENV|MOCK_MODE|WRITEBACK_ENABLED|FEATURE_ASK_AI|NEXT_PUBLIC_FEATURE_ASK_AI|COMPOSE_PROFILES)=' "$ENVF"
$DC exec -T postgres psql -U callloom -d callloom -c "SELECT version, dirty FROM schema_migrations;"
```

**Continue only if:** `env file: OK`, `server has 99618e0: OK`, `git status` empty, `origin/main` is the hash
Syed sent, `APP_ENV=production`, `WRITEBACK_ENABLED=false`, **`COMPOSE_PROFILES` absent or empty** in the env file
(no `intelligence` profile on the server), migration `32 | f`.

If the server is **older than `99618e0`**: stop and tell Syed (this pack assumes Stage 4b is already deployed).

### 0.2 Reconciliation BEFORE the deploy (keep this output)

⏰ **Before and after must run on the same Eastern day** (the queries cover the last full days up to yesterday,
Eastern; after midnight ET "yesterday" changes and the two outputs cannot be compared). So **do not start this
deploy between 08:30 and 10:00 PKT** (midnight ET is 09:00 PKT while the US is on daylight time, i.e. until
2026-11-01). From 2026-11-01 (US standard time) midnight ET is 10:00 PKT: then avoid **09:30–11:00 PKT**.

Save the two query files from go-live Step 2.4 **with the fixed versions** (they already exist in
`~/calyx-golive/` if you used the corrected ones from 2026-10-15; this rewrites them identically):

```bash
mkdir -p ~/calyx-golive && cd ~/calyx-golive
cat > recon-7days.sql <<'SQL'
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
cat > recon-30days.sql <<'SQL'
SELECT platform,
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
WHERE toDate(toTimeZone(event_at, 'America/New_York')) >= toDate(now(), 'America/New_York') - 29
  AND toDate(toTimeZone(event_at, 'America/New_York')) <  toDate(now(), 'America/New_York')
GROUP BY platform
ORDER BY platform
FORMAT PrettyCompactMonoBlock;
SQL
cd ~/deployments/calyx-ai
CH='clickhouse-client --password "$CLICKHOUSE_PASSWORD" --database default --multiquery'
$DC exec -T clickhouse sh -c "$CH" < ~/calyx-golive/recon-7days.sql  | tee ~/calyx-golive/before-7days.txt
$DC exec -T clickhouse sh -c "$CH" < ~/calyx-golive/recon-30days.sql | tee ~/calyx-golive/before-30days.txt
```

---

## Step 1 — Backups

```bash
BK=~/calyx-backups/deploy-v1-hotfix-2a
mkdir -p $BK && chmod 700 $BK
$DC exec -T postgres pg_dump -U callloom -d callloom -Fc > $BK/callloom-before.dump
cp "$ENVF" $BK/env-before.bak && chmod 600 $BK/env-before.bak
git rev-parse HEAD > $BK/commit-before.txt
ls -lh $BK      # the dump must be larger than 0
```

## Step 2 — Pull and build (the site stays up)

```bash
git pull --ff-only origin main
git log --oneline -1               # the hash Syed sent
$DC build api web platform-sync
```

## Step 3 — Flags OFF in the env file (nothing secret is printed)

```bash
umask 077
grep -vE '^(FEATURE_ASK_AI|NEXT_PUBLIC_FEATURE_ASK_AI)=' "$ENVF" > "$ENVF.new"
printf 'FEATURE_ASK_AI=false\nNEXT_PUBLIC_FEATURE_ASK_AI=false\n' >> "$ENVF.new"
mv "$ENVF.new" "$ENVF" && chmod 600 "$ENVF"
grep -E '^(APP_ENV|WRITEBACK_ENABLED|FEATURE_ASK_AI|NEXT_PUBLIC_FEATURE_ASK_AI)=' "$ENVF"
# expect: APP_ENV=production, WRITEBACK_ENABLED=false, FEATURE_ASK_AI=false, NEXT_PUBLIC_FEATURE_ASK_AI=false
```

No OpenAI key is added in this deploy. The web image reads `NEXT_PUBLIC_FEATURE_*` at **build** time; if
you changed them after Step 2, run `$DC build web` again before Step 4.

## Step 4 — Start

```bash
$DC up -d --remove-orphans
sleep 30
docker ps --filter name=deploy- --format '{{.Names}}  {{.Status}}  {{.Ports}}'
$DC exec -T postgres psql -U callloom -d callloom -c "SELECT version, dirty FROM schema_migrations;"
```

Expect the same 9 v1 services **Up**, every port starting with `127.0.0.1`, and migration **`33 | f`**
(new: `000033_ask_ai_v2`, two empty tables for Ask AI).

## Step 5 — Checks (📤 send output)

```bash
$DC exec -T api printenv APP_ENV WRITEBACK_ENABLED FEATURE_ASK_AI            # production / false / false
$DC exec -T platform-sync printenv APP_ENV WRITEBACK_ENABLED                  # production / false
$DC logs api --tail 30
$DC logs platform-sync --tail 30

for p in /scoring /ask /compliance /attribution /intelligence /dashboard /login; do
  printf '%-14s ' "$p"; curl -s -o /dev/null -w '%{http_code}\n' "http://127.0.0.1:3101$p"
done
# expect: /scoring 404, /ask 404, /compliance 404, /attribution 404, /intelligence 404, /dashboard 200, /login 200

printf 'API POST /ask  '; curl -s -o /dev/null -w '%{http_code}\n' -X POST http://127.0.0.1:3100/ask
printf 'API /alerts    '; curl -s -o /dev/null -w '%{http_code}\n' http://127.0.0.1:3100/alerts/rules
printf 'API /providers '; curl -s -o /dev/null -w '%{http_code}\n' http://127.0.0.1:3100/providers/catalog
# expect: 404, 404, 404 (closed even without logging in)
```

If any `curl` prints **`000`** (nothing answered), the ports differ on this server. Check the published ports and
use those instead of 3101 / 3100:

```bash
docker ps --filter name=deploy- --format '{{.Names}}  {{.Ports}}'   # e.g. 127.0.0.1:3101->3000/tcp for web
```

No `error` lines in the logs. A platform-sync line `"demo sync skipped"` is fine.

Browser (Incognito, `https://calyx.ridgerisemedia.com`, admin login): Dashboard, Pings, Reports, Coverage,
Settings open as before; the sidebar has **no** Ask AI item; a Retreaver call's detail shows
**"Buyer (target)"**.

## Step 6 — Reconciliation AFTER (📤 send output) — must match Step 0.2

Wait **30 minutes** after Step 4 (one sync cycle), then — **still on the same Eastern day as Step 0.2** (if midnight
ET has passed, run Step 0.2's queries again for the new day and compare only days that both outputs cover):

```bash
cd ~/deployments/calyx-ai
$DC exec -T clickhouse sh -c "$CH" < ~/calyx-golive/recon-7days.sql  | tee ~/calyx-golive/after-7days.txt
$DC exec -T clickhouse sh -c "$CH" < ~/calyx-golive/recon-30days.sql | tee ~/calyx-golive/after-30days.txt
diff ~/calyx-golive/before-7days.txt ~/calyx-golive/after-7days.txt && echo "7-day: IDENTICAL"
```

- **Pass:** every full past day has the same `calls` and `revenue` per platform as before. (`diff` may only
  differ if a platform itself changed a past call in those 30 minutes — e.g. a late conversion; then
  compare that day with the Ringba/Retreaver report.)
- Expected and fine: Retreaver calls that are fetched again are **republished once** with campaign name,
  buyer and recording yes/no; that adds a row per call and raises `sync_count` by 1, but `calls` and
  `revenue` do not change (reads take the latest row).

## Step 7 — The two "For Aryan" queries (read-only, 📤 send output)

**7.1 Recording share** (Retreaver values fill in as calls are re-fetched after this deploy):

```bash
cat > ~/calyx-golive/recording-share.sql <<'SQL'
SELECT platform, count() AS calls, countIf(rk = 1) AS known, countIf(hr = 1) AS recorded,
       round(100 * countIf(hr = 1) / nullIf(countIf(rk = 1), 0), 1) AS pct_recorded,
       round(100 * countIf(conv = 1 AND rev > 0 AND hr = 1) / nullIf(countIf(conv = 1 AND rev > 0 AND rk = 1), 0), 1) AS pct_converted_recorded
FROM (
  SELECT platform, external_call_id,
         argMax(has_recording, received_at) AS hr, argMax(recording_known, received_at) AS rk,
         argMax(has_converted, received_at) AS conv, argMax(revenue, received_at) AS rev
  FROM pings
  WHERE tenant_id = '11111111-1111-1111-1111-111111111111'
    AND external_call_id != ''
    AND external_call_id NOT LIKE 'apisync-%'
    AND external_call_id NOT LIKE 'demo-%'
    AND external_call_id NOT LIKE 'smoke-%'
  GROUP BY platform, external_call_id
)
GROUP BY platform
FORMAT PrettyCompactMonoBlock;
SQL
$DC exec -T clickhouse sh -c "$CH" < ~/calyx-golive/recording-share.sql
```

**7.2 Should-have-converted per day** (Eastern, last 30 full days, with minutes):

```bash
cat > ~/calyx-golive/should-convert-30d.sql <<'SQL'
SELECT platform,
       toDate(toTimeZone(event_at, 'America/New_York')) AS day_et,
       count() AS should_convert_calls,
       round(sum(dur) / 60, 1) AS minutes
FROM (
  SELECT platform, external_call_id,
         argMax(revenue, received_at) AS last_revenue,
         argMax(has_converted, received_at) AS last_conv,
         argMax(duration_seconds, received_at) AS dur,
         argMax(required_duration_seconds, received_at) AS req,
         max(if(occurred_at > toDateTime(0), occurred_at, received_at)) AS event_at
  FROM pings
  WHERE tenant_id = '11111111-1111-1111-1111-111111111111'
    AND external_call_id != ''
    AND external_call_id NOT LIKE 'apisync-%'
    AND external_call_id NOT LIKE 'demo-%'
    AND external_call_id NOT LIKE 'smoke-%'
  GROUP BY platform, external_call_id
)
WHERE last_conv = 0
  AND (last_revenue > 0 OR (req IS NOT NULL AND dur >= req))
  AND toDate(toTimeZone(event_at, 'America/New_York')) >= toDate(now(), 'America/New_York') - 30
  AND toDate(toTimeZone(event_at, 'America/New_York')) <  toDate(now(), 'America/New_York')
GROUP BY platform, day_et
ORDER BY platform, day_et
FORMAT PrettyCompactMonoBlock;
SQL
$DC exec -T clickhouse sh -c "$CH" < ~/calyx-golive/should-convert-30d.sql
```

**STOP.** Send Steps 0, 5, 6 and 7 to Syed.

---

## Rollback (only if Syed says so)

```bash
cd ~/deployments/calyx-ai
DC="docker compose -p deploy -f deploy/docker-compose.yml"
ENVF=$(grep -l '^NEXT_PUBLIC_API_URL=' .env deploy/.env 2>/dev/null); echo "env file: [$ENVF]"
[ "$(printf '%s\n' $ENVF | wc -l)" -eq 1 ] && [ -f "$ENVF" ] && echo "env file: OK" || echo "env file: NOT EXACTLY ONE - stop"
```

Only after `env file: OK`:

```bash
BK=~/calyx-backups/deploy-v1-hotfix-2a
git checkout "$(cat $BK/commit-before.txt)"
cp $BK/env-before.bak "$ENVF" && chmod 600 "$ENVF"
# Migration 33 only adds two empty Ask AI tables. Step the database back so the old code's
# migrate step (which knows only up to 32) can start:
$DC exec -T postgres psql -U callloom -d callloom -c "DROP TABLE IF EXISTS ask_ai_answers; DROP TABLE IF EXISTS ask_ai_usage_daily; UPDATE schema_migrations SET version = 32, dirty = false;"
$DC build api web platform-sync
$DC up -d
$DC exec -T postgres psql -U callloom -d callloom -c "SELECT version, dirty FROM schema_migrations;"   # 32 | f
```

The Retreaver rows republished with the new fields stay in ClickHouse; they are harmless for the old code
(same columns, latest row wins). The Postgres dump from Step 1 is the last resort (restore only with Syed).

---

## Ask AI smoke test (later, after the owner gives the OpenAI key)

Not part of this deploy. Run it only when Syed says so.

### Turn on

The key goes **only** into the server env file — never into chat, an editor or the terminal history. After the
same Step 0.1 check (`env file: OK`), paste it at a **hidden** prompt (nothing is shown or printed):

```bash
umask 077; read -rs -p 'Paste OpenAI key (hidden): ' K; echo
grep -v '^OPENAI_API_KEY=' "$ENVF" > "$ENVF.new" && printf 'OPENAI_API_KEY=%s\n' "$K" >> "$ENVF.new" && mv "$ENVF.new" "$ENVF" && chmod 600 "$ENVF"; unset K
grep -q '^OPENAI_API_KEY=.\{20,\}' "$ENVF" && echo "openai key: set" || echo "openai key: MISSING - stop"
```

Then switch Ask AI on:

```bash
umask 077
grep -vE '^(FEATURE_ASK_AI|NEXT_PUBLIC_FEATURE_ASK_AI)=' "$ENVF" > "$ENVF.new"
printf 'FEATURE_ASK_AI=true\nNEXT_PUBLIC_FEATURE_ASK_AI=true\n' >> "$ENVF.new"
mv "$ENVF.new" "$ENVF" && chmod 600 "$ENVF"
grep -q '^OPENAI_API_KEY=.\{20,\}' "$ENVF" && echo "openai key: set" || echo "openai key: MISSING - stop"
$DC build web                                  # the web switch is build-time
$DC up -d --no-deps --force-recreate api web
$DC exec -T api printenv FEATURE_ASK_AI        # true
```

Optional: `OPENAI_MODEL` (default `gpt-5-mini`), `ASK_AI_TOKEN_BUDGET_DAILY` (default 300000 tokens per
tenant per Eastern day). The API refuses to start in production with `FEATURE_ASK_AI=true` and no key.

### Turn off

Same as "Turn on" with both values `false`, then `$DC build web` and
`$DC up -d --no-deps --force-recreate api web`. `/ask` answers 404 again.

### The 5 questions (Syed, in the browser: Intelligence › Ask AI)

For each question, write down the answer and its **"Numbers used"** table, then Aryan runs the matching
query. They must give the **same numbers**. Ask AI uses Eastern time; "last 7/30 days" end **yesterday**.

Save the shared per-call subquery once:

```bash
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

Notes:
- "Converted" means flag **and** revenue > 0; revenue excludes platform duplicates; Ringba and Retreaver only;
  test rows excluded — the same rules as the dashboard and the go-live recon.
- If Ask AI chose a different range (it states the dates it used), re-run the query with those dates.
- After the five questions, send Syed the token use shown on the page ("Today: N of 300,000 AI tokens used").
- Any mismatch: note the question, Ask AI's numbers and the query's numbers; turn Ask AI **off**; send both.
