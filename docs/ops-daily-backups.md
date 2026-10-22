# Calyx — daily backups on the server (ops)

**Server:** `calyx.ridgerisemedia.com` · `~/deployments/calyx-ai` · compose project **`deploy`**
**Script:** `scripts/backup-calyx.sh` (on `main` since `5cdb46e`)
**People:** **Aryan** (server terminal) · **Syed** (approves, keeps the off-server copy later)

Rules: never `docker compose down` (at all), never `-v`, always `-p deploy`, never print secrets, never run
`docker compose config` without a pipe (it prints the env file). Do **not** touch Plane's containers. The
backup is read-only on the databases and needs no host port (after Part 1 only api 3100 and web 3101 have one).

---

## What is backed up, and from where

From `deploy/docker-compose.yml`:

| | Container | What | Volume (data on disk) |
|---|---|---|---|
| Postgres | `deploy-postgres-1`, image `pgvector/pgvector:pg16` | database **`callloom`**, user **`callloom`** (the superuser the image creates) — integrations, users, tenants, write-back audit, alert rules, Ask AI history, `schema_migrations` | `deploy_postgres_data` |
| ClickHouse | `deploy-clickhouse-1`, image `clickhouse/clickhouse-server:24.8` (laptop: 24.8.14.39) | database **`default`**, user `default` — **`pings`** (every call; the reports and the recon read it) and every other MergeTree table: `call_scores`, `transcripts`, `enrichment_attrs`, `compliance_*`, `attribution_*`, `financial_events`, the `*_dlq` tables, `nl_query_results_cache` (in v1 most are empty) | `deploy_clickhouse_data` |
| redis | `deploy-redis-1` | **not backed up**: only dedup hashes (10-day TTL) and sync locks. Losing them re-publishes unchanged calls once; reads take the latest row, so totals do not change | `deploy_redis_data` |

The script lists the ClickHouse tables itself every night (`system.tables`, engine `*MergeTree`), so a new table
is included without changing anything.

**Not in the backup: the env file.** `~/deployments/calyx-ai/.env` holds `CREDENTIAL_ENCRYPTION_KEY` and
`ANI_ENCRYPTION_KEY`. The integrations' API credentials and the caller numbers in the dumps are encrypted with
them, so a restore on a new server also needs those two keys. They do not change; Syed keeps them in his
password manager (never in chat, never in the off-server bucket in plain text).

## What one backup day looks like

`~/calyx-backups/daily/<YYYY-MM-DD>/` — the date is **Pakistan time**; folders `700`, files `600`:

| File | How it is made |
|---|---|
| `postgres-callloom.dump` | `pg_dump -Fc` through `docker exec` — one consistent snapshot of the whole database |
| `postgres-roles.sql` | `pg_dumpall --roles-only` — the app roles (`callloom_api`, `callloom_sync`, `callloom_ingestion`) with their password **hashes** (no plaintext) |
| `clickhouse/<table>.sql` | `SHOW CREATE TABLE` |
| `clickhouse/<table>.native.gz` | `SELECT * FORMAT Native`, gzip. One query per table: ClickHouse reads a fixed set of parts for the whole query, so each table is a consistent snapshot, and the service keeps running (no stop, no `FREEZE`) |
| `counts.tsv` | rows per table — ClickHouse counted **from the dump files**, Postgres counted right after the dump |
| `SHA256SUMS` | checksum of every file |

- The ClickHouse password never appears: `clickhouse-client` reads `CLICKHOUSE_PASSWORD` from the
  container's own environment. Postgres uses the container's local socket. Nothing secret is printed.
- A day is built in `<day>.partial/` and renamed only when every file is written and verified
  (`pg_restore --list`, `gzip -t`, row count, checksums). A failed run never replaces a good day.
- **Keeps 14 days** (the newest 14 complete days; only `YYYY-MM-DD` folders inside `~/calyx-backups/daily`
  are ever deleted).
- **Stops before writing anything if free disk is under 10 GB.**
- Prints only sizes and `OK` / `FAILED`; exit code 0 = complete, 1 = failed. Details of a failure go to
  `<day>.partial/errors.log` (600).
