# Calyx AI — Design System v2

Version 2.0 · Teal + Indigo hybrid brand · Claude Design aligned · Production UI

Canonical token source: [`web/src/styles/tokens/`](../web/src/styles/tokens/) (imported from Claude Design ZIP).

---

## Brand palette (hybrid)

| Token | Dark | Light | Usage |
|-------|------|-------|--------|
| `calyx-bg` | `#060B14` | `#F9FAFB` | Page background |
| `calyx-surface` | `#0D1421` | `#FFFFFF` | Sidebar, elevated panels |
| `calyx-card` | `#111827` | `#FFFFFF` | Cards, tables |
| `calyx-border` | `#1F2937` | `#E5E7EB` | Borders |
| `calyx-text` | `#F9FAFB` | `#111827` | Primary text |
| `calyx-muted` | `#9CA3AF` | `#6B7280` | Secondary text |
| `calyx-accent` | `#6366F1` | `#6366F1` | CTAs, links, active nav (indigo) |
| `calyx-teal` | `#00C9B7` | `#00A896` | Logo, intelligence highlights |
| `calyx-success` | `#10B981` | `#059669` | Positive metrics |
| `calyx-warning` | `#F59E0B` | `#D97706` | Medium compliance risk |
| `calyx-danger` | `#EF4444` | `#DC2626` | High risk, violations |

**Rule:** Indigo = actions/navigation. Teal = Calyx identity + intelligence data.

Tokens are CSS variables in `web/src/app/globals.css` — toggle via `.dark` / `.light` on `<html>`.

---

## Unique Calyx patterns

### Call Journey Timeline

Horizontal stepper: Ping → Enrich → Score → Compliance → Attribution.  
Component: `web/src/components/calyx/CallJourneyTimeline.tsx`

### Compliance Risk Ring

Circular gauge for risk score 0–100 + exposure USD.  
Component: `web/src/components/calyx/ComplianceRiskRing.tsx`

### Intelligence Command Card

Feature entry cards with teal accent bar + lucide icon.  
Component: `web/src/components/calyx/IntelligenceCommandCard.tsx`

---

## App shell

- **Desktop (lg+):** Fixed sidebar 240px + top bar + full-width main
- **Mobile:** Top bar + main + bottom nav
- **Tenant context:** `GET /auth/me` → tenant name in TopBar dropdown
- **Global search:** Call ID search in TopBar (desktop)
- **Theme:** Dark default; light mode toggle in TopBar

---

## Phase 1 — Complete

| Screen | Route | Status |
|--------|-------|--------|
| Login | `/login` | Redesigned |
| Dashboard | `/dashboard` | Redesigned (KPIs + compliance exposure) |
| Intelligence hub | `/intelligence` | Redesigned |

---

## Phase 2 — Complete

- Full redesign: Ask, Compliance, Attribution, Reports, Settings, Pings, Scoring
- Coverage heatmap page + `CoverageModule` API
- Provider bindings UI in Settings
- Global search, date range filters (client-side)
- `GET /auth/me` for tenant display name
- Shared primitives: `IntelligenceSubNav`, `DataTable`, `FilterBar`

---

## Phase 3 — Complete

- API-backed date/platform/search filters on pings + analytics
- Live ping feed (8s polling + row highlight)
- Server-side CSV export (`GET /pings/export`)
- Light mode + theme toggle
- Storybook component catalog (`npm run storybook`)
- `SubsectionHeader` replaces legacy `SectionHeader` / `Icons.tsx`

---

## Storybook

```bash
cd web && npm run storybook
```

Stories live in `web/src/**/*.stories.tsx`. Preview loads `globals.css` + Tailwind.

---

## QA checklist

- [x] No `loom-*` classes in `web/src`
- [x] Login → Dashboard → Intelligence flow works
- [x] Sidebar on desktop, bottom nav on mobile
- [x] Compliance exposure on dashboard
- [x] `/coverage` page + sidebar link
- [x] `npm run build` passes
- [x] Date filters refetch API data
- [x] Live ping badge on Dashboard + Pings
- [x] Light/dark theme toggle
