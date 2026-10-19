# Calyx v2 — Intelligence: Ask AI + Transcription + QA + Workflows (Deepgram + OpenAI)

_Updated 2026-10-16: added Phase 2D Workflows (alert → publisher pause), the full vertical catalog (Appendix A), caller facts and the QA feedback loop. Old 2D (Ask AI over transcripts) is now 2E._

**Branch:** `v2/intelligence`, created from the current `origin/main`. Report the hash you start from.
**Operator:** Talha · **Owner:** Syed
**Report:** `docs/v2-intelligence-report.md`. Add one section per phase.
**Research input (read first):** owner's `v2-recordings-research.md` (attached), covering Ringba and Retreaver recordings.

---

## Scope (owner decision)

**In v2:**
- what **Deepgram** (transcription) and **OpenAI** (QA, Ask AI) power, plus whatever new QA-related UI is needed to use them
- **Workflows: alert → action**, including **publisher pause** on Ringba and Retreaver (owner decision 2026-10-16, Phase 2D)

**Out of v2 (stays hidden behind `V1_ONLY`, services stay off):**
- Ad attribution: Meta CAPI, Google, the ad-attribution service, Ad Spend, CPA, ROI
- TCPA/DNC providers: DNC.com, the compliance-intelligence service, the TCPA risk number, the old Compliance journey step
- Audience Enrichment (mock), i.e. third-party data bought per phone number. The **caller facts** in 2C (what the caller said on the call) are in scope; they are our own data, not enrichment.

Never show a panel or number that is not produced by real Deepgram or OpenAI output, or by real call data. This is the same honesty rule as v1.

## ⛔ Rules

1. **No secrets in chat, logs, commits or reports.**
   - `DEEPGRAM_API_KEY` and `OPENAI_API_KEY` live only in the server `.env`. They must refuse dev placeholders in production, like the other secrets.
   - Ringba Recording Access credentials are stored encrypted per integration, entered only in Settings.
   - **Recording URLs are sensitive** (Ringba opens them without auth for the first 6 days): never log them, never store the short-lived resolved URLs, never render raw URLs in the UI.
   - The **audio itself is archived** in a private bucket (Phase 2B.5); that is different from storing links.
2. **The only write to Ringba or Retreaver is publisher pause / unpause (Phase 2D)**, behind its own kill switch, per-integration toggle and guardrails. v2 code writes nothing else. (The v1 conversion write-back is separate and unchanged.)
   - No Ringba or Retreaver write at all before the owner approves the 2D sandbox plan in writing.
   - Recordings are read only.
   - Do not enable Ringba auto-transcription or Retreaver AI Analyst.
   - Do not change the Live sync or the conversion write-back code.
3. **One feature switch per feature, all off by default:**
   - `FEATURE_ASK_AI`
   - `FEATURE_TRANSCRIPTION`
   - `FEATURE_QA`
   - `FEATURE_WORKFLOWS`
   - `FEATURE_ASK_AI_TRANSCRIPTS`

   Plus the action kill switch `WORKFLOW_ACTIONS_ENABLED=false` (server default). With it off, workflows can alert but never pause.

   `V1_ONLY` keeps hiding everything out of scope. Demo mode stays off in production.
4. **Cost caps are mandatory, and every cap is a hard stop, not a warning:**
   - daily transcription minutes (`TRANSCRIBE_DAILY_MINUTES_CAP`)
   - daily OpenAI tokens (`ASK_AI_TOKEN_BUDGET_DAILY` and a QA equivalent)
   - a monthly spend estimate, shown in Settings

   The owner's budget is about $200 a month for Deepgram and OpenAI together.
5. **Automated tests use fake Deepgram and fake OpenAI only.** Real API calls happen only on the server, with the owner's keys, after owner approval:
   - a small smoke test at the end of each phase (2A: about 5 questions; 2B: about 10 calls)
   - the accuracy test in Phase 2C.6
   - The feature flags stay off for users until the owner says so.
