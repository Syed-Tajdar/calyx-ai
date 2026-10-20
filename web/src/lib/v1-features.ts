/**
 * v1 scope gate.
 *
 * v1 is Ringba call data plus Convert / Cancel. Everything else in the build is either
 * **Chapter 2** (call recordings + transcription, AI call scoring, Ask AI) or **Later**
 * (DNC / TCPA compliance, ad attribution) and still runs on mock data.
 *
 * Showing a customer a working Convert button next to invented TCPA risk scores and a fake
 * publisher league table costs more credibility than the features gain. So those surfaces are
 * hidden by default rather than deleted — the code stays, but their pages answer HTTP 404 (and the
 * API routes 404 too, see `services/api/src/common/feature-gate.ts`). A single flag brings them
 * back for internal demos.
 *
 * Set `NEXT_PUBLIC_V1_ONLY=false` at build time to show everything again.
 * (`NEXT_PUBLIC_*` is baked into the bundle, so the web image must be rebuilt for a change.)
 */
export const V1_ONLY = process.env.NEXT_PUBLIC_V1_ONLY !== 'false';

/**
 * Out-of-v1 features, keyed the same way as the API's route gate
 * (`services/api/src/common/feature-gate.ts`) so a v2 flag re-opens exactly one feature.
 */
export type Feature =
  | 'ask_ai'
  | 'transcription'
  | 'qa'
  | 'workflows'
  | 'attribution'
  | 'compliance'
  | 'enrichment'
  | 'providers'
  | 'intelligence';

/**
 * v2 flags. `NEXT_PUBLIC_*` values are inlined at build time, so each one has to be referenced
 * literally here (no `process.env[name]`). A feature without a flag (`undefined`) stays closed
 * unless `NEXT_PUBLIC_V1_ONLY=false`. Attribution / compliance / enrichment never get a flag.
 */
export type FeatureFlags = Partial<Record<Feature, string | undefined>>;

const BUILD_FLAGS: FeatureFlags = {
  ask_ai: process.env.NEXT_PUBLIC_FEATURE_ASK_AI,
  transcription: process.env.NEXT_PUBLIC_FEATURE_TRANSCRIPTION,
  qa: process.env.NEXT_PUBLIC_FEATURE_QA,
  workflows: process.env.NEXT_PUBLIC_FEATURE_WORKFLOWS,
};

/** Route prefixes that are out of v1 scope, and the feature that owns each one. */
export const NON_V1_ROUTE_FEATURES = {
  '/scoring': 'qa', // scores + /scoring/scorecards
  '/ask': 'ask_ai',
  // v2 Phase 2D alert centre (bell + /alerts page).
  '/alerts': 'workflows',
  // Hub that links into every intelligence feature; no flag of its own yet.
  '/intelligence': 'intelligence',
  '/compliance': 'compliance',
  '/attribution': 'attribution',
} as const satisfies Record<string, Feature>;

export const NON_V1_ROUTES = Object.keys(NON_V1_ROUTE_FEATURES) as Array<
  keyof typeof NON_V1_ROUTE_FEATURES
>;

export function isFeatureEnabled(
  feature: Feature,
  v1Only: boolean = V1_ONLY,
  flags: FeatureFlags = BUILD_FLAGS,
): boolean {
  if (!v1Only) return true;
  return flags[feature] === 'true';
}

/** The feature that owns `href`, or `null` for a v1 route. */
export function featureForRoute(href: string): Feature | null {
  const path = href.split(/[?#]/)[0];
  for (const [prefix, feature] of Object.entries(NON_V1_ROUTE_FEATURES)) {
    if (path === prefix || path.startsWith(`${prefix}/`)) return feature as Feature;
  }
  return null;
}

/**
 * True when a route should be reachable in the current build. Used by the navigation and by the
 * segment layouts that answer 404 (`app/(app)/{scoring,ask,alerts,intelligence,compliance,attribution}`).
 */
export function isRouteVisible(
  href: string,
  v1Only: boolean = V1_ONLY,
  flags: FeatureFlags = BUILD_FLAGS,
): boolean {
  const feature = featureForRoute(href);
  return feature === null || isFeatureEnabled(feature, v1Only, flags);
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

/** v1 connects and filters Ringba and Retreaver. Other platform code stays in the build. */
export const V1_PLATFORMS = ['ringba', 'retreaver'] as const;

export function platformsForV1(
  platforms: readonly string[],
  v1Only: boolean = V1_ONLY,
): string[] {
  if (!v1Only) return [...platforms];
  return platforms.filter((platform) =>
    (V1_PLATFORMS as readonly string[]).includes(platform),
  );
}
