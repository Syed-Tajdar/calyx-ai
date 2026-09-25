# Calyx — S1 Part B: Core Resilience

**Scope:** make the v1 core path survive crashes, restarts, outages and deploys without
losing data or stalling silently. **v1 core services only:** `consumer`, `api`,
`platform-sync`, `ingestion`, and infra containers. **No intelligence services, no S2
security, no CI, no UI, no Retreaver, no Phase 2B. No push.**

**Deliverable:** `docs/s1b-report.md`

---

## ⛔ RULE

- Parts B1–B6 are **independent**. If one blocks, report it and continue.
- **Proven live on rebuilt images.** Reading code is not proof. Each part lists its proof.
- Label claims `[PROVEN]` / `[INFERRED]` / `[UNKNOWN]`.
- **Lessons already paid for — do not repeat:**
  - A non-essential dependency must never fail readiness or boot of a service that
    can still serve (fix2c: Kafka took the whole API out).
  - No IPs pinned at boot (K4).
  - A timeout / pool must be scoped to what it protects (fix2e: 1.5s on the whole API
    pool).
  - A transient outage must never be classified as a bad message (fix2e).
- **Permitted state changes for proofs:** stopping/starting/killing containers,
  sending `smoke-` pings, triggering syncs/sweeps. No data deletion. No Ringba writes.

---
---

## B1 — Sync lock heartbeat (F028)

**Why now:** S1A raised the single-flight lock TTL to **45 minutes** for long runs. With no
renewal, a crash mid-run holds the lock for up to 45 minutes and **no tick runs** for that
time.

**Change:**
- While a run holds the lock, renew it periodically (e.g. every 30s), using an
  **owner-token compare-and-extend** (only the holder extends; Lua, like release).
- With renewal in place, **reduce the TTL** to a short value (e.g. 90s) so a crashed
  holder releases quickly. Justify the numbers.
- **If renewal fails or ownership is lost, cancel the run's context** — stop publishing
  immediately. Two runners must never publish at once.
- A cancelled run must not advance `last_sync_at` (S1A semantics — confirm unchanged).

**Proof:**
- Unit: renewal extends TTL; lost ownership cancels the run; release still owner-only.
- Live: start a deep sweep (30d), then `docker kill deploy-platform-sync-1` mid-run.
  Restart it. Report: time until the next tick runs (target ≈ TTL, not 45 min), lock
  state in Redis, `last_sync_at` unchanged by the killed run.

## B2 — API boots without Kafka (V6)

**Why:** S0 V6 — `deploy-api-1` restarted 5× with
`api bootstrap failed: KafkaJSNumberOfRetriesExceeded`. If Kafka is not ready, **the whole
API does not start** — login, dashboard, reports all down.

**Change:**
- Bootstrap must not wait on Kafka. Live-feed and alert-engine consumers, and the
  producer, connect **in the background** with backoff.
- `/readyz` stays Postgres + ClickHouse only (unchanged). `/live-status` shows the
  consumer not joined until it is.
- Any endpoint that needs to **produce** to Kafka while it is unavailable returns a clear
  **503**, never a crash or hang. List which endpoints produce.

**Proof:**
- `docker stop deploy-redpanda-1`, then `docker restart deploy-api-1`.
  API starts; `/healthz` 200, `/readyz` 200, login 200, one analytics read 200;
  `/live-status` = not member. `RestartCount` does **not** increase.
- `docker start deploy-redpanda-1` → consumer joins **without** an API restart; ws-smoke
  PASS.

## B3 — Restart policies

**Why:** S0 — only `api` has `restart: unless-stopped`. After the laptop reboot nothing
else came back on its own.

**Change** in `deploy/docker-compose.yml`:
- `restart: unless-stopped` on every **long-running** v1 service and infra container:
  `consumer`, `platform-sync`, `ingestion`, `api`, `web`, `postgres`, `redis`,
  `clickhouse`, `redpanda`.
- One-shot init jobs (`migrate`, `kafka-init`, `clickhouse-init`): explicit
  `restart: "no"`.
- **Intelligence services: leave unchanged** — S4 decides whether they deploy at all.

**Proof:** `docker inspect` restart policy for every container (table). One live check:
make one v1 service process exit (not `docker stop`, which suppresses the policy) and show
it comes back with `RestartCount` +1.
Optional operator step: restart Docker Desktop — all v1 containers come back without
`compose up`.

## B4 — Consumer hardening (v1 core pipeline)

### B4.1 — Size-triggered flush must not kill the loop (F015)
S0 could not exercise it (ticker flushed first). Fix it defensively:
- A transient failure on the **size** path behaves exactly like the ticker path: keep the
  batch, retry with backoff, do not exit `Run`.
- **Bound memory:** while a flush is failing and the batch is full, **stop fetching** from
  Kafka (pause / backpressure) until a flush succeeds. The in-memory buffer must never
  grow without limit during a long ClickHouse outage.
- Offsets committed only after a successful write (unchanged).

