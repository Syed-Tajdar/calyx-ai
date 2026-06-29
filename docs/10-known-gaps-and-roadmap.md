# 10 — Known Gaps & Roadmap

## Presentation build scope (current)

This repository is **presentation-ready** with `MOCK_MODE=true`. The following are intentionally mock or simplified:

| Area | Current state | Post-approval work |
|------|---------------|-------------------|
| Transcription | Mock emitter / fixture text | Deepgram live + callback HMAC |
| AI scoring | Deterministic mock | OpenAI GPT-4 scoring prompts |
| DNC / TCPA lists | Rule-based mock | Real DNC vendor API |
| Enrichment | Fixture attributes | EnrollHere + Whitepages |
| Ad attribution | Mock HTTP log | Meta CAPI, Google Ads, TikTok |
| Ringba / TrackDrive | Dev HMAC token only | Per-tenant production webhooks |
| Elasticsearch ping search | ClickHouse list API | ES cluster + sync job |
| Kong gateway / rate limits | Direct local ports | EKS + Kong Terraform |
| Secrets rotation | `.env` local | AWS Secrets Manager + 30-day rotation |
| Mobile app | Responsive web only | Optional React Native shell |

## Technical debt (non-blocking for demo)

- Storybook covers key components; not 100% coverage
- Some analytics queries unbounded — production needs cursor pagination caps
- Ask AI SQL whitelist needs expansion per tenant schema evolution
- Light theme polish ongoing on edge-case components

## Roadmap phases

### Phase 4 — Production integrations (post-approval)

- `MOCK_MODE=false` provider wiring
- AWS EKS deploy (Terraform)
- Kong gateway + WAF
- First production tenant onboarding

### Phase 5 — Scale & search

- Elasticsearch ping search
- Redis cluster (session + provider cache at scale)
- ClickHouse replication

### Phase 6 — Ecosystem

- Partner native SSO
- White-label tenant branding
- Billing meter (per-ping / per-enrichment)

## What does NOT need rework after approval

- Next.js UI routes and design system
- NestJS API module structure
- Go ingestion hot path
- Kafka topic layout
- ClickHouse schema (tenant_id from day one)
- Provider config abstraction

## Success criteria for go-live

1. `make test` + `make verify-presentation` pass in CI
2. One real Ringba tenant ingesting >1k pings/day
3. Deepgram callback verified end-to-end
4. Compliance block rate measurable in Grafana
5. Zero plaintext ANI in logs (AES-256-GCM + hash only)
