# Calyx v2 — Phase 2B.2: Transcription Rules and per-tenant usage limits (spec only, no code)

**Status:** spec, owner answers recorded 2026-10-23 (end of document). Nothing here is built.
**Builds on:** Phase 2B as deployed (`docs/v2-intelligence-report.md` §2B, `docs/deploy-v2-2d-2b.md`).
**Migration:** **`000038_transcription_rules`**, the next after `000037_function_grants`.
**Labels:** [PROVEN] = code or a run in this repo · [INFERRED] = follows from that evidence · [OWNER] = the owner's
number, not re-checked here · [UNKNOWN] = not measured yet.

**Owner decisions (2026-10-23) this spec follows:**
- **No global automatic transcription.** `TRANSCRIBE_AUTO` (all converted calls) is not switched on. Rules replace it.
- **Backfill is never automatic.** It is manual only, per selected campaign and date range, with the cost estimate
  first. `TRANSCRIBE_BACKFILL` stays the platform switch for it.
- **Every rule is forward-only.** It applies only to calls after the rule (or a scope added to it) is created, never
  retroactively.
- **Limits move from one env cap to per-tenant limits in the database.** The env settings stay as platform-wide safety
  limits.

---

## 0. What exists today (baseline) [PROVEN]

**Queue.** `transcription_jobs` holds one row per call, `UNIQUE (tenant_id, platform, external_call_id)`:
- `reason` is one of `converted | should_convert | on_demand | backfill | archive_only`;
- `state` is one of `queued | running | done | no_recording | failed | capped`.

**Enqueue.** The API job (`services/api/src/transcription/transcription-enqueuer.service.ts`) runs every 15 min, one
instance at a time.
- It is idle unless `FEATURE_TRANSCRIPTION` **and** `TRANSCRIBE_AUTO` are on.
- Its window starts at the later of `now − 48 h` and `TRANSCRIBE_AUTO_SINCE` (`autoWindowStart`, `:15`).
- Candidates are the dashboard's per-call latest view (`transcription-candidates.service.ts`).

**Worker.** platform-sync (`services/platform-sync/internal/transcribe/worker.go:77`) runs **one sequential loop**:
claim → fetch once → archive → Deepgram → scrub → transcript.
- **Cap check:** before any fetch, against **one** env cap per tenant per Eastern day: `TRANSCRIBE_DAILY_MINUTES_CAP`,
  default 800.

**Usage.** `transcription_usage_daily (tenant_id, usage_date, minutes, calls)`, minutes only, no $ and no per-rule split.

**Data.**
- **A call's versions:** ClickHouse `pings` keeps **every synced version** of a call, one row per version. Content-hash
  dedup means only changed content adds a row. Each row carries `received_at`, `revenue`, `payout`, `has_converted`,
  `platform_duplicate`, `campaign_id`, `campaign_name`, `publisher_id`, `publisher_name`, `duration_seconds`,
  `has_recording` and `recording_known`.
- **No verticals:** there is no vertical column. `vertical` exists only as free text in scorecards
  (`services/api/src/scorecards/scorecards.schemas.ts:11`).

**Alerts (2D).** `alert_events` stores `rule_id`, a free-text `trigger_type`, a `scope`, `platform`, `entity_id` (the
publisher id), `window_from`/`window_to` and `created_at` (`000035_workflows.up.sql:47`). The bell and `/alerts` read
it.

**The void shape** (`docs/retreaver-stage4-report.md:2006`): seven Retreaver voids, each version 1
`converted=true, revenue>0` → version 2 `converted=false, revenue=0`. "A stored `converted=false, revenue=0` cannot
tell a void from a call that never converted." So a void is detectable **only by comparing versions**. The owner saw
the same on Retreaver on 15 and 16 Oct [OWNER].

---

## Part A — Rules

### A.1 Rule model (common to every rule)

| Field | Meaning |
|---|---|
| `type` | `scope` · `sampling` · `void_return` · `should_convert` · `new_publisher` · `alert_sample` (the button is not a rule; see A.2 (7)) |
| `name`, `is_active` | Admin label; on/off. Off keeps the rule and its history |
| `platforms` | `ringba`, `retreaver` or both |
| `scope` | Campaigns (`campaign_id` per platform) and/or verticals (via `campaign_verticals`, A.4). Empty = all campaigns of the tenant (allowed only for the trigger rules (3), (5), (6)) |
| `params` | Per type (A.2) |
| `min_duration_seconds` | Calls shorter than this are never queued. Default 20; never below the platform floor `TRANSCRIBE_MIN_SECONDS` |
| `daily_minutes_cap` | The rule's own cap per Eastern day, **under** the tenant cap (Part B). Required |
| `active_since` | Set to `NOW()` when the rule is created and again when it is re-activated. **Forward-only:** a call qualifies only if its qualifying event (A.3) happened at or after `active_since`. A campaign added to the scope later gets its own `added_at` in `transcription_rule_scope` and qualifies only from then on |
| `priority` | Fixed by type (B.3), not editable |
| `created_by`, `updated_by`, timestamps | Every change is audited (A.6) |

