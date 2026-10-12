# Calyx — v1 go-live prep (last UI fixes + facts for moving live sync to the server)

**Branch:** `v1/golive-prep`, created from `main` @ `14c3068` · **Operator:** Talha · **Owner:** Syed
**Output:** `docs/v1-golive-prep-report.md`
**Status:** v1 hardening is deployed on the server (`APP_ENV=production`, new secrets, only the
v1 services running). The server has **no live data yet**. Live sync still runs only from the
laptop (Rule 10).

## Why

The owner reviewed the server after the hardening deploy and found four problems:
- Reports shows **made-up money**: 0 pings, but Ad Spend $370, CPA $370.00, ROI −100% and
  Profit −$370.
- Settings still shows a **Providers** tab (Meta CAPI, Deepgram, DNC.com, OpenAI, all
  "Mock"). None of these are in v1.
- The sidebar badge says **"Mock · Demo Tenant"**, and platform-sync logs `"mock_mode": true`.
- The 30-day chart's date labels overlap and cannot be read.

Before the owner enters the real Ringba and Retreaver keys on the server, these must be fixed,
and we need exact facts about how a new Live integration behaves on the server.

## ⛔ Rules

1. The same rules as `docs/v1-hardening-spec.md` apply:
   - The admin login does not change.
   - Never print secrets.
   - No Live sync, no Ringba or Retreaver writes.
   - Gates run with `pwsh-preview` in `-V1` mode.
   - One commit per item (prefix `v1-golive:`).
   - Push the branch only, **never `main`**. Stop at every 🙋.
2. Do not change the laptop `.env` or its encryption key.
3. Nothing is deleted from the database, including the demo rows (owner decision). Hide things
   behind the existing v1 flag or the A6 demo switch; do not remove code.

---

## Part A — UI fixes

**A1 — Reports: no made-up money.**
1. Find where the **$370 ad spend** comes from (table, seed or demo row) and why it shows with
   "Show demo" **off**. Report the source.
2. v1 has no ad-spend source. With the v1 flag on:
   - Hide **Ad Spend, CPA and ROI** everywhere, including the P&L chart.
   - "Profit" becomes **"Profit (gross)" = revenue − payout**.
   - Nothing on Reports may show a number that does not come from real calls.
3. Proof: Reports with Show demo off shows 0 / $0 everywhere. With Show demo on, the numbers
   come only from demo calls, and gross profit = revenue − payout.

**A2 — Settings: v1 tabs only.**
- Hide the **Providers** tab with the v1 flag.
- **Alerts** tab: tell me what alert rules exist and what data they use. If they depend on
  scoring, compliance or anything else that is off in v1, hide the tab too. If they work on
  v1 data (calls, revenue, sync errors), keep it and show one rule working. 🙋 if unsure.

**A3 — Chart labels.** The 30-day (and all-time) x-axis must show at most ~8 readable date
labels. Points stay one per day. The tooltip still shows the exact date.

**A4 — "Mock" badge and `MOCK_MODE`.**
1. For each v1 service (api, web, platform-sync, ingestion, consumer), say exactly what
   `MOCK_MODE` changes. Give file:line.
   - Does `mock_mode=true` in platform-sync ever stop a **Live** integration from pulling real
     data, given `per_integration_sync_mode=true`?
   - Does it generate any fake data on the server?
2. Propose the value the server should use for v1, and make it env-driven if it is not.
3. The sidebar badge must tell the truth:
   - **"Live"** when the tenant has at least one active Live integration
   - **"Test"** otherwise
   - Never "Mock" in v1
4. Proof: badge screenshots in both states, on the rehearsal stack (dummy Live integration
   with fake keys; it will fail to sync, which is fine).

**A5 — Tenant name "Devdabs" (owner decision).** On the server, the tenant that the owner
logs into (today "Demo Tenant") must be called **`Devdabs`**.
1. Find where the tenant name is stored and shown: the sidebar, and anywhere else.
2. **Rename** the existing tenant. Do **not** create a new one. Its users, integrations and
   data stay as they are.
3. Demo rows stay in the database, but they stay hidden while "Show demo" is off.
4. If there is no rename in the UI or API, write a one-line `UPDATE` for the server deploy
   note. It must change only the name, and target the tenant by its id, which a read-only
   query finds first.
5. Check that nothing uses the tenant name as a key: slug, cache key, ClickHouse partition or
   anything similar. 🙋 if something does.
6. Proof: on the rehearsal stack, the sidebar shows "Devdabs", login still works, and the
   counts are unchanged.