6. Gates run with `pwsh-preview` `-V1` and must stay green. One commit per item, prefix `v2:`. Push the branch only, **never `main`**. **STOP** at the end of every phase.

---

## Phase 0 — Inventory and design (read-only, no product code) 🙋 STOP

Write findings with file:line.

1. **Existing intelligence code.** For scoring, ask-ai, enrichment, compliance-intelligence, ad-attribution and providerconfig, say what each does today (mock vs real), what can be reused for Deepgram and OpenAI, and what stays off.
   - Include the Kafka topics: `call_recording_ready`, `call_transcript_ready`, `call_scored`.
   - Include the alert engine, which reads `call_scored`.
   - Include `DEEPGRAM_CALLBACK_SECRET` and the existing DB tables for transcripts and scores.
2. **Recording data we already sync.**
   - Does the Ringba and Retreaver sync store the recording URL (or `recording_url`, `inbound_leg_recording_url`)?
   - On the laptop data: what share of real calls has a recording, per platform, overall and for converted calls?
3. **Recording access test, read-only, one call per platform, HEAD or a few bytes only.**
   - **Ringba:** confirm the redirect (no-redirect GET, read `Location`) and that Basic Auth with Recording Access credentials is needed.
     - The owner creates the `calyx-server` credentials and gives them to you through a password manager.
     - Put them only in your **local, git-ignored** `.env` for this test, never in chat, code or commits. No product code in Phase 0.
   - **Retreaver:** does `recording_url` open without auth? Mono or stereo? Which format?
   - Download no full audio unless needed. Store nothing.
4. **Design proposal**, half a page each:
   - **Transcription:** sync or async (Deepgram callback vs worker queue).
   - **Language:** detection for Spanish campaigns (ACA Spanish, Debt Spanish).
   - **Speakers:** labelling caller vs agent (diarization plus an LLM pass).
   - **Storage:** where transcripts and QA results live (Postgres vs ClickHouse).
     - Retention: archived audio has a fixed 24 months (owner decision).
     - Transcripts and QA stay as long as the call data stays. That is the general data-retention decision, still pending with the owner; until then, no automatic deletion of transcripts or QA.
   - **Providers' data handling:** confirm, from the official docs, how the Deepgram and OpenAI APIs treat our data (retention, training use) and which account settings to choose. Quote the docs.
   - **Ask AI method:** tool-calling over existing analytics queries (preferred, no free SQL) vs text-to-SQL. Recommend one.
   - **Keys:** server env vs per-tenant provider config. The owner prefers server env for v1 of v2.
   - **Recording archive (2B.5):** the bucket layout, the encryption, the S3 client already in the repo or a new one, lifecycle plus the cleanup job, and the audit log table.
   - **Cost:** a model per month at today's volume, about 8,200 calls a month, roughly 35% converted. Deepgram Nova-3 batch is $0.0043/min. Pick the OpenAI model: a small model by default, configurable.
5. **Workflow facts (for 2D), docs and GET only, no writes:**
   - **Alert engine today:** rules table, evaluator, how often it runs, what it can notify (in-app, email?). Is there an email sender in the repo? If not, propose one (SMTP env vars).
   - **Ringba publisher pause by API:** find the documented endpoint in Ringba's official API docs. Quote it with the URL. Which token permission does it need, and does our current token have it?
     - Ringba's help center says the Publisher-list pause "pauses the publisher across all campaigns". Is there also a documented per-campaign pause by API?
     - If no documented API exists, say so. Do not reverse-engineer the web app's private calls. 🙋
   - **Retreaver affiliate pause:** confirm `PUT /api/v1/affiliates/{id}.json` with `{"affiliate":{"paused":true}}` (learn.retreaver.com/api/affiliates). Does a GET return `paused`, so we can read back? Is there a per-campaign pause by API?
   - **Publisher mapping:** do we already store the Ringba publisher id and the Retreaver affiliate id per call, so an alert maps to the exact entity to pause?

