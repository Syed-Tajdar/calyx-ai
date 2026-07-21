/**
 * Per-platform call deep-link patterns.
 * Catalog: shared/providers/platform-deep-links.json
 * Env overrides (NEXT_PUBLIC_*) win over defaults — never hardcode only in table UI.
 *
 * Retreaver: per-call URL with {id}.
 * Ringba: report-level only (no per-call URL; platform limitation).
 */

const DEFAULT_PATTERNS: Record<string, string> = {
  // Official support example: https://retreaver.com/calls/{uuid}
  retreaver: 'https://retreaver.com/calls/{id}',
  // Report-level — Ringba has no standalone call URL (operator-confirmed 2026-07-20).
  ringba: 'https://app.ringba.com/#/dashboard/call-logs/report/new',
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
 * True when the vendor has no per-call URL — we open a report/list page and should
 * copy the call id to the clipboard so the operator can filter.
 */
export function platformDeepLinkCopiesCallId(platform: string): boolean {
  const pattern = patternFor(platform);
  return Boolean(pattern && !pattern.includes('{id}'));
}

/**
 * Build an external platform call URL, or null if no pattern / empty id.
 * If the pattern includes `{id}`, it is replaced with encodeURIComponent(callId).
 * Patterns without `{id}` are report-level entry points (still require a non-empty callId
 * so the UI only shows the link in a call context).
 */
export function buildPlatformCallUrl(
  platform: string,
  callId: string,
): string | null {
  const id = callId.trim();
  if (!id || !platform) return null;
  const pattern = patternFor(platform);
  if (!pattern) return null;
  if (pattern.includes('{id}')) {
    return pattern.replaceAll('{id}', encodeURIComponent(id));
  }
  return pattern;
}

/** True when we can deep-link this platform+id into the vendor UI. */
export function hasPlatformCallDeepLink(platform: string, callId: string): boolean {
  return buildPlatformCallUrl(platform, callId) != null;
}
