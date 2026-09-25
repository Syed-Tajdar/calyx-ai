# S1 Part B — Core Resilience

**Scope:** B1–B6 only (v1 core: consumer, api, platform-sync, ingestion, infra). No intelligence services, S2, CI, UI, Retreaver, Phase 2B. **Not pushed.**

**Spec:** `docs/s1b-spec.md`  
**Evidence:** `docs/evidence/s1b/`

---

## 1. Scoreboard

| Part | Result | Notes |
|---|---|---|
| **B1** Sync lock heartbeat | **PASS** | TTL 90s + renew 30s; orphan free ~91s after kill; units for renew/ownership |
| **B2** API boots without Kafka | **PASS** | RestartCount unchanged; health/ready/login/analytics 200; live rejoins; ws-smoke PASS |
| **B3** Restart policies | **PASS** | All v1 long-running `unless-stopped`; init `no`; live RestartCount+1 |
| **B4** Consumer hardening | **PASS** | Size flush soft-fail + pause; bounded poison; DLQ; `/metrics` |
| **B5** platform-sync `/readyz` | **PASS** | Redis down → 503; Redpanda down → 200; restored 200 |
| **B6** Graceful shutdown | **PASS** | SIGTERM cancels run + releases lock in ~12ms; consumer flush; api stop ~3.5s |

---

## 2. Chosen numbers

| Knob | Value | Justification |
|---|---|---|
| Lock TTL | **90s** | Crash → next tick within ~TTL (not 45m). Covers ≥2 missed renewals. |
| Renew interval | **30s** | ≥2 renewals before expiry under load. Deep sweeps here finish ~18–20s so renew may not fire mid-run; units cover renew. |
| Unknown-insert poison N | **5** | Enough for brief CH blips while Ping still succeeds; avoids infinite stall (F013). |
| DLQ retention | **30d** (`retention.ms=2592000000`) | Spec minimum. |
| Shutdown budget | **25s** | Below K8s default 30s grace. |

---

## 3. Per-part before/after + live proof

### B1 — Sync lock heartbeat (F028)

**Before:** `syncLockTTL = 45 * time.Minute` with no renew (`sync_lock.go`). Crash mid-run blocked ticks for up to 45m.

**After:** TTL 90s; Lua `pexpire` owner-only renew every 30s; lost ownership cancels run context; cancelled tick does not advance `last_sync_at` (`syncer.go` `withLock` / `lockHeartbeat`).

**Unit:** `TestRenewExtendsTTL`, `TestRenewFailsAfterOwnershipLost`, `TestReleaseOnlyDeletesOwnToken` — PASS.

**Live `[PROVEN]`:**
- Lock acquired with `pttl≈89914` (≈90s) during deep_sweep — `b1_orphan_ttl.txt`
- `docker kill` mid-hold → orphan free after **91s** (not 2700s); `last_sync_at` unchanged
- Init log: `lock_ttl=1m30s`, `lock_renew=30s`

**Limitation:** Live renew-while-held not observed because Ringba 30d deep sweep completes in ~20s (&lt; renew interval). Renew behaviour covered by units.

### B2 — API boots without Kafka (V6)

**Before:** `KafkaService.onModuleInit` awaited `producer.connect()` — Nest bootstrap failed when Redpanda down.

**After:** Producer connects in background with backoff; consumers start in background; `startResilientConsumer` does not throw out on initial connect failure; `publish` → **503** when unavailable.

**Endpoints that produce:** `POST /scoring/deepgram/callback` (via `KafkaService.publish`).

**Live `[PROVEN]`:** `b2_api_without_kafka.txt`, `b2_login_analytics.txt`, `b2_rejoin.txt`, `b2_ws_smoke.txt`
- Redpanda stopped + API restart → RestartCount **0→0**, `/healthz` 200, `/readyz` 200, login **201**, analytics **200**, `/live-status` not member
- Redpanda start → consumer joins **without** API restart; **WS SMOKE PASS**

### B3 — Restart policies

