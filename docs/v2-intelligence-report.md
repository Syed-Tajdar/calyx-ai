# Calyx v2 — Intelligence report

**Spec:** `docs/v2-intelligence-spec.md` · **Research input:** `docs/v2-recordings-research.md`
**Branch:** `v2/intelligence`, created from `origin/main` @ **`99618e0`** (`merge: Stage 4b — Retreaver
convert / void write-back`). First commit on the branch: `bec1c66` (spec and research).
**Operator:** Claude Code on the owner's laptop · **Owner:** Syed

Labels: **[PROVEN]** shown by code read, a command or a request in this run · **[PROVEN-docs]** quoted from
the vendor's official docs, with URL · **[INFERRED]** follows from that evidence · **[UNKNOWN]** not observed.

---

# Phase 0 — Inventory and design (read-only, no product code)

**Date:** 2026-10-19. **Writes to Ringba or Retreaver:** none. **Product code changed:** none. One
read-only probe was added for item 3 (`services/platform-sync/cmd/probe-v2-recording-head`, not in any
image).

## 0. Scoreboard

| Item | Result |
|---|---|
| 1. Existing intelligence code | ✅ Done. Every intelligence service is mock today; the scoring pipeline shape and the Ask AI plumbing are reusable; there is no Deepgram client, no OpenAI call, no email sender, no object storage. §1 |
| 2. Recording data we sync | ✅ Done. **No recording URL is stored anywhere.** Ringba stores only a `has_recording` flag; Retreaver stores nothing. Share of recorded calls: [UNKNOWN] on the laptop (no Ringba data there); a server query is provided. §2 |
| 3. Recording access test | ⚠️ Partly. **Retreaver** tested: public S3 MP3, no auth, joint stereo. **Ringba** not tested: no Ringba API key or Recording Access credentials on this machine 🙋. §3 |
| 4. Design proposal | ✅ Done. §4 |
| 5. Workflow facts | ✅ Done. §5 |
| Cost model | ✅ Done. §6 |
| Gates | Unit tests and builds unchanged (no product code). Full `pwsh-preview -V1` stack gates not run on the laptop (old stack; laptop platform-sync must stay stopped). §8 |

---

## 1. Existing intelligence code

All six intelligence services are Go, read `MOCK_MODE` (default **true**,
`services/scoring/internal/config/config.go:37`) and fall back to `"mock"` when it is on
(`scoring/internal/config/config.go:84-92`) [PROVEN]. In `deploy/docker-compose.yml` all six sit in the
`intelligence` profile (providerconfig `:298`, compliance-intelligence `:319`, ask-ai `:359`,
ad-attribution `:391`, scoring `:472`, enrichment `:509`) with `MOCK_MODE: "true"` **hard-coded**, and no
`DEEPGRAM_API_KEY` / `OPENAI_API_KEY` passed to any container [PROVEN].

| Service | Today | Reusable for v2 | v2 decision |
|---|---|---|---|
| **scoring** (`services/scoring/cmd/server/main.go`, :8082) | **Mock.** `MockAI.Score` hashes the call id into a fixed 60–99.5 score, always 8 rules passed / 2 failed (`internal/scoring/ai.go:24-40`). The mock emitter turns every ping into a canned transcript (`internal/mockemitter/emitter.go:53-66`). `OpenAIProvider.Score` returns "not implemented" (`ai.go:52-55`). **No Deepgram client exists.** `TranscriptionProvider` / `RecordingSource` are read and logged only (`main.go:33-40`). Scorecards are ignored and `call_scores.scorecard_id` is never written (`engine.go:61-64`) | The pipeline shape: consume `call_transcript_ready` → PII redaction (`internal/pii/redactor.go:11-15`, SSN and card only) → `AIProvider` interface (`ai.go:11-13`) → ClickHouse insert → publish `call_scored` (`consumer.go:122-132`). Config already requires the Deepgram key, the callback secret and the OpenAI key when `MOCK_MODE=false` (`config.go:69-82`) | **Reuse and extend** for 2B/2C. The mock emitter stays off |
| **ask-ai** (`services/ask-ai/cmd/server/main.go`, :8086; `POST /v1/ask`, `GET /v1/history` `:74-75`) | **Template answers.** Keyword intent matching (`internal/nlsql/intent.go:18-34`) into 5 fixed SQL templates (`internal/nlsql/templates.go:6-47`) and canned prose (`internal/nlquery/mock.go:18-58`). `OpenAIProvider.FormatAnswer` returns "not implemented" (`internal/nlquery/openai.go:21-23`). The "SQL whitelist" of `docs/10-known-gaps-and-roadmap.md:25` is the template set plus a validator: SELECT only, no `;`, a keyword blocklist and a required `tenant_id = '…'` substring (`nlsql/validator.go:9-31`). Tenant id is interpolated with `fmt.Sprintf` (`templates.go:12`) | `nlquery.Provider` interface (`provider.go:6-8`), an in-memory per-tenant token budget (`internal/cost/budget.go:10-41`), the query log in PG `ask_ai_query_log` under RLS (`pkg/postgres/store.go:48,67,100,134`) | **Rebuild the core** as tool-calling (§4.6). Keep the query log and budget idea; replace templates and the free-SQL validator |
| **enrichment** (:8083) | **Mock.** 520 fake attributes per caller hash (`internal/providers/provider.go:23-36`) | — | **Stays off** (out of v2 scope) |
| **compliance-intelligence** (:8085) | **Mock.** DNC flags ~1% by hash (`internal/dnc/provider.go:28-37`); STIR/SHAKEN = hash mod 3 (`internal/attestation/analyzer.go:6-17`) | — | **Stays off** (out of v2 scope) |
| **ad-attribution** (:8087) | **Mock.** `MockPlatform.Fire` only logs (`internal/platforms/mock.go:26-36`) | — | **Stays off** (out of v2 scope) |
| **providerconfig** (:8084, `POST /v1/resolve` `main.go:94`) | Real plumbing, mock adapters only: registry knows only `"mock"` (`internal/registry/registry.go:10-14`); catalog has 4 mock entries and no transcription domain (`shared/providers/catalog.json`). AES-256-GCM decryptor (`internal/crypto/crypto.go:17-30`) | Per-tenant provider bindings, if ever needed | **Not used in v2.** Keys live in the server env (owner preference, §4.7) |

