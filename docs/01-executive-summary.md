# 01 — Executive Summary

## What is Calyx AI?

Calyx AI is a **pay-per-call analytics and intelligence platform** — a Kaliper-class competitor built for the Call Loom ecosystem. It unifies ping intelligence, call quality AI, financial analytics, TCPA compliance, audience enrichment, and ad attribution in one multi-tenant product.

## Why now?

- **Market gap:** Competitors offer fragments (ping tools OR scoring OR compliance). Calyx combines all layers with a single call story.
- **Unique moat:** TCPA compliance intelligence tied to every ping — no competitor ships this at the data layer.
- **Speed to revenue:** Architecture is production-shaped; current build runs in **MOCK_MODE** for zero external API cost during evaluation.

## What is ready today (presentation build)

| Capability | Status |
|------------|--------|
| Real-time ping ingestion (Ringba webhook, HMAC) | ✅ Live |
| Dashboard + ping feed + export | ✅ Live |
| Call scoring + transcripts | ✅ Mock pipeline |
| Audience enrichment | ✅ Mock data |
| Financial analytics (CPA, RPC, P&L) | ✅ Live (ClickHouse) |
| Coverage heatmaps | ✅ Live |
| TCPA compliance risk | ✅ Mock + rules engine |
| Ask AI (natural language reporting) | ✅ Mock SQL + safe queries |
| Ad attribution (Meta, Google, etc.) | ✅ Mock fire log |
| Provider bindings UI | ✅ Live |
| Light/dark UI, mobile-responsive | ✅ Live |

## Investment ask

Approve **Phase 4 — Production integrations**:

1. Flip `MOCK_MODE=false` with real provider keys (Deepgram, OpenAI, DNC vendors, ad platforms).
2. Connect live Ringba / TrackDrive / CallGrid tenants.
3. EKS deployment + Kong gateway hardening (Terraform modules exist in roadmap).

**No UI rewrite required** — same screens, same API contracts; only backend provider adapters activate.

## Risk summary

| Risk | Mitigation |
|------|------------|
| TCPA liability | Compliance engine blocks high-risk pings; audit trail in ClickHouse |
| Multi-tenant data leak | JWT-only tenant_id + PostgreSQL RLS |
| Vendor lock-in | Provider config service abstracts all external APIs |

## Recommendation

**Approve presentation → fund production integration sprint (4–6 weeks)** to onboard first paying tenant on Call Loom infrastructure.
