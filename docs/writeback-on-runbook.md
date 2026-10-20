# Calyx — write-back ON day (Convert / Cancel live)

**Server:** `calyx.ridgerisemedia.com` · `~/deployments/calyx-ai` · compose project **`deploy`**
**Code needed on the server:** `main` @ **`99618e0`** or later (`merge: Stage 4b — Retreaver convert / void
write-back`). Retreaver write-back and migration `000032` exist only from that commit.
**People:** **Aryan** (server terminal) · **Owner** (browser, platform UIs)

Docs only. Nothing here is run until the owner says so.

## Rules for everyone

- 🔴 No API key, password, internal token or `.env` value in chat, WhatsApp or email.
- Never `docker compose down -v`, never delete volumes, always `-p deploy`.
- Only the **two** amounts in this runbook are written: **$1** convert and the cancel/void of that $1.
  Only on the test calls the owner names in Step 0.
- On anything unexpected: **stop** (Step 6, kill switch), send the output, do not retry on your own.

## How write-back is gated (three locks, all must be open)

| Lock | Where | Default | Opened in |
|---|---|---|---|
| Global kill switch `WRITEBACK_ENABLED` | server `.env`, read by **platform-sync** (`services/platform-sync/internal/config/config.go:68`) | `false` (`deploy/docker-compose.yml:455`) | Step 2 |
| Per-integration `writeback_enabled` | Postgres `integrations` column (`db/postgres/migrations/000025_writeback_enabled.up.sql`) | `false` | Step 3 |
| Admin role | the logged-in user | — | always |

There is **no UI toggle** for `writeback_enabled` today (it is shown, not editable). Step 3 uses one
targeted SQL `UPDATE`.

---

## Step 0 — Preconditions (Owner) 🙋

All of these must be true before Step 1.

1. **Go-live is verified:** Ringba and Retreaver are Live on the server, both `ok`, and the reconciliation
   numbers matched the platform reports.
2. **Test calls in the same accounts the server uses.** Calyx refuses a call that belongs to another
   account (403, audited), so:
   - **Ringba:** a **fresh** call on a sandbox campaign in the **same Ringba account** as the server's Live
     Ringba integration: internal buyer and publisher, $0 terms, **recording off**, **no pixels, webhooks
     or postbacks** on the campaign, its buyer/target or its publisher, and no prior convert or void on the
     call. Note its `inboundCallId` (`RGB…`).
   - **Retreaver:** a call in the **same Retreaver company** as the server's Live Retreaver integration
     (Ridge Rise Media). ⚠️ The Stage 4b sandbox calls (`a82a7a80…`, `d662e436…`) are in a **different**
     company; the server's integration cannot write to them. Create a sandbox campaign in Ridge Rise Media
     ($0 terms, no webhooks/pixels, **recording off**), place one call **under 90 s**, and note its UUID.
3. **Retreaver API user has recording access** (Retreaver → Settings → Users), or every recorded call
   answers 422 "Access denied to 'recording_access'".
4. **Both test calls are synced into Calyx** (they appear in Pings) and show **Not converted**.
5. No reconciliation is running during the test.

## Step 1 — Deploy `99618e0` if the server is older (Aryan) 📤

```bash
cd ~/deployments/calyx-ai
DC="docker compose -p deploy -f deploy/docker-compose.yml"
git fetch origin && git log --oneline -1 origin/main     # 99618e0 or later
git log --oneline -1                                      # what the server runs now
```

If the server is older than `99618e0`:

```bash
BK=~/calyx-backups/writeback-on && mkdir -p $BK && chmod 700 $BK
$DC exec -T postgres pg_dump -U callloom -d callloom -Fc > $BK/callloom-before.dump && ls -lh $BK
git pull --ff-only origin main
$DC build api web platform-sync
$DC up -d --remove-orphans          # the migrate step applies 000032
$DC exec -T postgres psql -U callloom -d callloom -c "SELECT version, dirty FROM schema_migrations;"
# expect: version 32 (or higher), dirty f
```

`WRITEBACK_ENABLED` is still `false` after this step. Users see no change.

## Step 2 — Open the global kill switch (Aryan) 📤

**2a. Check — changes nothing.**

```bash
cd ~/deployments/calyx-ai
DC="docker compose -p deploy -f deploy/docker-compose.yml"
ENVF=.env   # the real env file; deploy/.env is a symlink to it — never write to deploy/.env directly
[ -f .env ] && [ "$(readlink -f deploy/.env)" = "$(readlink -f .env)" ] && echo "env file: OK (deploy/.env -> .env)" || echo "env file: CHECK - stop"
# The internal token must be a real value (not empty, not the dev default). Prints only yes/no.
grep -qE '^WRITEBACK_INTERNAL_TOKEN=.{20,}' "$ENVF" && ! grep -q 'dev_writeback_internal_token' "$ENVF" && echo "token: set" || echo "token: MISSING - stop"
```

