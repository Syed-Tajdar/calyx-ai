#!/usr/bin/env bash
set -euo pipefail

CLICKHOUSE_HOST="${CLICKHOUSE_HOST:-localhost}"
CLICKHOUSE_PORT="${CLICKHOUSE_PORT:-9000}"

for file in db/clickhouse/migrations/*_table.up.sql; do
  echo "Applying $file"
  docker exec -i "$(docker compose -f deploy/docker-compose.yml ps -q clickhouse)" \
    clickhouse-client --multiquery < "$file"
done
