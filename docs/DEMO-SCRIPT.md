# Demo Script — 10 Minutes

**Prerequisites:** `make dev-up` running, `make demo-seed` completed  
**URL:** http://localhost:3001  
**Login:** `admin@calyx.ai` / `calyx_dev`

---

## 0:00 — Login & first impression (1 min)

- Open login page — note brand hero, dark theme default
- Log in → land on **Dashboard**
- Point out: total pings, revenue, platform chart, performance table

> "This is live data from our ingestion pipeline — not screenshots."

---

## 0:01 — Live ping feed (2 min)

- Navigate to **Pings**
- Show **Live** badge and date filters
- Optional: run `make smoke-test` in terminal to show a new row appear within 8 seconds
- Click **Export CSV** — server-side download

---

## 0:03 — Intelligence hub (2 min)

- Open **Intelligence** from sidebar
- **Ask AI:** type *"What is my total revenue?"* → show formatted answer
- **Compliance:** risk ring + exposure summary
- **Attribution:** platform status grid (mock fires)

---

## 0:05 — Call quality (1.5 min)

- **Scoring** → pick a scored call
- Open **Call detail** (`/calls/[id]`) — native deep view: ping + score + compliance + attribution in one panel
- Show transcript panel (mock text in demo mode)

---

## 0:06 — Financial & coverage (1.5 min)

- **Reports** → CPA, RPC, ROI, P&L cards
- **Coverage** → geographic heatmap of ping demand
- Toggle **light theme** in top bar (theme persistence)

---

## 0:08 — Settings & integrations (1 min)

- **Settings** → Integrations list (Ringba, TrackDrive, etc.)
- Provider bindings — show catalog + tenant binding UI
- Alert rules section

---

## 0:09 — Architecture close (1 min)

> "Everything you saw runs on our production architecture: Go ingestion, Kafka, ClickHouse, multi-tenant API. We're in MOCK_MODE for external APIs — flipping one env flag activates Deepgram, OpenAI, and real ad platforms with **no UI changes**."

Show README phase diagram or `docs/02-product-overview.md` module map.

---

## Q&A backup commands

```powershell
make verify-presentation   # API health + all Phase 3 endpoints
make step1b-test           # Go unit tests
powershell -File scripts/smoke-test.ps1
```

**Tenant UUID (dev):** `11111111-1111-1111-1111-111111111111`