Push the report, give the hash, **STOP** for owner approval.

---

## Phase 2A — Ask AI on numbers (OpenAI, no transcripts)

1. An "Ask AI" page or box: the admin types a question; the answer comes **only** from Calyx data.
   - Examples: "Which publisher had the lowest conversion yesterday?", "Texas revenue this week vs last week", "Campaigns that dropped in 7 days".
2. Data access:
   - Read-only and tenant-scoped (RLS).
   - Through approved tools or queries only: metrics by platform, campaign, publisher, buyer, state and date range, in **Eastern time** by default.
   - No free-form SQL against the database.
3. Every answer shows the **numbers it used** (a small table) and the date range. If the data cannot answer, it says so. It never invents a number.
4. Token budget per day (hard stop with a clear message), plus logging of usage only, never prompt content with PII.
5. Tests:
   - a fake OpenAI
   - a question with no data
   - a budget hit
   - tenant isolation
   - a prompt-injection attempt in a campaign name, which must not change behaviour

**STOP**: report + screenshots.

---

## Phase 2B — Transcription (Deepgram)

1. **Which calls:**
   - **Automatic:** converted calls **and** "should have converted" calls.
   - **On demand:** a "Transcribe" button on call detail, admin only.
   - **Never:** calls under a minimum duration (`TRANSCRIBE_MIN_SECONDS`, default 20). A call is never transcribed twice (idempotent).
2. **Pipeline:**
   - Resolve the recording at transcription time. Ringba: no-redirect + `Location`, with the Recording Access credentials.
   - Send it to Deepgram Nova-3 pre-recorded with diarization, punctuation and language detection.
   - Store the transcript as utterances: speaker, role (caller or agent), start, end, text.
   - **Never store the resolved (short-lived) URL.** Audio is stored **only** in the archive bucket (2B.5), never in the DB or on the server disk.
   - **Sensitive numbers:**
     - Turn on Deepgram redaction of card numbers and SSNs (`pci`, `ssn`) in the transcript text. Confirm the exact parameter names in Phase 0.
     - Callers on SSDI, Medicare and ACA calls can say these numbers out loud. With redaction they never reach the DB or OpenAI.
     - The archived audio stays admin-only.
   - **When:**
     - Queue a call when it **becomes** converted or should-have-converted. Conversions can arrive later through the sweeps.
     - Skip platform duplicates.
     - On first enable, offer an **optional backfill of the last N days** (default 7), showing the minutes and the $ estimate before it starts.
   - Retries with backoff. The daily minutes cap stops the queue cleanly.
3. **"No recording" is a normal state**, stored with a reason:
   - not answered (Ringba)
   - recording disabled
   - blocked or cut (Retreaver)
   - fetch failed
4. **Ringba integration form:** optional "Recording access username / password", stored encrypted, never shown again, the same pattern as the API key.
5. **Recording archive (owner decision: keep our own copy for disputes).**

   Fetch the audio **once**. Use the same bytes for Deepgram **and** for the archive. Never fetch it from Ringba or Retreaver a second time.
   - **What is archived:**
     - every call that gets transcribed (converted, should-have-converted, on demand)
     - plus an admin switch **"Archive all recorded calls"** (default off)
   - **Where:**
     - a **private, encrypted object-storage bucket**: Backblaze B2 (preferred, cheapest) or Cloudflare R2, through the S3-compatible API
     - credentials in the server `.env` only, with an application key **restricted to this one bucket**
     - never on the server disk; no public access
   - **Retention: 24 months** (owner decision), configurable.
     - Delete with a bucket lifecycle rule **and** a Calyx cleanup job.
     - The DB row is marked "expired" afterwards; the transcript and QA stay.
   - **Proof of integrity**, stored with each recording:
     - SHA-256 of the audio
     - size and duration
     - platform, external call id
     - download time (UTC)
     - Deepgram request id and model
     - the transcript version

     The transcript links to that hash, so a "you made up the transcript" claim can be answered.
   - **Dispute evidence pack**, admin only, on call detail: one download with the audio, transcript, QA result with quotes and timestamps, and the hash/metadata file.
   - **Access:**
     - admin-only playback in Calyx through an API proxy (streamed from the bucket, never a raw or signed public URL in the page)
     - every play or download is **audit-logged**: who, when, which call
   - **Cost:**
     - show the archive size and estimated $ in the Providers/usage panel
     - expected about 8–30 GB a month; B2 is about $6.95/TB/month and R2 about $0.015/GB-month
   - A failed archive upload must not block transcription. Retry, and show the state "not archived".
   - Tests:
     - single fetch for both uses
     - the hash is stored and verified on download
     - expiry deletes the audio but keeps the transcript
     - a non-admin gets 403
     - playback is audited
     - no bucket credentials or URLs in logs