**After:** `deploy/docker-compose.yml` — `unless-stopped` on postgres, redis, redpanda, clickhouse, ingestion, consumer, api, platform-sync, web; `restart: "no"` on migrate, kafka-init, clickhouse-init. Intelligence services unchanged.

**Live `[PROVEN]`:** `b3_restart_policies.txt`, `b3_restart_live.txt` — consumer `kill 1` → RestartCount **0→1**, status running.

### B4 — Consumer hardening

**B4.1** Size flush failure keeps batch, sets `paused`, does not exit `Run`; resume flush before fetch. Log marker `size-triggered flush`. Offsets only after successful write.

**B4.2** CH unhealthy → retry forever; CH healthy + same message ≥5 unknown failures → poison → DLQ.

**B4.3** Topic `callloom.pings.v1.dlq` (kafka-init, 30d retention). Full value + headers (`x-source-topic/partition/offset`, `x-poison-reason`, `x-poisoned-at`). DLQ produce failure → no commit.

**B4.4** `GET /metrics` Prometheus text: `poison_total`, `dlq_total`, `flush_failures_total`, `retries_total`, `buffered_messages`, `last_successful_flush_timestamp`. Not wired to liveness.

**Unit:** `TestSizeTriggeredFlushKeepsRunAlive`, `TestUnknownErrorPoisonsAfterNWhenCHHealthy`, `TestUnknownErrorRetriesForeverWhenCHUnhealthy`, `TestDLQFailureDoesNotCommit` — PASS.

**Live `[PROVEN]`:**
- CH down + 8 smoke seeds (BATCH_SIZE=5): `flush_failures_total`↑, `retries_total`↑, PID unchanged — `b4_metrics_*.txt`, `b4_pid.txt`
- Poison `tenant_id=not-a-uuid` → DLQ with full payload + metadata; `poison_total=1` `dlq_total=1` — `b4_dlq_consume.txt`

### B5 — platform-sync readiness (F050)

**After:** `GET /readyz` = Postgres + Redis (dedicated 1.5s clients, hostname not IP-pinned). Kafka via `GET /kafka-status` only.

**Live `[PROVEN]`:** `b5_*.txt` — Redis down: healthz 200 / readyz **503**; Redpanda down: readyz **200**; restored 200.

### B6 — Graceful shutdown

**platform-sync:** SIGTERM → `rootCancel` (tickers + HTTP sync/window) → wait inflight ≤23s → owner lock release → exit. Budget 25s.

**api:** `enableShutdownHooks()`; Kafka disconnect in `OnModuleDestroy`.

**consumer:** cancel Run → final flush → commit written only; shutdown_ms logged; budget 25s.

**Live `[PROVEN]`:**
- `docker stop` mid deep_sweep: cancel + **sync lock released** in **12ms**; lock TTL **-2**; next tick HTTP 200 — `b6_ps_lockfix.txt`, `b6_ps_logs2.txt`
- Consumer stop with buffered seeds: shutdown flush; **6/6** smoke rows present after restart — `b6_consumer_summary.txt`
- API stop **~3577ms** — `b6_api_stop.txt`
- Checklist: `terminationGracePeriodSeconds ≥ 30s` + DLQ replay procedure

---

## 4. Gates

| Gate | Exit |
|---|---|
| verify-system (15/15, 11/11, LiveStatus, WebSocket) | **0** `[PROVEN]` |
| verify-ingestion-slice | **0** `[PROVEN]` |
| verify-presentation | **0** `[PROVEN]` |

Evidence: `docs/evidence/s1b/gate_system.txt`, `gate_ingestion.txt`, `gate_presentation.txt`.

---

## 5. Honest limitations / not fixed

- Live lock **renew** during a held run not observed (deep sweep ~20s &lt; 30s renew). Units cover it.
- Intelligence services still lack `restart: unless-stopped` (S4).
- DLQ replay tooling out of scope (procedure documented).
- Optional Docker Desktop reboot proof not run (operator step).

---

## 6. Commit / secret scan

Local commit only. Secret scan of `origin/main..HEAD` with prior patterns. **Do not push.**

STOP. S2 not started.
