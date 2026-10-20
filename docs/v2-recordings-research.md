# Calyx v2 — recordings research (Ringba + Retreaver), 2026-10-16

Input for the Chapter 2 spec (Deepgram transcription + OpenAI QA / Ask AI). All facts come from official vendor pages. "Unknown" items must be verified by Talha, read-only, or asked of vendor support.

## Ringba
- Recording cost is per minute, not per call:
  - Business $0.01/min
  - Professional $0.005/min
  - Enterprise custom
  - Calyx adds no recording cost; it only reads recordings that already exist.
- Native transcription:
  - $0.04/min (Business), $0.035/min (Professional).
  - "Summarization, sentiment analysis, conversion detection, and other services may incur additional costs."
  - It can auto-transcribe per campaign (switch "Automatically Transcribe Campaign"). Keep it OFF, so we don't pay twice.
- Retention: "Your call recordings are kept indefinitely as long as you have the Ringba account."
- Access:
  - Recording URLs require credentials. Use Basic Auth with **Recording Access credentials**, created under Security > Recording Access. These credentials can only download recordings: no portal, no API. Create a separate one for Calyx.
  - The URL in the call log is permanent, but it redirects. Send a GET with redirects disabled and read the `Location` header. That final URL "may be good for as little as 6 hours", so resolve it each time it is needed and never store it.
  - For the first 6 days after a call, anyone with the URL can open the recording without credentials. Treat recording URLs as sensitive: never log them, never show them raw in the UI.
- Coverage gap: recording starts only when the final target answers, so calls that are not answered have no recording.
- Format: mono or dual-channel is not documented. Assume mono and use Deepgram diarization (included free for pre-recorded audio).

## Retreaver
- API fields:
  - `recording_url`: "URL of the MP3 recording, when recording is enabled on the campaign" (v1–v4).
  - v5 adds `inbound_leg_recording_url`, the inbound leg only, when it exists. This could later help separate the caller from the agent.
- Settings that create gaps:
  - Campaign "Max recording duration" can cut recordings short.
  - Buyer "Block Recording" means no recording for calls routed to that buyer.
- Native AI Analyst:
  - Transcription plus custom prompts (disposition, intent).
  - $0.05/min, Superuser only, results go out through a webhook.
  - Keep it OFF for Calyx calls.
- Unknown, to verify or ask support:
  - recording retention
  - whether `recording_url` needs auth
  - pricing for recording
  - dual-channel

## Implications for the v2 spec
1. Cost:
   - Deepgram Nova-3 at $0.0043/min is about 8–12x cheaper than Ringba's ($0.035–0.04) or Retreaver's ($0.05) native transcription. Calyx's value is unified QA across both platforms, plus should-have-converted dispute evidence.
   - The ~$200/mo cap stands.
2. New integration fields:
   - Ringba integration gets optional Recording Access username and password, stored encrypted like the API key, entered only in Settings.
   - Retreaver uses the existing key if its URL needs auth; Talha verifies.
3. Pipeline:
   - Resolve the URL at transcription time (Ringba: no-redirect + Location).
   - Fetch the audio once.
   - Send it to Deepgram, and archive the same bytes in a private bucket (owner decision, 24 months, for dispute evidence).
   - Never store the short-lived URL.
   - Ringba charges for recording by call length at record time; its docs mention no fee for downloading or playback.
   - Transcribe soon after the call (Ringba keeps recordings forever; Retreaver retention is unknown).
4. "No recording" is a normal state, not an error:
   - not answered (Ringba)
   - Block Recording or max duration (Retreaver)
   - recording disabled on the campaign (e.g. Ringba Calyx-Sandbox)
   - QA shows "No recording" and skips audio-based checks.
5. Speakers: diarization plus an LLM pass to label caller and agent. Retreaver's inbound leg is a later improvement.
6. Playback and evidence:
   - Admin-only playback from our archive through an API proxy (auth, no raw URL), with an audit log.
   - A "dispute evidence pack": audio + transcript + QA quotes + SHA-256 hash.
   - Archive storage: Backblaze B2 ($6.95/TB/month, first 10 GB free) or Cloudflare R2 ($0.015/GB-month, free egress). About 8–30 GB a month.
7. Owner actions:
   - Create a Ringba Recording Access credential "calyx-server".
   - Confirm the Ringba plan (Business or Professional).
   - Ask Retreaver support about retention, URL auth and dual-channel.
   - Keep native transcription/AI Analyst off on both platforms.

## Sources
- https://www.ringba.com/pricing/
- https://support.ringba.com/hc/en-us/articles/42723133630487 (recordings FAQ)
- https://support.ringba.com/hc/en-us/articles/38047032942103 (final recording URL)
- https://support.ringba.com/hc/en-us/articles/42526465466647 (recording access credentials)
- https://support.ringba.com/hc/en-us/articles/24289787424023 (transcription)
- https://learn.retreaver.com/api/calls
- https://learn.retreaver.com/guides/configuring-call-recording-settings
- https://learn.retreaver.com/guides/ai-analyst-turn-calls-into-insights
- https://learn.retreaver.com/guides/pricing
- https://deepgram.com/pricing
- https://support.ringba.com/hc/en-us/articles/29804960197527 (recording billing: "based on the length of the call and your license plan")
- https://www.backblaze.com/cloud-storage/pricing
- https://developers.cloudflare.com/r2/pricing/
