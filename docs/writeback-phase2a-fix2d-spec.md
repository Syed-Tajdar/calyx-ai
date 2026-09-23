# Calyx — Phase 2A-fix2d: Readiness Under Load + Poison-Message Handling

**Scope:** two questions fix2c left open. **No Phase 2B. No deletions. No push.**
**Deliverable:** `docs/writeback-phase2a-fix2d-report.md`

## ⛔ RULE
Parts independent. Find causes by evidence before changing code. Label `[PROVEN]` /
`[INFERRED]` / `[UNKNOWN]`. Evidence from rebuilt images.

## PART 1 — Why did `/readyz` fail?

fix2c: `/readyz` checks only Postgres + ClickHouse, yet failed once (short broker outage)
and 3 times (long outage, "reconnect storm"). Both databases were healthy.

**1.1 — Report:** for each failure — HTTP status or timeout, response time, timestamp,
and whether the failures were **consecutive**. What timeout does the probe use?

**1.2 — Cause:** determine by evidence. Candidates: event-loop blocking during KafkaJS
reconnect (measure event-loop lag during a broker outage); readyz DB queries slowed by
something else; probe timeout too tight. Rule each in or out.

**1.3 — Fix the named cause** so `/readyz` stays responsive through a broker outage.
Prove with the long-outage test (≥100s): `/readyz` **0 failures**, event-loop lag reported
before/during/after.

## PART 2 — What happens to a bad message?

fix2 lag probes (`ani_hash="abc"`, ~5,000 messages) were rejected by ClickHouse
(`ani_hash` is `FixedString(64)`) and never reached `pings`.

**2.1 — Report the consumer's behaviour on an insert failure**, from code and logs:
- Is the failed message skipped individually, or does the **whole batch** fail?
- If the batch fails, are the valid messages in it written, retried, or lost?
- Is the offset committed past the bad message, or does the consumer retry it forever?
- Is anything logged or counted (dead-letter, metric)?

**2.2 — Reconcile against Ringba, not against ClickHouse.** The fix2c "KPI match" compared
two ClickHouse reads. For the time window when the probe batches were consumed, compare
**Ringba API distinct calls** vs **ClickHouse distinct calls**. Report both and any
missing ids.

**2.3 — If valid messages can be lost with a bad one:** that is the finding. Fix so a bad
message is isolated — logged, counted, skipped or dead-lettered — and valid messages in
the same batch are written. Prove: produce a batch mixing valid `smoke-` pings with one
malformed message → valid rows land, bad one is reported, consumer keeps moving.
All probe traffic must use the `smoke-` prefix.

## PART 3 — Checklist
Add under a new **Infrastructure** section in `docs/deploy-checklist.md`:
```
- [ ] **Kubernetes probes** — readiness (`/readyz`) must depend only on what is needed
      to serve requests (Postgres, ClickHouse). Kafka / live-feed health is monitored via
      `/live-status`, never wired into readiness or liveness.
```

## PART 4 — Gates and commit
All three gates exit 0 with sync live. Commit locally, report hash, clean `git status`.
**Do not push.** STOP. Do not start Phase 2B.