**Run 2b only if 2a printed `env file: OK (deploy/.env -> .env)` and `token: set`.** Anything else: stop and send the output.

**2b. Edit — same terminal as 2a** (it uses `$ENVF` and `$DC` from 2a).

```bash
mkdir -p ~/calyx-backups/writeback-on && chmod 700 ~/calyx-backups/writeback-on
cp "$ENVF" ~/calyx-backups/writeback-on/env-before.bak && chmod 600 ~/calyx-backups/writeback-on/env-before.bak
umask 077
grep -v '^WRITEBACK_ENABLED=' "$ENVF" > "$ENVF.new" && echo 'WRITEBACK_ENABLED=true' >> "$ENVF.new"
mv "$ENVF.new" "$ENVF" && chmod 600 "$ENVF"
grep '^WRITEBACK_ENABLED=' "$ENVF"                     # WRITEBACK_ENABLED=true

$DC up -d --no-deps --force-recreate api platform-sync  # only these two, never -v
$DC exec -T api printenv WRITEBACK_ENABLED             # true
$DC exec -T platform-sync printenv WRITEBACK_ENABLED   # true
$DC logs platform-sync --tail 30 | grep -i -E 'writeback|error'
```

**Stop** if either `printenv` is not `true`, or the log shows
`WRITEBACK_ENABLED is on but WRITEBACK_INTERNAL_TOKEN is empty`.

Every integration still has `writeback_enabled = false`, so nothing can be written yet.

## Step 3 — Turn on write-back for the two Live integrations (Aryan) 📤

First look, change nothing:

```bash
$DC exec -T postgres psql -U callloom -d callloom -c "
SELECT id, platform, name, connection_mode, sync_mode, is_active, writeback_enabled
FROM integrations
WHERE deleted_at IS NULL AND connection_mode = 'api_sync'
ORDER BY platform;"
```

Expect exactly **one** Ringba and **one** Retreaver row with `sync_mode = real` and `is_active = t`.
If there are more, or fewer, **stop** and send the output.

Then enable **exactly those two rows by id**. Copy the two `id` values from the SELECT output above into
the command (replace both placeholders; keep the quotes):

```bash
RINGBA_ID='<ringba id from the SELECT>'
RETREAVER_ID='<retreaver id from the SELECT>'
$DC exec -T postgres psql -U callloom -d callloom -c "
UPDATE integrations
   SET writeback_enabled = TRUE, updated_at = NOW()
 WHERE id IN ('$RINGBA_ID', '$RETREAVER_ID')
RETURNING id, platform, name, sync_mode, is_active, writeback_enabled;"
```

You must see **`UPDATE 2`**, one `ringba` and one `retreaver` row, both `sync_mode = real`,
`is_active = t`, `writeback_enabled = t`. Anything else: Step 6, then send the output.

## Step 4 — Owner's $1 check in the server UI 🙋

Log in as an **admin**. Open **Pings**, search the call id, open the call detail page. The **Conversion**
card shows the Convert / Cancel controls only for an admin, on a Live integration with write-back on.

### 4a. Ringba call (`RGB…` from Step 0)

| # | Do | Expect on screen | Aryan checks (Step 5) |
|---|---|---|---|
| 1 | Before: read the card | **Not converted**, $0.00 | — |
| 2 | **Convert**, amount **1.00**, confirm in the popup (it shows Ringba, the call id and $1.00) | Toast **"Converted in Ringba"** · "Confirmed by Ringba — now $1.00". Card: **Converted**, $1.00 | newest row `convert`, `confirmed`, amount 1.00 |
| 3 | **Cancel**, confirm | Toast **"Cancelled in Ringba"** · "Confirmed by Ringba — now $0.00". Card: **Not converted** | newest row `void`, `confirmed` |

### 4b. Retreaver call (UUID from Step 0)

| # | Do | Expect on screen | Aryan checks (Step 5) |
|---|---|---|---|
| 1 | Before: read the card | **Not converted**, $0.00 | — |
| 2 | **Convert**, amount **1.00**. The popup shows "RETREAVER · CONVERT", the UUID, "Convert this call for $1.00?" and the red line *"Retreaver may fire this call's conversion pixels/postbacks to the buyer or publisher."* Confirm | Toast **"Converted in Retreaver"** · "Confirmed by Retreaver — now $1.00". Card: **Converted**, $1.00 | newest row `convert`, `confirmed`, `platform = retreaver`, `read_back.after.revenue = 1`, `fired_pixels_delta = 0` |
| 3 | **Void conversion (sets revenue to $0)**, confirm | Toast **"Voided in Retreaver"** · "Confirmed by Retreaver — now $0.00". Card: **Not converted** | newest row `void`, `confirmed`, `read_back.after.revenue` null or 0 |

