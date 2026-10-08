# Calyx — v1 cleanup + production hardening (before going live on the server)

**Branch:** `v1/hardening`, created from `main` @ `7dc3a66` · **Operator:** Talha · **Owner:** Syed
**Output:** `docs/v1-hardening-report.md`, plus a server deploy note (Part D)
**Order:** do this **before R3**. The R3 branch `stage4/r3` stays untouched.

## Why

The server works, but it still runs as a dev setup:
- `APP_ENV=development`
- every secret is a dev default that is written in the repo
- Postgres, Redis and ClickHouse were reachable from the internet for at least 7 days with
  those passwords (fixed in `7dc3a66`)

Before the owner enters the real Ringba and Retreaver API keys on the server, those keys must
be encrypted with a key that is **not** in the repo. All database passwords must be new.

## ⛔ Rules

1. **Owner decision: the app login stays exactly as it is** (`admin@calyx.ai`, same password).
   - Do not change, reset, re-seed or re-hash any user.
   - If production mode would block or change this login, **stop 🙋**. Do not work around it.
2. Never print, log or commit a secret value. Generated values go only into a local `.env` that
   is git-ignored. In the report, mask every value as `***`.
3. **Your laptop stack is not touched.** It holds the stored Retreaver credentials, which are
   encrypted with the current dev key. Rotating that key on the laptop would make them
   unreadable.
   - Do the full rehearsal (Part C) in a **separate compose project**, `-p calyx-rehearsal`,
     with its own volumes.
   - Stop the laptop stack first with `stop`, never `down`. Start it again at the end.
4. Never `down -v` on any project except `calyx-rehearsal`. Even there, only after the owner
   says so.
5. No Live sync. No Ringba or Retreaver writes. Gates run with `pwsh-preview` in **`-V1`**
   mode.
6. Verify before changing, one commit per item (prefix `v1-hardening:`), and prove every item.
   Push the branch only, **never `main`**. Stop at every 🙋.

---

## Part A — v1 cleanup (from the owner's server review)

**A1 — "All" means Ringba + Retreaver only.** With the v1 flag on, every view and export
counts only `ringba` and `retreaver`:
- Pings list and its count
- Dashboard KPIs, ping chart and "Platform mix"
- Coverage
- CSV export

Today, Call Loom, CallGrid and TrackDrive demo rows still show under "All" and in Platform mix.
Filter them out; do not delete any rows. Proof: with "Show demo" on, the Platform mix shows
only Ringba and Retreaver, and All = Ringba + Retreaver for calls and revenue.

**A2 — The chart covers the whole period, up to today.**
- 7 days and 30 days: one point per day, from (today − N + 1) to **today**. Days with no
  calls show 0.
- All time: weekly or monthly points, up to the current week or month.
- The chart total still equals the KPI ping count.

Today the server's "30 days" chart only runs 09-22 → 09-29, the days that have data.

**A3 — Security headers: no console warnings.** The browser logs "Error with Feature-Policy
header" and "Unrecognized feature: 'ambient-light-sensor' / 'battery' / 'document-domain'".
- Find where these headers are set: Next config, Nest/helmet, or
  `deploy/nginx/calyx.ridgerisemedia.com.conf`.
- Remove the deprecated `Feature-Policy` header. Keep only recognised features in
  `Permissions-Policy`.
- Do not weaken the CSP.
- Proof: zero Permissions-Policy or Feature-Policy messages in the console on Login,
  Dashboard, Pings and Settings.
- If the header comes from the **live server nginx**, write the exact line change into the
  Part D deploy note.

**A4 — No intelligence calls in v1.** The server dashboard still requests
`/scores?limit=5&offset=0`. With the v1 flag on, the web must make **no** request to scoring,
ask-ai, compliance, enrichment or ad-attribution endpoints. Proof: a Network tab listing for
Dashboard, Pings and a call detail.

**A5 — Copy.**
- Dashboard subtitle: "…financial metrics, and intelligence at a glance" → drop "and
  intelligence".
- Sidebar section label "INTELLIGENCE" (above Reports and Coverage) → **"ANALYTICS"**.
- The logo tagline stays.

## Part B — Production hardening (repo changes)

**B1 — Inventory first.** List every secret and credential in `deploy/docker-compose.yml` and
the service configs on current `main`. For each one, give:
- which service uses it
- whether it is a literal or `${VAR:-default}`
- what breaks if it changes

The list must include at least:
- Postgres superuser `callloom` (used by `migrate`)
- roles `callloom_api`, `callloom_ingestion`, `callloom_sync` (the passwords inside each
  `DATABASE_URL`)
- `CLICKHOUSE_PASSWORD`, wherever it appears, including the `clickhouse-init` command
- `JWT_SECRET`
- `CREDENTIAL_ENCRYPTION_KEY`
- `ANI_ENCRYPTION_KEY`
- `WRITEBACK_INTERNAL_TOKEN`
- `DEEPGRAM_CALLBACK_SECRET`
- Redis, which has no password today

**B2 — Every secret comes from env.**
- No literal password stays in the compose file. Each becomes `${VAR:-<dev default>}`, so local
  dev keeps working, including the passwords inside every `DATABASE_URL` and
  `CLICKHOUSE_URL`.
