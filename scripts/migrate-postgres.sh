#!/usr/bin/env bash
set -euo pipefail

DATABASE_URL="${DATABASE_URL:-postgres://callloom:callloom_dev@localhost:5432/callloom?sslmode=disable}"

docker run --rm \
  -v "$(pwd)/db/postgres/migrations:/migrations" \
  --network host \
  migrate/migrate:v4.17.1 \
  -path=/migrations \
  -database "$DATABASE_URL" \
  up
