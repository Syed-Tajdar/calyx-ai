# Calyx — S0: Verify Before Hardening (READ-ONLY)

**Purpose:** confirm or refute, with runtime evidence, the audit findings that the fix
slices depend on — and collect the runtime facts the audit could not (Docker was down).
**No code, config, or schema changes. No push. No Phase 2B.**

**Deliverable:** `docs/s0-verify-report.md`

---

## ⛔ RULE

- **Runtime evidence decides.** A finding "proven" by reading code is not proven if the
  running system contradicts it. Every verdict cites what was observed.
- Read-only by default. **The only permitted state changes** are in V2: stopping and
  starting the ClickHouse container, and sending `smoke-`-prefixed webhook pings.
- **V1 and V2 must not overlap.** A ClickHouse outage during V1's observation window
  corrupts V1's result.
- Label `[PROVEN]` / `[INFERRED]` / `[UNKNOWN]`. "Inconclusive" is an acceptable verdict —
  state what would make it conclusive.

## Pre-check
`docker compose ps` — report every container's state, `RestartCount`, and configured
restart policy (`docker inspect`). All must be running before V1–V5 start.

---

## V1 — F003: does the Ringba sync miss new calls? (P0 if true)

**The audit claim** (from code, `ringba_adapter.go:64-88`, `:111-124`): after a successful
sync the stored cursor holds an absolute `[ReportStart, ReportEnd]`; the next tick decodes
it and **overwrites** the fresh `last_sync − overlap → now` window — so the window freezes
and new calls are missed.

**Runtime evidence against it:** Phase 2A-fix saw ticks publish new calls as the call
count rose; fix2d reconciled Ringba 124 = ClickHouse 124 for Sep 22–24. Both may have
happened shortly after a cursor clear, so neither settles it.

### V1.1 — Code path, precisely
State exactly what window each tick requests, step by step, with `file:line`. If the code
reading is ambiguous, say so.

### V1.2 — Observe over time (the decisive test)
Integration `ce6fd338`. Do **not** clear the cursor. Sample at T0, T0+30m, T0+60m,
T0+120m (minimum 2 hours):

| Sample | `sync_cursor` (decoded) | `last_sync_at` | Latest `occurred_at` in CH (Ringba, real) | Latest `callDt` on Ringba (read-only Get Call Log, last 3h) |
|---|---|---|---|---|

Also from platform-sync logs for the same span: each tick's requested window if logged,
`calls` / `published`.

### V1.3 — Reconcile the observation window
For `[T0 − 1h, T0 + 120m]`: Ringba API distinct calls vs ClickHouse distinct calls
(by `occurred_at`). List any Ringba ids missing from ClickHouse.

**Traffic check:** if Ringba received **fewer than 10 calls** in the window, the result is
**inconclusive** — say so and extend the window. Ringba traffic follows US business hours.

**Verdict:** F003 **true** / **false** / **inconclusive**, with the evidence.

---

## V2 — F015: does the consumer die on a size-triggered flush during an outage?

fix2e proved outage safety with 20 messages — the **ticker** flush path. The audit says the
**size-triggered** path (`consumer.go:91-93`) exits `Run` on a transient error while
`/healthz` and `/readyz` stay green, and the container has no restart policy.

**Run only when V1's window is closed.**

1. Report the consumer's batch size and flush interval from config. **If the batch size is
   above 2,000, STOP this part and report** — do not change config to make it testable.
2. Record: `poison_total`, consumer container state, consumer group membership / lag,
   `/healthz`, `/readyz`.
3. `docker stop deploy-clickhouse-1`.
4. Send **batch size + 50** `smoke-s0-*` pings through ingestion (the normal webhook path).
5. Wait ≥ 60s. Record the same five things **during** the outage.
6. `docker start deploy-clickhouse-1`. Wait for recovery. Record again.
7. Count `smoke-s0-%` rows in ClickHouse.

**Verdict:** does the consumer process keep running? Does health stay green while the
consume loop is dead? Do all pings land after recovery?
- All land, loop alive → F015 size path **false**.
- Loop exits, health green, pings stuck → F015 **true** (P0 confirmed).

---

## V3 — Live Kafka and container facts (audit ✗ reason 3)

- `rpk topic describe` for every topic: partitions, replication factor, `retention.ms`,
  `retention.bytes`, cleanup policy.
- `rpk group list` + `describe` for every consumer group: state, members, lag.
- Restart policy of every container (from `docker inspect`) — complete list.
- Redis: `maxmemory`, `maxmemory-policy`, `evicted_keys` from `INFO`.

---

## V4 — Tenant isolation: every ClickHouse and Postgres read in v1 scope

v1 services: `ingestion`, `consumer`, `platform-sync`, `api`, `web`.

### V4.1 — ClickHouse
List **every** ClickHouse query in `services/api` (and any other v1 service that reads
tenant data):

| `file:line` | Endpoint / caller | Filters `tenant_id`? | Tenant value from | Parameterised or interpolated? |
|---|---|---|---|---|

"Tenant value from" must be one of: JWT/auth context · request body/query · none.
**Any row that is not "filtered, from auth context, parameterised" is a finding.** Do not
summarise — list every query.

### V4.2 — Postgres
For every tenant-scoped table: RLS enabled? `FORCE ROW LEVEL SECURITY`? Which role do the
app paths use — can it bypass RLS? (`SELECT relname, relrowsecurity, relforcerowsecurity
FROM pg_class …`)

---

## V5 — Time handling

### V5.1 — Ringba `callDt`
Show the **raw** `callDt` string from Get Call Log for 3 calls (format, offset / `Z` or
none). Show the stored `occurred_at` for the same 3 calls in ClickHouse. Is the conversion
correct? If `callDt` carries no offset, what does the adapter assume?

**Operator cross-check:** name one of the 3 call ids and its expected time in Ringba's UI,
so the operator can confirm the timezone against what Ringba displays.

### V5.2 — Retreaver `created_at`
Code only (Retreaver sync is off): format expected, parsing, stored timezone.

### V5.3 — Period filters
Confirm F053 at runtime: for "7 days", what `from`/`to` does the web send, and what does
the API use? State the timezone of each.

---

## Output — `docs/s0-verify-report.md`

1. **Verdict table**

| Item | Verdict | Severity now | Evidence |
|---|---|---|---|
| F003 frozen cursor | true / false / inconclusive | | |
| F015 size-flush exit | | | |
| Kafka retention vs outage | | | |
| Tenant filter — every CH query | all clean / N findings | | |
| Postgres RLS / FORCE | | | |
| Ringba `callDt` timezone | | | |
| Period-filter timezone | | | |

2. Per item: what was run, what was observed, raw output.
3. Any new finding discovered during verification — listed separately.
4. Container state after V2 recovery — everything back to running.

STOP. No fixes. No Phase 2B. No push.