6. **UI:** the call detail **Transcript** panel shows caller and agent, timestamps, search within the transcript, and its state: pending / ready / no recording / failed. The panel includes the admin-only audio player from the archive.
7. **Settings → Providers tab, back, Deepgram and OpenAI only (plus the archive storage status):**
   - shows configured yes/no (never the key)
   - this month's minutes, tokens and estimated $
   - caps
8. Tests:
   - a fake Deepgram
   - Ringba redirect handling
   - redaction (no URL or key in logs)
   - idempotency
   - the cap
   - the no-recording states
   - a Spanish call
   - card numbers and SSNs are redacted in the stored transcript
   - duplicates are skipped
   - a call that becomes converted later is queued once
   - the backfill shows its estimate before it starts

**STOP**.

---

## Phase 2C — QA, sentiment and performance (OpenAI)

1. **One OpenAI pass per transcript**, with structured JSON output and a versioned rubric.

   **Core QA, every call, any vertical:**
   - **Outcome:** sale/qualified, not interested, wrong number, already covered, voicemail/IVR, hang-up, other.
   - **Connected:** was there a real conversation?
   - **Traffic quality flags:** robocall, silence, repeat caller, "I didn't call", incentivized, confused caller.
   - **Should-have-converted reason:** only when the call is in that bucket. It gives dispute evidence.
   - **Objections:** list.
   - **Caller statements:** the caller asked not to be called again, or denied consent. This comes only from the transcript; no DNC provider.
   - **Sentiment:** caller at start and at end, and agent tone.
   - **Agent/buyer performance:** answered, greeting, hold, close attempt. Sub-scores and an overall 0–100.
   - **Evidence:** a short quote and a timestamp for every flag and score.
   - **Language:** QA output is always in English. Quotes from Spanish calls are kept in the original language, with an English translation next to them.
   - **Caller facts (our own first-party profile, not enrichment):** the qualifying facts the vertical pack asks for, **only when the caller said them**. Examples: age or age band, state/zip, homeowner, currently insured, number of vehicles, debt band, accident date, project timeline.
     - Each fact has a quote and a timestamp. Never guessed. "Not mentioned" is a valid value.
     - No medical details beyond the pack's yes/no qualifying answers.
     - Shown on call detail and usable as Pings/Reports filters (e.g. "homeowner = yes, not converted").
     - Never sent to any ad platform or third party.
2. **Vertical packs**, for the "qualified?" check, stored as **config (DB/JSON), not code**, so new verticals and buyer-specific criteria need no deploy. The full list is in **Appendix A**.
   - **Active at launch:** Appendix A part 1, the verticals already running in our tenant (about 25). Each has qualifying criteria, caller-fact fields and campaign-name keywords.
   - **Catalog (`prepared`):** every other pay-per-call vertical in Appendix A part 2 is created as a pack with name, keywords and draft criteria. It becomes `active` as soon as a campaign is mapped to it, and its accuracy is tracked like the others. No deploy needed.
   - **Buyer criteria:** the admin can add buyer-specific rules to a pack (e.g. "age 50–80, no Medicaid"). Every edit is a new pack version.
   - **Regulated verticals** (marked R in Appendix A): QA may check whether a required disclaimer was read, e.g. the Medicare TPMO disclaimer, as "read / not read" with a quote. Never a legal verdict.
   - **Generic fallback:** for unmapped campaigns, the AI infers the vertical from the transcript and checks intent, decision maker and eligibility mentions. Core QA always runs.
