# Calyx — Write-Back Phase 1b: Close Gate 1

**Scope: close the three Gate 1 FAILs only. Nothing else.**

Phase 1's backend is complete and its guards are proven. Three items failed for reasons
that were **not code defects**:

| FAIL | Cause |
|---|---|
| Live *confirmed* convert | Only sandbox call in the 7d window was the G-void call — already converted-then-voided (dirty). A voided/adjusted call does not reconcile via inbound conversion |
| Live *confirmed* void | Same call; it was not converted at read-back, so void correctly refused |
| `verify-ingestion-slice` / `verify-system` exit 0 | Host has no Go toolchain; `go run ./cmd/platformsig` failed |

**This phase fixes the test conditions, not the feature.** If a code change appears
necessary, **STOP and report** — do not change the write path to make a proof pass.

**STOP RULE:** complete 1b, write `docs/writeback-phase1b-report.md`, then STOP.
**Do not start Phase 2** until explicitly instructed.

**Deliverable:** `docs/writeback-phase1b-report.md`

---

## SANDBOX — fresh clean call (operator supplied)

```
Campaign:  CALYX-SANDBOX-DO-NOT-USE
Call ID:   RGB512060522F815F149F10AD1B143CCF4BD5E5F709V3LIN01

Target already set to convert on Postback/Webhook (from Phase G).
Publisher/buyer internal, $0 terms. Recording disabled.
This call has never been converted, voided, or adjusted.
Placed by the operator just now; untouched since.
```

**The old G-void call `…V3NYI01` is not a target in this phase. Do not write to it.**

**API sync is NOT needed and must NOT be turned on.** Write-back resolves the call directly
from Ringba via `LookupCall` (7-day Get Call Log scan) — the call does not need to be in
ClickHouse. Leave the Ringba integration at `sync_mode=mock`. Turning sync live would
re-ingest thousands of calls and undo pending cleanup.

---

## 🛡️ Safety — unchanged

- The **only** write target is the call ID above.
- One request per step. Read back after every write.
- Stop and report on anything unexpected; do not quietly correct it.
- Token redacted in captures.

---

## 1b.1 — Verify the call is genuinely clean (before any write)

Read the new call via `LookupCall` / Get Call Log and confirm:
```
hasConverted             ABSENT        ← the decisive signal: never converted
conversionAmount         absent / 0
payoutAmount             absent / 0
wasConversionAdjusted    absent / false
wasPayoutAdjusted        absent / false
hasConnected             true          ← the call actually connected
campaignName             = the sandbox campaign
```

**If the call is not found**, report why (7-day window, pagination, account) — do not
turn on sync to find it.

**If `hasConverted` is present, or any adjustment flag is set, STOP** and request another
call. Phase 1's confirmed-path proof is only meaningful on a virgin call.

Also report `connectedCallLengthInSeconds` — it feeds the Phase 2 duration threshold later.

---

## 1b.2 — Live CONFIRMED convert

Through the **product path** (not the research probe) — this is what Phase 1 built:

- Nest: `POST /integrations/:id/writeback/convert`, tenant A JWT, amount **$1.00**
- Report: HTTP status, response body, and the settle behaviour
- Report the audit row: outcome must be **`confirmed`**

Read back and report:
```
hasConverted = ?     conversionAmount = ?
```
Confirmed means `hasConverted=true` **AND** `conversionAmount` matches the amount sent.

**If it lands pending instead:** report it as pending with the full poll timeline, and say
plainly whether the 12s window was too short for this call. Do not extend the window to
manufacture a pass — report the real timing and let it be a finding.

**Also capture, if it occurs naturally:** the Nest **202** mapping on a pending response
(Phase 1 fixed a wrong 201 but could not re-fire to verify). If this convert confirms
rather than pends, 202 stays `[UNKNOWN]` — that is acceptable; note it.

---

## 1b.3 — Live CONFIRMED void

On the same call, now converted at $1.00:

- Nest void endpoint, tenant A JWT
- Report HTTP status, body, audit row outcome — must be **`confirmed`**
- Read back: `hasConverted`, `conversionAmount`, `payoutAmount`,
  `wasConversionAdjusted`, `wasPayoutAdjusted`

Report what void actually did to each field on a **cleanly converted** call — this is the
first time we see void on a call with a known-clean history, so the field signature is
worth recording precisely.

**Reversibility:** do **not** re-convert in this phase. Reversibility is already
`[PROVEN]` from G-void research. Leave the call voided.

---

## 1b.4 — Verify scripts: run them where Go exists

`verify-ingestion-slice.ps1` fails on the host because it shells out to
`go run ./cmd/platformsig`. The rest of the stack runs in Docker.

- Make the gate scripts runnable: install the Go toolchain on the host **or** run them in
  an environment where Go is available (e.g. the `golang:1.22-bookworm` image already used
  for unit tests). Choose one and state which.
- Report raw output and exit codes for **both**:
  - `scripts/verify-ingestion-slice.ps1`
  - `scripts/verify-system.ps1`
- If a script cannot run for a structural reason (needs the host Docker socket,
  PowerShell, etc.), report the reason plainly rather than skipping it.

**Do not modify the scripts' assertions to pass.** If a script needs a change to run in
the chosen environment, report what change would be needed.

---

## 1b.5 — Two small items from the Phase 1 report

- **README** still documents the retired `RINGBA_WRITEBACK_ALLOWED_CAMPAIGN_IDS`. Remove
  that line so the docs match the code (per-tenant flag replaced it). Docs only.
- **409 already-in-progress writes no audit row** — it is rejected at the lock before the
  audit insert. Do **not** change it now. Record it in the report as a known gap: a user's
  repeated attempts are visible only in logs, not in the audit table.

---

## Gate 1 — final scoreboard in the report

Restate all nine Gate 1 items with their final status:

| Gate 1 item | Phase 1 | Phase 1b |
|---|---|---|
| Auth proof | PASS | — |
| Two-level gate + migrations with down | PASS | — |
| Cross-tenant refusal | PASS | — |
| Double-click → single fire | PASS | — |
| Confirmed / pending / failed all demonstrated | FAIL | ? |
| Void + read-back | FAIL | ? |
| 400-not-500 on invalid amount | PASS | — |
| Audit rows per attempt | PASS | — |
| Gate scripts exit 0 | FAIL | ? |

**State plainly: is Gate 1 now closed — yes or no?** If any item is still FAIL, say so and
say why. A FAIL honestly reported is the correct outcome; a manufactured PASS is not.

---
## Out of scope

- Phase 2 (call list / status / filter) — do not start
- Any UI
- Turning API sync live / changing `sync_mode`
- Changing the write path, settle window, or lock behaviour to make a proof pass
- Writing to `…V3NYI01` or any production call
- Re-converting after the void
- Modifying verify-script assertions