**A6 — Demo mode OFF in production, nothing erased (owner decision).** The demo rows **stay in
the database**. Production must never show or create demo data. Laptop and dev keep demo mode
exactly as today.
1. Add one env switch, e.g. `DEMO_MODE_ENABLED`:
   - default `true` for local dev
   - **forced `false` when `APP_ENV=production`**, unless explicitly set. 🙋 if you think it
     should be allowed at all in production.
2. With demo mode off, the **API** (not just the web) excludes demo rows from every endpoint:
   - metrics, ping-series, platform-mix, coverage, reports (including any ad-spend source from
     A1)
   - the pings list and its count
   - CSV export
   - call detail: a demo call id returns 404

   The web cannot ask for demo rows; a `show_demo`/`include_demo` parameter is ignored.
3. With demo mode off, the web hides:
   - the **"Show demo"** checkbox and the "DEMO INCLUDED" badge on every page
   - the **"Test — demo calls"** data-source option in Settings, so a new integration can only
     be **Live**
4. With demo mode off, nothing **creates** demo data. Any existing integration in
   Test/mock mode does not generate calls on the server. Show the platform-sync code path
   (file:line) and a log line proving it skips them.
5. Existing Test/mock integrations stay in the table, unchanged. List what exists on the server
   tenant, if you can tell from the code or seed.
6. Proof, on the rehearsal stack with `APP_ENV=production`:
   - Dashboard, Pings, Reports, Coverage and CSV export all show 0 / $0, although demo rows
     exist in the database (show the database count).
   - A demo call id returns 404.
   - No Show-demo toggle and no Test option are visible.
   - Then on the laptop (dev), everything is as before, including the gate default view of
     3,652.

---

## Part B — Facts for the go-live day (no code unless a gap is found 🙋)

Answer each with file:line and, where possible, a test or log line.

**B1 — New Live integration, first pull.** When the owner saves a **new** Ringba or Retreaver
integration as Live on the server:
- What date range does the first sync pull? What does the "Lookback" column mean?
- How do we get a full **30-day backfill** (the same range as the laptop certification)? Is it
  automatic, a UI action, or a command for Aryan? Give the exact command if it is a command.
- How long should 30 days take for Ringba (75 req/min) and for Retreaver (2/s, 60/min), using
  the laptop numbers: Ringba 4,716 calls, Retreaver 3,493 calls per 30 days?

**B2 — Sweeps on the server.** Confirm the hourly 7-day and daily 30-day sweeps run for a Live
integration under `APP_ENV=production`, and at what times (UTC).

**B3 — Health in the UI.** Where does the owner see that a sync is working or failing?
- `sync_status`
- last sync time
- `last_sync_error` text
- the degraded state

Screenshot each state from the rehearsal stack (a fake key gives the error state).

**B4 — Reconciliation query (read-only, for Aryan).** Write one ClickHouse query that Aryan
can run on the server. It must use the same deduplicated view the dashboard uses
(argMax/FINAL). It gives, **per platform per day**:
- call count
- revenue
- converted count

Requirements:
- **The day boundary must be in the platform's account timezone**, as a parameter (e.g.
  `America/New_York`), because the owner will compare against Ringba and Retreaver reports.
- Show it on the laptop data for the last 7 full days, and show that it matches the laptop
  dashboard.
- Also give the matching Postgres or ClickHouse query for "sync errors in the last 24 h".

**B5 — Laptop handover.** List every integration on the laptop with platform, name, mode
(Test/Live), `is_active` and `sync_status`, without secrets.
- Confirm that **none is Live**, or say which ones are.
- On the go-live day the laptop must pull from nobody. Write the exact laptop steps to make
  sure of that: pause in the UI, and/or stop platform-sync with `stop`, never `down`.

**B6 — Anything else that differs between laptop and server** that could make server numbers
differ from the laptop certification: timezone settings, dedup TTL, `RINGBA_RATE_PER_MIN`,
`WRITEBACK_SETTLE_SECONDS`, env defaults. Give a table of laptop value vs server default,
without secrets.

---

## Output and gates

`docs/v1-golive-prep-report.md`:
1. Scoreboard: A1–A6, B1–B6
2. The A1 source of the $370
3. The A4 `MOCK_MODE` table and the proposed server value
4. B1–B6 answers
5. The B4 queries, copy-paste ready
6. A short **server deploy note** for Part A: which images to rebuild, any new env vars, and the
   A5 rename step
7. "Found, not fixed"

Requirements:
- v1 gates pass in `-V1` mode.
- Laptop Retreaver all-time unchanged: **3,652** calls / **$33,143.85**.
- Secret scan of every commit.

Push `v1/golive-prep`, give the hashes, and **STOP**.