2b. **Feedback loop (accuracy improves as the team uses it):**
   - Every "QA correct / incorrect" click stores the call, the field that was wrong, the right value and the note, per vertical.
   - **Settings → QA** shows accuracy per vertical: agreement % over the last 30 days and the number of labelled calls.
   - Corrected calls become examples in that vertical's prompt (a few, tenant-scoped, already redacted). Applying them is a new rubric version: re-run on a sample, show accuracy before vs after, and the admin approves before it goes live.
   - **New vertical seen:** when the generic fallback sees the same unknown vertical on ≥ 10 calls in 7 days, Calyx suggests activating a catalog pack or creating one.
   - No model fine-tuning in v2. Prompt examples and pack criteria only.
3. **Campaign → vertical mapping.**
   - Automatic from the campaign name (keywords, or the LLM once per new campaign).
   - The admin can change it in Settings, on a new **QA** tab.
   - Show the mapping for all current Ringba and Retreaver campaigns.
4. **UI:**
   - **Call detail:** a QA card with the score, outcome, flags, sentiment and evidence quotes, plus a **"QA correct / incorrect"** button with an optional note. Feedback is stored for later prompt examples.
   - **Pings:** QA score column; filters "Qualified but not converted", "Fraud/quality flag", "No recording".
   - **Reports:**
     - publisher quality: average QA, flag %, should-have-converted %, by publisher and campaign
     - buyer/agent performance
     - top objections
   - Publish `call_scored` so the alert engine of Phase 2D can use QA triggers.
   - **Sidebar:** an "Intelligence" section only for the features that are on.
5. **Re-run QA** on chosen calls after a rubric change: admin, with a cost estimate shown first.
6. **Accuracy test, 🙋 owner approval needed.** On the server, with the real keys:
   - Take about 30 real calls across the top verticals (Auto, Final Expense, Medicare, ACA, Home services), transcribed and scored.
   - The owner or the team labels outcome and qualified for each.
   - Report the agreement %.
   - **Target ≥ 85%** on outcome and qualified before `FEATURE_QA` is turned on for users.
7. Tests:
   - a fake OpenAI with fixed JSON
   - schema validation
   - an unknown vertical falls back to generic
   - a prepared pack becomes active when a campaign is mapped to it
   - a caller fact that was never said is stored as "not mentioned", never guessed
   - feedback is stored per vertical and shows in the accuracy view
   - the cap
   - tenant isolation
   - prompt injection inside a transcript ("ignore instructions…") must not change the output format or scores

**STOP**.

---

## Phase 2D — Workflows: alert → action, with publisher pause (owner decision)

**Goal:** Calyx watches the numbers and the QA results, alerts the team, and can **pause a publisher** on Ringba or Retreaver. It does that with one click by an admin, or automatically for a rule where the admin has turned automatic mode on.

**Order:** the triggers on call data need no AI, so they can be built right after 2A. The QA triggers plug in after 2C. Do not wait for 2C to start 2D.

1. **Rules (Settings → Alerts tab, back):** IF *trigger* over *window* (with a minimum number of calls) THEN *actions*.
   - Scope: per publisher (default), per campaign, or the whole tenant. Optional platform filter.
   - The evaluator runs every 15 minutes, in **Eastern time**, on the same deduplicated data as the dashboard.