**Kafka topics** — all JSON, created by `deploy/docker-compose.yml:99-101` [PROVEN]:

| Topic | Producer | Consumer | Schema |
|---|---|---|---|
| `callloom.calls.call_recording_ready.v1` | **none** | **none** | `shared/contracts/events/call_recording_ready.schema.json:5-13` (optional `recording_url`) |
| `callloom.calls.call_transcript_ready.v1` | scoring mock emitter (`emitter.go:69`); API `POST /scoring/deepgram/callback` (`services/api/src/scoring/scoring.controller.ts:70`) | scoring (`main.go:60`) | `call_transcript_ready.schema.json:5-14` (one `transcript_text` string, no utterances) |
| `callloom.calls.call_scored.v1` | scoring (`consumer.go:132`) | API alert engine (`services/api/src/alerts/alert-engine.service.ts:14,40`), live feed (`live/kafka-live-consumer.service.ts:46,142`) | `call_scored.schema.json:5-13` (`overall_score`, `sentiment`) |

⚠️ **`call_recording_ready` must not carry a URL in v2:** recording URLs are sensitive (§3). The worker
resolves the URL itself at transcription time.

**Alert engine** [PROVEN]: rules table `alert_rules` (`db/postgres/migrations/000004_alert_rules.up.sql:1-23`:
`condition JSONB`, `threshold`, `channels TEXT[]` default `{mock}`, RLS). API `GET/POST /alerts/rules`,
`DELETE /alerts/rules/:id` (`alerts.controller.ts:18-40`). **Event-driven only**: a Kafka consumer on
`call_scored`; it fires only for `condition.field = 'overall_score'` below the threshold
(`alert-engine.service.ts:37-66`). No scheduler. The only notification is `MockNotificationChannel`, a JSON
line to stdout (`alerts/channels/mock.channel.ts:15-33`). No notification table, no in-app inbox. The demo
rules (`000015_demo_alert_rules.up.sql:5-36`) use `metric` instead of `field`, so **they can never fire**.
**No email sender exists anywhere in the repo** (grep for nodemailer/smtp/sendgrid/ses/mailgun/postmark/resend
in code, package.json and go.mod: no hits).

**`DEEPGRAM_CALLBACK_SECRET`** [PROVEN]: route `POST /scoring/deepgram/callback`
(`scoring.controller.ts:57-82`, proxied at `deploy/nginx/calyx.ridgerisemedia.com.conf:131`). It compares a
plain header string (`x-deepgram-callback-secret`, `:59,64`), not constant-time, **skipped when
`MOCK_MODE=true`** (`:63`), and accepts a made-up body (`tenant_id, call_id, transcript_text…`, `:10-16`), not
Deepgram's real callback payload. Production refusal of a placeholder exists in the API (`env.ts:72,105`).
**Not reusable as is** (§4.1).

