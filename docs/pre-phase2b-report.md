# Pre-Phase-2b report — source integrity + approved cleanup

Scope held: no Phase 2, API sync left paused, no write-back behaviour change, no deletion other than the two approved statements.

## 1 — Git integrity

`HEAD` is `9a9da2a`. The running images were built from the uncommitted working tree, not from `HEAD`. That uncommitted work is the write-back path, the `has_converted` column, the read-time test-data exclusion, and the Pre-Phase-2 script changes. It is listed below and committed in step 7.

### `config.go` vs `a67e7a6` (last commit of this file)

The diff is additive. `Load()` still returns `}, nil` and the function-closing brace (those tokens match `HEAD`; they were restored earlier and are not a remaining deletion). No config field, env read, or default was removed.

Added only:

- `WritebackEnabled` / `WritebackAuthToken`
- `WRITEBACK_ENABLED` (`true` or `1`, default `false`)
- `WRITEBACK_INTERNAL_TOKEN` via `os.Getenv`
- alignment whitespace on existing fields

`Port`, `DatabaseURL`, `RedisURL`, Kafka, encryption keys, `SyncInterval`, `DedupTTL`, and `MockMode` are still read and returned.

### Deep view vs `9a9da2a`

`services/api/src/native-integration/deep-query.service.ts` only adds `has_converted`:

- `CallDeepViewPing.has_converted: boolean`
- `DeepViewRow.has_converted: string`
- outer `toString(p.has_converted) AS has_converted`
- inner `argMax(has_converted, received_at) AS has_converted`
- mapper `Number(row.has_converted ?? 0) === 1`

No other lines were deleted. Integrity check did not stop the rebuild.

### Working tree at the start of this step

Modified: `.env.example`, `README.md`, `deploy/docker-compose.yml`, `docs/README.md`, `docs/enterprise-roadmap.md`, `packages/pingdedup/content.go`, `packages/pingdedup/content_test.go`, `scripts/smoke-test.ps1`, `scripts/verify-api-sync.ps1`, `scripts/verify-ingestion-slice.ps1`, `scripts/verify-system.ps1`, `scripts/ws-smoke-test.mjs`, `services/api/src/common/demo-rows.spec.ts`, `services/api/src/common/demo-rows.ts`, `services/api/src/common/tenant.decorator.ts`, `services/api/src/integrations/integrations.controller.ts`, `services/api/src/integrations/integrations.service.ts`, `services/api/src/native-integration/deep-query.service.ts`, `services/consumer/internal/models/ping.go`, `services/consumer/internal/writer/writer.go`, `services/consumer/pkg/clickhouse/clickhouse.go`, `services/platform-sync/cmd/server/main.go`, `services/platform-sync/internal/adapter/ringba_adapter.go`, `services/platform-sync/internal/adapter/ringba_adapter_test.go`, `services/platform-sync/internal/config/config.go`, `services/platform-sync/internal/models/models.go`, `services/platform-sync/internal/producer/producer.go`, `services/platform-sync/internal/store/integrations.go`, `services/platform-sync/internal/syncer/syncer.go`, `web/src/components/native/CallDeepViewPanel.tsx`, `web/src/lib/api.ts`, `web/src/lib/use-hybrid-ping-feed.ts`.

Untracked: `calyx-AI.code-workspace`, ClickHouse `000012_pings_has_converted.up.sql`, Postgres `000024`–`000026` up/down, `docs/evidence/pre-phase2/`, `docs/evidence/pre-phase2b/`, Ringba write-back evidence dirs, Pre-Phase-2 and write-back specs/reports, `docs/pre-phase2b-spec.md`, platform-sync `probe-ringba-*` commands, `services/platform-sync/internal/writeback/`.

## 2 — Rebuild

`docker compose build api platform-sync web consumer ingestion` exited 0. Each image was named `latest`:

| Image | Manifest list |
|---|---|
| deploy-api | `sha256:76fad77f25fad748414b5ce771a1cffa2963c762a1c398df9ee5658984a79031` |
| deploy-platform-sync | `sha256:94f424ecc6318fdb0073c76a5dbc034c5fe7fb5601adc7af57a0c6725ddb3982` |
| deploy-web | `sha256:fbb0be48243150a1f75885c8f2e7d60d772510fd026bdc4dc8e93992b6e2c518` |
| deploy-consumer | `sha256:2612dc88691cc608811b872a9d13c6d131be0e3b9ee10264dc9265e3b8374e92` |
| deploy-ingestion | `sha256:d6aae33f71d28a31f128e6ac2981083f95455cb8ce96cad16cb92749a832139b` |

API compile steps were cache hits (that source had already been built). platform-sync and ingestion compiled in this build. Consumer compiled; its final binary copy layer was a cache hit. `docker compose up -d --force-recreate` then started api, platform-sync, web, consumer, and ingestion (exit 0).

## 3 — Write path on the rebuilt platform-sync

No Ringba write.

| Probe | Result |
|---|---|
| `POST :8088/writeback/convert`, no Authorization | **401** |
| wrong Bearer | **401** |
| valid Bearer, empty JSON | **400** — body `integration_id, tenant_id, and inbound_call_id required` |
| `printenv WRITEBACK_ENABLED` | **true** |

## 4 — Web live-feed filter

Rebuilt client chunk `/app/.next/static/chunks/487-61d14fd905a88647.js` contains `apisync-`, `demo-`, and `smoke-`. That matches `web/src/lib/use-hybrid-ping-feed.ts`, which hides those prefixes unless `show_demo` is set.

## 5 — Approved deletions

Pre-run counts matched, so both statements ran. Nothing else was deleted.

| Family | Before | After mutation |
|---|---|---|
| old smoke (14 ids) | 15 rows / 14 calls | 0 |
| `apisync-%` | 3 | 0 |
| `demo-%` | 53 | 53 |
| `smoke-%` | 10 rows / 9 calls | 10 rows / 9 calls |
| Real Ringba `RGB%` and `sync_path='api_sync'` | 4100 | 4100 |

Mutations `mutation_39.txt` (id list) and `mutation_40.txt` (`apisync-%`) both `is_done=1`, empty `latest_fail_reason`.

Default KPI (distinct calls, three-prefix exclusion, `sumIf` of `argMax` revenue):

| | Before | After |
|---|---|---|
| `total_pings` | 4114 | 4100 |
| revenue | 19222.76 | 19027.76 |

Drop is 14 calls and $195.00, the old-smoke `argMax` sum. The after snapshot is immediately after the mutations. Step 6 then inserted new `smoke-` rows; those stay excluded from the default KPI and were not deleted.

## 6 — Gates

| Script | Exit | Notes |
|---|---|---|
| `scripts/verify-system.ps1` | **0** | Docker 15/15, Healthz 11/11, API 20/20, web 13/13 |
| `scripts/verify-ingestion-slice.ps1` | **0** | `show_demo=true` 4167→4168, default view 4100→4100 |
| `scripts/verify-presentation.ps1` | **0** | fixture pings 4100, `demo-call-1` deep view / score / transcript pass |

## 7 — Commit

`5838d804feba75287f67c624473fdc3f6a53f605` on `main`. 100 files. `git status` after that commit was clean. This paragraph is the only follow-up, so the report itself records the hash.

Phase 2 was not started. API sync was not turned live.
