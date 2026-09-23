# Calyx — Phase 2A-fix2c: Blast Radius + Probe Data + Rescan

**Scope:** fix a design error in fix2b before anything is pushed. **No Phase 2B. No
deletions without approval. No push.**

**Deliverable:** `docs/writeback-phase2a-fix2c-report.md`

## ⛔ RULE
Parts are independent — if one blocks, report and continue. Label claims `[PROVEN]` /
`[INFERRED]` / `[UNKNOWN]`. Evidence from rebuilt images. **Do not push.**

---

## PART 1 — Consumer health must not take down the API

**Design error (spec, not implementation):** fix2b made `/readyz` return 503 when the live
consumer is not a group member, and added a watchdog that `process.exit(1)`s after 60s.
In Kubernetes, a failing readiness probe removes the pod from service. A Kafka outage would
therefore make the **entire API** unreachable — login, dashboards, reports — although
ClickHouse and Postgres are healthy. A long outage would also restart the API every ~60s.

**1.1 — Separate the signal.** Move consumer health to its own endpoint (e.g.
`GET /live-status`: group, member yes/no, last join, lag if available). `/readyz` must
reflect only what is required to serve API requests.

**1.2 — Keep the gate honest.** `verify-system.ps1` must check the new endpoint and fail
when the live consumer is not a member — so a dead live feed still turns the gate red.
Coverage change only; do not weaken other checks.

**1.3 — Watchdog recovers the consumer, not the process.** On sustained non-membership,
tear down and recreate the **consumer instance** with backoff. Do not exit the process for
consumer-only failures. Applies to both live feed and alert-engine consumers. If you judge
a process restart still necessary for some case, state the case and justify it.

**1.4 — Prove both paths (fix2b proved neither):**
- Short broker interruption (<60s): consumer rejoins on its own. API HTTP stays 200
  throughout (`/readyz`, `/auth/login`, one analytics read). `/live-status` shows down →
  up.
- **Long** broker interruption (well over the watchdog window): watchdog recreates the
  consumer; API HTTP stays 200 the whole time; `/live-status` down → up after the broker
  returns. Report time-to-recover and API restart count (expected 0).
- ws-smoke PASS after each.

## PART 2 — Probe traffic in `pings`

fix2 produced ~3,000 messages to `callloom.pings.v1` for a lag test; fix2b reports
`ani_hash = "abc"` probe traffic.

**Report only — no deletion:**
- How those messages were built (IDs, prefixes, platform, tenant).
- Whether they landed in ClickHouse `pings` (count, ID pattern).
- Whether they are caught by the test-data exclusion (`apisync-` / `demo-` / `smoke-`).
- Default KPI `total_pings` now vs real Ringba distinct calls — do they still match?

If uncaught rows exist, list them and propose an **exact-ID** delete statement. **Do not
run it.** Future probe traffic must use the `smoke-` prefix — note it.

## PART 3 — Lock release safety

Confirm the single-flight lock is released only by its owner (compare-and-delete on a
unique token), so a run that ends after its lock expired cannot delete another run's lock.
Report the code path. If it is a plain `DEL`, fix it and add a unit test.

## PART 4 — Gates, commit, rescan

- Gates with sync live: `verify-system` (exit 0, 15/15, 11/11, WS PASS, live-status
  check), `verify-ingestion-slice`, `verify-presentation`.
- Commit locally; report hash, clean `git status`.
- **Rescan all commits since `8f4c8f7`** with correct patterns: Ringba account IDs
  (`RA` + hex), long token-like strings (≥32 chars) outside known placeholders, JWTs
  (`eyJ`), non-placeholder encryption keys, phone numbers (10–11 digits) in `docs/evidence/`
  other than `15551234567`. Show the patterns used. List dev placeholders separately.

**Do not push.** STOP after the report. Do not start Phase 2B.