**Existing tables** [PROVEN]: PG `scorecards` (`000003`, seeded by `000014`), `alert_rules` (`000004`),
`platform_credentials` (`000007`), `tenant_provider_bindings` (`000008`), `ask_ai_query_log` (`000012`, uses
pgvector). ClickHouse `transcripts` (one text blob per call) and `call_scores` (one score, TTL 24 months) in
`db/clickhouse/migrations/000002_phase2_analytics.up.sql`. **No table for recordings, utterances, per-rule QA
results, caller facts, notifications or alert events.**

**Other facts** [PROVEN]: no S3 / object-storage client anywhere (no aws-sdk, minio, s3 in any manifest).
The Go intelligence services have **no** production placeholder refusal (only ingestion, consumer and
platform-sync have `refuseProductionPlaceholders()`, e.g. `platform-sync/internal/config/config.go:92-114`).
Web `V1_ONLY` (`web/src/lib/v1-features.ts:16`, on unless `NEXT_PUBLIC_V1_ONLY=false`, baked at build time)
hides `/scoring`, `/intelligence`, `/ask`, `/compliance`, `/attribution` from navigation only
(`app-navigation.ts:54,68`) — the pages are still reachable by URL [INFERRED]. The API has no route gating by
`V1_ONLY` (only `analytics/adspend.source.ts:11-18`). Existing UI to reuse: `components/scoring/TranscriptPanel.tsx`,
`ScoreCard.tsx`, `components/settings/ProviderBindingsSection.tsx` (already labels Deepgram/OpenAI rows,
`lib/provider-domains.ts:11-35`), `components/settings/AlertRulesSection.tsx`, `components/ask/ChatWindow.tsx`.

## 2. Recording data we already sync

| Question | Answer |
|---|---|
| Ringba recording URL stored? | **No.** The adapter requests and maps only `hasRecording` (`services/platform-sync/internal/adapter/ringba_adapter.go:252,331,368-369`); `recordingUrl` appears only in probe CLIs (`cmd/probe-ringba-sandbox/main.go:99,137-138`) [PROVEN] |
| Retreaver recording URL stored? | **No.** The row struct and mapping have no recording field (`retreaver_adapter.go:334-363,448-470`), so `recording_known` stays 0 [PROVEN] |
| Where the flag lives | `models.RemoteCall.HasRecording / RecordingKnown` (`internal/models/models.go:46-48`) → Kafka `callloom.pings.v1` (`producer/producer.go:65-66`) → ClickHouse `pings.has_recording`, `pings.recording_known` (`db/clickhouse/migrations/000011_pings_ringba_parity_b1.up.sql:8-9`). No URL column [PROVEN] |
| Share of real calls with a recording (laptop) | **[UNKNOWN].** The laptop has 3,652 Retreaver calls and **no** Ringba API data; all Retreaver rows have `recording_known = 0` [PROVEN] (query below) |

Server query for Aryan (read-only; Ringba only, because Retreaver has no flag yet):

```sql
SELECT platform, count() AS calls, countIf(rk = 1) AS known, countIf(hr = 1) AS recorded,
       round(100 * countIf(hr = 1) / nullIf(countIf(rk = 1), 0), 1) AS pct_recorded,
       round(100 * countIf(conv = 1 AND rev > 0 AND hr = 1) / nullIf(countIf(conv = 1 AND rev > 0 AND rk = 1), 0), 1) AS pct_converted_recorded
FROM (SELECT platform, external_call_id, argMax(has_recording, received_at) hr, argMax(recording_known, received_at) rk,
             argMax(has_converted, received_at) conv, argMax(revenue, received_at) rev
      FROM pings WHERE external_call_id != '' AND external_call_id NOT LIKE 'demo-%' AND external_call_id NOT LIKE 'smoke-%' AND external_call_id NOT LIKE 'apisync-%'
      GROUP BY platform, external_call_id)
GROUP BY platform FORMAT PrettyCompactMonoBlock;
```

**Design consequence:** v2 does **not** need to store URLs, and must not (§3, rules). The transcription
worker fetches the call once more at transcription time (Ringba: Get Call Log by id, `recordingUrl`;
Retreaver: `GET /api/v2/calls/{uuid}.json`, `recording_url`) through the shared per-company limiter. That is
a single-call read, **not** a change to the Live sync. A small, optional sync change — Retreaver
`recording_url != null` → `has_recording` — would let the queue skip unrecorded Retreaver calls without an
API call; it needs owner approval because the spec freezes the Live sync 🙋 (Q4).

## 3. Recording access test (read-only, one call per platform)

### 3.1 Retreaver `[PROVEN]`

Probe `services/platform-sync/cmd/probe-v2-recording-head` (stored "My test" credentials, decrypted
in-process; laptop platform-sync stayed stopped). Call: sandbox `a82a7a80-f1fd-46a5-a018-379dd7893ea7`
(recorded on 2026-10-12). Requests: GET v2 call, GET v5 call, 1 HEAD and 2 Range GETs of 10 and 4 bytes. No
audio stored; the URL never printed.