- Optional settings (env, rarely needed): `CALYX_BACKUP_KEEP_DAYS` (14), `CALYX_BACKUP_MIN_FREE_GB` (10).

---

## 1. One-time manual run (📤 send output)

Run it from the server's current checkout. On 2026-10-23 that is `FINAL_MAIN`, after `docs/deploy-37-grants.md`. **No
pull**: the checkout is pinned (`docs/aryan-run-order-2026-10-23.md`).

```bash
cd ~/deployments/calyx-ai
git log --oneline -1                                   # FINAL_MAIN (the pinned checkout)
ls scripts/backup-calyx.sh                             # present (on main since 5cdb46e)
df -h ~ | tail -1                                      # free space before (expect ~37 GB)
bash scripts/backup-calyx.sh; echo "exit=$?"
```

Expected (sizes differ; every line ends in `OK`, the last line is `BACKUP OK`, `exit=0`):

```
free disk before                   37 GB OK
postgres dump                      2.1M OK
postgres roles                     4.0K OK
clickhouse attestation_events      1.0K OK
...
clickhouse pings                   310M OK
...
this backup                        315M OK
all backups (1 days kept, max 14)  315M
free disk after                    36 GB
BACKUP OK (2026-10-22, 95 s) — /home/<you>/calyx-backups/daily/2026-10-22
exit=0
```

Then:

```bash
D=~/calyx-backups/daily/$(TZ=Asia/Karachi date +%F)
ls -la ~/calyx-backups ~/calyx-backups/daily "$D" | head -20     # drwx------ folders, -rw------- files
grep -E 'pings|integrations' "$D/counts.tsv"                     # row counts (not secret)
(cd "$D" && sha256sum -c --quiet SHA256SUMS && echo "checksums OK")
```

If a line says `FAILED`: send the output and `cat "$D.partial/errors.log"` (it holds database error messages,
no passwords). Nothing was changed in the databases.

**Size check (📤):** send `du -sh "$D"`. 14 days of that must stay far below the free space: if
`14 × size` is more than about 20 GB, tell Syed (he picks fewer days, e.g. `CALYX_BACKUP_KEEP_DAYS=7`).

## 2. Cron job — every day at 03:30 PKT

03:30 PKT is 22:30 UTC. The server clock is Europe time: **00:30 CEST** until Sunday 2026-10-25, then
**23:30 CET** (winter time) until the end of March. So the entry runs at both 23:30 and 00:30 server time and
only goes ahead when it is 03 o'clock in Pakistan — it never has to be changed for summer/winter time:

```bash
touch ~/calyx-backups/backup.log && chmod 600 ~/calyx-backups/backup.log
( crontab -l 2>/dev/null; echo '30 23,0 * * * [ "$(TZ=Asia/Karachi date +\%H)" = 03 ] && cd $HOME/deployments/calyx-ai && { date -u "+== \%F \%T UTC"; bash scripts/backup-calyx.sh; } >> $HOME/calyx-backups/backup.log 2>&1' ) | crontab -
crontab -l | grep backup-calyx            # the line, once
timedatectl | grep -i 'time zone'         # for Syed: the server's zone (CEST/CET)
```

Check the next morning (📤):

```bash
tail -30 ~/calyx-backups/backup.log       # a "== <date> 22:30 UTC" header and "BACKUP OK"
ls ~/calyx-backups/daily/
```

`date -u` in the header shows **22:30 UTC** for the run that went ahead; the other one is skipped silently.
If the cron run fails but a manual run works, the usual cause is `docker` not in cron's `PATH` or the user not
in the `docker` group — the script sets `PATH` itself; send the log.

## 3. Restore test — into THROWAWAY containers only (📤 send output)

Run once now, then once a month (Syed reminds). It never touches `deploy-postgres-1`, `deploy-clickhouse-1` or
Plane: it starts two temporary containers with **no network at all** (`--network none`) from the images the
server already has, restores the backup into them, compares row counts, and stops them (`--rm` deletes them and
their data). Before: check there is room and memory (the restore needs about the uncompressed size of the
backup on the Docker disk, plus ~3 GB RAM while it runs):

```bash
df -h /var/lib/docker | tail -1; free -h | head -2
```

