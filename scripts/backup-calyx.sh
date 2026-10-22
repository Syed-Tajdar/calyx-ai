#!/usr/bin/env bash
# Daily Calyx backup — Postgres and ClickHouse, on the server. See docs/ops-daily-backups.md.
#
#   bash scripts/backup-calyx.sh          # by hand, from ~/deployments/calyx-ai
#   (cron: see the doc)
#
# Writes ~/calyx-backups/daily/<YYYY-MM-DD, Pakistan time>/ (folders 700, files 600):
#   postgres-callloom.dump    pg_dump -Fc of the callloom database (one consistent snapshot)
#   postgres-roles.sql        pg_dumpall --roles-only (role names + password hashes, no plaintext)
#   clickhouse/<table>.sql    SHOW CREATE TABLE for every MergeTree table in the database
#   clickhouse/<table>.native.gz   SELECT * FORMAT Native, gzip — one query per table, so each
#                             table is one consistent snapshot of its parts; the service keeps running
#   counts.tsv                rows per table, counted from the dumps themselves (restore test target)
#   SHA256SUMS                checksums of every file above
#
# Safety:
#   - read-only on the databases: dumps and SELECTs through `docker exec`, nothing is stopped,
#     no host port is used, no compose command is run (no `config`, never `down`, never `-v`);
#   - the ClickHouse password stays inside the container: clickhouse-client reads
#     CLICKHOUSE_PASSWORD from the container's own environment, it is never on a command line;
#     Postgres uses the container's local socket. No secret is printed;
#   - stops before writing if free disk is under CALYX_BACKUP_MIN_FREE_GB (default 10);
#   - builds the day in "<day>.partial" and renames it only when everything is verified, so a
#     failed run never replaces a good backup and never counts towards the 14 kept days;
#   - keeps the newest CALYX_BACKUP_KEEP_DAYS (default 14) complete days and deletes older ones,
#     only folders named YYYY-MM-DD inside the backup root.
#
# Prints only sizes and OK / FAILED. Error details go to <day>.partial/errors.log (600).
# Exit code 0 = backup complete and verified, 1 = failed (the previous days are untouched).

set -uo pipefail
umask 077
export PATH="/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:$PATH"

BACKUP_ROOT="${CALYX_BACKUP_ROOT:-$HOME/calyx-backups/daily}"
KEEP_DAYS="${CALYX_BACKUP_KEEP_DAYS:-14}"
MIN_FREE_GB="${CALYX_BACKUP_MIN_FREE_GB:-10}"
PG_CONTAINER="${CALYX_PG_CONTAINER:-deploy-postgres-1}"
CH_CONTAINER="${CALYX_CH_CONTAINER:-deploy-clickhouse-1}"
PG_USER="${CALYX_PG_USER:-callloom}"
PG_DB="${CALYX_PG_DB:-callloom}"
CH_DB="${CALYX_CH_DB:-default}"

DAY="$(TZ=Asia/Karachi date +%F)"
FINAL="$BACKUP_ROOT/$DAY"
WORK="$BACKUP_ROOT/$DAY.partial"
ERR="$WORK/errors.log"
START=$(date +%s)

say()  { printf '%-34s %s\n' "$1" "$2"; }
size() { du -sh "$1" 2>/dev/null | cut -f1; }
fail() {
  say "$1" "FAILED"
  echo "BACKUP FAILED ($DAY) — details: $ERR"
  exit 1
}

mkdir -p "$BACKUP_ROOT" && chmod 700 "$BACKUP_ROOT" || { echo "BACKUP FAILED: cannot create $BACKUP_ROOT"; exit 1; }
parent="$(dirname "$BACKUP_ROOT")"
[ "$(basename "$parent")" = "calyx-backups" ] && chmod 700 "$parent"

# One run at a time (cron and a manual run must not overlap).
if command -v flock >/dev/null 2>&1; then
  exec 9>"$BACKUP_ROOT/.lock"
  flock -n 9 || { echo "BACKUP FAILED: another backup is running"; exit 1; }
fi

# Free disk, before anything is written.
avail_kb=$(df -Pk "$BACKUP_ROOT" | awk 'NR==2 {print $4}')
avail_gb=$(( ${avail_kb:-0} / 1024 / 1024 ))
if [ "${avail_kb:-0}" -lt $(( MIN_FREE_GB * 1024 * 1024 )) ]; then
  say "free disk ${avail_gb} GB (min ${MIN_FREE_GB} GB)" "FAILED"
  echo "BACKUP FAILED ($DAY) — not enough free disk; nothing was written"
  exit 1
fi
say "free disk before" "${avail_gb} GB OK"

rm -rf "$WORK"
mkdir -p "$WORK/clickhouse" && chmod 700 "$WORK" "$WORK/clickhouse" || { echo "BACKUP FAILED: cannot create $WORK"; exit 1; }
: > "$ERR"

# Both containers must be running; nothing is started or restarted here.
for c in "$PG_CONTAINER" "$CH_CONTAINER"; do
  [ "$(docker inspect -f '{{.State.Running}}' "$c" 2>>"$ERR")" = "true" ] || fail "container $c running"
done

# ---- Postgres ---------------------------------------------------------------------------
docker exec "$PG_CONTAINER" pg_dump -U "$PG_USER" -d "$PG_DB" -Fc > "$WORK/postgres-callloom.dump" 2>>"$ERR" \
  || fail "postgres dump"
docker exec -i "$PG_CONTAINER" pg_restore --list < "$WORK/postgres-callloom.dump" > /dev/null 2>>"$ERR" \
  || fail "postgres dump readable"
say "postgres dump" "$(size "$WORK/postgres-callloom.dump") OK"