| Fact | Result |
|---|---|
| `recording_url` (v2) | present; host `callpixels.s3.amazonaws.com`, `.mp3`, **no query string** (no signature, no key) |
| `inbound_leg_recording_url` (v5) | absent on this call |
| Auth needed? | **No.** HEAD without credentials → `200`, `audio/mpeg`, 598,144 bytes, `Accept-Ranges: bytes`, no `WWW-Authenticate`, no redirect |
| Range support | yes: `206`, `Content-Range: bytes 0-9/598144` |
| Format | ID3v2.3 tag (1,370 bytes), then **MPEG-2 Layer III, 22,050 Hz, joint stereo** |

[INFERRED] from these facts:
- **Retreaver recording links are public and permanent**: anyone holding the link can download the audio.
  They are as sensitive as Ringba's (more: no 6-day limit). Never log, store or render them.
- **Two channels.** Joint stereo suggests a two-leg recording (caller and agent on separate channels), which
  would make speaker labelling exact with Deepgram `multichannel`. Not proven: joint stereo can also carry
  the same audio on both channels. 🙋 Ask Retreaver support (Q6), or prove it in 2B on one call.
- ~64 kbit/s (598 KB for ~74 s): about **0.5 MB per minute** for the archive.
- This was the sandbox company; that Ridge Rise Media uses the same storage is [INFERRED].

### 3.2 Ringba `[UNKNOWN]` 🙋

Not run. This machine has no Ringba API key (only dev webhook rows, no `api_sync` Ringba integration) and no
Recording Access credentials. What the research doc says stands as [PROVEN-docs] (§5.1, §4): Basic Auth with
Recording Access credentials, a redirect with a `Location` valid "as little as 6 hours", and no auth for the
first 6 days.

To finish it (owner action): create the Ringba Recording Access credential `calyx-server`, then run one
no-redirect GET on the **server** (where the Live Ringba key is) with a probe like 3.1 extended for Ringba:
fetch one call's `recordingUrl` via Get Call Log, `HEAD`/no-redirect → read only `Location` host and status,
with and without Basic Auth, and 4 bytes for the format. Recording Access credentials would go only in a
git-ignored `.env`. 🙋 (Q1)

## 4. Design proposal

### 4.1 Transcription: worker queue, synchronous Deepgram request, upload the bytes

- **Queue in Postgres** (`transcription_jobs`: call, platform, reason `converted | should_convert | on_demand |
  backfill`, state, attempts, next_try_at, unique on `(tenant, platform, external_call_id)` → idempotent).
  A call is enqueued when it **becomes** converted or should-have-converted (the consumer already sees every
  new ping version) or by the admin button. Platform duplicates and calls under `TRANSCRIBE_MIN_SECONDS` are
  skipped at enqueue.
