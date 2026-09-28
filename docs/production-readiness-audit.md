# Calyx — Production-Readiness Audit (READ-ONLY)

**Date:** 2026-09-25  
**Spec:** `docs/production-readiness-audit-spec.md`  
**Scope:** all listed services, `packages/pingdedup`, compose/Dockerfiles/migrations/CI/scripts.  
**Constraint:** no code/config/data/integration changes; no Ringba writes; no Phase 2B; no push.

Evidence is from source (`file:line`) and read-only commands (`govulncheck`, `npm audit`). Previous reports were context only. Docker daemon was unavailable for live `rpk`/`docker inspect` (see §7).

Late pass reconciled additional evidence from [Go pipeline audit](ff0bd340-f6ef-43f6-91eb-f8ec7d8ae5e5), [platform-sync audit](55f27cd5-7b0f-4ddd-b8c4-588ab794638f), [API/web audit](9f653c60-630c-427d-88a8-b41aebaaf5a1), and [infra audit](675f4cf9-d6f7-4e1b-9d0f-dfca23a5c17e) — notably **K22 corrected** and F066–F071 added.

---

## 1. Findings table

| ID | Class | Service | `file:line` | Finding | Label | Severity | Fix shape | Effort |
|---|---|---|---|---|---|---|---|---|
| F001 | D | scoring, enrichment, compliance-intelligence, ad-attribution | `services/scoring/internal/consumer/consumer.go:72-77` (same pattern siblings) | Handle error is logged then offset **always committed** — transient CH/API failure permanently skips the message | [PROVEN] | **P0** | Commit only on success; retry/backoff or pause; DLQ only after N failures | M |
| F002 | D/E | scoring (+ siblings) | `services/scoring/pkg/clickhouse/clickhouse.go:61-66`; migrations `000002` DLQ | CH DLQ stores **payload_hash only** — cannot replay; `_ = InsertDLQ` then commit → CH down = silent loss | [PROVEN] | **P0** | Persist raw bytes (Kafka DLQ value); do not commit if primary write and DLQ both fail | M |
| F003 | A | platform-sync | `services/platform-sync/internal/adapter/ringba_adapter.go:64-88`, `:111-124` | After success, next cursor stores absolute `[ReportStart,ReportEnd]`; next tick `DecodeCursor` **overwrites** fresh `last_sync−overlap→now` — frozen window can **miss new calls** | [PROVEN] | **P0** | Ignore stored times; always compute window from `last_sync_at` (+ overlap) to `now` | M |
| F004 | N/M | ask-ai | `services/ask-ai/internal/handlers/ask.go:24-76`; `cmd/server/main.go:74-75` | `/v1/ask` and `/v1/history` **unauthenticated**; `tenant_id` from body/query; compose publishes `:8086` | [PROVEN] | **P0** | Internal auth (token/mTLS); bind tenant from verified identity only | M |
| F005 | M/N | ask-ai | `services/ask-ai/internal/nlsql/templates.go:9-27` | `tenant_id` string-interpolated into SQL; validator is substring-only — quote breakout can bypass isolation | [PROVEN] | **P0** | Parameterized queries; inject filter server-side after UUID validate | M |
| F006 | N/M | providerconfig | `services/providerconfig/internal/handlers/resolve.go:22-47`; `cmd/server/main.go:94` | `POST /v1/resolve` **unauthenticated**; client-supplied `tenant_id`; can decrypt secrets when non-mock; `:8084` | [PROVEN] | **P0** | Network-internal + shared secret; never trust client tenant without caller auth | M |
| F007 | N | api | `services/api/src/auth/auth.controller.ts:16-19` | Login has **no rate limit / lockout** | [PROVEN] | **P0** | IP+email throttle (Nest Throttler / Redis) | S |
| F008 | M/N | api | `services/api/src/scoring/scoring.controller.ts:10-11,57-73` | Deepgram callback trusts body `tenant_id`; secret check **skipped when `MOCK_MODE`** (compose default true) | [PROVEN] | **P0** | Always require secret; bind tenant from signed claim, not body | M |
| F009 | N | web | `web/src/lib/auth.ts:4-14` | JWT stored in `localStorage` / `sessionStorage` (XSS → full account) | [PROVEN] | **P0** | httpOnly Secure cookie + CSRF strategy | M |
| F010 | U/N | api / infra | `deploy/docker-compose.yml:218-241`; `services/api/src/config/env.ts` | Dev JWT / writeback token / encryption keys / `MOCK_MODE=true` in compose — copy-to-prod ships secrets | [PROVEN] | **P0** | Fail boot without prod secrets; no insecure defaults | S |
| F011 | N | platform-sync | `services/platform-sync/cmd/server/main.go:79-84`; compose `:396-397` | **K2:** `POST /sync` and `POST /test-auth` unauthenticated on published `:8088` | [PROVEN] | **P0** | Require same Bearer (or mTLS) as writeback; bind network-internal | S |
| F012 | A | platform-sync | `services/platform-sync/internal/adapter/retreaver_adapter.go:144-149`, `:118-125` | **K1:** `page`/`per_page` over changing `updated_at` desc window — skip/dup under live updates | [PROVEN] | **P1** | Keyset cursor (`updated_at`+id) or larger overlap + tolerate via content-hash | M |
| F013 | D | consumer | `services/consumer/internal/consumer/consumer.go:222-234`, `:163-199` | **K5:** poison = substring deny-list; unmatched insert errors retry with **no limit** | [PROVEN] | **P1** | Cap retries; unknown → DLQ + alert after N | M |
| F014 | D | consumer | `services/consumer/internal/consumer/consumer.go:203-215`, `:249-254` | **K6:** poison skip log-only; offset committed; **no payload DLQ** | [PROVEN] | **P1** | Produce full message to DLQ topic before commit | S |
| F015 | D/G | consumer (+ most Go services) | `consumer.go:91-93`; `main.go:43-59,74-78`; compose `:210` | Transient flush error exits `Run` while HTTP `/healthz`/`/readyz` stay green; only **api** has `restart: unless-stopped` | [PROVEN] | **P0** | Don’t exit on transient; consumer-alive in readyz; `restart: unless-stopped` on all app services | S |
| F016 | D/T | consumer | `services/consumer/internal/consumer/consumer.go:47-49`; `cmd/server/main.go:43-59` | `PoisonCount` unused by HTTP; no lag/poison metrics on health | [PROVEN] | **P1** | Expose counter on `/metrics` or `/readyz`; alert | S |
| F017 | G | api | `services/api/src/database/postgres.service.ts:14-37,56-61`; `clickhouse.service.ts:6-25` | **K4:** readyz PG/CH clients pin IPv4 via `getent` at boot — stale after IP change | [PROVEN] | **P1** | Re-resolve on readyz failure or TTL refresh | M |
| F018 | H | api | `services/api/src/main.ts:8-24` | No `enableShutdownHooks()` — SIGTERM may skip Nest `onModuleDestroy` (Kafka live drain) | [PROVEN] | **P1** | `app.enableShutdownHooks()` + HTTP drain timeout | S |
| F019 | H | platform-sync | `services/platform-sync/cmd/server/main.go:109-126` | SIGTERM: HTTP Shutdown only; sync ticker not cooperatively stopped; in-flight `RunAll` not cancelled; lock orphan until 15m TTL | [PROVEN] | **P1** | Cancel sync ctx on signal; wait/drain; release lock | M |
| F020 | N | ingestion | HMAC files + connectors | Webhook HMAC/token only — **no timestamp/nonce replay window** | [PROVEN] | **P1** | Signed timestamp ±skew + Redis nonce TTL | M |
| F021 | N | ingestion | `services/ingestion/internal/handlers/common.go:10-15` | Body read via `LimitReader` truncates — claimed hard reject path ineffective | [PROVEN] | **P1** | `http.MaxBytesReader` / reject when truncated | S |
| F022 | N | web | `web/next.config.mjs:1-5` | No CSP / security headers | [PROVEN] | **P1** | Add `headers()` CSP, frame-ancestors, referrer-policy | S |
| F023 | N | api | `services/api/src/main.ts:20` | `enableCors({ origin: true })` reflects any Origin | [PROVEN] | **P1** | Allowlist production web origins | S |
| F024 | N | api | many controllers e.g. `auth.controller.ts:18` | Raw Zod `.parse` → Nest 500 with issue dump (only scorecards use `parseZod`) | [PROVEN] | **P1** | Global Zod filter or always `parseZod` → 422 | S |
| F025 | N | web | `npm audit --omit=dev` | **Critical/high** Next.js advisories (incl. RCE-class advisories) + PostCSS/socket.io-parser high | [PROVEN] | **P1** | Upgrade Next / deps to patched releases | M |
| F026 | N | api | `npm audit --omit=dev` | **High:** multer DoS chain, socket.io-parser memory exhaustion | [PROVEN] | **P1** | Bump Nest platform-express / socket.io-parser | M |
| F027 | N | platform-sync | `services/platform-sync/Dockerfile:14-19` | Image runs as **root** (no `USER`) | [PROVEN] | **P1** | `USER nobody` (match other Go images) | S |
| F028 | B | platform-sync | `services/platform-sync/internal/syncer/sync_lock.go:18-20,64` | **K11:** sync timeout 10m vs lock TTL 15m; **no renewal/heartbeat** — crash holds lock 15m | [PROVEN] | **P1** | Heartbeat `PEXPIRE` while running; document margin | S |
| F029 | C | platform-sync / pingdedup | `packages/pingdedup/dedup.go:76-98`; `syncer.go:136-138` | Content-hash stored **before** Kafka publish — publish fail can suppress retry until content changes | [PROVEN] | **P1** | Set hash only after successful publish (or Lua CAS after ack) | M |
| F030 | U | intelligence stack | compose `MOCK_MODE: "true"`; scoring/enrichment/compliance configs | **K16:** mock defaults; UI shows mock TCPA/scores; compliance DNC non-mock silently uses MockProvider | [PROVEN] | **P1** | Prod must force `MOCK_MODE=false`; fail closed on unknown providers | S |
| F031 | F | scoring | `services/scoring/internal/scoring/engine.go:61-64` | `Score` ignores `ctx` — no AI timeout | [PROVEN] | **P1** | Honor context / hard timeout | S |
| F032 | F | enrichment | `services/enrichment/internal/providers/provider.go:22-50` | `Enrich` has no context/HTTP timeout (unbounded when real) | [PROVEN] | **P1** | Pass ctx + client timeouts | S |
| F033 | B | ask-ai | `services/ask-ai/internal/cost/budget.go:10-40` | Token budget is in-process map — multi-replica bypass | [PROVEN] | **P1** | Redis/Postgres budget counters | M |
| F034 | T | compliance | `services/compliance-intelligence/internal/consumer/consumer.go` publish ignores | `_ = publisher.Publish*` then mark processed — Kafka emit failure still commits | [PROVEN] | **P1** | Fail handle if publish fails; outbox | M |
| F035 | T | silent paths | consumer poison; writeback pending; sync skip; offset fallback | Logs only — **no counters/alerts** in-repo for poison skip, writeback pending, sync skip, Ringba offset fallback | [PROVEN] | **P1** | Metrics + alert rules for each silent-failure path | M |
| F036 | O | platform-sync | `writeback/convert.go:163-172`; `errors.go:32-34` | **K7:** 409 already-in-progress returns **before** `writeAudit` — no audit row | [PROVEN] | **P2** | Optional audit row for lock-contention outcomes | S |
| F037 | O | platform-sync | `writeback/convert.go:15-17,240` | **K10:** settle window 12s (tight vs observed ~10.4s convert) | [PROVEN] | **P2** | Raise settle window / make configurable | S |
| F038 | I | db / api | `db/clickhouse/migrations/000012_pings_has_converted.up.sql:1`; `deep-query.service.ts:289` | **K8:** `has_converted UInt8 DEFAULT 0` — cannot represent “not provided” | [PROVEN] | **P2** | Nullable or presence flag (product decision) | M |
| F039 | O | api | `integrations.service.ts:292-293` | **K9:** Nest 202 on pending coded; **not observed live** this audit (no writeback call) | [PROVEN code] / [UNKNOWN live] | **P2** | Integration test asserting 202 + pending body | S |
| F040 | W | web | `web/src/app/(app)/dashboard/page.tsx:154-158` | **K15:** “Live” badge = socket `connected`, not `/live-status` Kafka membership | [PROVEN] | **P2** | Combine socket + poll `/live-status` | S |
| F041 | V | scripts | `scripts/verify-ingestion-slice.ps1:77-79` | **K14:** asserts default KPI absolute unchanged — flakes under live sync | [PROVEN] | **P2** | Assert smoke prefix excluded, not absolute total | S |
| F042 | V | scripts | `scripts/verify-system.ps1` | **K18:** does not invoke `verify-api-sync.ps1` | [PROVEN] | **P2** | Add as gate step or document optional | S |
| F043 | K | Redis | `deploy/docker-compose.yml:38-44` | **K17:** `volatile-ttl` — under memory pressure TTL’d dedup keys evict early → republish storms | [PROVEN] | **P2** | Size Redis / monitor eviction; reserve headroom | M |
| F044 | A | platform-sync | `ringba_adapter.go:77-97`; `registry.go:100-110` | **K19:** `page1_refresh` idle for Ringba (HasMore=false) — **intended**; still active for deep-paging platforms | [PROVEN] | — | Document as intended; no code change required | — |
| F045 | S | CI | `.github/workflows/go.yml:24-28` | **K3:** root `go build ./...` — **no root `go.mod`**; workflow cannot pass; Go 1.20 vs Docker 1.22 | [PROVEN] | **P1** | Matrix per module or remove broken root job | S |
| F046 | P | ClickHouse | `db/clickhouse/migrations/000010_enrichment_attrs_replacing.up.sql:5-29` | **K13:** copy + rename write window on live table | [PROVEN] | **P1** (procedure) | Don’t re-run on live without freeze; use atomic engine alter where possible | M |
| F047 | P | ClickHouse | migrations 000003–000005 | Ups exist; **no `.down.sql`** | [PROVEN] | **P2** | Add downs or document irreversible | S |
| F048 | E | Kafka / infra | `deploy/docker-compose.yml:86-97`; `scripts/init-kafka-topic.sh:16-18` | Topics created partitions=3 replicas=1; **no retention knobs** in repo | [PROVEN] | **P1** | Set explicit retention ≥ max consumer outage; RF≥3 in prod | M |
| F049 | Q | Infra | compose / scripts | **No backup** config for Postgres/ClickHouse/Redis in repo | [PROVEN] | **P1** (deploy) | Document + automate backups before prod | L |
| F050 | G | platform-sync | `cmd/server/main.go:75-78` | Only `/healthz` always-ok; **no `/readyz`** (PG/Redis/Kafka unchecked) | [PROVEN] | **P1** | Add readyz with dependency pings | S |
| F051 | R | api / CH | `000001_pings_table.up.sql:17`; analytics/pings argMax queries | `ORDER BY (tenant_id, platform, received_at, …)` vs period filters on `occurred_at` / full-tenant argMax — large scans as data grows (Phase 2B status filter relevant) | [INFERRED] | **P2** | Projections / align ORDER BY for period + argMax | L |
| F052 | A | api | `pings.service.ts:110-111` | Offset pagination over live inserts can skip/dup | [PROVEN] | **P2** | Keyset cursor on `(event_at, id)` | M |
| F053 | J | web | `web/src/lib/date-filter.tsx:20-26,87` | Period “7d/30d” = browser local midnight, not tenant TZ | [PROVEN] | **P2** | Compute bounds in tenant TZ or explicit UTC | M |
| F054 | N | Go modules | `govulncheck ./...` each module | Multiple Go stdlib vulns on **go1.22.12** (crypto/tls, net/http, x509, …) — upgrade Go toolchain | [PROVEN] | **P1** | Rebuild on patched Go (≥ fixed minor) | M |
| F055 | C | pingdedup | `packages/pingdedup/dedup.go:44-98` | Redis errors **fail-OPEN** (documented) — duplicates under Redis outage | [PROVEN] | **P2** | Keep design; alert on fail-OPEN count; Redis HA | S |
| F056 | B | pingdedup | `packages/pingdedup/dedup.go:76-98` | GET-then-SET (no SETNX) — concurrent first writers both PublishNew | [PROVEN] | **P2** | Atomic compare-and-set Lua | S |
| F057 | W | enrichment | `providers/provider.go:53-58` | `whitepages` constructs EnrollHere provider (wrong constructor) | [PROVEN] | **P2** | Separate type or fail closed | S |
| F058 | W | scoring | `scoring/engine.go:37-40` | Idempotency keys `SET` TTL 0 (forever) vs Redis `volatile-ttl` — eviction → re-score | [PROVEN] | **P2** | Finite TTL aligned to retention | S |
| F059 | N | ingestion | `handlers/callloom_native.go`; `main.go:67` | Unauthenticated native list stub on public mux | [PROVEN] | **P2** | Auth gate or remove until ready | S |
| F060 | U | platform-sync | `config.go` MOCK_MODE; `main.go:72` | Compose `MOCK_MODE=true` logged but **unused** for adapter selection (`sync_mode` drives) — misleading | [PROVEN] | **P2** | Remove or wire; document sync_mode | S |
| F061 | W | K22 | `docs/evidence/ringba-writeback-probe/postman-collection.json` | **K22 changed:** Postman collection **present** (~1.4MB, git-tracked) — prior “missing” status was wrong | [PROVEN] | — | Keep path; refresh if stale vs live Ringba API | — |
| F062 | E | Kafka | live `rpk topic describe` | Could not read live retention/replication — Docker daemon down | [UNKNOWN] runtime | — | See §7 / §8 | — |
| F063 | O | writeback Phase 1 | convert.go, auth.go, 000026, main.go | Bearer, WRITEBACK_ENABLED, per-tenant flag, account bind, idempotency lock, settle outcomes, immutable audit, amount→400 — **all intact** | [PROVEN] | — | Keep; address K7/K10 separately | — |
| F064 | K | ClickHouse | `000001_pings_table.up.sql:18` | **K12:** `pings` TTL `received_at + 24 MONTH` — present | [PROVEN] | — | Confirm retention policy with ops | — |
| F065 | N | api K21 | `Dockerfile:26`; compose `:216` | **K21:** `UV_THREADPOOL_SIZE=64` set for API only (intentional for DNS); other Node = web only, unset | [PROVEN] | — | Keep on API; web N/A unless similar DNS load | — |
| F066 | C/O | ad-attribution | `services/ad-attribution/internal/consumer/consumer.go:147-155` | Failed platform fire still `MarkProcessed` (+ commit-always) — **no retry** on money-path conversion | [PROVEN] | **P0** | Mark only on success; leave for retry / DLQ with payload | M |
| F067 | O | platform-sync | `ringba_adapter.go:585-586` | Writeback `LookupCall` searches only a **7-day** Call Log window — older calls → not-found | [PROVEN] | **P2** | Widen lookup window or accept callDt hint | M |
| F068 | I | platform-sync / pingdedup | `ringba_adapter.go:337`; `packages/pingdedup/content.go:22-54` | `WasPayoutAdjusted` mapped from Ringba but **absent from content hash** — payout-adjust flips may not republish | [PROVEN] | **P2** | Add field to Content + producer if product requires it | S |
| F069 | M | Infra / ask-ai | `db/postgres/migrations/000012_ask_ai_query_log.up.sql:21-24` | `ask_ai_query_log` has ENABLE RLS + policy but **no FORCE ROW LEVEL SECURITY** | [PROVEN] | **P1** | `ALTER … FORCE ROW LEVEL SECURITY` like other tenant tables | S |
| F070 | K | ClickHouse | `000002` / `000003` / `000005` DLQ tables | `scoring_dlq`, `enrichment_dlq`, `compliance_dlq`, `attribution_dlq` have **no TTL** | [PROVEN] | **P2** | Add TTL or ops purge | S |
| F071 | P | Infra | `deploy/docker-compose.yml:122-134` | `clickhouse-init` applies each SQL with `\|\| true` — migration failures swallowed | [PROVEN] | **P1** | Fail the init container on first SQL error | S |