Not during the backup (03:30 PKT) and not between 08:30 and 10:00 PKT. Then paste this whole block (from `(` to `)`):

```bash
( # one subshell: a "stop" below ends only this block, never your terminal
B="${B:-$HOME/calyx-backups/daily/$(TZ=Asia/Karachi date +%F)}"   # the backup day to test
T="calyx-restore-test"
[ -f "$B/counts.tsv" ] || { echo "no backup in $B - stop"; exit 1; }
(cd "$B" && sha256sum -c --quiet SHA256SUMS) && echo "checksums: OK" || { echo "checksums: FAILED - stop"; exit 1; }
# Throwaway servers: no network at all (cannot reach Calyx, Plane or the internet), removed on stop (--rm).
docker run -d --rm --name $T-pg --network none --memory 1g \
  -e POSTGRES_USER=callloom -e POSTGRES_PASSWORD=restore-test-only -e POSTGRES_DB=callloom pgvector/pgvector:pg16 > /dev/null
docker run -d --rm --name $T-ch --network none --memory 2g \
  -e CLICKHOUSE_PASSWORD=restore-test-only clickhouse/clickhouse-server:24.8 > /dev/null
for i in $(seq 1 60); do docker exec $T-pg pg_isready -U callloom -d callloom > /dev/null 2>&1 && break; sleep 2; done
for i in $(seq 1 60); do docker exec $T-ch clickhouse-client --query "SELECT 1" > /dev/null 2>&1 && break; sleep 2; done
sleep 3   # the postgres image restarts once after its first init

# Postgres: roles first (the "role callloom already exists" line is expected), then the dump.
docker exec -i $T-pg psql -U callloom -d postgres -q < "$B/postgres-roles.sql" > /dev/null 2>&1
docker exec -i $T-pg pg_restore -U callloom -d callloom < "$B/postgres-callloom.dump" 2> /tmp/$T-pg-restore.log \
  && echo "postgres restore: OK" || echo "postgres restore: FAILED (see /tmp/$T-pg-restore.log)"
docker exec $T-pg psql -U callloom -d callloom -XAtq -F $'\t' -c \
  "SELECT 'postgres', table_name, (xpath('/row/c/text()', query_to_xml(format('SELECT count(*) AS c FROM %I.%I', table_schema, table_name), false, true, '')))[1]::text
     FROM information_schema.tables WHERE table_schema = 'public' AND table_type = 'BASE TABLE' ORDER BY table_name" > /tmp/$T-counts.tsv

# ClickHouse: every table into a throwaway database "restore_test".
docker exec $T-ch clickhouse-client --query "CREATE DATABASE restore_test"
for s in "$B"/clickhouse/*.sql; do
  t=$(basename "$s" .sql)
  sed '1s/^CREATE TABLE [^ ]*\./CREATE TABLE restore_test./' "$s" | docker exec -i $T-ch clickhouse-client --multiquery \
    || echo "clickhouse $t schema: FAILED"
  if [ "$(gzip -dc "$B/clickhouse/$t.native.gz" | head -c 1 | wc -c)" -gt 0 ]; then
    gzip -dc "$B/clickhouse/$t.native.gz" | docker exec -i $T-ch clickhouse-client --query "INSERT INTO restore_test.\`$t\` FORMAT Native" \
      || echo "clickhouse $t data: FAILED"
  fi
  printf 'clickhouse\t%s\t%s\n' "$t" "$(docker exec $T-ch clickhouse-client --query "SELECT count() FROM restore_test.\`$t\`")" >> /tmp/$T-counts.tsv
done

# Compare: restored rows vs counts.tsv written by the backup.
awk -F'\t' 'NR==FNR {want[$1" "$2]=$3; next} {k=$1" "$2; got=$3; w=(k in want)?want[k]:"missing";
            st=(got==w)?"OK":"DIFF"; if (st!="OK") bad++; printf "%-48s backup=%-10s restored=%-10s %s\n", k, w, got, st}
            END {print (bad ? "RESTORE TEST: " bad " table(s) differ - see above" : "RESTORE TEST: PASS (every table restored with the same row count)")}' \
  "$B/counts.tsv" /tmp/$T-counts.tsv

# Clean up: stopping removes both throwaway containers and their data (--rm). Nothing else is touched.
docker stop $T-pg $T-ch > /dev/null
rm -f /tmp/$T-counts.tsv /tmp/$T-pg-restore.log
docker ps -a --filter name=$T --format '{{.Names}}' | grep -q . && echo "cleanup: CHECK" || echo "cleanup: OK (throwaway containers gone)"
)
```

