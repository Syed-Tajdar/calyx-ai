# Calyx — Phase 2A-fix2e: Outage Safety of Poison Handling

**Scope:** three open points from fix2d. **No Phase 2B. No deletions. No push.**
**Deliverable:** `docs/writeback-phase2a-fix2e-report.md`

## ⛔ RULE
Parts independent. Evidence before code changes. Label `[PROVEN]` / `[INFERRED]` /
`[UNKNOWN]`. Evidence from rebuilt images. All probe traffic uses the `smoke-` prefix.

## PART 1 — 🔴 A ClickHouse outage must not be classified as poison

fix2d added: on batch failure, retry per message; messages that fail are skipped as poison
and their offsets committed. It was tested with a malformed message only.

**Risk:** during a ClickHouse outage every per-message insert fails. If those failures are
classified as poison, every message is skipped and committed — the whole outage window's
calls are lost permanently, logged only as "poison skipped".

**1.1 — Report the classifier:** exactly which errors count as poison and which as
transient. Show the code.

**1.2 — Prove it under a real outage:**
- Record `poison_total`.
- Stop ClickHouse (`docker stop deploy-clickhouse-1`).
- Send N `smoke-` pings (e.g. 20) through ingestion.
- Wait ≥60s. Start ClickHouse.
- Report: all N land in `pings`; `poison_total` unchanged; consumer log shows retry, not
  skip; offsets committed only after the successful write.

If any message is skipped as poison during the outage, that is the finding — fix the
classifier so only deterministic data errors are poison, then re-run the proof.

**1.3 — Skipped messages must be recoverable in principle:** a poison skip must log enough
to identify the message (external_call_id, tenant, platform, partition/offset, reason).
Report what is logged now; add what is missing.

## PART 2 — Postgres timeout scope

fix2d set `query_timeout` / `statement_timeout` / `connectionTimeoutMillis` = 1500 ms on
`pg.Pool`.

**2.1 — Report:** is that pool used only by `/readyz`, or by application queries too?
**2.2 — If shared:** give `/readyz` its own dedicated client or small pool with the short
timeouts, and restore application queries to a timeout chosen for real workloads (report
the value and why). Re-run the ≥100s broker-outage readyz proof: 0 failures.

## PART 3 — Label correction + checklist

**3.1 —** In the fix2d report, the `/readyz` cause ("stopping the broker stalls PG/CH pings
over the Docker network") is not demonstrated — the fix is. Add a note that the mechanism
is `[INFERRED]`.

**3.2 —** Add under **Data** in `docs/deploy-checklist.md`:
```
- [ ] **Data retention (TTL)** — `pings` rows expire at `received_at + 24 months`.
      Decide the retention period with management (billing disputes, customer history),
      and whether it should key on `occurred_at` (call time) instead of `received_at`
      (last sync time — every republish resets it).
```

## PART 4 — Gates, commit, scan
All three gates exit 0 with sync live. Commit locally. Secret scan of every commit since
`b47335e` (same patterns as fix2c — show them). **Do not push.** STOP. Do not start 2B.