---

## 2. Known items K1–K22 — current state

| # | Status | Evidence |
|---|---|---|
| **K1** | **Still true** | `retreaver_adapter.go:144-149`, `:118-125` |
| **K2** | **Still true** | `platform-sync/cmd/server/main.go:79-84` |
| **K3** | **Still true** | `.github/workflows/go.yml:24-28`; no root `go.mod` |
| **K4** | **Still true** | `postgres.service.ts:26-37`; `clickhouse.service.ts:14-25` |
| **K5** | **Still true** | `consumer.go:222-234`, `:163-199` |
| **K6** | **Still true** | `consumer.go:203-215`, `:249-254` |
| **K7** | **Still true** | `convert.go:163-172` — 409 before audit |
| **K8** | **Still true** | `000012_….up.sql:1` |
| **K9** | Code present; live still unverified | `integrations.service.ts:292-293` (no live writeback this audit) |
| **K10** | **Still true** | `SettleWindow = 12s` `convert.go:15-17` |
| **K11** | **Still true — no renewal** | lock TTL 15m `sync_lock.go:20`; sync 10m `main.go:113`; SetNX once |
| **K12** | **Still true** | `000001_pings_table.up.sql:18` |
| **K13** | **Still true** | `000010_….up.sql:5-29` |
| **K14** | **Still true** | `verify-ingestion-slice.ps1:77-79` |
| **K15** | **Still true** | `dashboard/page.tsx:154-158` vs `/live-status` |
| **K16** | **Still true** | compose `MOCK_MODE: "true"` on intelligence + web |
| **K17** | **Still true (configured)** | compose Redis `volatile-ttl` |
| **K18** | **Still true** | `verify-system.ps1` omits `verify-api-sync` |
| **K19** | **Still true — intended** | Ringba `HasMore: false` → no page1_refresh |
| **K20** | **Still true** | compose JWT/WRITEBACK/encryption; demo admin seed |
| **K21** | **Still true for API** | API Dockerfile + compose only |
| **K22** | **Changed — present** | `docs/evidence/ringba-writeback-probe/postman-collection.json` (~1.4MB, git-tracked) |