docker exec "$PG_CONTAINER" pg_dumpall -U "$PG_USER" --roles-only > "$WORK/postgres-roles.sql" 2>>"$ERR" \
  || fail "postgres roles"
say "postgres roles" "$(size "$WORK/postgres-roles.sql") OK"

# Postgres row counts: the live tables, counted right after the dump. (Counting inside the dump
# would need a restore, which is the separate restore test in the doc.) A table written during
# those seconds can differ by a few rows; the restore test says how to read that.
docker exec "$PG_CONTAINER" psql -U "$PG_USER" -d "$PG_DB" -XAtq -F $'\t' -c \
  "SELECT 'postgres', table_name, (xpath('/row/c/text()', query_to_xml(format('SELECT count(*) AS c FROM %I.%I', table_schema, table_name), false, true, '')))[1]::text
     FROM information_schema.tables
    WHERE table_schema = 'public' AND table_type = 'BASE TABLE'
    ORDER BY table_name" > "$WORK/counts.tsv" 2>>"$ERR" \
  || fail "postgres row counts"

# ---- ClickHouse -------------------------------------------------------------------------
tables=$(docker exec "$CH_CONTAINER" clickhouse-client --query \
  "SELECT name FROM system.tables WHERE database = '$CH_DB' AND engine LIKE '%MergeTree' AND NOT is_temporary ORDER BY name FORMAT TSV" 2>>"$ERR") \
  || fail "clickhouse table list"
[ -n "$tables" ] || fail "clickhouse table list (empty)"

for t in $tables; do
  case "$t" in *[!A-Za-z0-9_]*) echo "unexpected table name: $t" >>"$ERR"; fail "clickhouse $t";; esac
  docker exec "$CH_CONTAINER" clickhouse-client --query "SHOW CREATE TABLE \`$CH_DB\`.\`$t\` FORMAT TSVRaw" \
    > "$WORK/clickhouse/$t.sql" 2>>"$ERR" || fail "clickhouse $t schema"
  docker exec "$CH_CONTAINER" clickhouse-client --query "SELECT * FROM \`$CH_DB\`.\`$t\` FORMAT Native" 2>>"$ERR" \
    | gzip -1 > "$WORK/clickhouse/$t.native.gz"
  st=("${PIPESTATUS[@]}")   # copy at once: the next command resets PIPESTATUS
  [ "${st[0]}" -eq 0 ] && [ "${st[1]}" -eq 0 ] || fail "clickhouse $t dump"
  gzip -t "$WORK/clickhouse/$t.native.gz" 2>>"$ERR" || fail "clickhouse $t dump readable"
  if [ "$(gzip -dc "$WORK/clickhouse/$t.native.gz" | head -c 1 | wc -c)" -eq 0 ]; then
    rows=0   # an empty table dumps to an empty Native stream
  else
    rows=$(gzip -dc "$WORK/clickhouse/$t.native.gz" 2>>"$ERR" \
      | docker exec -i "$CH_CONTAINER" clickhouse-local --input-format Native --query "SELECT count() FROM table" 2>>"$ERR")
  fi
  case "$rows" in ''|*[!0-9]*) fail "clickhouse $t dump readable";; esac
  printf 'clickhouse\t%s\t%s\n' "$t" "$rows" >> "$WORK/counts.tsv"
  say "clickhouse $t" "$(size "$WORK/clickhouse/$t.native.gz") OK"
done

# ---- Seal: checksums, permissions, rename, retention ------------------------------------
( cd "$WORK" && find . -type f ! -name SHA256SUMS ! -name errors.log -print0 | sort -z | xargs -0 sha256sum > SHA256SUMS ) 2>>"$ERR" \
  || fail "checksums"
find "$WORK" -type d -exec chmod 700 {} + 2>>"$ERR"
find "$WORK" -type f -exec chmod 600 {} + 2>>"$ERR"

if [ -d "$FINAL" ]; then rm -rf "$FINAL.old" && mv "$FINAL" "$FINAL.old"; fi   # same day re-run
mv "$WORK" "$FINAL" || fail "finalize $FINAL"
rm -rf "$FINAL.old"
[ -s "$FINAL/errors.log" ] || rm -f "$FINAL/errors.log"
ERR="$FINAL/errors.log"

# Keep the newest KEEP_DAYS complete days; only YYYY-MM-DD folders, nothing else is touched.
old=$(find "$BACKUP_ROOT" -mindepth 1 -maxdepth 1 -type d -regextype posix-extended \
        -regex '.*/[0-9]{4}-[0-9]{2}-[0-9]{2}' -printf '%f\n' 2>/dev/null | sort -r | tail -n +$(( KEEP_DAYS + 1 )))
for d in $old; do rm -rf "${BACKUP_ROOT:?}/$d"; done
find "$BACKUP_ROOT" -mindepth 1 -maxdepth 1 -type d -name '*.partial' -mmin +1440 -exec rm -rf {} + 2>/dev/null

kept=$(find "$BACKUP_ROOT" -mindepth 1 -maxdepth 1 -type d -regextype posix-extended \
         -regex '.*/[0-9]{4}-[0-9]{2}-[0-9]{2}' 2>/dev/null | wc -l)
avail_kb=$(df -Pk "$BACKUP_ROOT" | awk 'NR==2 {print $4}')
say "this backup" "$(size "$FINAL") OK"
say "all backups (${kept} days kept, max ${KEEP_DAYS})" "$(size "$BACKUP_ROOT")"
say "free disk after" "$(( ${avail_kb:-0} / 1024 / 1024 )) GB"
echo "BACKUP OK ($DAY, $(( $(date +%s) - START )) s) — $FINAL"
exit 0