Expected: `checksums: OK`, `postgres restore: OK`, one `OK` line per table, **`RESTORE TEST: PASS`**,
`cleanup: OK`. ClickHouse tables must match exactly (their counts come from the dump files). A Postgres table that
was written during the backup's few seconds (e.g. `integrations` status updates do not change the count;
`writeback_audit` could) may show `DIFF` by a row or two — send it to Syed, that is not a broken backup. To test
an older day: `B=~/calyx-backups/daily/2026-10-20` before the block.

## 4. Sizes and disk (any time, read-only)

```bash
df -h ~ | tail -1                                  # free space on the backup disk (stop sign: under 10 GB)
du -sh ~/calyx-backups/daily/* | tail -15          # one line per day
du -sh ~/calyx-backups/daily                       # all days
ls ~/calyx-backups/daily | wc -l                   # 14 at most
grep -E '^== |BACKUP (OK|FAILED)' ~/calyx-backups/backup.log | tail -10   # last runs
docker system df                                   # Docker's own disk use (images, volumes) — read-only
```

Disk budget: 96 GB, ~37 GB free before backups. The script refuses to run under 10 GB free, so a full disk can
never be caused by the backup itself; if that line appears in the log, tell Syed the same day.

---

## Later (not now): an off-server copy to Backblaze B2

The daily backups live on the same server, so they protect against a bad deploy, a broken migration or deleted
rows — **not** against losing the server. The next step, only when Syed sets it up:

1. Syed creates a **private** B2 bucket (e.g. `calyx-backups-<random>`), with **object lock or lifecycle rules**
   (keep 30 days), and an **application key limited to that bucket**, with write and list but **no delete**
   permission, so a compromised server cannot wipe the copies.
2. **Encrypt before upload:** the dumps contain call data and hashed passwords. Use `rclone` with a `crypt`
   remote (or `age`) so B2 only ever holds ciphertext; the encryption passphrase lives in Syed's password
   manager, not on the server in plain text.
3. On the server: install `rclone` (Aryan, after Syed's OK), configure the B2 key and the crypt passphrase at
   **hidden prompts** (`rclone config`; the config file is `~/.config/rclone/rclone.conf`, `chmod 600`).
4. After each successful backup, the cron line uploads only that day:
   `rclone copy ~/calyx-backups/daily/<day> calyx-b2-crypt:daily/<day>` (no `sync`, so nothing is ever deleted
   remotely by the server).
5. The env file's two encryption keys never go to B2; Syed keeps them separately.
6. A quarterly restore test from B2 into the same throwaway containers as Section 3.

---

## What was proven on the laptop

Windows laptop, Docker Desktop, the dev stack (dev data only), Git Bash:

- `bash scripts/backup-calyx.sh` → `BACKUP OK` in 18 s: Postgres dump and roles, all 16 ClickHouse tables,
  `counts.tsv`, `SHA256SUMS` (35 files) verified, no `errors.log`.
- A failing table (an empty ClickHouse table, before the fix) left only `<day>.partial` with `errors.log`, the
  previous days untouched, exit 1; the next run replaced the `.partial`.
- 17 dated folders + one other folder → the newest 14 dated folders kept, the other folder untouched; a second
  run on the same day replaced that day.
- `CALYX_BACKUP_MIN_FREE_GB` above the free space → `FAILED`, nothing written or deleted, exit 1.
- The restore block above → `RESTORE TEST: PASS`: 17 Postgres and 16 ClickHouse tables, every row count equal
  (e.g. `pings` 91/91, `integrations` 10/10); the throwaway containers were gone afterwards; the live laptop
  stack (9 containers) was untouched and no `restore_test` database appeared in the live ClickHouse.
- On the laptop `TZ=Asia/Karachi` is ignored by Git Bash (the folder was named by UTC); on the Linux server it is
  Pakistan time.