---

## 3. Summary — P0 / P1 / P2 counts per service

Counts are distinct finding IDs primarily owned by that service (shared findings counted once on the worst-affected service; infra/CI counted under Infra).

| Service | P0 | P1 | P2 |
|---|---|---|---|
| ingestion | 0 | 2 | 1 |
| consumer | 1 | 3 | 0 |
| scoring | 1 (shared commit-always) | 3 | 1 |
| enrichment | (shared F001/F002) | 2 | 1 |
| providerconfig | 1 | 0 | 0 |
| compliance-intelligence | (shared F001) | 1 | 0 |
| ask-ai | 2 | 1 | 0 |
| ad-attribution | 1 (F066 + shared F001) | 0 | 0 |
| platform-sync | 2 | 5 | 5 |
| api | 3 | 5 | 3 |
| web | 1 | 2 | 2 |
| packages/pingdedup | 0 | 0 | 2 |
| Infra (compose/CI/DB/scripts) | 1 | 7 | 4 |

**Rough totals:** ~**13 P0**, ~**32 P1**, ~**18 P2** (some IDs span services).

**Deploy blockers (P0 themes):** unauthenticated internal APIs with client `tenant_id` (ask-ai, providerconfig, platform-sync `/sync`), Kafka commit-after-fail on intelligence consumers, attribution mark-on-failed-fire, consumer loop exit while health green, Ringba frozen sync cursor, login without rate limit, Deepgram body tenant + mock secret skip, JWT in localStorage, compose secret defaults.

