#!/usr/bin/env bash
set -euo pipefail

docker compose -f deploy/docker-compose.yml -f deploy/docker-compose.dev.yml up -d
