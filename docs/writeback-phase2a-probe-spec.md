# Calyx — Phase 2A Probe: Per-Target Conversion Rules (READ-ONLY)

**RESEARCH ONLY. No writes. No product code. No UI. No sync, integration, or config
change. No Phase 2B.**

**Deliverable:** `docs/writeback-phase2a-probe-report.md`

---

## Why this exists

Phase 2A proposed a single duration threshold (13s) for the ⚠ Miss status. **That approach
is wrong** and is withdrawn: Calyx integrates with a whole Ringba account, which may have
20+ targets, each with its own conversion rule. One global number cannot be correct for all
of them, and a wrong threshold either hides real misses or floods the list with calls that
were never eligible to convert.

`[PROVEN]` from Ringba's Routing Plans documentation: conversion is configured **per
target / routing plan**, with `Convert on` = **Call length** (plus `Length` in seconds and
`Start Call Length On`: Incoming / Dial / Connect), **Call Successfully Connected**,
**Dialed**, or **Postback/Webhook**.

`[PROVEN]` (operator): the `Dynamic Duration` call-log column is **RTB-only**, so it cannot
serve as a universal per-call rule.

This probe establishes whether the rule can be derived per call from data Ringba exposes,
or whether it cannot be derived at all.

---

## Working standard

- Label every finding `[PROVEN]` (live response / official docs), `[INFERRED]`, `[UNKNOWN]`.
- **UI titles are not API identifiers.** Confirm ids against the columns dump or a live
  accept/reject.
- Report absence as absence. Never substitute a similar-looking field.
- "Not found" is a valid and useful answer. Do not work around it.
- No recommendations in the report — facts only.

---

## P1 — Targets API: are conversion settings readable?

`GET https://api.ringba.com/v2/{accountId}/targets` (confirm the current path; if it
redirects or 404s, report the response and try the path listed in the saved Postman
collection `docs/evidence/ringba-writeback-probe/postman-collection.json`).

Report:
1. Status code, and how many targets the account has.
2. **Full JSON of one target**, credentials redacted, so the shape is visible.
3. Whether each target exposes its conversion configuration, specifically:

| Ringba UI concept | Present in the API response? | Field name | Example value |
|---|---|---|---|
| Convert on (call length / connected / dialed / postback) | | | |
| Length (seconds) | | | |
| Start Call Length On (Incoming / Dial / Connect) | | | |

4. If a single target's detail endpoint (e.g. `GET /targets/{id}`) returns more than the
   list endpoint, report that too.

**If conversion settings are not in the targets API, say so plainly.** That is the decisive
negative answer and the probe continues to P2.

---

## P2 — How many distinct rules are actually in this account?

If P1 returned conversion settings, report a table of **every** target:

| Target name | Target ID | Convert on | Length (s) | Start counting on |
|---|---|---|---|---|

Then report:
- How many distinct rules exist
- Whether any target uses `Postback/Webhook` (those never auto-convert on duration — an
  important distinction for status logic)

This quantifies the operator's point: a single global threshold cannot be correct.

---

## P3 — 🔴 The decisive question: can the rule be derived per call?

**Can Calyx determine, for any given call, the duration that call needed in order to
convert — without hardcoding a number?**

### P3.1 — `Dynamic Duration` coverage

Find the API id for `Dynamic Duration` (and `Winning Bid Dynamic Duration`,
`Winning Bid Max Duration`) in the columns dump. Then, over the 30-day window, report:

| Field | present | absent | example values |
|---|---|---|---|

Break the presence down by call type where possible (RTB / ring-tree vs ordinary). The
operator states this is RTB-only — **confirm or refute it with counts.**

### P3.2 — Any other per-call field carrying the requirement

Search the columns dump for any field that could express a per-call duration requirement or
conversion criterion — e.g. target/routing-plan duration, conversion criteria, minimum
duration. List every candidate with its id and description, probe each, and report
present/absent counts with example values.

### P3.3 — Empirical check: does target + duration explain conversions?

This works whether or not P1 succeeded, and it is the strongest evidence available.

For the 30-day window, group calls **by `targetId`** and report:

| Target | Converted: n, min, p10, median | Not converted (connected, not blocked): n, max, p90, median |
|---|---|---|

The question: **for each target, is there a clean duration boundary** — a value where
calls above it converted and calls below it did not?

- If yes, report the apparent boundary per target and whether it matches P1's configured
  `Length` (that would be strong corroboration).
- If the pools overlap heavily for a target, say so — that target's conversions are not
  duration-driven (likely Postback/Webhook or Connected).

**Report the per-target numbers. Do not compute a global threshold.**

### P3.4 — The 8 short converted calls

Phase 2A found 8 converted calls in the 10–30s bucket while most converted calls were
300s+. Identify them and report: which target, which duration, `wasConversionAdjusted`,
and — if P1 gave rules — what that target's rule is.

The question: are they a different target with a short rule, a Postback/Webhook target, or
manually adjusted calls?

---

## P4 — Which duration field does the rule actually measure?

`[PROVEN]` Ringba's `Start Call Length On` can be **Incoming, Dial, or Connect**. Calyx
currently stores `connectedCallLengthInSeconds` (talk time only) as `duration_seconds`,
while `callLengthInSeconds` (whole call, including ring/IVR) is stored separately.

If a target counts from **Incoming**, comparing its configured Length against talk time is
comparing the wrong two numbers.

Report:
- For the targets in P2 (if available), which start event each uses.
- On sample calls, both values side by side (`connectedCallLengthInSeconds` vs
  `callLengthInSeconds`) and the size of the gap.
- Whether the boundary found in P3.3 aligns better with talk time or whole-call length.

---

## Report structure — `docs/writeback-phase2a-probe-report.md`

1. **One-line answer to P3:** can the per-call conversion requirement be derived — yes (by
   what mechanism), or no.
2. P1: targets API — conversion settings readable, yes/no, with the sample JSON.
3. P2: full target/rule table, count of distinct rules.
4. P3.1–P3.4: coverage counts, per-target duration table, the 8 short conversions.
5. P4: which duration field the rule measures.
6. Honest limitations.
7. Found but not fixed.

No recommendations. The status rule is decided after this report, together with the operator.

---

## Out of scope

- Any write endpoint; any change to sync, integration rows, config, or code
- Phase 2B (status derivation, filter, list UI)
- Choosing or implementing a threshold
- The pagination-skip fix (431 missed calls) — separate work, already reported
- Retreaver; TrackDrive; CallGrid