---

## 4. Suggested grouping into fix slices

Grouping only — order/scope are operator decisions.

1. **Tenant isolation & auth on internal surfaces** — F004–F006, F008, F011, F069 (ask-ai, providerconfig, platform-sync `/sync`/`/test-auth`, Deepgram, ask_ai RLS FORCE).
2. **Kafka consumer hardening** — F001, F002, F013–F016, F034, F066 (commit policy, DLQ payload, poison classifier, restart/readyz, attribution mark-on-fail).
3. **Platform-sync data completeness & shutdown** — F003, F012, F019, F028, F029, F050, F067–F068 (Ringba cursor, Retreaver paging, graceful stop, lock renewal, readyz, LookupCall window, content-hash fields).
4. **API/web security baseline** — F007, F009, F010, F022–F026, F054 (login RL, cookie auth, CSP, CORS, Zod, npm/Go vulns, compose secrets).
5. **Write-back polish** — F036, F037, F039 (409 audit, settle window, 202 live proof).
6. **Ops / infra readiness** — F015, F045–F049, F017, F035, F070–F071 (restart policies, CI, migrations/backups, Kafka retention, IP-pin readyz, alerts, DLQ TTL, CH init fail-open).
7. **Product/data semantics & gates** — F038, F040–F044, F051–F053, F030 (has_converted, Live badge, verify scripts, MOCK_MODE, query ORDER BY).

