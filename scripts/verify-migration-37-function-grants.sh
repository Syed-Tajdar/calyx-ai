#!/usr/bin/env bash
# LAPTOP ONLY. Migration 000037 on a THROWAWAY Postgres container (never the stack DB, never the server):
# up to 36 (ACLs saved) -> 37 up -> checks -> down (ACLs must equal the saved 36 state) -> up -> checks.
# The container and its network are removed at the end.  Usage: bash scripts/verify-migration-37-function-grants.sh
set -euo pipefail
export MSYS_NO_PATHCONV=1
SQL="$(cd "$(dirname "$0")" && (pwd -W 2>/dev/null || pwd))/verify-migration-37-function-grants.sql"
NET=calyx-mig37-net
PG=calyx-mig37-pg
MIG="$(cd "$(dirname "$0")/.." && (pwd -W 2>/dev/null || pwd))/db/postgres/migrations"
cleanup() { docker rm -f "$PG" >/dev/null 2>&1 || true; docker network rm "$NET" >/dev/null 2>&1 || true; }
trap cleanup EXIT
cleanup
docker network create "$NET" >/dev/null
docker run -d --name "$PG" --network "$NET" -e POSTGRES_USER=callloom -e POSTGRES_PASSWORD=throwaway -e POSTGRES_DB=callloom pgvector/pgvector:pg16 >/dev/null
for i in $(seq 1 60); do docker exec "$PG" pg_isready -U callloom -d callloom >/dev/null 2>&1 && break; sleep 1; done
sleep 2
DB="postgres://callloom:throwaway@$PG:5432/callloom?sslmode=disable"
mig() { docker run --rm --network "$NET" -v "$MIG:/migrations:ro" migrate/migrate:v4.17.1 -path=/migrations -database "$DB" "$@" 2>&1 | tail -n 2; }
psqlq() { docker exec "$PG" psql -U callloom -d callloom -tAc "$1"; }
# Canonical grants (order-independent): one line per function, its sorted "grantee:privilege" list.
ACLS="SELECT string_agg(line, E'\n' ORDER BY line) FROM (
  SELECT p.proname || '(' || pg_get_function_identity_arguments(p.oid) || ') ' ||
         (SELECT string_agg(g, ',' ORDER BY g) FROM (
            SELECT CASE WHEN a.grantee = 0 THEN 'PUBLIC' ELSE a.grantee::regrole::text END || ':' || a.privilege_type AS g
              FROM aclexplode(COALESCE(p.proacl, acldefault('f', p.proowner))) a) x) AS line
    FROM pg_proc p WHERE p.prosecdef AND p.pronamespace = 'public'::regnamespace) s"
run_checks() {
  docker cp "$SQL" "$PG:/tmp/mig37.sql"
  docker exec "$PG" psql -U callloom -d callloom -v ON_ERROR_STOP=1 -q -1 -f /tmp/mig37.sql 2>&1 \
    | grep -vE '^\s*$|^-+$|\(1 row\)|^ *result *$' 
}
echo "== up to 36"; mig goto 36
before=$(psqlq "$ACLS")
echo "   older functions callloom_ingestion could run only through PUBLIC at 36: $(psqlq "SELECT count(*) FROM pg_proc p WHERE p.prosecdef AND p.pronamespace='public'::regnamespace AND has_function_privilege('callloom_ingestion', p.oid, 'EXECUTE') AND NOT EXISTS (SELECT 1 FROM aclexplode(p.proacl) a WHERE a.grantee = 'callloom_ingestion'::regrole)")"
echo "== up 1 (37)"; mig up 1
echo "== checks at 37"; run_checks
echo "== down 1"; mig down 1
after_down=$(psqlq "$ACLS")
if [ "$before" = "$after_down" ]; then echo "PASS down restores the exact 36 ACLs"; else echo "FAIL down differs from 36:"; diff <(echo "$before") <(echo "$after_down") || true; exit 1; fi
echo "== up 1 again"; mig up 1
echo "== version"; mig version
docker exec "$PG" psql -U callloom -d callloom -tAc "DELETE FROM users WHERE email = 'grants@example.test'; DELETE FROM integrations WHERE id = '37373737-0000-4000-8000-0000000000c1'; DELETE FROM tenants WHERE id = '37373737-0000-4000-8000-000000000001';" >/dev/null
echo "== checks after up again"; run_checks
