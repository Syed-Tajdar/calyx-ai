# Calyx server — host ports and neighbours

**Server:** `calyx.ridgerisemedia.com` · 8 GB RAM, shared by **Calyx, Plane and trackitin**
**Source:** Aryan's Step 0.2 output (`docs/deploy-v1-sync-health.md`) on 2026-10-22. Update this file whenever a
deploy pack's port check shows something new.

## Rule

**Calyx never publishes a host port except `127.0.0.1:3100` (api) and `127.0.0.1:3101` (web)** — the two nginx
proxies to. Everything else is reached inside the compose network (`postgres:5432`, `redis:6379`, …) or through
`docker exec`. Any new host port (a new service, a probe, a debug port) must be checked against the table below
**before** it goes into `deploy/docker-compose.yml` or a deploy pack. On 2026-10-18 a collision with Plane's
redis on 6379 stopped every sync for three days.

`deploy/docker-compose.dev.yml` (laptop only) publishes more ports for the laptop gates; it is never used on the
server.

## Who holds which host port

| Owner | Container / service | Host port(s) |
|---|---|---|
| **Calyx** (after Part 1) | `deploy-api-1` | `127.0.0.1:3100` |
| | `deploy-web-1` | `127.0.0.1:3101` |
| | postgres, redis, redpanda, clickhouse, ingestion, consumer, platform-sync | none (compose network only) |
| **Plane** | web | `127.0.0.1:3000` |
| | admin | `3001` |
| | space | `3002` |
| | plane-live | `3003` |
| | api | `8000` |
| | plane-minio | `9000` |
| | plane-db (postgres), plane-redis, plane-mq | none (internal only) |
| **Other redis containers** | redis-dnis_map | `7001`, `7002` |
| | redis-publisher | `7003`, `7004` |
| | redis-buyer | `7005`, `7006` |
| | redis-campaign | `7007`, `7008` |
| | redis-state | `7101`, `7102` |
| **trackitin** | SIP | `5060/tcp`, `5060/udp`, `5090-5092/udp` |
| | (HTTP) | `18080` |
| | RTP | `30000-30200/udp` |
| | trackitin-nest-core | `3010` |
| | trackitin-next-frontend | `3011` |
| | trackitin-postgres | `5433` |
| | trackitin-redis | `6380` |

Where the list above shows no `127.0.0.1`, Aryan's output did not show the bind address; treat those ports as
taken on every address.

## Notes

- Plane's `plane-db` and `plane-redis` have no host port today. If Plane ever publishes 5432 or 6379 again,
  nothing in Calyx collides any more (Calyx publishes neither since Part 0 / Part 1).
- `3000` is Plane's web; Calyx's containers listen on 3000 **inside** their own network namespace only
  (published as 3100/3101), which is why the old "avoid 3000–3003" comments in the compose file exist.
- Memory: 8 GB for all three stacks. Anything new that runs on the server — a throwaway restore test
  (`docs/ops-daily-backups.md`, ~3 GB while it runs), a 2D probe, an extra service — must check `free -h` first.
