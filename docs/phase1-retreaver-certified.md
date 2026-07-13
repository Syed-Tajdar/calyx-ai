# Phase 1 — Retreaver Certified

**Date:** 2026-07-10  
**Verdict: Retreaver CERTIFIED: YES**  
**Integration under test:** `4ec62a84-c1ba-48de-bc50-baa79e3fc19c` (“test rtt acc”)  
**End state:** flipped back to **Test** (`sync_mode=mock`). Only one platform was Live during the phase.

Credentials are never printed below.

---

## 1.1 Clean re-sync (`occurred_at` real everywhere)

### Before / delete / Redis

```
=== 1.1 BEFORE count (real retreaver rows, not apisync) ===
435

=== DELETE pre-fix real-pull rows ===
AFTER delete count:
0

=== REDIS dedup keys ===
key_count=454
sample: 11111111-1111-1111-1111-111111111111:dedup:retreaver:003a5c3d-…
deleted=454
```

### Live sync

```
=== Flip Retreaver Live + sync ===
sync_mode=real
sync={"status":"sync_triggered"}

=== AFTER re-sync counts ===
227	227
```

(`with_time=227`, `total=227` for new real rows in the last hour — all `occurred_at > 0`)

### Distinct call times (not one batch timestamp)

```
9438f4c8-6318-494e-8537-2f6683eeaee9	2026-07-10 15:13:26	0	1
882605ef-9404-4140-b328-c0d0b9ff12b1	2026-07-10 15:13:09	0	1
4a264d58-7d90-4ac4-aa0e-37a34bf252bc	2026-07-10 14:59:56	0	0
ac1737c7-7b84-42d6-8ce8-830539fb4ef7	2026-07-10 14:59:04	0	0
c16ea1c1-3a2a-4d3e-8222-17af1b39138e	2026-07-10 14:58:56	0	0
0584aa8a-c856-4368-89de-5513b237f1dc	2026-07-10 14:58:46	0	0
9e97bb4c-1d29-4a7e-b9a1-d6e004203a2b	2026-07-10 14:51:27	0	0
373826a5-e760-4a43-8555-fbd04bc1284e	2026-07-10 14:51:02	0	0
8e626c86-4738-4518-901f-fbe453711ecc	2026-07-10 14:50:52	0	0
82c417e2-2d50-497f-9ec3-f67e6f618fa0	2026-07-10 14:43:29	46.5	0
…
```

Webhook smoke rows without `occurred_at` (expected; not API-sync):

```
eed10fa6-…	1970-01-01	5.5	incoming
81560ccb-…	1970-01-01	9.75	incoming
```

---

## 1.2 D9 — update semantics

Synthetic update pair (revenue 0 → 46.5 with `is_duplicate=1`):

```
=== D9 re-insert update pair callId=phase1-d9-update-8e9e7500 ===
phase1-d9-update-8e9e7500	0	0
phase1-d9-update-8e9e7500	46.5	1
1	46.5
pings_total=2
rev=0 dup=0
rev=46.5 dup=1
```

Real non-zero revenue still visible via `argMax`:

```
a4acb071-ac36-4f57-ba47-a4b8ee191832	56.46	1
d94fa217-7f4f-4648-9b41-bc67d5698d0b	46.5	1
45d70345-7764-48bd-9f8b-c71118850d2b	46.5	1
…
```

**D9: PASS** — latest revenue wins; call counted once; `/pings` still shows both rows.

---

## 1.3 Golden fixture

`services/platform-sync/internal/adapter/testdata/retreaver_calls.json`

- Header: `source: real-account (scrubbed)`
- Doc: Retreaver Apiary calls reference
- Shape: nested V2 `{"call":{…}}`

---

## 1.4 Conformance — `retreaver` → certified

```
=== RUN   TestRegistryMembershipComplete
--- PASS: TestRegistryMembershipComplete (0.00s)
=== RUN   TestCertifiedAdapterConformance
=== RUN   TestCertifiedAdapterConformance/retreaver
--- PASS: TestCertifiedAdapterConformance (0.00s)
    --- PASS: TestCertifiedAdapterConformance/retreaver (0.00s)
PASS
ok  	…/internal/adapter	0.866s
```

Pending (occurred_at **not** mapped yet): `ringba`, `trackdrive`, `callgrid`.

---

## 1.5 D11 verify scripts + flip to Test

```
API checks: 20 / 20 passed
PRESENTATION VERIFY PASS

ALL PLATFORMS PASS (4/4)

=== API Sync verify complete ===
PASS: analytics.total_pings increased …
PASS: found apisync ping …

INGESTION SLICE VERIFY PASS

WS SMOKE PASS: 3 events, types: ping_received, compliance_violation
```

### Flip back to Test

```
flipped sync_mode=mock name=test rtt acc platform=retreaver

 platform  | name                   | sync_mode
 retreaver | test rtt acc           | mock
 retreaver | api-sync-verify-…      | mock
```

---

## Definition of Done checklist (Retreaver)

| ID | Status |
|----|--------|
| D1 Research | ✅ (prior + research doc) |
| D2 Fixture real-account | ✅ |
| D3 Conformance certified | ✅ |
| D4 occurred_at mapped | ✅ (`created_at`) |
| D5 NormalizeState | ✅ |
| D6 Redaction | ✅ (prior + suite) |
| D7 Empty-id guard | ✅ (suite + syncer test) |
| D8 A1 proof | ✅ (prior phase: HTTP 200 by UUID) |
| D9 Updates | ✅ |
| D10 Volume | ✅ (~227 distinct after clean pull; order-of-magnitude vs prior 435 raw rows which included dups) |
| D11 Scripts | ✅ 20/20, 4/4, api-sync, ingestion, ws |
| D12 This report | ✅ |

---

## Gate

**Do not start Phase 2 (Ringba) until this file says CERTIFIED: YES.**

**Retreaver CERTIFIED: YES**
