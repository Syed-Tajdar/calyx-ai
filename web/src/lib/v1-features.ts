/**
 * v1 scope gate.
 *
 * v1 is Ringba call data plus Convert / Cancel. Everything else in the build is either
 * **Chapter 2** (call recordings + transcription, AI call scoring, Ask AI) or **Later**
 * (DNC / TCPA compliance, ad attribution) and still runs on mock data.
 *
 * Showing a customer a working Convert button next to invented TCPA risk scores and a fake
 * publisher league table costs more credibility than the features gain. So those surfaces are
 * hidden by default rather than deleted — the code, routes and APIs all stay, and a single flag
 * brings them back for internal demos.
 *
 * Set `NEXT_PUBLIC_V1_ONLY=false` at build time to show everything again.
 * (`NEXT_PUBLIC_*` is baked into the bundle, so the web image must be rebuilt for a change.)
 */
export const V1_ONLY = process.env.NEXT_PUBLIC_V1_ONLY !== 'false';

/** Route prefixes that are out of v1 scope. */
export const NON_V1_ROUTES = [
  '/scoring',
  '/intelligence',
  '/ask',
  '/compliance',
  '/attribution',
] as const;

/** True when a route should be reachable in the current build. */
export function isRouteVisible(href: string): boolean {
  if (!V1_ONLY) return true;
  return !NON_V1_ROUTES.some((r) => href === r || href.startsWith(`${r}/`));
}

/**
 * Dashboard panels that read from mock-only pipelines.
 *
 * - `callScore` / `topScores` — scoring service (Chapter 2)
 * - `complianceExposure` — compliance service (Later)
 * - `intelligenceCards` — Ask AI / TCPA / Attribution entry points
 * - `publishers` — hardcoded fixture in `demo-publishers.ts`, ignores the demo toggle entirely,
 *   so it shows invented revenue even when the rest of the dashboard says "No pings yet"
 */
export const SHOW_MOCK_PANELS = !V1_ONLY;
