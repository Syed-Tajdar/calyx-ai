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