2. **Triggers on real call data (v1 data):**
   - publisher conversion rate below X% over the last N calls
   - share of short calls (under X seconds) above Y%
   - repeat/duplicate caller share above Y%
   - call volume up or down by X% vs the 7-day average for the same hours
   - revenue down X% vs the same day last week
   - a campaign has had no calls for X hours inside set hours
   - integration sync failing or degraded (notify only, never pause)
3. **Triggers on QA (after 2C):**
   - quality flag rate (robocall, "I didn't call", incentivized…) above X%
   - average QA score below X
   - should-have-converted share above X%
   - caller asked not to be called again: N calls in 24 h
4. **Actions:**
   - **Notify:** an in-app alert centre (bell) plus email to the admins. Each alert shows the numbers it used, the window and a link to the filtered Pings list. Slack/WhatsApp come later.
   - **Pause publisher**, set per rule, in three modes:
     - **Notify only** (default)
     - **Suggest:** the alert has a "Pause publisher" button; the admin confirms in a popup
     - **Auto:** Calyx pauses the publisher itself, then notifies at once with an "Unpause" button
   - **Unpause:** always a manual admin action, or an optional "pause for N hours" timer set on the rule.
5. **What pausing does on each platform:**
   - **Ringba:** pausing from the Publisher list "pauses the publisher across all campaigns" [PROVEN-docs: support.ringba.com/hc/en-us/articles/17993410144663]. The API endpoint is [UNKNOWN] until Phase 0 item 5 finds it.
   - **Retreaver:** `PUT /api/v1/affiliates/{id}.json` with `{"affiliate":{"paused":true}}`. "Paused affiliates are excluded from active routing" [PROVEN-docs: learn.retreaver.com/api/affiliates]. Whether a GET shows `paused` (for read-back) is [UNKNOWN].
   - **So a pause is global for that publisher:** all its calls on all campaigns stop, and so does its revenue. The confirmation popup says this in bold, with the publisher's calls and revenue for the last 7 days.
   - If Ringba has no documented pause API: Ringba gets **Suggest mode with a link** to that publisher in Ringba ("pause it there"), and Auto is disabled for Ringba. 🙋
   - If Phase 0 finds a documented per-campaign pause, offer the scope "this campaign only" as well. 🙋
6. **Guardrails (all mandatory):**
   - **Switches:** kill switch `WORKFLOW_ACTIONS_ENABLED=false` on the server, plus a per-integration toggle "Allow Calyx to pause publishers" (off by default; admin turns it on in Settings; Live integrations only, never Test).
   - **Suggest first:** Auto can be turned on for a rule only after it has run in Suggest mode for at least 7 days. The rule page shows what it would have paused in that time.
   - **Minimum sample:** default 20 calls in the window. No pause on 3 bad calls.
   - **Daily limit:** at most 3 automatic pauses per tenant per day (`WORKFLOW_MAX_AUTO_PAUSES_DAILY`). After that, Suggest only, plus an alert.
   - **Protected publishers:** a list the admin keeps. They are never auto-paused (Suggest only).
   - **Cooldown:** one alert per rule per publisher every 6 hours (configurable). No spam.
   - **Same safety as write-back:** admin only (403), CSRF, idempotency lock (409), the shared per-company rate limiter with sync, retry only on 429/5xx, API keys redacted in every log and error.
   - **Read-back** after every pause and unpause. A mismatch is a failure alert.
   - **Audit row for every attempt**, including blocked ones: rule, numbers used, mode, who (or "auto"), request with the key redacted, read-back, result.
   - Calyx never deletes a publisher and never changes payouts, campaigns, targets or routing. Only paused on/off.
7. **Sandbox proof (🙋 owner approval; no phone call needed):**
   - The owner creates a publisher `CALYX-TEST-PUB-DO-NOT-USE` in Ringba and an affiliate with the same name in Retreaver: no numbers, no campaigns, no traffic.
   - GET only first: prove it has no numbers or campaigns. Write the plan table (read → pause → read → unpause → read), push, **STOP** for written approval.
   - After approval: a probe tool (not in any image) runs exactly those steps on that test publisher only and records every read-back.