**One job per call stays.** A call that matches several rules is queued once. The job records **all** matching rules
(`transcription_job_rules`). Minutes are counted against the **highest-priority** matching rule's cap, and the other
matches are kept for reporting. A call already `done`, for example by the button, is never queued again. Re-transcription
is a separate, explicit admin action (out of scope here).

### A.2 The seven rule types

1. **Campaign / vertical scope** (`scope`). Every converted call (flag **and** revenue > 0, the dashboard's
   definition) in the selected campaigns or verticals. Optional `converted_only=false` widens it to every recorded
   call ≥ `min_duration_seconds` in scope; it is off by default and the UI shows its cost.

2. **Sampling** (`sampling`). A percentage of converted calls in scope, with a per-publisher minimum.
   - **The sample is stable:** a call is in it when `cityHash64(platform, external_call_id) % 10000 < pct × 100`. That is
     the same answer every time, so re-scans never flip a call in or out, and it is reproducible in an audit.
   - **Per-publisher minimum:** `min_calls_per_publisher_week` (default **5 per week**, owner) keeps each publisher's
     quality measurable.
     If a publisher has fewer than N transcribed calls (any reason) in the current ISO week (Eastern), its next converted
     calls in scope are added until it reaches N, even outside the percentage.
   - **Params:** `pct` (1–100, default 20), `min_calls_per_publisher_week`.

3. **Void / return trigger** (`void_return`). **Dispute evidence.** A call qualifies when one of its versions had
   `has_converted = 1 AND revenue > 0` and a **later** version has a lower revenue (to 0 or partly). The qualifying event
   is the `received_at` of the version where the revenue went down. Detection is in Part C.1.
   - **Params (owner defaults):** `min_drop_usd` **$1.00**; `include_partial` **true**, so a partial return such as
     57 → 21 counts as well as a full void such as 13.77 → 0.
   - **Scope:** default all campaigns.
   - **Evidence:** the job stores `trigger_detail = {"from": 13.77, "to": 0, "version_received_at": …}`, and the
     evidence pack includes it.

4. **Should-have-converted** (`should_convert`). Calls that are not converted but earned revenue or reached the buyer's
   required duration (the Pings filter, `services/api/src/pings/pings.filters.ts`).
   - **Disabled until sized.** The rule cannot be activated until the owner has run Aryan's 30-day query (report
     §2A.6 "Should-have-converted per day") on the server.
   - **The UI** shows that query's result per day before the owner can switch the rule on. The platform
     `TRANSCRIBE_SHOULD_CONVERT` stays `false` until then. It becomes the platform permission for this rule type (Part
     C.6).

5. **First N calls of a new publisher** (`new_publisher`). A publisher is new when its first call ever for the tenant
   and platform (`min(event_at)` over all versions) is at or after `active_since`. Its first `n` recorded calls
   ≥ `min_duration_seconds` are queued, converted or not.
   - **Params (owner defaults):** `n` **20**, `converted_only` **false** (any call, not only converted), with the normal
     `min_duration_seconds` **20 s**. The first calls are where quality problems show.
   - **Scope:** default all campaigns.

6. **2D alert trigger** (`alert_sample`). When an `alert_events` row is **created** with `scope = 'publisher'` and a
   trigger type in the rule's list, up to `k` recent calls of that publisher are queued.
   - **Trigger types (owner): only these.**
     - `conversion_rate_below`, `short_call_share_above` and `repeat_caller_share_above`;
     - after 2C, the QA flag-rate trigger(s) (`qa_flag_rate_above`).
     - **Never** `volume_change`, `revenue_drop`, `no_calls` or `integration_unhealthy`: the API refuses them in
       `trigger_types`.
   - **Which calls:** the **10 most recent** calls (`k` = 10) inside the alert's `window_from`..`window_to`, recorded
     and ≥ `min_duration_seconds`.
   - **Conversion drops:** for `conversion_rate_below`, **non-converted calls first**. They fill `k` before any
     converted call is taken, because they show why the conversion fell.
   - **Params (owner defaults):** `k` **10**, `trigger_types` as above, `prefer_non_converted` **true** for
     `conversion_rate_below`.
   - **Limits:** once per alert (keyed on `alert_events.id`); alerts created before `active_since` never trigger.
   - **Order:** the alert's own calls usually already happened, so they qualify by the **alert's** `created_at`, not by
     the call time. This is the one sanctioned look-back, bounded by the alert window (≤ 720 h by the 2D schema) and by
     `k`.

7. **The Transcribe button** (as now). An admin, one call, any age. It is not a rule and has no `active_since`. It has
   the highest priority (B.3) and uses the tenant **reserve** (B.2). It stays audited as today.

