#!/usr/bin/env bash
set -euo pipefail

TOPIC="${KAFKA_TOPIC_PINGS:-callloom.pings.v1}"

docker exec "$(docker compose -f deploy/docker-compose.yml ps -q redpanda)" \
  rpk topic create "$TOPIC" --partitions 3 --replicas 1 || true