**Proof:** unit test — writer fails transiently on a size-triggered flush → `Run` stays
  alive, no fetch beyond the bound, all messages written after recovery, offsets committed
  only after. Live: ClickHouse outage with enough smoke pings to fill a batch (report how
  you guaranteed the size path fired this time — e.g. produced fast enough, or a log marker
  on the size path) → consumer PID unchanged, all rows land.

### B4.2 — Unknown bad message: bounded retry (F013)
Today: an error matching no poison rule is retried **forever** — one bad message can stall
the partition.

Rule to implement:
- If **ClickHouse is unhealthy** (ping fails) → it is an outage → retry indefinitely
  (never poison). *(fix2e lesson.)*
- If ClickHouse **is healthy** and the **same message** keeps failing → after N attempts
  (choose N, justify), treat it as poison → B4.3.

**Proof:** unit tests for both branches. Live: send one message that fails for a reason
  not on the deny-list while ClickHouse is healthy → it goes to DLQ after N attempts,
  consumer keeps moving; a ClickHouse outage during the same run is **not** treated as
  poison.

### B4.3 — Dead-letter topic (F014)
Today poison is logged and skipped — the data is gone.
- Before committing past a poison message, produce the **full original message** (key,
  value, headers) plus source topic/partition/offset, reason, and timestamp to a DLQ topic
  (e.g. `callloom.pings.v1.dlq`), created by `kafka-init` with retention **≥ 30 days**.
- **If the DLQ produce fails, do not commit** — retry. A poison message must never be both
  un-written and un-dead-lettered.
- Document how to replay a DLQ message (procedure only; a replay tool is out of scope).

**Proof:** poison message → present in DLQ with full payload and metadata; offset
  committed after; DLQ unavailable → offset not committed.

### B4.4 — `/metrics` (F016)
S0: consumer `/metrics` is 404; `poison_total` exists only in logs.
Expose Prometheus-format counters/gauges, at least: `poison_total`, `dlq_total`,
`flush_failures_total`, `retries_total`, `buffered_messages`,
`last_successful_flush_timestamp`.
**Do not** wire these into liveness — a ClickHouse outage must not make Kubernetes restart
the consumer in a loop. Visibility only; alerting is S3.

**Proof:** `curl /metrics` before, during and after a ClickHouse outage — values move as
  expected.

## B5 — platform-sync readiness (F050)

Today: only `/healthz` (always ok).
- Add `/readyz` checking **Postgres + Redis** (both required to run a sync or a
  write-back). **Not Kafka** (blast-radius lesson) — report Kafka state separately, e.g.
  via `/metrics` or a status endpoint.
- Dedicated short-timeout clients for the probe; no pinned IPs.

**Proof:** Redis stopped → `/readyz` 503, `/healthz` 200; Redpanda stopped → `/readyz`
  stays 200; both restored → 200.

## B6 — Graceful shutdown (F018, F019)

Kubernetes sends SIGTERM on every deploy. Today in-flight work can be lost or its lock
orphaned.

### platform-sync
On SIGTERM: stop the ticker; **cancel the in-flight run's context**; wait up to a bounded
time for it to stop; **release the lock** (owner-only); exit. The cancelled run must not
advance `last_sync_at`.

### api
`app.enableShutdownHooks()`; drain HTTP for a bounded time; disconnect Kafka consumers /
producer cleanly.

### consumer
On SIGTERM: stop fetching, flush the buffered batch, commit offsets for what was written,
exit. Anything not written must **not** be committed (it is re-delivered on start —
at-least-once; duplicates are absorbed by `argMax`).

### All three
Shutdown must complete **within 25s** (below Kubernetes' default 30s grace period).

**Proof:**
- `docker stop deploy-platform-sync-1` **during a deep sweep** → logs show cancel + lock
  released; restart → next tick runs immediately (not after TTL).
- `docker stop deploy-consumer-1` with buffered messages → after restart, every smoke row
  present; report duplicates if any.
- `docker stop deploy-api-1` during a request → request completes or fails cleanly;
  shutdown time reported.
- Add to `docs/deploy-checklist.md` → **Infrastructure**:
  *"Kubernetes `terminationGracePeriodSeconds` ≥ 30s for api, consumer, platform-sync —
  shutdown handlers are bounded to 25s."*

---

## Gates, commit, scan

- All three gates exit 0 with sync live: `verify-system` (15/15, 11/11, LiveStatus,
  WebSocket), `verify-ingestion-slice`, `verify-presentation`.
- Commit locally. Secret scan of every unpushed commit (same patterns). **Do not push.**

## Output — `docs/s1b-report.md`
1. Scoreboard B1–B6: PASS / FAIL, no "partially".
2. Per part: before/after behaviour with `file:line`, then live proof with raw evidence.
3. Chosen numbers (lock TTL, renewal interval, retry N, DLQ retention, shutdown budget)
   with justification.
4. Honest limitations; anything found but not fixed.

STOP. Do not push. Do not start S2.

## Out of scope
Intelligence services (scoring, enrichment, compliance, ask-ai, ad-attribution,
providerconfig) · S2 security (F007, F009, F011, F020–F027 …) · CI (F045) · alerts (F035)
· UI · Retreaver · Phase 2B · DLQ replay tooling · data deletion · Ringba writes.