---

## 5. Coverage matrix

Legend: **✓** examined · **N/A** (reason) · **✗** (see §7)

| Service | A | B | C | D | E | F | G | H | I | J | K | L | M | N | O | P | Q | R | S | T | U | V | W |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| ingestion | N/A no live page | ✓ | ✓ | N/A producer | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | N/A no TTL own | N/A no KPI | ✓ | ✓ | N/A | N/A | N/A | N/A | N/A | ✓ | ✓ | N/A | ✓ |
| consumer | N/A | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | N/A pass-through | N/A | N/A | N/A Kafka msg tenant | ✓ | N/A | N/A | N/A | N/A | N/A | ✓ | ✓ | N/A | ✓ |
| scoring | N/A | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | N/A | N/A | N/A | ✓ | ✓ | N/A | N/A | N/A | N/A | N/A | ✓ | ✓ | N/A | ✓ |
| enrichment | N/A | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | N/A | N/A | N/A | ✓ | ✓ | N/A | N/A | N/A | N/A | N/A | ✓ | ✓ | N/A | ✓ |
| providerconfig | N/A | ✓ | N/A read | N/A | ✓ | ✓ | ✓ | ✓ | N/A | N/A | N/A | N/A | ✓ | ✓ | N/A | N/A | N/A | N/A | N/A | ✓ | ✓ | N/A | ✓ |
| compliance-intelligence | N/A | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | N/A | N/A | N/A | ✓ | ✓ | N/A | N/A | N/A | N/A | N/A | ✓ | ✓ | N/A | ✓ |
| ask-ai | N/A | ✓ | N/A | N/A | ✓ | ✓ | ✓ | ✓ | N/A | N/A | N/A | N/A ask path | ✓ | ✓ | N/A | N/A | N/A | ✓ SQL | N/A | ✓ | ✓ | N/A | ✓ |
| ad-attribution | N/A | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | N/A | N/A | N/A | ✓ | ✓ | N/A | N/A | N/A | N/A | N/A | ✓ | ✓ | N/A | ✓ |
| platform-sync | ✓ | ✓ | ✓ | N/A producer | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ Redis TTL | N/A | ✓ | ✓ | ✓ | N/A uses PG mig | N/A | N/A | N/A | ✓ | ✓ | N/A | ✓ |
| api | ✓ | ✓ | ✓ | ✓ live | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | N/A | ✓ | ✓ | ✓ | ✓ | N/A | N/A | ✓ | N/A | ✓ | ✓ | ✓ | ✓ |
| web | N/A | N/A browser | N/A | N/A | N/A | N/A | N/A static | N/A SPA | ✓ UI | ✓ | N/A | ✓ via API | ✓ token | ✓ | N/A | N/A | N/A | N/A | N/A | N/A UI | ✓ mock | N/A | ✓ |
| packages/pingdedup | N/A | ✓ | ✓ | N/A | ✓ Redis | N/A | N/A | N/A | ✓ hash | N/A | ✓ caller TTL | N/A | N/A | N/A | N/A | N/A | N/A | N/A | N/A | ✓ | N/A | N/A | ✓ |
| Infra | ✓ compose | N/A | N/A | ✓ restart | ✓ Kafka cfg; live retention ✗ | N/A | N/A | N/A | N/A | N/A | ✓ TTL inventory | N/A | N/A | ✓ Docker USER | N/A | ✓ | ✓ repo; prod ✗ | N/A | ✓; history ✗ | N/A | ✓ | ✓ | ✓ |

