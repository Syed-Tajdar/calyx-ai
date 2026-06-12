# Call Loom Intelligence Platform

Pay-per-call analytics and intelligence — Kaliper competitor, part of the Call Loom ecosystem.

## Phase 1 — Complete

End-to-end pipeline:

```
Ringba Pixel → Ingestion (Go) → Kafka → Consumer (Go) → ClickHouse
                                              ↓
                                    API (NestJS) → Web (Next.js)
```

### Quick start (local)

```bash
# Prerequisites: Docker Desktop, Go 1.22+, Node.js 20+
cp .env.example .env
make dev-up
```

| Service   | URL |
|-----------|-----|
| Ingestion | http://localhost:8080/healthz |
| Consumer  | http://localhost:8081/healthz |
| API       | http://localhost:3000/healthz |
| Web       | http://localhost:3001 |

**Dev login:** `admin@callloom.dev` / `callloom_dev`

### Ringba pixel setup

1. In Ringba: **Integrations → Pixels → Add Pixel**
2. Event: **Incoming** (or Completed)
3. URL (GET or POST JSON):

```
http://localhost:8080/webhooks/ringba/dev_ringba_token_change_me?CallID=[Call:InboundCallId]&caller_id=[tag:InboundNumber:Number]&state=[tag:Geo:State]&event=incoming&call_revenue=[Call:ConversionPayout]
```

4. Enable **POST Pixel** with JSON if preferred — same field names supported.

### Smoke test

```powershell
powershell -File scripts/smoke-test.ps1
```

### Architecture

- **Ingestion:** Ringba parser, Redis dedup, Kafka producer, AES-256-GCM ANI encryption
- **Consumer:** Batched ClickHouse inserts (min 500 rows or 5s flush)
- **API:** JWT auth, tenant-scoped ClickHouse queries
- **Web:** Login + ping dashboard

See `.cursor/rules/` for full architecture rules.