What the other results mean:
- **"Sent. … had not confirmed yet (pending)"** (HTTP 202): the platform accepted the write but the
  read-back did not match within 20 s. The call is locked for 15 minutes. Wait, refresh the page, and let
  Aryan check the audit row. **Do not click again.**
- **"the Retreaver API user needs recording access…"**: Step 0 item 3 is not done. Nothing was written.
- **"Write-back is turned off for this account."**: Step 3 did not apply to that integration.
- **403 / account mismatch**: the call is not in the same account as the server's integration (Step 0 item 2).
- Anything else, or a pixel/postback fired: **Step 6 at once**.

End state for both calls: **Not converted, $0**. Each platform keeps its own edit history.

## Step 5 — Audit check after each click (Aryan, read-only) 📤

```bash
$DC exec -T postgres psql -U callloom -d callloom -c "
SELECT a.requested_at, COALESCE(a.platform, 'ringba') AS platform, a.action, a.amount, a.outcome,
       left(a.inbound_call_id, 12) AS call, left(COALESCE(a.message, ''), 80) AS message,
       a.read_back->'after'->>'revenue'      AS rb_revenue,
       a.read_back->>'fired_pixels_delta'    AS pixels_delta,
       u.email AS by_user
FROM writeback_audit a
LEFT JOIN users u ON u.id = a.user_id
ORDER BY a.requested_at DESC
LIMIT 10;"
```

The table is append-only (no update or delete is possible). It holds no keys or URLs. Ringba rows have
`platform` and `read_back` empty; that is expected.

## Step 6 — Kill switch: turn it all off again (Aryan)

Fastest first. Step 6a alone stops every write within a minute.

**6a. Global switch off**

Check first — changes nothing:

```bash
cd ~/deployments/calyx-ai
DC="docker compose -p deploy -f deploy/docker-compose.yml"
ENVF=.env   # the real env file; deploy/.env is a symlink to it — never write to deploy/.env directly
[ -f .env ] && [ "$(readlink -f deploy/.env)" = "$(readlink -f .env)" ] && echo "env file: OK (deploy/.env -> .env)" || echo "env file: CHECK - stop"
```

**Only after it prints `env file: OK (deploy/.env -> .env)`**, in the same terminal:

```bash
umask 077
grep -v '^WRITEBACK_ENABLED=' "$ENVF" > "$ENVF.new" && echo 'WRITEBACK_ENABLED=false' >> "$ENVF.new"
mv "$ENVF.new" "$ENVF" && chmod 600 "$ENVF"
$DC up -d --no-deps --force-recreate api platform-sync
$DC exec -T platform-sync printenv WRITEBACK_ENABLED   # false
```

If it printed `CHECK - stop`, do not edit by hand in a hurry: stop and send the output (write-back stays
as it is until the file question is answered; Step 6b can still close each integration).

With it off, every Convert / Cancel is refused with 403 "write-back is disabled" and an audit row
`rejected`; no request reaches Ringba or Retreaver.

**6b. Per-integration off (optional, also after a normal test day if write-back should stay closed)**

```bash
$DC exec -T postgres psql -U callloom -d callloom -c "
UPDATE integrations SET writeback_enabled = FALSE, updated_at = NOW()
 WHERE platform IN ('ringba', 'retreaver') AND writeback_enabled
RETURNING id, platform, name, writeback_enabled;"
```

**6c. If a write landed and must be undone**: the owner opens the call in Ringba or Retreaver and sets it
back by hand. Calyx's next sync and hourly sweep pick up the change.

## Done when

- [ ] Step 0 preconditions met, both test calls visible in Calyx as Not converted
- [ ] Server on `99618e0`+, migration 32, `WRITEBACK_ENABLED=true` in api and platform-sync
- [ ] `UPDATE 2` on the two Live integrations
- [ ] Ringba: Convert $1 → confirmed → Cancel → confirmed; two `confirmed` audit rows
- [ ] Retreaver: Convert $1 → confirmed → Void → confirmed; two `confirmed` audit rows, `fired_pixels_delta = 0`
- [ ] Both calls end Not converted, $0
- [ ] The owner decides: leave write-back on, or run Step 6
