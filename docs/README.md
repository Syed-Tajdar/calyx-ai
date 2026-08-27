# Calyx AI — Documentation Index

Management and engineering documentation for the Calyx AI presentation package.

## Executive & product

| Document | Audience | Purpose |
|----------|----------|---------|
| [01 — Executive Summary](01-executive-summary.md) | Leadership | One-page business case and approval ask |
| [02 — Product Overview](02-product-overview.md) | Product / Ops | Modules, user journeys, demo flow |
| [Design System v2](design-system-v2.md) | Design / Engineering | UI tokens, components, routes |
| [10 — Known Gaps & Roadmap](10-known-gaps-and-roadmap.md) | Engineering / Leadership | What is mock vs real, post-approval plan |
| [Enterprise roadmap](enterprise-roadmap.md) | Engineering | Deferred gaps (quota, rotation, recording, write-back gates) |
| [Demo Script](DEMO-SCRIPT.md) | Presenter | 10-minute live demo walkthrough |

## Platform certification

| Document | Status |
|----------|--------|
| [platform-certification.md](platform-certification.md) | Retreaver **YES** · Ringba **YES** |
| [phase2-ringba-live-certified.md](phase2-ringba-live-certified.md) | Ringba LIVE cert (D8/D9/D10/D5/D8b) |
| [retreaver-final-certification-report.md](retreaver-final-certification-report.md) | Retreaver FINAL cert |

## Ringba parity (Part 2)

| Document | Status |
|----------|--------|
| [ringba-parity-a-b1-report.md](ringba-parity-a-b1-report.md) | **PASS** — Bucket A + B1 |
| [ringba-parity-delta-report.md](ringba-parity-delta-report.md) | **PASS** — P2D1–P2D5 |
| [call-detail-parity-ringba-audit.md](call-detail-parity-ringba-audit.md) | Audit trail |
| [ui-credibility-fixes-report.md](ui-credibility-fixes-report.md) | C1–C3 credibility fixes |

## Ringba write-back (research only — not shipped)

| Document | Status |
|----------|--------|
| [ringba-writeback-probe-report.md](ringba-writeback-probe-report.md) | **Done** — read paths, columns, fake-ID probes |
| [ringba-writeback-sandbox-report.md](ringba-writeback-sandbox-report.md) | **Done** — payments/void/annotate/webhook CRUD; convert loop **open** |
| `ringba-writeback-phase-g-report.md` | **Pending** — needs clean sandbox call from operator |

Evidence dirs: `docs/evidence/ringba-writeback-probe/` · `docs/evidence/ringba-writeback-sandbox/` · `docs/evidence/ringba-parity-a-b1/` · `docs/evidence/ringba-parity-delta/`

## API sync & ingestion

| Document | Purpose |
|----------|---------|
| [ingestion-slice-report.md](ingestion-slice-report.md) | Phase 0 real ingestion |
| [apisync-ringba-report.md](apisync-ringba-report.md) | Ringba API sync |
| [apisync-retreaver-report.md](apisync-retreaver-report.md) | Retreaver API sync |
| [mode-strategy-report.md](mode-strategy-report.md) | Test / Live per integration |
| [redis-maxmemory-report.md](redis-maxmemory-report.md) | Redis ops hardening |

## Coverage & UI

| Document | Purpose |
|----------|---------|
| [north-america-coverage-report.md](north-america-coverage-report.md) | US + Canada normalization |
| [coverage-north-america-ui-report.md](coverage-north-america-ui-report.md) | NA map UI |
| [feed-coverage-count-report.md](feed-coverage-count-report.md) | Distinct-calls headline |
| [ui-backend-coverage.md](ui-backend-coverage.md) | UI ↔ API matrix |

---

**Repository:** [github.com/Syed-Tajdar/calyx-ai](https://github.com/Syed-Tajdar/calyx-ai)

**Demo credentials:** `admin@calyx.ai` / `calyx_dev`