---

## 6. Files read — per service

### ingestion
`cmd/server/main.go`, `internal/handlers/{webhook,health,common,callloom_native,ringba}.go`, `internal/security/{hmac,platform_hmac}.go`, `internal/connectors/{ringba_connector,retreaver,callgrid,trackdrive}.go`, `internal/producer/producer.go`, `internal/config/config.go`, `pkg/kafka/kafka.go`, `Dockerfile`, compose service block

### consumer
`internal/consumer/{consumer.go,poison_test.go}`, `internal/config/config.go`, `internal/writer/writer.go`, `pkg/kafka/kafka.go`, `cmd/server/main.go`, `Dockerfile`

### scoring
`internal/consumer/consumer.go`, `internal/scoring/{engine.go,ai.go}`, `internal/mockemitter/emitter.go`, `internal/config/config.go`, `pkg/{kafka,clickhouse}/*.go`, `cmd/server/main.go`, `Dockerfile`

### enrichment
`internal/consumer/consumer.go`, `internal/providers/provider.go`, `internal/config/config.go`, `pkg/clickhouse/clickhouse.go`, `cmd/server/main.go`, `Dockerfile`

### providerconfig
`cmd/server/main.go`, `internal/handlers/resolve.go`, `internal/loader/{resolver,types}.go`, `internal/config/config.go`, `registry/registry.go`, `Dockerfile`