- **Worker** (Go, a new `transcriber` package inside the existing **scoring** service, so no new container):
  1. single-call read of the platform for the recording link (§2), through the shared per-company limiter;
  2. **fetch the audio once** into memory (Ringba: no-redirect GET → `Location`, Basic Auth with the Recording
     Access credentials; Retreaver: plain GET);
  3. SHA-256 → upload to the archive (§4.8);
  4. **upload the same bytes** to Deepgram: `POST /v1/listen` with `Content-Type: audio/mpeg`
     ([PROVEN-docs] https://developers.deepgram.com/docs/pre-recorded-audio). No URL is ever sent to Deepgram,
     so no credentialed or public link leaves Calyx;
  5. store utterances, publish `call_transcript_ready` (now carrying a transcript id, not the text) → QA (2C).
- **Synchronous, not callback.** Pay-per-call recordings are minutes long, well under Deepgram's limits
  ("Maximum 2 GB", [PROVEN-docs] same URL). A synchronous request needs no public endpoint, and the current
  `/scoring/deepgram/callback` route (plain-string secret, skipped in mock mode, made-up payload, §1) stays
  **unused and is removed in 2B**. Deepgram callbacks would only offer Basic Auth in the URL and "The
  `dg-token` header is not guaranteed" ([PROVEN-docs] https://developers.deepgram.com/docs/callback).
- **Caps and retries:** before each job the worker checks today's minutes against
  `TRANSCRIBE_DAILY_MINUTES_CAP` (hard stop: jobs stay queued with state `capped`); retries with backoff on
  429/5xx/network, never on 4xx; "no recording" is a final state with a reason.

### 4.2 Language

- Request: `model=nova-3`, `detect_language=true` for normal campaigns ([PROVEN-docs] "`model=nova-3-general&detect_language=true`",
  https://developers.deepgram.com/docs/language-detection). Nova-3 supports "Spanish: `es`, `es-419`"
  ([PROVEN-docs] https://developers.deepgram.com/docs/models-languages-overview).
- Campaigns whose name contains `Spanish` / `Español` (ACA Spanish, Debt Spanish) send `language=multi` for
  code-switching ($0.0052/min instead of $0.0043, [PROVEN-docs] https://deepgram.com/pricing). The mapping is
  config, editable in the QA tab.
- ⚠️ **Redaction is weaker in Spanish:** "Specific entity types … are available for English only"; for non-English
  audio "only numbers are redacted" ([PROVEN-docs] https://developers.deepgram.com/docs/redaction). So:
  English `redact=pci&redact=ssn`; Spanish `redact=numbers`; **plus** Calyx's own scrub (the existing
  `scoring/internal/pii/redactor.go` SSN/card patterns) on every transcript before it is stored or sent to
  OpenAI. Exact parameter names are now confirmed: `redact=pci`, `redact=ssn`, `redact=numbers` [PROVEN-docs].

### 4.3 Speakers (caller vs agent)

- Default: `diarize_model=latest` ("Resolves to the latest GA batch diarizer (currently v2)", compatible with
  Nova-3; the boolean `diarize` is deprecated) [PROVEN-docs] https://developers.deepgram.com/docs/diarization,
  with `utterances=true`, `smart_format=true`, `punctuate=true`. Diarization is "Included" in the price
  [PROVEN-docs] https://deepgram.com/pricing.
- Role labelling (speaker 0/1 → caller/agent) is one extra field in the 2C OpenAI QA pass (the model reads the
  first utterances: greeting, "thank you for calling" → agent). No separate LLM call.
- Retreaver records **joint stereo** (§3.1). If 2B proves the two channels are the two legs, use
  `multichannel=true` for Retreaver instead of diarization: exact roles, no guessing. Ringba format is
  [UNKNOWN] until §3.2 runs. 🙋 Q6

### 4.4 Storage

| Data | Store | Why |
|---|---|---|
| Jobs, transcripts (utterances: speaker, role, start, end, text, language), QA results (versioned JSON + typed columns for score, outcome, flags), caller facts (key, value, quote, ts), vertical packs and versions, feedback labels, recordings metadata, playback audit, alerts, alert events | **Postgres**, RLS on `tenant_id` like every other table | Relational, editable (QA re-runs, feedback, pack versions), small (~10k rows a month), RLS already proven |
| QA columns needed by Pings filters and Reports (score, outcome, flags, qualified, should-have-converted reason) | **ClickHouse** `call_qa` (ReplacingMergeTree by version), written by the QA step | Joins with `pings` at analytics speed, same pattern as `call_scores` |
| Audio | **Archive bucket only** (§4.8) | Never DB, never server disk |

The old ClickHouse `transcripts` (one text blob) and `call_scores` tables stay for the mock path and are not
used by v2. Retention: archived audio **24 months**, fixed (owner decision); transcripts, QA and facts have
**no automatic deletion** until the owner's data-retention decision.

### 4.5 Providers' data handling

- **Deepgram:** requests "participate in the Model Improvement Program (MIP) by default, and Deepgram retains
  them" ([PROVEN-docs] https://developers.deepgram.com/trust-security/your-data). Opt out with
  "`mip_opt_out=true` as a query parameter of all API requests"
  ([PROVEN-docs] https://developers.deepgram.com/docs/the-deepgram-model-improvement-partnership-program); with
  it, audio is "Retained only for the duration needed to process the request". **Calyx sends
  `mip_opt_out=true` on every request (enforced in code and tested)**, and the owner asks Deepgram for
  account-level enforcement. The price impact of opting out is [UNKNOWN] ("refer to our Docs for pricing
  impacts") 🙋 Q7.
- **OpenAI:** "data sent to the OpenAI API is not used to train or improve OpenAI models"; abuse-monitoring logs
  are "retained for up to 30 days" ([PROVEN-docs] https://developers.openai.com/api/docs/guides/your-data).
  "Responses are stored by default … set `store: false`"
  ([PROVEN-docs] https://developers.openai.com/api/docs/guides/migrate-to-responses). **Calyx sends
  `store: false` on every call.** Zero Data Retention needs "prior approval by OpenAI" (same your-data URL);
  recommended to apply for, because transcripts are call content (they are already redacted). 🙋 Q8
- Only redacted transcript text goes to OpenAI; never audio, phone numbers, recording links or caller names
  beyond what was said.

### 4.6 Ask AI method: tool-calling (recommended), not text-to-SQL

- OpenAI **Responses API** ("Responses is recommended for all new projects", [PROVEN-docs]
  https://developers.openai.com/api/docs/guides/migrate-to-responses) with **strict function calling**
  ("We recommend always enabling strict mode", [PROVEN-docs]
  https://developers.openai.com/api/docs/guides/function-calling).
- About 6 fixed tools that wrap the **existing, tested analytics queries in the API** (the same dedup view as
  the dashboard): `metrics(group_by, from, to, filters)`, `compare_periods(...)`, `top(n, metric, group_by, …)`,
  `trend(metric, by_day, …)`, `campaigns_changed(days, threshold)`, `list_dimensions(kind)`. Dimensions:
  platform, campaign, publisher, buyer, state, day; Eastern time by default.
- The model only chooses tools and arguments (validated by zod); **the API runs the queries with the tenant
  from the JWT under RLS**; the answer must cite the tool results, which the UI shows as the "numbers used"
  table. No free SQL, so the current template/validator approach (§1) is retired.
- Lives in the **NestJS API** (it already owns analytics, auth, RLS and CSRF), so the Go `ask-ai` service stays
  off. Campaign or publisher names inside tool results are passed as data, never as instructions (tested with
  an injection string in a campaign name).

### 4.7 Keys

Server env, as the owner prefers: `DEEPGRAM_API_KEY`, `OPENAI_API_KEY`, `OPENAI_MODEL`, archive credentials.
Added to `assertProductionSecrets` in `services/api/src/config/env.ts:95-119` and to a
`refuseProductionPlaceholders()` in the scoring service (copied from `platform-sync/internal/config/config.go:92-114`),
passed through `deploy/docker-compose.yml` as `${VAR:-}` (today none is passed, §1). Ringba Recording Access
credentials are **per integration**, encrypted like the API key (same `CREDENTIAL_ENCRYPTION_KEY` blob),
entered only in Settings and never shown again. `providerconfig` per-tenant bindings are not used.

### 4.8 Recording archive

- **Provider:** Backblaze B2 (cheapest at our size): "$6.95 / TB / mo", first 10 GB "always free", egress free
  up to 3× storage ([PROVEN-docs] https://www.backblaze.com/cloud-storage/pricing). Application key **limited to
  the one bucket** ([PROVEN-docs] https://www.backblaze.com/docs/cloud-storage-application-keys) with
  listFiles/readFiles/writeFiles/deleteFiles. **SSE-B2** (AES-256) set as the bucket default explicitly
  ([PROVEN-docs] https://www.backblaze.com/docs/cloud-storage-server-side-encryption). R2 is the fallback:
  $0.015/GB-month, free egress ([PROVEN-docs] https://developers.cloudflare.com/r2/pricing/).
- **Layout:** `recordings/{tenant_id}/{platform}/{yyyy}/{mm}/{sha256}.mp3`. Content-addressed, so a re-upload
  is a no-op; no phone number or call id in the key.
- **Client:** none exists in the repo (§1). Add the AWS S3 SDK (S3-compatible endpoint): `aws-sdk-go-v2/service/s3`
  in the worker (upload, delete) and `@aws-sdk/client-s3` in the API (streamed `GetObject` for playback and the
  evidence pack).
- **Lifecycle:** a B2 lifecycle rule on the `recordings/` prefix (hide after `RECORDING_RETENTION_MONTHS`, delete
  1 day later, [PROVEN-docs] https://www.backblaze.com/docs/cloud-storage-lifecycle-rules) **and** a daily
  Calyx cleanup job that deletes objects past retention and marks the DB row `expired` (transcript and QA
  stay).
- **Tables:** `call_recordings` (call, sha256, bytes, duration, content type, object key, downloaded_at UTC,
  Deepgram request id and model, transcript version, state `archived | not_archived | expired`) and
  `recording_access_audit` (append-only, immutable trigger like `writeback_audit`: who, when, call, `play |
  download | evidence_pack`).
- Audio contains what callers said out loud (card numbers are redacted only in the **text**): playback is
  admin-only through the API proxy, never a public or signed URL in the page.

### 4.9 Cost

See §6: about **$110–170 a month** at today's volume with Deepgram redaction on, inside the ~$200 budget, with
a recommended daily minutes cap of **800**.

## 5. Workflow facts (for 2D) — docs and code only, no writes

### 5.1 Ringba publisher pause

| Question | Answer |
|---|---|
| Documented API? | **Yes, but only by example.** `PATCH https://api.ringba.com/v2/{accountId}/Publishers/{publisherId}`; example body includes `"enabled": true`; "This request can be used to modify any of the settings described in the Request Body section of the 'POST Create New Publisher' request". `GET` single publisher returns `"enabled": true` [PROVEN-docs] https://developers.ringba.com/ (Publishers › Edit a Publisher). |
| Does `enabled:false` equal the UI pause? | **[UNKNOWN].** `enabled` is not described in the Create Publisher parameter table. The UI behaviour is documented: "When you use the pause button on the Publisher list screen, Ringba pauses the publisher across all campaigns" [PROVEN-docs] https://support.ringba.com/hc/en-us/articles/17993410144663. Ringba states "All functions found within the Ringba portal are also available programmatically through our API" [PROVEN-docs] https://developers.ringba.com/. To be proven on the 2D sandbox publisher (read → PATCH `enabled:false` → read `enabled` → re-enable). No private web-app calls were inspected. |
| Read-back | `GET /v2/{accountId}/Publishers/{publisherId}` returns `enabled` [PROVEN-docs] → read-back is possible. |
| Token permission | "There are 3 user roles: Admin, Publisher, and Buyer … a 403 response means your current role does not have permission" [PROVEN-docs] https://developers.ringba.com/. **No per-token scopes are documented**: a token acts as the user who created it [INFERRED]. Editing publishers is almost certainly Admin [INFERRED]. Whether the **server's** Ringba token is an Admin token is [UNKNOWN] 🙋 Q9 (check in Ringba → Security → API Access Tokens who created it). A read-only token is not possible, so the Live sync token can write. |
| Per-campaign pause | **RTB campaigns:** `PATCH .../campaigns/{campaignId}/rtb/publishers/{publisherId}` `{"enabled": false}` — "If set to false, RTB will be disabled for this publisher" [PROVEN-docs]. **Standard campaigns:** pause the publisher's **numbers**, `PATCH .../numbers/{numberId}` `{"enabled": false}` — "calls to the number will be paused but the number will remain associated with your account" [PROVEN-docs]. Both per the same help article (per-campaign routes). Source: https://developers.ringba.com/ |

### 5.2 Retreaver affiliate pause

| Question | Answer |
|---|---|
| Endpoint | `PUT https://api.retreaver.com/api/v1/affiliates/{id}.json` (also `.../affiliates/afid/{client_afid}.json`) with `{"affiliate":{"paused":true}}` — "To pause an affiliate so it is excluded from active routing"; "Paused affiliates are excluded from active routing" [PROVEN-docs] https://learn.retreaver.com/api/affiliates |
| Does GET return `paused`? | **No, not in the documented response** (id, afid, first_name, last_name, company_name, alternative_id, created_at, updated_at) [PROVEN-docs] same URL. **Read-back is therefore [UNKNOWN]**: the 2D sandbox plan must first GET the test affiliate and see whether an undocumented `paused` key appears; if not, Calyx can only confirm the PUT's own response and track state itself. 🙋 Q10 |
| Per-campaign pause | **None documented** [PROVEN-docs] (no campaign-affiliate endpoint; Numbers have no paused field, https://learn.retreaver.com/api/numbers). A Retreaver pause is always global for that affiliate. |
| Permission | Not documented (the key is company-wide). The Stage 4b PUT on calls needed recording access; whether affiliate PUT needs another permission is [UNKNOWN]. |

### 5.3 Do we store the publisher per call?

[PROVEN] (`services/platform-sync/internal/models/models.go`, adapters, ClickHouse `pings`):

| Field | Ringba | Retreaver | ClickHouse |
|---|---|---|---|
| Publisher id | `publisherId` (`ringba_adapter.go:244,326`) | `afid`, else `client_afid` (`retreaver_adapter.go:343-344,383-386`); `system_affiliate_id` is **not** read | `publisher_id` |
| Publisher name | `publisherName` (`:245,362`) | `affiliate_name` (`:363,469`) | `publisher_name` |
| Campaign id / name | `campaignId` / `campaignName` | `system_campaign_id` / **name not mapped** | `campaign_id` / `campaign_name` |
| Buyer | `buyerId` / `buyer` | **not mapped** | `buyer_id` / `buyer_name` |

So an alert maps to the exact entity: Ringba `publisherId` → `PATCH /Publishers/{publisherId}`; Retreaver
`afid` → `PUT /affiliates/afid/{afid}.json` (the documented alternate form) [INFERRED]. Retreaver campaign name
and buyer are missing, which matters for Ask AI and per-campaign rules; mapping them is a sync change 🙋 Q4.

### 5.4 Alert engine and email (facts in §1)

Today: event-driven on `call_scored`, stdout only, no scheduler, no store, no email. For 2D: a scheduled
evaluator every 15 minutes in the API (no scheduler library is installed; add `@nestjs/schedule` or a guarded
`setInterval` with a Postgres advisory lock so only one API instance evaluates), `alert_events` table for the
bell, and an SMTP sender (`nodemailer`) with `SMTP_HOST`, `SMTP_PORT`, `SMTP_USER`, `SMTP_PASSWORD`,
`SMTP_FROM`, `ALERT_EMAIL_TO` (secret refused in production like the others). 🙋 Q11: which mailbox/provider.

## 6. Monthly cost model (~8,200 calls/month, ~35% converted)

Assumptions [INFERRED, to be replaced by the server numbers after §2's query]: converted 2,870 calls;
should-have-converted another ~10% of calls (820); **~90% of those recorded**; average recorded length of these
calls **~6 minutes** (Retreaver overview showed an average call of 5:10 across all calls; converted calls run
longer); on-demand ~300 calls.

| Item | Volume | Unit price | Per month |
|---|---|---|---|
| Deepgram Nova-3 (mono), auto | ~3,330 calls × 6 min ≈ **20,000 min** | $0.0043/min [PROVEN-docs] | $86 |
| Deepgram redaction | 20,000 min | $0.0020/min [PROVEN-docs] | $40 |
| Deepgram on demand + Spanish uplift | ~2,000 min | $0.0063–0.0072/min | $14 |
| OpenAI QA, `gpt-5-mini` (default) | ~3,600 calls × (~4k input + ~0.8k output tokens) | $0.25 / $2.00 per 1M [PROVEN-docs] | $10 |
| OpenAI Ask AI | ~1,500 questions × (~6k in + ~0.5k out) | same | $4 |
| B2 archive | ~0.5 MB/min → ~11 GB added a month; ~130 GB after a year, ~265 GB at 24 months | $6.95/TB, 10 GB free | $1–2 |
| **Total** | | | **≈ $155 / month** (≈ $115 without Deepgram redaction) |

- With `gpt-5-nano` ($0.05/$0.40) OpenAI drops to ~$2; the accuracy test (2C.6) decides. Batch tier halves it
  for backfills.
- **Recommended caps:** `TRANSCRIBE_DAILY_MINUTES_CAP=800` (≈ $150/month worst case),
  `ASK_AI_TOKEN_BUDGET_DAILY=300000`, `QA_TOKEN_BUDGET_DAILY=1500000`. The Providers tab shows the month-to-date
  estimate.
- Free credit: "Every new account receives $200 in free credit" at Deepgram [PROVEN-docs] https://deepgram.com/pricing.

## 7. Open questions for the owner 🙋

1. **Ringba recording test:** create Recording Access credential `calyx-server` and agree to run the §3.2 probe
   on the server (no full audio, nothing stored).
2. **Ringba plan** (Business or Professional) for the research doc's recording-cost note.
3. **Recording share:** Aryan runs the §2 query on the server so the cost model uses real numbers.
4. **Sync changes** (the spec freezes the Live sync): approve small, additive mappings — Retreaver
   `recording_url` presence → `has_recording`, Retreaver `campaign_name`, `buyer` — or keep the sync frozen and
   fetch per call at transcription time (works, costs one GET per transcribed call).
5. **Should-have-converted** volume: today's rule (`revenue > 0 OR duration ≥ required`) may be large; confirm it
   should be auto-transcribed, or only `revenue > 0`.
6. **Retreaver recordings:** ask support whether joint-stereo files are two legs (caller/agent), the retention
   period, and confirm the links are public by design.
7. **Deepgram:** account-level MIP opt-out and its price impact.
8. **OpenAI:** apply for Zero Data Retention / Modified Abuse Monitoring?
9. **Ringba token role:** is the server's Ringba API token an Admin token? Publisher pause needs it; there are no
   scoped tokens.
10. **Retreaver pause read-back:** accept "PUT response only" if the sandbox GET shows no `paused` field?
11. **Email for alerts:** which SMTP provider / sender address / recipient list.
12. **Call summary:** the meeting notes asked for a short summary per call. Add a 2–3 sentence `summary` field to
    the 2C QA JSON (no extra API call, ~+100 output tokens)?

## 8. Gates and safety in this phase

- Product code: **unchanged**. Added: docs and one read-only probe (`go vet` clean, not in any image).
- Requests to vendors: Retreaver 2 GETs + 1 HEAD + 2 Range GETs (14 bytes). Ringba: none. Docs pages: public
  only; no authenticated calls, no private endpoints.
- Laptop platform-sync stayed **stopped**; `WRITEBACK_ENABLED`, the Live sync and the write-back code untouched.
- Full `pwsh-preview -V1` gates were not run: they test the running laptop stack, which is built from older
  code and needs laptop platform-sync running (forbidden). No product code changed, so they cannot regress;
  they run on the server with Phase 2A.
- Secret scan: every commit scanned (key-shaped strings, S3 object paths, phone numbers). Clean.

**STOP.** Waiting for the owner's written approval before Phase 2A.