8. **UI:**
   - **Alerts tab:** rules list; a create/edit form with trigger, window, minimum calls, scope, actions and mode; a "Dry run" button that shows which publishers would match today, without acting.
   - **Alert centre:** history with status (open / acknowledged / actioned), the action log and the Unpause button.
   - Sidebar "Intelligence" shows Alerts when `FEATURE_WORKFLOWS` is on.
9. **Tests (fake Ringba and Retreaver APIs):**
   - each trigger, in ET, on the dedup view; the minimum sample blocks a small window
   - Notify / Suggest / Auto; Auto refused before 7 days of Suggest
   - kill switch off → alert only, no write; integration toggle off; Test integration → never
   - daily limit, protected publisher, cooldown
   - pause → read-back OK; read-back mismatch → failure alert; 429 then success; 4xx not retried
   - 403 non-admin, CSRF, 409 lock, audit row on every path, no key in any log
   - unpause timer
   - Ringba without an API → link only, no write

**STOP**: report, screenshots (fake APIs), the sandbox plan.

---

## Phase 2E — Ask AI over transcripts and QA

- Questions such as "Why are publisher X calls not converting?", "Top objections this week for Medicare", "Show calls where the caller said they didn't call".
- Answers cite the calls (links to call detail) and quotes, read-only and tenant-scoped.
- Same budget and injection safeguards as 2A.

**STOP**.

---

## Deploy notes (in the report, per phase; Aryan runs them later)

- New env vars:
  - `DEEPGRAM_API_KEY`, `OPENAI_API_KEY`, the model name
  - the archive bucket: endpoint, bucket, key id/secret; `RECORDING_RETENTION_MONTHS=24`
  - the caps
  - the feature flags (default off)
  - `TRANSCRIBE_MIN_SECONDS`
  - `WORKFLOW_ACTIONS_ENABLED=false`, `WORKFLOW_MAX_AUTO_PAUSES_DAILY=3`, and the email sender settings if a new one is added
- Which services must run (only the ones needed, e.g. scoring and ask-ai). Ad-attribution, compliance-intelligence and enrichment stay off.
- Migrations, in order.
- Images to rebuild.
- How to turn each feature on and off.
- `WRITEBACK_ENABLED` and the sync are untouched.

## Output per phase

- Scoreboard
- Findings with file:line
- Test list
- Screenshots (fake providers until 2C.6)
- Cost numbers
- Deploy note
- Open questions for the owner
- Secret scan of every commit

Push `v2/intelligence`, give the hashes, **STOP**.

---

## Appendix A — Vertical catalog

