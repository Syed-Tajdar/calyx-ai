#!/usr/bin/env bash
# LAPTOP ONLY. Migration 000036 on a THROWAWAY Postgres container (never the stack DB, never the server):
# 35 -> 36 -> down -> up, then the role / RLS / function checks in verify-migration-36-transcription.sql.
# The container and its network are removed at the end.  Usage: bash scripts/verify-migration-36-transcription.sh
set -euo pipefail
export MSYS_NO_PATHCONV=1
SQL="$(cd "$(dirname "$0")" && (pwd -W 2>/dev/null || pwd))/verify-migration-36-transcription.sql"
NET=calyx-mig36-net
PG=calyx-mig36-pg
MIG="$(cd "$(dirname "$0")/.." && (pwd -W 2>/dev/null || pwd))/db/postgres/migrations"
cleanup() { docker rm -f "$PG" >/dev/null 2>&1 || true; docker network rm "$NET" >/dev/null 2>&1 || true; }
trap cleanup EXIT
cleanup
docker network create "$NET" >/dev/null
docker run -d --name "$PG" --network "$NET" -e POSTGRES_USER=callloom -e POSTGRES_PASSWORD=throwaway -e POSTGRES_DB=callloom pgvector/pgvector:pg16 >/dev/null
for i in $(seq 1 60); do docker exec "$PG" pg_isready -U callloom -d callloom >/dev/null 2>&1 && break; sleep 1; done
sleep 2
DB="postgres://callloom:throwaway@$PG:5432/callloom?sslmode=disable"
mig() { docker run --rm --network "$NET" -v "$MIG:/migrations:ro" migrate/migrate:v4.17.1 -path=/migrations -database "$DB" "$@" 2>&1 | tail -n 3; }
echo "== up to 35"; mig goto 35
echo "== up 1 (36)"; mig up 1
echo "== version"; mig version
echo "== down 1"; mig down 1
docker exec "$PG" psql -U callloom -d callloom -tAc "SELECT count(*) FROM information_schema.tables WHERE table_name IN ('transcription_jobs','call_recordings','call_transcripts','call_transcript_utterances','recording_access_audit','transcription_usage_daily','transcription_settings')" | sed 's/^/tables after down: /'
docker exec "$PG" psql -U callloom -d callloom -tAc "SELECT count(*) FROM information_schema.columns WHERE table_name='integrations' AND column_name LIKE 'recording_access%'" | sed 's/^/integrations columns after down: /'
docker exec "$PG" psql -U callloom -d callloom -tAc "SELECT count(*) FROM pg_proc WHERE proname IN ('claim_transcription_job','get_call_transcript_latest','list_transcription_tenants','recording_access_audit_immutable')" | sed 's/^/functions after down: /'
echo "== up 1 again"; mig up 1
echo "== version"; mig version
echo "== checks"
docker cp "$SQL" "$PG:/tmp/mig36_test.sql"
docker exec "$PG" psql -U callloom -d callloom -v ON_ERROR_STOP=1 -q -f /tmp/mig36_test.sql 2>&1 | grep -vE '^\s*$|set_config|^-+$|\(1 row\)'