**Manual backfill** is the only way to transcribe older calls in bulk:
- per selected campaigns and a date range (max 30 days), with the estimate first (calls, minutes, $, days at the cap);
- an admin, then a confirm;
- gated platform-wide by `TRANSCRIBE_BACKFILL`, and counted against the tenant quota (not the rule caps);
- **never automatic** and never started by a rule.

### A.3 Qualifying event per rule (what "after the rule" means)

| Rule | The call qualifies when this happens at or after `active_since` (or the scope's `added_at`) |
|---|---|
| scope, sampling | The version in which the call **became converted** is received (`received_at` of the first version with `has_converted=1 AND revenue>0`). A call that became converted before the rule never qualifies, even if a later version arrives after |
| void_return | The version in which the revenue **went down** is received |
| should_convert | The first version matching the should-convert test is received |
| new_publisher | The publisher's first call ever (`min(event_at)`) is at or after `active_since`; then its first N calls |
| alert_sample | The alert is created |

This is the per-rule form of the `TRANSCRIBE_AUTO_SINCE` logic (`autoWindowStart`). The enqueuer's scan window stays
"since the last successful scan, at most `TRANSCRIBE_LOOKBACK_HOURS`", so a short API outage loses nothing. The rule's
`active_since` is the floor, so a new rule never reaches back.

### A.4 Campaigns, verticals and the "new campaign detected" notice

- **`campaign_verticals`** (tenant, platform, `campaign_id`, `vertical`, `updated_by`): the admin maps each campaign to a
  vertical in Settings. The vertical names are free text, shared with scorecards (for example `aca`, `medicare`,
  `final_expense`, `debt`). A rule scoped to a vertical covers every campaign mapped to it, including a campaign mapped
  later (from its mapping time).
- **New campaign detected.** Once a day, the API lists the `campaign_id`s that had calls in the last 24 h and are in no
  active rule's scope, in no "all campaigns" rule, and not dismissed.
  - Each one becomes a **notice in the bell**: an `alert_events` row with `trigger_type = 'transcription_new_campaign'`,
    `rule_id` NULL, `scope = 'campaign'`, `entity_id` = the campaign id. It reads "New campaign *X* (Ringba) has calls and
    no transcription rule. Set a rule?" and links to Settings → Transcription → Rules with that campaign pre-selected.
  - **Dismiss** stores it in `transcription_campaign_dismissals` so it is not shown again.
  - The notice never starts any transcription by itself.

### A.5 Cost estimate before saving

Saving or activating a rule first shows `POST /transcription/rules/estimate` (the same body as the rule):
- the calls and minutes per day it **would** have matched over the last 30 full days (Eastern);
- $ per month at the tenant's price (C.4);
- the projected days per month it would hit its own cap and the tenant cap.

It uses the per-call ClickHouse view, so it reads history **only to estimate**; nothing is queued. The admin must tick
"I have seen the estimate" to save. Estimate for `void_return` and `alert_sample` from the last 30 days of voids and
alerts.

### A.6 Audit

`transcription_rule_audit`, append-only (the immutable trigger like `recording_access_audit`). It records who, when,
`action` (`create | update | activate | deactivate | delete | scope_add | scope_remove`), and the before/after JSON of
the rule (no secrets in it). Limit changes (Part B) are audited the same way in `transcription_limit_audit`.

---

## Part B — Usage limits for many companies (multi-tenant)

### B.1 Per-tenant limits in the database (replacing the single env cap)

**`transcription_plans`** (seeded; platform-owned, not tenant-editable):

**Owner proposal (2026-10-23).** Final prices are a management decision. Transcription **and AI QA (2C)** are included in
the price.

| Plan | Included minutes / month | Price / month | Overage | Default daily cap (monthly / 30 × 2) | `over_quota_mode` allowed |
|---|---:|---:|---:|---:|---|
| Starter | 3,000 | $69 | $0.03/min | 200 | `stop`, `overage` |
| Growth | 15,000 | $299 | $0.03/min | 1,000 | `stop`, `overage` |
| Scale | 50,000 | $899 | $0.025/min | 3,333 | `stop`, `overage` |
| Enterprise | 100,000+ by contract | by contract (~$0.015/min) | by contract | 6,667 at 100,000 (by contract) | `stop`, `overage`, `notify_only` |
| **Internal** | 25,000 | — (no billing) | — | 1,667 | `stop`, `overage`, `notify_only` |

**Internal** is our own tenant, **instar.io**:
- it is never billed, and the internal usage report shows its cost with a billed amount of $0;
- its quota is 25,000 min/month, which is above both models at today's volume (rules ~6,100, global ~16,500; C.5).

**Our cost if a plan's minutes are fully used [INFERRED]:**
- **What goes into it:** transcription at $0.0063/min, plus AI QA at ~$0.0005/min. That is §6's ~$10 OpenAI QA for
  ~3,600 calls a month, at ~6 min per call; it is re-checked when 2C is built.

| Plan | Our cost (fully used) | Price | Gross margin |
|---|---:|---:|---:|
| Starter | ~$20 | $69 | ~70 % |
| Growth | ~$102 | $299 | ~66 % |
| Scale | ~$340 | $899 | ~62 % |
| Enterprise at 100,000 min | ~$680 | ~$1,500 | ~55 % |

- **Overage:** $0.03 or $0.025 against ~$0.0068 cost, about 4×.

**`tenant_transcription_limits`** (one row per tenant):
- `plan`;
- `monthly_included_minutes` (from the plan, overridable by the platform);
- `daily_cap_minutes` (default `monthly / 30 × 2`, overridable);
- `monthly_budget_usd` (optional, set by the **tenant admin**: a ceiling below the plan);
- `reserve_pct` (default 10);
- `over_quota_mode` (`stop | overage | notify_only`; `notify_only` only on the **Enterprise** and **Internal**
  plans, owner) and `overage_limit_usd`;
- `deepgram_key_id` (optional, B.5);
- `updated_by`, `updated_at`.

**Effective limits for a job**, all per Eastern day or month:
- **tenant daily:** `min(daily_cap_minutes, what is left of the monthly quota (or budget/price), platform safety limit)`;
- **rules share:** `(100 − reserve_pct)%` of the tenant daily cap is available to rules (1)–(6) together, and each rule is
  limited further by its own `daily_minutes_cap`;
- **reserve:** the button and `void_return` may use the full 100%. The reserve keeps room for an admin's call and for
  dispute evidence when sampling has used its share.

### B.2 Over quota (per tenant) — jobs wait, never get dropped

| `over_quota_mode` | At the monthly quota | At the daily cap |
|---|---|---|
| `stop` | New jobs wait as `capped` until the 1st (Eastern) | Wait as `capped` until the next Eastern midnight (as today) |
| `overage` | Continue, billed as overage, until `overage_limit_usd` this month; then as `stop` | As `stop` (the daily cap still protects a runaway day) |
| `notify_only` (**Enterprise and Internal plans only**; owner) | Continue; the bell and email say so; billed as overage with no ceiling (Internal: not billed). The platform safety limit (B.5) still applies | As `stop` |

- **For other plans the API refuses `notify_only`:** Starter, Growth and Scale get `stop` or `overage` up to
  `overage_limit_usd`.
- `capped` jobs keep their place, their priority and their reason. Nothing is deleted.
- When room returns (midnight, the 1st, a limit raised), they run in priority order (B.3).
- A tenant `monthly_budget_usd` acts like `stop` at that $ amount, whatever the mode.

### B.3 Priority near the cap

`transcription_jobs.priority`, a small integer, is fixed by the job's reason:

| Priority | Source | May use the reserve |
|---|---|---|
| 1 | Transcribe button (`on_demand`) | yes |
| 2 | `void_return` | yes |
| 3 | Rules: `scope`, `new_publisher`, `alert_sample`, `should_convert` | no |
| 4 | `sampling` (including the per-publisher top-up) | no |
| 5 | Manual backfill | no (and at most 50% of the tenant daily cap, so a backfill never starves the day) |

- **Claim order:** priority, then `next_try_at`, then `created_at`.
- **Before each job the worker checks its own priority's limit.** A priority-4 job that would cross the rules' share is
  `capped`, while a priority-1 job behind it can still use the reserve.

### B.4 Usage alerts and the usage page

- **At 50 %, 80 % and 100 %** of the monthly quota (and of the tenant budget, if set), and when the daily cap is hit:
  - one `alert_events` row (`trigger_type = 'transcription_usage'`, scope `tenant`, numbers = used / quota / $) appears in
    the bell, once per threshold per month;
  - an email goes to the tenant admins through the 2D SMTP setup (off while `SMTP_HOST` is empty).
- **Settings → Transcription → Usage** (tenant admin):
  - this month's minutes and $ against the quota, budget and overage;
  - today against the daily cap;
  - per rule: minutes, calls and $, today and this month;
  - per reason;
  - the queue (queued / capped / done / failed);
  - a 30-day chart;
  - CSV export.

### B.5 Platform protection (the operator's side)

- **Platform safety limit (env, kept):**
  - `TRANSCRIBE_DAILY_MINUTES_CAP` becomes the **platform-wide** daily minutes across all tenants;
  - new `TRANSCRIBE_PLATFORM_DAILY_USD_LIMIT` caps our total Deepgram spend per Eastern day;
  - when either is hit, every non-button job waits as `capped`; the button needs a platform operator override.
  - These are not tenant settings.
- **Fair queue across tenants.**
  - **Claiming:** `claim_transcription_job()` becomes round-robin. It picks the tenant with due jobs whose last claim
    is oldest, then that tenant's best job by B.3.
  - **Concurrency:** at most **10 running jobs per tenant** and **40 in total**. That stays under Deepgram's **50
    concurrent pre-recorded requests per project** [OWNER] and leaves headroom for the button.
  - **Workers:** the worker becomes a pool (`TRANSCRIBE_WORKERS`, default 8) instead of today's single loop. The
    per-tenant and total limits are enforced in the claim function (a `running` count under `FOR UPDATE SKIP LOCKED`).
- **Dedicated Deepgram key per enterprise tenant (optional).**
  - `tenant_provider_keys`: a Deepgram key per tenant, encrypted with `CREDENTIAL_ENCRYPTION_KEY` like the API keys,
    write-only, set by a platform operator.
  - A tenant with its own key uses its own Deepgram project, so its own rate limits apply (Deepgram limits are per
    project [OWNER]), and bills directly.
  - Its concurrency limit can then be raised.
- **Internal admin usage report** (platform operator, not tenants):
  - per tenant per month: minutes by language, **our cost**, the **billed amount** (included vs overage at the plan
    price) and the margin;
  - our cost: English **$0.0063/min** (Nova-3 $0.0043 + redaction $0.0020) and Spanish **~$0.0072/min** (multi
    $0.0052 + redaction).
  - **Access (owner):** for now, a SQL view plus `GET /internal/transcription/usage-report?month=` (JSON) and `.csv`,
    behind an internal token (there is no platform-operator UI role yet). **An operator page comes when the second
    paying company is onboarded.**

### B.6 Market reference for pricing [OWNER — not re-checked here]

| Product | What it charges |
|---|---|
| CallRail | 7,500–10,000 minutes included, $0.02–0.05/min overage |
| Ringba native transcription | ~$0.035–0.04/min |
| Retreaver AI Analyst | $0.05/min |
| Kaliper QC | $499 for 350 hours (~$0.024/min) |

**What this suggests [INFERRED]:**
- **Overage:** **$0.02–0.03/min**. That is 3–5× our $0.0063 cost and at or under the platform-native options.
- **Included minutes:** sized so a typical tenant on rules (not global) stays inside its plan (C.5).
- The owner decides the prices.

---

## Part C — Data, UI, API, tests and cost

### C.1 Detecting a void / return from synced versions

**From ClickHouse, per call** (tenant, platform, `external_call_id`), excluding `platform_duplicate` rows and the test
prefixes, as the dashboard does:

```sql
SELECT platform, external_call_id,
       maxIf(revenue, has_converted = 1 AND revenue > 0)            AS max_conv_rev,   -- highest earned amount seen
       argMax(revenue, received_at)                                AS last_rev,
       max(received_at)                                            AS last_received,
       argMax(publisher_id, received_at)                           AS publisher_id,
       argMax(campaign_id, received_at)                            AS campaign_id
FROM pings
WHERE tenant_id = {tenant:UUID} AND platform IN ('ringba', 'retreaver')
  AND platform_duplicate = 0
GROUP BY platform, external_call_id
HAVING max_conv_rev > 0
   AND max_conv_rev - last_rev >= {min_drop:Float64}
   AND last_received >= {scan_since:DateTime}       -- the drop arrived in this scan window
```

- **A void** is `last_rev = 0` (Retreaver also sets `has_converted = 0`). **A partial return** is
  `0 < last_rev < max_conv_rev`.
- **The qualifying time** is `last_received`, which must be ≥ the rule's `active_since` (A.3).
- **Only the real versions count:** content-hash dedup [PROVEN, ingestion slice "identical resend added ZERO rows"]
  means an unchanged re-sync adds no version, so a sweep cannot fake a drop.
- **Edge cases:**
  - A drop followed by a re-conversion (→ 0 → back up) still qualifies; it is dispute evidence too.
  - A call converted at $0 (the 119 "converted-with-$0" rows) never qualifies, because `max_conv_rev` needs
    revenue > 0.
- **Ringba:** a void there can keep `hasConverted` true with revenue 0 (report R3). The revenue test covers both
  platforms.

### C.2 Tables — migration `000038_transcription_rules` (additive)

| Table / change | Purpose |
|---|---|
| `transcription_rules` | A.1 fields; `type` CHECK; `params` JSONB validated by the API per type |
| `transcription_rule_scope` | (rule, platform, `campaign_id` or `vertical`, `added_at`); a campaign added later is forward-only from `added_at` |
| `campaign_verticals` | A.4 mapping |
| `transcription_job_rules` | (job, rule, `matched_at`); every rule a job matched |
| `transcription_jobs` + columns | `priority SMALLINT NOT NULL DEFAULT 3`, `rule_id` (the counted rule), `trigger_detail JSONB`, `estimated_minutes`; `reason` CHECK widened with `scope`, `sampling`, `void_return`, `new_publisher`, `alert_sample` |
| `transcription_usage_daily` + columns | `rule_id` (nullable = tenant total), `reason`, `usd_cost`, `language`; the PK becomes (tenant, date, rule or 0, reason) |
| `transcription_plans`, `tenant_transcription_limits` | B.1 |
| `tenant_provider_keys` | B.5, encrypted, write-only |
| `transcription_rule_audit`, `transcription_limit_audit` | A.6, immutable |
| `transcription_campaign_dismissals` | A.4 |
| `transcription_alert_markers` | (tenant, threshold, month), so each 50/80/100 % alert fires once |
| `transcription_enqueue_cursor` | Per tenant and rule type, the last scan time (the window start) |

- **RLS** on every tenant table; `callloom_api` gets table grants under RLS.
- **The worker** gets only `SECURITY DEFINER` functions, with **`REVOKE ... FROM PUBLIC` and one `GRANT` each** (the
  000036 / 000037 rule). `claim_transcription_job()` is replaced, with the same signature plus new output columns:
  fair queue, priority, limits.
- **The 37 grant test** (`scripts/verify-migration-37-function-grants.sql`) lists every `SECURITY DEFINER` function and
  must be extended with the new ones. Its count check fails otherwise, by design.
- **Down migration:** drops the new tables and columns and restores the old `reason` CHECK and claim function. Rows
  with the new reasons are deleted first, and the down file says so.

### C.3 API, worker and UI

**API** (`services/api/src/transcription/`, all behind `FEATURE_TRANSCRIPTION`, admin only unless noted):
- **Rules:** `GET/POST /transcription/rules`, `PATCH/DELETE /transcription/rules/:id`, `POST /transcription/rules/estimate`,
  `POST /transcription/rules/:id/activate|deactivate`.
- **Verticals:** `GET/PUT /transcription/campaign-verticals`.
- **Notices:** `GET /transcription/notices/new-campaigns`, `POST /transcription/notices/new-campaigns/:id/dismiss`.
- **Usage:** `GET /transcription/usage?month=` (tenant admin; also per rule) and `GET /transcription/usage.csv`.
- **Limits:** `GET/PUT /transcription/limits`. The tenant admin may set only `monthly_budget_usd` and alert recipients;
  the plan and overrides are for the platform operator through the internal route.
- **Backfill:** `POST /transcription/backfill/estimate` and `POST /transcription/backfill` take
  `{campaign_ids[], from, to}` (max 30 days); refused while `TRANSCRIBE_BACKFILL` is off.
- **Enqueuer:** one pass per rule type every 15 min. Each pass reads its cursor, applies `active_since` and scope,
  inserts jobs with `ON CONFLICT DO NOTHING` plus the `transcription_job_rules` rows, and moves its cursor only on
  success.
- **Alert trigger:** `alert_sample` runs right after the 2D evaluator writes an `alert_events` row (same process), with
  the 15-minute pass as a safety net.
- **Daily jobs:** the new-campaign notice and the usage-threshold alerts (also checked after each usage update).

**Worker** (platform-sync):
- a pool of `TRANSCRIBE_WORKERS`;
- claims through the new fair `claim_transcription_job()`;
- **limits before any fetch:** platform, tenant monthly and budget, tenant daily, the reserve, the rule cap and the
  priority share. Over any of them the job is `capped`, with `next_try_at` at the next Eastern midnight, or the 1st for
  monthly limits;
- usage is written per rule, reason, language and $;
- a per-tenant Deepgram key when present.

**UI** (Settings → **Transcription**, which replaces today's Providers block; admin):
1. **Rules:** a list (type, scope, cap, today's minutes, on/off), then a form per type with the estimate panel (A.5)
   and "I have seen the estimate".
2. **Campaigns and verticals:** the mapping table, plus the new-campaign notices.
3. **Usage:** B.4.
4. **Limits:** read-only plan, quota, cap and reserve; the editable budget and alert recipients.
5. **Backfill:** campaigns, date range, estimate, confirm.
6. **Call detail:** the Transcript panel shows *why* a call was transcribed ("Void/return: $13.77 → $0 on Oct 16"),
   the rules it matched, and its priority when it is capped.

### C.4 Price used for $ in the product

- **Our cost** comes from `TRANSCRIBE_PRICE_PER_MINUTE` (0.0063) and a new `TRANSCRIBE_PRICE_PER_MINUTE_MULTI` (0.0072)
  for `language=multi`.
- **The tenant's billed price** comes from its plan's overage price; included minutes show $0.
- Both are stored per usage row, so a later price change does not rewrite history.

### C.5 Cost math [INFERRED from the owner's volume; the trigger volumes are UNKNOWN until measured]

**Assumptions:**
- **Today:** ~**550 converted minutes/day** [OWNER] (~92 converted calls × ~6 min), English cost $0.0063/min;
  30-day month.
- **Trigger volumes (INFERRED, to measure on the server):**

  | Trigger | Assumption at 1× | How it scales | 1× (min/day) |
  |---|---|---|---:|
  | Void / return | 3 % of converted calls | with volume | 16.5 |
  | New publisher | ~1 new publisher/week × 20 calls (owner N = 20) × 6 min | ×3 at 5×, ×10 at 20× | 17.1 |
  | Alert samples | ~0.5 alert/day × 10 calls × 6 min | ×3 / ×10 | 30 |
  | Per-publisher top-up | — | ×5 / ×20 | ~30 |
- **The button** (~60 min/day) is the same in both models, so it is left out of the comparison.

| Size | Global: all converted (min/day → $/month) | Rules: sampling 20 % + triggers (min/day → $/month) | Rules vs global |
|---|---|---|---|
| **1× (today)** | 550 → **$104** | 110 + 16.5 + 17.1 + 30 + 30 = **204** → **$38** | 37 % |
| **5×** | 2,750 → **$520** | 550 + 82.5 + 51.4 + 90 + 150 = **924** → **$175** | 34 % |
| **20×** | 11,000 → **$2,079** | 2,200 + 330 + 171.4 + 300 + 600 = **3,601** → **$681** | 33 % |

(Monthly $ = minutes/day × 30 × $0.0063. With ~10 % Spanish, multiply by ~1.015.)

**Plan fit [INFERRED], at the owner's proposed plans (B.1):**
- **1×:** rules use ~6,100 min/month → **Growth** (15,000, $299) with room; global (16,500) would be over Growth.
  - **Our own tenant (Internal, 25,000 min):** both fit.
- **5×:** rules use ~27,700 → **Scale** (50,000, $899); global (82,500) would be over Scale.
- **20×:** rules use ~108,000 → **Enterprise** (100,000+).

**Throughput [INFERRED]:**
- **Volume:** at 20× global, ~1,830 calls/day, about 180/hour over a 10-hour business day and maybe ~300/hour at peak.
- **Today's single worker:** at ~10–30 s per job it handles roughly 120–360 per hour. That is borderline, and the reason
  B.5 has a worker pool and per-tenant concurrency.
- **Rules at 20×:** ~600 calls/day, comfortable.

**Measure before deciding** (one server query each, added to the implementation's Step 0):
- the void/return rate (C.1's SQL over 30 days);
- new publishers per week;
- `publisher`-scope alerts per day;
- the publisher count and its weekly call distribution (for the top-up);
- the should-convert 30-day minutes (report §2A.6).

### C.6 Existing env settings → platform-level safety limits

| Env | Today | After 2B.2 |
|---|---|---|
| `FEATURE_TRANSCRIPTION` | Feature on/off | Unchanged: the platform kill switch |
| `TRANSCRIBE_AUTO` | Global automatic (all converted) | **Platform switch for all automatic rules.** `false` = only the button and manual backfill. The global "all converted" mode is **removed**: "all converted" is now a `scope` rule with an empty scope, which needs the estimate and a cap |
| `TRANSCRIBE_AUTO_SINCE` | Floor for the global window | Platform floor under every rule's `active_since` (kept for safety) |
| `TRANSCRIBE_BACKFILL` | Backfill allowed | Unchanged: the platform switch for **manual** backfill (stays `false` until wanted) |
| `TRANSCRIBE_SHOULD_CONVERT` | Widens the global scan | **Platform permission** for the `should_convert` rule type (stays `false` until sized) |
| `TRANSCRIBE_DAILY_MINUTES_CAP` | Per-tenant daily cap | **Platform-wide** daily minutes across all tenants. Per-tenant caps move to the DB (B.1) |
| new `TRANSCRIBE_PLATFORM_DAILY_USD_LIMIT` | — | Platform-wide $ per Eastern day (B.5) |
| `TRANSCRIBE_MIN_SECONDS` | Minimum duration | Platform floor; rules may only raise it |
| `TRANSCRIBE_MAX_ATTEMPTS`, `TRANSCRIBE_LOOKBACK_HOURS`, `TRANSCRIBE_SPANISH_CAMPAIGN_PATTERN`, `RECORDING_RETENTION_MONTHS` | — | Unchanged (the Spanish pattern stays the fallback until a campaign-level language setting exists) |
| `TRANSCRIBE_PRICE_PER_MINUTE` (+ new `_MULTI`) | Estimate price | Our cost (C.4) |
| new `TRANSCRIBE_WORKERS` | — | Worker pool size (B.5), default 8 |
| `DEEPGRAM_API_KEY`, `ARCHIVE_S3_*` | — | Unchanged; the platform default key and bucket |

### C.7 Tests (to write with the code)

- **Forward-only:** a rule created at T queues nothing that became converted before T, even when a later version
  arrives after T; a campaign added at T2 qualifies only from T2; re-activation resets `active_since`.
- **Sampling:** the hash is stable across scans; the per-publisher weekly top-up reaches N and stops; the percentage
  holds over a 10,000-call fixture (±1 %).
- **Void / return** on version fixtures:
  - 13.77 → 0 and 57 → 21 qualify;
  - an unchanged re-sync and converted-at-$0 do not;
  - a drop before `active_since` does not;
  - `trigger_detail` is stored and reaches the evidence pack.
- **Should-convert:** cannot be activated before sizing; the platform permission is enforced.
- **New publisher:** the first-ever call at or after `active_since`, the first N only, and an older publisher never.
- **Alert sample:** once per alert, `k` calls inside the alert window, and alerts before `active_since` ignored.
- **One job per call:** several matched rules → one job, every match recorded, the highest priority counted.
- **Limits:**
  - each cap enforced before any fetch: platform minutes and $, tenant monthly, budget, daily, the reserve (the button
    and void get through when rules are at their share), the rule cap, and backfill ≤ 50 %;
  - capped jobs keep their state, priority and reason and resume in order;
  - all three over-quota modes; alerts fire once per threshold per month (bell + email fake).
- **Fair queue:** two tenants, one with 1,000 due jobs → round-robin claims; ≤ 10 running per tenant and ≤ 40 total;
  `FOR UPDATE SKIP LOCKED` with parallel claimers (a real Postgres test like `store_pg_test.go`).
- **Estimates:** they match a direct ClickHouse query on fixtures and queue nothing.
- **Audit and RLS:** every rule and limit change is audited and immutable; every new table is isolated per tenant; the
  grant matrix for every `SECURITY DEFINER` function is extended (the 37 test).
- **New campaign:** the notice appears once, dismiss holds, and it never queues anything.
- **Gates:** the full laptop gates with every flag off; then rules on against the fake providers (fake Deepgram + S3),
  including a void fixture end to end.

### C.8 Rollout (after the owner approves this spec)

1. Implementation on a branch, with the fake providers only; the server is not touched.
2. Server **Step 0**, read-only: the C.5 measurements and the should-convert query (Aryan, GET / SELECT only).
3. Deploy with every flag off. Migration 000038 sets:
   - every existing tenant → **Growth**, `stop`, `reserve_pct` 10;
   - then **our tenant (instar.io) → Internal**: 25,000 min/month, daily cap 1,667, `over_quota_mode` `stop` (the owner
     may switch it to `notify_only`), `reserve_pct` 10. The deploy pack sets this by its **tenant id**, confirmed by Syed;
     the migration never guesses a tenant by name.

   The env `TRANSCRIBE_DAILY_MINUTES_CAP` becomes the platform-wide limit. `TRANSCRIBE_AUTO` stays `false`.
4. The owner creates the first rules in the UI, estimate first: for example `void_return` (all campaigns) and
   `sampling` 20 % on two campaigns.
5. `TRANSCRIBE_AUTO=true` (the platform switch for rules) only after that, with a check after 20 minutes, like
   today's 6(d) stop rule.
6. Usage page and alerts reviewed after one week, before widening.

### Owner answers (2026-10-23) — recorded in the sections above

1. **Plans** (a proposal; final prices are a management decision; transcription + AI QA included):
   - **Starter:** 3,000 min, $69/month, overage $0.03/min;
   - **Growth:** 15,000 min, $299/month, overage $0.03/min;
   - **Scale:** 50,000 min, $899/month, overage $0.025/min;
   - **Enterprise:** 100,000+ by contract (~$0.015/min);
   - **Internal**, our own tenant **instar.io**: no billing, 25,000 min/month.

   See B.1.
2. **Void / return:** `min_drop_usd` default **$1**; partial returns **count**. (A.2 (3))
3. **Per-publisher minimum:** **5 per week**. (A.2 (2))
4. **New publisher:** **N = 20**, **any** calls (not only converted), normal minimum duration **20 s**. (A.2 (5); C.5
   recomputed)
5. **Alert trigger:**
   - only `conversion_rate_below`, `short_call_share_above`, `repeat_caller_share_above` and the QA flag-rate
     trigger(s) after 2C;
   - **not** volume, revenue, `no_calls` or `integration_unhealthy`;
   - `k` = **10** most recent calls in the alert window;
   - for conversion drops, **non-converted calls first**. (A.2 (6))
6. **`notify_only`:** only the **Enterprise** and **Internal** plans. Other plans get `stop` or `overage` up to $X.
   (B.1, B.2)
7. **Internal usage report:** an internal route + CSV for now; **an operator page when the second paying company is
   onboarded**. (B.5)

**Tests added by these answers** (on top of C.7):
- the API refuses `notify_only` for Starter, Growth and Scale;
- the API refuses `alert_sample` with `volume_change`, `revenue_drop`, `no_calls` or `integration_unhealthy`;
- `conversion_rate_below` samples fill `k` with non-converted calls first;
- `new_publisher` queues the first 20 calls ≥ 20 s, converted or not;
- the Internal tenant has a billed amount of $0 in the usage report.