### compliance-intelligence
`internal/consumer/consumer.go`, `internal/publisher/publisher.go`, `internal/dnc/provider.go`, `internal/config/config.go`, `pkg/clickhouse/clickhouse.go`, `cmd/server/main.go`, `Dockerfile`

### ask-ai
`cmd/server/main.go`, `internal/handlers/ask.go`, `internal/nlsql/{templates.go,validator.go}`, `internal/pipeline/pipeline.go`, `internal/cost/budget.go`, `internal/config/config.go`, `Dockerfile`

### ad-attribution
`internal/consumer/consumer.go`, `internal/publisher/publisher.go`, `internal/config/config.go`, `cmd/server/main.go`, `Dockerfile`

### platform-sync
`cmd/server/main.go`, `internal/config/config.go`, `internal/syncer/{syncer.go,sync_lock.go}`, `internal/writeback/{convert.go,lock.go,auth.go,errors.go}`, `internal/adapter/{retreaver_adapter.go,ringba_adapter.go,ringba_windowing.go,registry.go,httpclient.go}`, `internal/dedup/dedup.go`, `internal/store/integrations.go`, `internal/producer/producer.go`, `pkg/kafka/kafka.go`, `Dockerfile`

### api
`src/main.ts`, `src/config/env.ts`, `src/health/health.controller.ts`, `src/auth/{auth.controller,auth.service}.ts`, `src/common/{tenant.decorator,demo-rows,demo-rows.spec,zod-parse,ping-event-time}.ts`, `src/database/{postgres,clickhouse}.service.ts`, `src/integrations/{integrations.controller,integrations.service}.ts`, `src/scoring/scoring.controller.ts`, `src/analytics/analytics.service.ts`, `src/pings/{pings.service,pings.filters}.ts`, `src/coverage/coverage.service.ts`, `src/compliance/compliance.service.ts`, `src/attribution/attribution.service.ts`, `src/enrichment/enrichment.service.ts`, `src/native-integration/deep-query.service.ts`, `src/ask-ai/ask-ai.service.ts`, `src/live/{live.gateway,kafka-live-consumer.service}.ts`, `src/kafka/{kafka-client,resilient-consumer}.ts`, `Dockerfile`

