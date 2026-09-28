# Calyx — Production-Readiness Audit (READ-ONLY)

**Purpose:** find, in one pass, the infrastructure and correctness risks that have so far
surfaced one at a time under live load — and the ones that have not surfaced yet because
production conditions (multiple replicas, rolling deploys, long outages, real customers)
have not occurred. **No fixes. No code, config, data, or integration changes. No write to
Ringba. No Phase 2B. No push.**

**Deliverable:** `docs/production-readiness-audit.md`

---

## ⛔ RULE

- **Read the source code.** Previous reports are context, not evidence. Every finding cites
  `file:line`.
- **List the files you read, per service.** A service summarised from two files is not
  audited.
- **Nothing is skipped silently.** Every service × class is either examined, not
  applicable, or marked ✗ for one of the three permitted reasons in section 5.
- **The checklist is not complete.** Anything risky you notice outside it goes under
  class W.
- Read-only commands only: reading files, grep, `SELECT` queries, `docker inspect`,
  `rpk ... describe`, `npm audit`, `govulncheck`. Nothing that changes project state,
  data, containers, or integrations. Installing a missing read-only analysis tool is
  allowed.
- Label each finding `[PROVEN]` (seen in code or runtime) / `[INFERRED]` / `[UNKNOWN]`.
- Fix *shape* only, one or two lines. No redesigns, no code.

---

## Scope — all of these

**Services:** `services/ingestion`, `services/consumer`, `services/scoring`,
`services/enrichment`, `services/providerconfig`, `services/compliance-intelligence`,
`services/ask-ai`, `services/ad-attribution`, `services/platform-sync`, `services/api`,
`web`.

**Packages:** `packages/pingdedup` and any other shared package found.

**Infra:** `deploy/docker-compose.yml`, every `Dockerfile`, `db/postgres` and
`db/clickhouse` migrations, `.github/workflows/`, `scripts/`.

If you find a service, package or config directory not listed here, audit it too and say
so.

---

## Checklist — apply to every service where relevant

### A. Data completeness
- Pagination over a live/changing result set (offset or page over a moving window)?
- Cursor semantics: can it skip or re-fetch? Is it written conditionally?
- Can the service's output be reconciled against its source?

### B. Concurrency
- Periodic / scheduled jobs: cross-replica lock?
- In-process state (maps, caches, counters, locks) that breaks with more than one replica?

### C. Idempotency
- Any write (DB, external API, Kafka publish) that can happen twice on retry or double
  request?

### D. Kafka consumers — every consumer in every service
- One bad message fails the whole batch?
- How is poison identified? Substring matching on error text?
- **Unknown bad message:** error matches no poison rule → retried forever? Stalls the
  partition? Retry limit, dead-letter, alert?
- Poison skip: is the message data kept anywhere (DLQ) or only logged?
- Offset commit only after a successful write?
- **In-memory buffering bound:** during a long downstream outage, does the consumer keep
  fetching and buffering without limit? Backpressure / pause?
- Crash recovery; container `restart:` policy.
- Health visible (endpoint / metric)?

### E. Dependency outages — Kafka, ClickHouse, Postgres, Redis, each external API
- Behaviour when down: retry, drop, crash, stall?
- Can data be lost silently?
- **Kafka topic retention** vs the longest plausible consumer outage — could data expire
  from the topic before it is consumed? Report topic retention, partitions, replication.

### F. Timeouts & pools
- Every client: timeout values and scope (shared vs dedicated).
- Any timeout applying to more than intended?

### G. Health / readiness
- Each service's `/healthz` / `/readyz`: what exactly do they check?
- Can a non-essential dependency fail readiness and take the service out of rotation?
- **Pinned IPs / resolve-once at boot** anywhere — behaviour when the target IP changes.

### H. Graceful shutdown
- On SIGTERM (Kubernetes rolling deploy): are in-flight batches flushed, offsets committed,
  locks released, HTTP requests drained? Or lost / double-processed?
- Examine the shutdown handling **in code**. Only the live Kubernetes behaviour may be ✗.

### I. Data semantics
- Columns where "not provided" and "false/0" collapse into one value.
- Defaults that can be mistaken for real data.
- Content-hash coverage: fields that should trigger republish but are missing, or present
  but should not.
- Aggregates: consistent `(platform, external_call_id)` + `argMax` everywhere?

### J. Time handling
- Timezone of each platform timestamp as received (Ringba `callDt`, Retreaver
  `created_at`), and how it is stored.
- All stored timestamps UTC?
- Period filters ("7 days", "30 days", date buckets): computed in which timezone? Could a
  tenant's day boundaries be wrong?

### K. Retention / TTL
- Every ClickHouse and Postgres table: TTL, key column, value.

### L. Test-data exclusion
- Every read path feeding KPIs, lists, counts, charts: uses the shared exclusion helper?

### M. Multi-tenant isolation
- `tenant_id` taken only from the auth context — never from request body / query / headers
  a client controls?
- **Every ClickHouse query** reading tenant data filters by `tenant_id`? List any that do
  not.
- Postgres: RLS enabled and enforced on tenant tables? Any bypass role used by app paths?
- Internal service-to-service calls that accept `tenant_id` in the body (e.g. the
  write-back internal-token path): who can reach them?

### N. Security
- Unauthenticated endpoints on every service and port; which ports compose exposes.
- **Public webhook ingestion:** HMAC verification, replay protection (timestamp / nonce),
  request size limits, rate limiting.
