#!/usr/bin/env bash
set -euo pipefail

BROKER="${KAFKA_BROKER:-redpanda:9092}"
CONTAINER="$(docker compose -f deploy/docker-compose.yml ps -q redpanda)"

TOPICS=(
  "callloom.pings.v1"
  "callloom.calls.call_recording_ready.v1"
  "callloom.calls.call_transcript_ready.v1"
  "callloom.calls.call_scored.v1"
  "callloom.scoring.dlq.v1"
  "callloom.enrichment.dlq.v1"
)

for topic in "${TOPICS[@]}"; do
  docker exec "$CONTAINER" \
    rpk topic create "$topic" --brokers "$BROKER" --partitions 3 --replicas 1 || true
done
