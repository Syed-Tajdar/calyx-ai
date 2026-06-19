# 02 — Product Overview

## User personas

| Persona | Primary screens | Goal |
|---------|-----------------|------|
| Media buyer | Dashboard, Pings, Reports | ROI, volume, platform mix |
| Compliance officer | Compliance, Pings | TCPA exposure, risk scores |
| Ops manager | Settings, Integrations | Webhooks, alerts, provider bindings |
| Analyst | Ask AI, Reports, Coverage | Ad-hoc questions, geo demand |

## Module map

```
┌─────────────────────────────────────────────────────────────┐
│                        Calyx Web (Next.js)                   │
├──────────┬──────────┬──────────┬──────────┬──────────┬────────┤
│ Dashboard│  Pings   │ Scoring  │Intel hub │ Reports  │Settings│
│          │  + Live  │+ Calls   │Ask/Compl │+ Coverage│+ Alerts│
│          │  export  │ detail   │/Attrib   │+ P&L     │+ Providers│
└────┬─────┴────┬─────┴────┬─────┴────┬─────┴────┬─────┴────┬───┘
     │          │          │          │          │          │
     └──────────┴──────────┴──── API (NestJS) ──┴──────────┘
                              │
     ┌────────────────────────┼────────────────────────┐
     │                        │                        │
 Ingestion              ClickHouse                 PostgreSQL
 (Go webhooks)          (analytics)                (auth, config)
     │
   Kafka → Consumer, Scoring, Compliance, Attribution, Enrichment, Ask AI
```

## Key user journeys

### 1. Ping arrives → dashboard updates

Ringba pixel → Go ingestion → Kafka → ClickHouse → API → Web poll (8s) with Live badge.

### 2. Call completes → quality score

Mock emitter (or Deepgram in prod) → `call_transcript_ready` → Scoring service → score + transcript on `/scoring` and `/calls/[id]`.

### 3. Compliance check

Compliance consumer scores ping against DNC / state rules → risk ring on Compliance page → exposure dollar summary.

### 4. Ask a question

User types natural language → Ask AI resolves intent → safe read-only ClickHouse query → formatted answer + history.

### 5. Attribution

Conversion event → mock Meta/Google fire → status grid on Attribution page.

## Web routes (16)

| Route | Module |
|-------|--------|
| `/login` | Auth |
| `/dashboard` | Overview metrics + charts |
| `/pings` | Ping table, filters, CSV export |
| `/scoring` | Call scores list |
| `/calls/[callId]` | Native deep view |
| `/intelligence` | Hub |
| `/ask` | Ask AI chat |
| `/compliance` | Risk + exposure |
| `/attribution` | Conversion log |
| `/reports` | Financial analytics |
| `/coverage` | Geo heatmap |
| `/settings` | Integrations, alerts, providers |

## MOCK_MODE behavior

When `MOCK_MODE=true` (default):

- Transcription uses synthetic transcripts
- AI scoring uses deterministic mock scores
- DNC / enrichment return fixture attributes
- Ad platforms log mock HTTP fires (no real spend)

When `MOCK_MODE=false`:

- Services require real API keys at boot (fail fast)
- Same UI and API contracts — provider adapters swap in

## Demo data

Run `make demo-seed` after `make dev-up` to inject 25 signed pings across US states for a populated dashboard.

See [DEMO-SCRIPT.md](DEMO-SCRIPT.md) for the live presentation flow.