Source marks:
- **T** = already running in our tenant (from the owner's campaign list)
- **Rg** = listed on Ringba's official "Pay Per Call Verticals & Industries" page (ringba.com/blog/resources/verticals-and-industries)
- **I** = common pay-per-call vertical in the industry, not yet seen in our traffic; kept as `prepared`
- **R** = regulated: extra disclosure checks in QA (read / not read, with a quote), never a legal verdict

In Phase 2C.3, Talha compares this list with the real Ringba and Retreaver campaign names. Any vertical found in the tenant but missing here is added to part 1.

### Part 1 — Active at launch (T)

| # | Pack | Qualifying facts (caller facts) | Mark |
|---|---|---|---|
| 1 | Medicare | 65+ or has Medicare A/B, state, not in a facility | R |
| 2 | ACA / U65 health (incl. Spanish) | under 65, no employer coverage, state, household size | R |
| 3 | Final Expense | age about 50–85, decision maker | R |
| 4 | Auto insurance | vehicle, licence, currently insured or not, zip | |
| 5 | Home insurance | homeowner, zip | |
| 6 | SSDI | age, stopped working, medical treatment, no attorney or benefits yet | R |
| 7 | MVA / personal injury | recent accident, injured, treated, not at fault, no attorney | R |
| 8 | Debt relief (incl. Spanish) | unsecured debt over the threshold, not in bankruptcy | R |
| 9 | Travel | interest, eligibility | |
| 10 | Home security | homeowner, interest | |
| 11 | Medical alert | senior or caregiver, interest | |
| 12 | Home warranty | homeowner, interest | |
| 13 | Roofing | homeowner, project type, timeline, service-area zip | |
| 14 | HVAC | same home-services set | |
| 15 | Plumbing | same | |
| 16 | Windows & doors | same | |
| 17 | Bath & kitchen remodel | same | |
| 18 | Siding & gutters | same | |
| 19 | Flooring | same | |
| 20 | Solar | same, plus roof/electric bill | |
| 21 | Water damage / restoration | same, plus damage date | |
| 22 | Pest control | same | |
| 23 | Tree service | same | |
| 24 | Garage doors | same | |
| 25 | Foundation & insulation | same | |
| 26 | Walk-in tubs & hot tubs | same, plus senior for walk-in tubs | |

### Part 2 — Catalog (`prepared`, activated by mapping a campaign)

**Insurance**
- Life insurance (Rg) · Health insurance, other than ACA/Medicare (Rg, R) · Medicare Supplement / Part D as sub-types of Medicare (I, R)
- Dental & vision (I) · Supplemental: hospital indemnity, cancer, accident (I, R) · Short-term health (I, R)
- Pet insurance (I) · Renters (I) · Motorcycle / RV / boat (I) · Commercial / business insurance (I)

**Financial**
- Credit repair (Rg, R) · Tax debt relief (Rg, R) · Student loan consolidation (Rg, R) · Mortgage / refinance (Rg, R)
- Payday loans (Rg, R) · Merchant services (Rg) · Reverse mortgage (I, R) · Personal loans (I, R)
- Auto / title loans (I, R) · Business funding (I) · Bankruptcy help (I, R)

**Legal** (all R)
- Criminal (Rg) · Bankruptcy (Rg) · Divorce (Rg) · Family (Rg) · Medical malpractice (Rg) · Personal injury, other than MVA (Rg)
- Mass tort, one sub-pack per case type (I) · Workers' comp claims (I) · Employment / wrongful termination (I) · DUI (I) · Veterans disability claims (I)

**Home services** (Rg unless marked)
- Appliance repair · Electrical · Locksmith · Lawn care · Home cleaning / maid · Towing · Moving (I) · Junk removal (I)
- Water treatment (I) · Mold remediation (I) · Fire damage (I) · Septic (I) · Pool (I) · Painting (I) · Fencing (I)
- Concrete (I) · Chimney (I) · Carpet cleaning (I) · Handyman (I) · Generators (I) · Stair lifts (I)

**Telecom and utilities**
- Cable / satellite TV (Rg) · Internet (Rg) · Wireless / home phone (I) · Energy / utility switching (I)

**Auto**
- Auto dealerships (Rg) · Auto repair (Rg) · Auto warranty / vehicle service contracts (I, R) · Auto glass (I) · Auto transport (I) · Cash for cars / junk cars (I)

**Health and care** (all R)
- Dental (Rg) · Weight loss (Rg) · Drug & alcohol rehab (Rg) · Hair removal (Rg) · Plastic surgery (Rg)
- Hearing aids (I) · Vision / LASIK (I) · Medical equipment and supplies, e.g. CGM, CPAP, braces (I) · Home health care (I) · Senior living (I)

**Consumer and other** (Rg unless marked)
- Dating · Flowers · Identity theft protection · IT services / tech support · Psychic
- Timeshare exit (I, R) · Real estate, sell house fast (I) · Education / online degrees (I, R) · Funeral pre-need (I) · Business phone / VoIP (I)

**Generic fallback** for anything not listed: core QA plus intent, decision maker and eligibility mentions.
