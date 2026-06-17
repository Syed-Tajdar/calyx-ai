# Calyx AI

Pay-per-call analytics and intelligence platform — Kaliper competitor.

## Phase 1 — Complete

End-to-end pipeline:

```
Ringba Pixel → Ingestion (Go) → Kafka → Consumer (Go) → ClickHouse
                                              ↓
                                    API (NestJS) → Web (Next.js)
```

## Phase 2 — Mock-first intelligence (MOCK_MODE=true)

```
Ping → Enrichment (Go) → ClickHouse attrs        [pre-bidding]
Ping → Mock emitter → call_transcript_ready → Scoring (Go) → scores + transcripts
Deepgram callback → API → Kafka → Scoring consumer
call_scored → Alert engine (API) → mock notifications
```

### Quick start (local)

```bash
cp .env.example .env
make dev-up
```

| Service    | URL |
|------------|-----|
| Ingestion  | http://localhost:8080/healthz |
| Consumer   | http://localhost:8081/healthz |
| Scoring    | http://localhost:8082/healthz |
| Enrichment | http://localhost:8083/healthz |
| Provider config | http://localhost:8084/healthz |
| Compliance    | http://localhost:8085/healthz |
| Ask AI        | http://localhost:8086/healthz |
| Ad attribution | http://localhost:8087/healthz |
| API        | http://localhost:3000/healthz |
| Web        | http://localhost:3001 |

**Dev login:** `admin@callloom.dev` / `callloom_dev`

### MOCK_MODE switch

Default is `MOCK_MODE=true` — no external API keys required.

To go live later:

```env
MOCK_MODE=false
TRANSCRIPTION_PROVIDER=deepgram
AI_PROVIDER=openai
DEEPGRAM_API_KEY=...
DEEPGRAM_CALLBACK_SECRET=...
OPENAI_API_KEY=...
```

Services **fail fast on boot** if real mode secrets are missing (no silent mock fallback).

### Phase 2 API endpoints

| Endpoint | Description |
|----------|-------------|
| `GET /scoring/scores` | List call scores (tenant-scoped) |
| `GET /scoring/transcript?callId=` | Get transcript |
| `POST /scoring/deepgram/callback` | Deepgram → Kafka publish |
| `GET /enrichment/:aniHash` | Enrichment attributes |
| `GET /coverage/heatmap` | Geo demand heatmap |
| `GET /providers/catalog` | Provider catalog (Step 1b) |
| `GET /providers/bindings` | Tenant provider bindings |
| `POST /providers/bindings` | Create binding (invalidates Redis cache) |
| `POST /providers/resolve` | Resolve provider for tenant+domain |
| `GET /compliance/risk-scores` | TCPA risk scores (ClickHouse) |
| `GET /compliance/violations` | Compliance violations |
| `GET /compliance/exposure` | Liability exposure summary |
| `POST /ask` | Natural language analytics question |
| `GET /ask/history` | Prior Ask AI questions |
| `GET /attribution/events` | Conversion fire log (ClickHouse) |
| `GET /attribution/status` | Per-platform fired/failed summary |
| `GET /native/call-deep-view?callId=` | Full call story (ping + score + compliance + attribution) |
| `GET /analytics/metrics` | CPA/RPC/ROI |
| `GET /analytics/pnl` | P&L summary |
| `GET /alerts/rules` | Alert rules CRUD |

### Ringba pixel setup

1. In Ringba: **Integrations → Pixels → Add Pixel**
2. Event: **Incoming** (or Completed)
3. URL (GET or POST JSON):

```
http://localhost:8080/webhooks/ringba/dev_ringba_token_change_me?CallID=[Call:InboundCallId]&caller_id=[tag:InboundNumber:Number]&state=[tag:Geo:State]&event=incoming&call_revenue=[Call:ConversionPayout]
```

4. **HMAC required:** every request must include `X-Ringba-Signature`. Dev secret: `dev_ringba_hmac_secret_change_me`

### Smoke test

```powershell
powershell -ExecutionPolicy Bypass -File scripts/smoke-test.ps1
make step1b-test
```

See `.cursor/rules/` for full architecture rules.