### web
`src/lib/{auth.ts,use-live-feed.ts,date-filter.tsx,ping-timeline.ts}`, `src/app/(app)/dashboard/page.tsx`, `next.config.mjs`, `Dockerfile`

### packages/pingdedup
`dedup.go`, `content.go`, `dedup_test.go`, `content_test.go`, `go.mod`

### Infra
`deploy/docker-compose.yml`, all 11 Dockerfiles, `.github/workflows/go.yml`, `db/postgres/migrations/000001`–`000026` (inventory + writeback/RLS samples), `db/clickhouse/migrations/000001`–`000014` (TTL/engine focus), `scripts/{verify-system,verify-ingestion-slice,verify-presentation,verify-api-sync,init-kafka-topic,migrate-*.sh}.ps1|.sh`, `docs/production-readiness-audit-spec.md`

**Also in scope (not a separate matrix service):** `shared/providers/catalog.json` (copied into images — noted, not deep-audited as a runtime service).

---

## 7. Not examined — each ✗ with permitted reason

| Cell / item | Reason | Detail |
|---|---|---|
| Infra × E — **live** topic retention / RF via `rpk topic describe` | **3 — access unavailable** | Docker daemon not running (`dockerDesktopLinuxEngine` pipe missing). Compose/topic-create scripts **were** examined (no retention flags). |
| Infra × Q — **production** backup/restore infrastructure | **1 — infra not built yet** | Repo has no backup jobs; prod backup systems (if any) not in this workspace. |
| Infra × S — GitHub Actions **run history** | **3 — access unavailable** | `gh` CLI not installed / not on PATH. Workflow **file** examined (K3). |
| K9 live Nest 202 observation | **2 — would require write / live writeback** | Code path proven; firing convert against Ringba forbidden this audit. |

No ✗ used for “too large,” “too long,” or “not enough context.”

---

## 8. Deploy-prep items

Carry forward when production infrastructure exists (reason **1** gaps + related ops):

1. **Postgres / ClickHouse / Redis backups** — schedule, encryption, restore drill, RPO/RTO.
2. **Kafka/Redpanda** — set explicit `retention.ms` / local retention ≥ max consumer outage; replication factor ≥ 3; monitor lag.
3. **Confirm live topic config** with `rpk topic describe` once cluster is up.
4. **Multi-replica** — verify Redis locks (sync/writeback), ask-ai budget, API in-process maps under ≥2 replicas.
5. **Rolling deploy** — prove SIGTERM drains (API shutdown hooks, consumer finalFlush, platform-sync lock release) on real kube.
6. **Secret injection** — no compose-style defaults; rotate all K20 values.
7. **Network policy** — do not publish internal ports (`8080–8088`, `9092`, DB) on public interfaces.
8. **Re-run** `govulncheck` / `npm audit` on production base images after Go/Next bumps.

---

## 9. Evidence commands (reproducible)

```powershell
# Tooling
go version
# gh --version   # failed: not installed → CI history ✗ reason 3

# Install + run govulncheck (read-only analysis)
go install golang.org/x/vuln/cmd/govulncheck@latest
foreach ($m in @(
  'services/ingestion','services/consumer','services/scoring','services/enrichment',
  'services/providerconfig','services/compliance-intelligence','services/ask-ai',
  'services/ad-attribution','services/platform-sync','packages/pingdedup'
)) { Push-Location $m; govulncheck ./...; Pop-Location }

# npm audit (high/critical reported)
Push-Location services\api; npm audit --omit=dev; Pop-Location
Push-Location web; npm audit --omit=dev; Pop-Location

# Live Kafka describe (failed — Docker daemon down)
# docker compose -f deploy/docker-compose.yml ps
# docker exec <redpanda> rpk topic describe callloom.pings.v1 --brokers localhost:9092

# Static greps / reads used throughout (examples)
# rg -n "Commit\(|isPoison|Writeback|getent|MOCK_MODE|page1_refresh" services
# rg -n "TTL |ENABLE ROW LEVEL" db
```

**govulncheck (summary):** all Go modules report multiple go1.22.12 stdlib findings (e.g. GO-2026-6090 tls, GO-2026-6089 net/http, GO-2026-5037 x509). Treat as **upgrade Go toolchain** (F054); full IDs in terminal capture.

**npm audit (high/critical only):**
- **api:** multer high (DoS chain), socket.io-parser high; 3 high / 6 moderate / 1 low.
- **web:** Next.js advisories including critical (RCE-class on Windows / image opt) + PostCSS high + socket.io-parser high; 1 critical / 4 high.

---

## STOP

Deliverable complete: `docs/production-readiness-audit.md`.  
No code changes. No Phase 2B. No push.
