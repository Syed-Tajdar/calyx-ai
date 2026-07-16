/**
 * Per-platform call deep-link patterns.
 * Catalog: shared/providers/platform-deep-links.json
 * Env overrides (NEXT_PUBLIC_*) win over defaults — never hardcode only in table UI.
 *
 * Phase 1.6 D8b Case A: Retreaver Call Logs shows UUID (= our external_call_id).
 * Pending Syed screenshot confirmation.
 */

const DEFAULT_PATTERNS: Record<string, string> = {
  // Official support example: https://retreaver.com/calls/{uuid}
  retreaver: 'https://retreaver.com/calls/{id}',
  // Best-effort (Phase 2.A A4); override via NEXT_PUBLIC_RINGBA_CALL_URL_PATTERN
  ringba: 'https://app.ringba.com/#/call/{id}',
};

/** Env var name for a platform call URL pattern override. */
export function callUrlPatternEnvKey(platform: string): string {
  return `NEXT_PUBLIC_${platform.toUpperCase().replace(/-/g, '_')}_CALL_URL_PATTERN`;
}

function patternFor(platform: string): string | undefined {
  const key = callUrlPatternEnvKey(platform);
  const fromEnv = process.env[key]?.trim();
  if (fromEnv) return fromEnv;
  return DEFAULT_PATTERNS[platform.toLowerCase()];
}

/**
 * Build an external platform call URL, or null if no pattern / empty id.
 * Pattern must include `{id}` (replaced with encodeURIComponent(callId)).
 */
export function buildPlatformCallUrl(
  platform: string,
  callId: string,
): string | null {
  const id = callId.trim();
  if (!id || !platform) return null;
  const pattern = patternFor(platform);
  if (!pattern || !pattern.includes('{id}')) return null;
  return pattern.replaceAll('{id}', encodeURIComponent(id));
}

/** True when we can deep-link this platform+id into the vendor UI. */
export function hasPlatformCallDeepLink(platform: string, callId: string): boolean {
  return buildPlatformCallUrl(platform, callId) != null;
}