- Add a `deploy/.env.example` that lists every variable with an empty value and a one-line
  comment.
- Confirm `.env` is in `.gitignore`.

**B3 — Redis password.** Add `requirepass ${REDIS_PASSWORD}` and put the password into every
`REDIS_URL`. If a service cannot take a password in its URL, report it 🙋 and do not hack
around it.

**B4 — `APP_ENV=production` works for the whole v1 stack:** api, web, platform-sync, ingestion
and consumer.
- The existing startup check must refuse **every** dev placeholder from B1, not just some.
  Prove it for each variable: one start with a placeholder must fail, and the error must name
  the variable.
- Production settings come from env:
  - `WEB_ORIGINS=https://calyx.ridgerisemedia.com`
  - `COOKIE_DOMAIN` left empty
  - `TRUSTED_PROXIES` = the Docker gateway IP that nginx traffic arrives from. Document the
    command Aryan uses to find it:
    `docker network inspect <network> -f '{{(index .IPAM.Config 0).Gateway}}'`.
- Cookies must be `Secure` in production. The site is HTTPS.
- Rule 1 applies: the admin login must keep working.

**B5 — Encryption-key rotation impact.** Find out exactly what is stored encrypted with
`CREDENTIAL_ENCRYPTION_KEY` and with `ANI_ENCRYPTION_KEY` (tables and columns), and what
happens to old rows after the key changes. Old rows must **not crash** anything. A row that
cannot be decrypted shows as unavailable, or the integration shows "re-enter credentials".

On the server, the existing data is demo data plus 4 dev webhook rows, which the owner is
deleting. If anything there would need re-encryption, say exactly what 🙋.

**B6 — v1 services only.** Make the server able to run **without** scoring, ask-ai,
compliance-intelligence, enrichment, ad-attribution and providerconfig, the same set the
`-V1` gates already skip. Use one env switch, e.g. `INTELLIGENCE_SCALE=0` or a compose
profile. The full set must still start for local dev. Prove that api and web work with them
stopped: login, dashboard, pings, call detail, settings.

## Part C — Full rehearsal (`-p calyx-rehearsal`)

Simulate the server exactly:
1. Start `main` @ `7dc3a66` (the server's version) in `calyx-rehearsal` with **dev** secrets,
   up to migration 31.
   - Log in and confirm the demo data shows.
   - Create one integration with dummy credentials. **Never real Ringba or Retreaver keys.**
2. Switch to `v1/hardening` and follow **your own Part D deploy note, word for word**:
   - new secrets
   - Postgres `ALTER ROLE … PASSWORD` for every role, because `POSTGRES_PASSWORD` only applies
     to a fresh volume
   - ClickHouse password change
   - Redis password
   - `APP_ENV=production`
   - intelligence services off
   - rebuild and `up -d`
3. Prove:
   - Postgres has no volume wipe: demo calls and the integration rows are still there.
   - Login works with the **unchanged** admin login.
   - Every old dev password now fails: one `psql` attempt per role, plus Redis and ClickHouse.
   - The dummy integration behaves as B5 says.
   - v1 gates pass in `-V1` mode.
   - The console is clean.
   - `docker compose ps` shows only v1 services, every port on `127.0.0.1`.
4. Stop `calyx-rehearsal`. Do not delete its volumes. Restart your normal laptop stack and show
   it is unchanged: same counts as before, and the stored Retreaver integration still reads.

## Part D — Server deploy note (in the report)

Exact terminal steps for Aryan. Use `DC="docker compose -p deploy -f deploy/docker-compose.yml"`
and real commands, no placeholders except where Aryan types a generated secret:
1. Before:
   - `git log`, `git status`
   - `pg_dump` backup
   - a backup copy of the current `.env`
2. Generate every secret on the server with `openssl rand -hex 32` and write it into `.env`
   (`chmod 600`). Nothing is pasted into chat.
   - Tell Aryan to give the owner a copy of the `.env`. If it is lost, the stored credentials
     cannot be decrypted.
3. `git pull --ff-only` → `ALTER ROLE` for every Postgres role → ClickHouse and Redis password
   steps → build api, web, platform-sync (and any other changed image) → `up -d`.
4. Verify:
   - `printenv APP_ENV` = production
   - `WRITEBACK_ENABLED` = false
   - migration 31
   - `ps`: only v1 services, ports on 127.0.0.1
   - an old dev password fails
   - login works
5. Rollback steps.

## Output and gates

`docs/v1-hardening-report.md`:
1. Scoreboard: A1–A5, B1–B6, C, each DONE / NOT DONE with proof
2. The B1 secret inventory, values masked
3. The B5 findings
4. The rehearsal log
5. The Part D deploy note
6. "Found, not fixed"

Requirements:
- v1 gates pass: system, ingestion and presentation, in `-V1` mode
- Laptop Retreaver all-time unchanged: **3,652** calls / **$33,143.85**
- Secret scan of every commit

Push `v1/hardening`, give the hashes, and **STOP**. The owner reviews before any merge.