- Login and API rate limiting / brute-force protection.
- Error responses: stack traces or internal details returned to clients? Raw 500s from
  validation?
- Secrets, tokens, or PII (caller numbers, ANI) in logs, errors, or committed files.
- Web: where the auth token is stored (localStorage vs httpOnly cookie); CSP.
- Dockerfiles: running as root? Base images pinned?
- Dependency vulnerabilities: `npm audit` (api, web), `govulncheck` (each Go module).
  Report high / critical only.

### O. Write-back (money path) — regression check
Confirm each Phase 1 guard is still present and intact in current code:
- bearer auth on platform-sync write routes
- global `WRITEBACK_ENABLED` + per-tenant `writeback_enabled`
- per-tenant credential binding; call must belong to the tenant's Ringba account
- idempotency lock (convert / void)
- settle window + three outcomes (confirmed / pending / failed); never "failed" for
  pending
- `writeback_audit` immutable (trigger)
- amount validation → 400, not 500

### P. Migrations
- Every migration has `up` and `down`, numbered without gaps or collisions, wired into
  compose init.
- Any migration unsafe on a live table (copy + rename with a write window)?

### Q. Backup & recovery
- Any backup configured for Postgres, ClickHouse, Redis in the repo or compose? Restore
  procedure documented?
- Report what exists in the repo. Only production backup infrastructure may be ✗.

### R. Query performance
- ClickHouse `pings` `ORDER BY` / partitioning vs the queries the API runs (argMax per call,
  period filters). Any query that scans the whole table per request? Relevant to the
  Phase 2B server-side status filter.

### S. CI / source-vs-deployed
- Every workflow in `.github/workflows/`: what it runs, and whether it can pass — examine
  the workflow file itself.
- Run history: via `gh` CLI if available; if not, that part only may be ✗.
- Any code present only in built images and not in the repo?

### T. Observability
- For each silent-failure path (poison skip, offset fallback, sync skipped, consumer down,
  lag, write-back pending): log? counter? anything that alerts a human?

### U. Config
- Dev defaults that would reach production if compose were copied.
- Mock flags and what depends on them.

### V. Gate coverage
- What `verify-system.ps1`, `verify-ingestion-slice.ps1`, `verify-presentation.ps1`
  actually exercise — and which critical paths they do **not**.

### W. Other — anything the checklist missed
This checklist is written from risks already seen. It is not complete.
While reading the code, report **any other risk** you notice — correctness, data loss,
security, reliability, performance, maintainability — even if it fits no class above.
Same rules: `file:line`, label, severity, fix shape. Read-only. No fixes.
Do not dismiss something because it is not on the list.

---

## Known items — VERIFY current state, do not rediscover

| # | Item | Last known |
|---|---|---|
| K1 | Retreaver adapter `page`/`per_page` over a changing `updated_at` window | open |
| K2 | platform-sync `/sync` and `/test-auth` unauthenticated | open |
| K3 | GitHub Actions "Go" workflow fails every push (root has no `go.mod`) | open, never fixed |
| K4 | Readyz PG/CH clients pin IPv4 at boot | risk on IP change |
| K5 | Poison classifier = substring deny-list; unmatched errors retried with no limit | verify |
| K6 | Poison-skipped messages only logged — no dead-letter copy of the data | verify |
| K7 | `writeback_audit` has no row for 409 already-in-progress | known gap |
| K8 | `pings.has_converted` `UInt8` — cannot represent "not provided" | known |
| K9 | Nest 202 on pending write-back never observed live | known |
| K10 | Write-back settle window 12s; confirmed convert took 10.4s | tight |
| K11 | Sync timeout 10m vs single-flight lock TTL 15m | verify margin & renewal |
| K12 | `pings` TTL `received_at + 24 months` | on checklist |
| K13 | `enrichment_attrs` ReplacingMergeTree migration used copy + rename (write window) | production-unsafe procedure |
| K14 | `verify-ingestion-slice.ps1` asserts default KPI unchanged — can flake under live sync | open |
| K15 | Dashboard "Live" badge reflects socket, not `/live-status` | roadmap |
| K16 | Intelligence services `MOCK_MODE=true`; UI shows mock TCPA / scores | decision pending |
| K17 | Redis `volatile-ttl` eviction vs dedup / content-hash keys | roadmap |
| K18 | `verify-api-sync.ps1` not in the gate | known |
| K19 | `page1_refresh` idle for Ringba under windowing; still active for others | verify intended |
| K20 | Compose dev keys / internal token / dev login | on checklist |
| K21 | `UV_THREADPOOL_SIZE=64` set for API | verify other Node services |
| K22 | Saved Postman collection missing from workspace | verify |

For each: still true? `file:line`. Changed?

---

## Output — `docs/production-readiness-audit.md`

### 1. Findings table — one row per finding

| ID | Class | Service | `file:line` | Finding | Label | Severity | Fix shape | Effort |
|---|---|---|---|---|---|---|---|---|

**Severity**
- **P0** — silent data loss, money moved wrongly, cross-tenant exposure, or an exploitable
  security hole. Blocks deploy.
- **P1** — must fix before production: reliability, recovery, graceful shutdown,
  observability of silent failures.
- **P2** — should fix; may follow GA with a documented trigger.

**Effort:** S (< half day) · M (≤ 2 days) · L (more).

### 5. Coverage matrix — mandatory
One row per service (plus one row for Infra), one column per class A–W.

STOP. No fixes. No Phase 2B. No push.
