/**
 * Per-platform capabilities — the single source of truth (Stage 4 R1 / C6).
 *
 * `has_converted` is `UInt8 DEFAULT 0` in ClickHouse (audit F038 / K8), so a stored `0` means
 * two different things depending on the platform:
 *
 *   - Ringba   — `hasConverted` is requested and mapped, so 0 genuinely means "not converted".
 *   - Retreaver — `converted` is now mapped (R1), so 0 genuinely means "not converted".
 *   - TrackDrive / CallGrid — conversion state is never requested, so 0 means "we never asked".
 *
 * Treating "we never asked" as "not converted" is what made every TrackDrive and CallGrid call
 * with revenue appear in the "should have converted" list — a false positive across whole
 * platforms.
 *
 * The web app must not re-implement this rule: the API stamps `conversion_reported` onto every
 * ping row, so the badge and the server-side filter cannot drift apart.
 */
export const PLATFORMS_REPORTING_CONVERSION_STATE = ['ringba', 'retreaver'] as const;

export function reportsConversionState(platform: string): boolean {
  return (PLATFORMS_REPORTING_CONVERSION_STATE as readonly string[]).includes(
    (platform ?? '').toLowerCase(),
  );
}

/**
 * SQL predicate form, for queries that filter across platforms in one statement.
 * Returns e.g. `platform IN ('ringba','retreaver')`.
 */
export function reportsConversionStateSql(column = 'platform'): string {
  const list = PLATFORMS_REPORTING_CONVERSION_STATE.map((p) => `'${p}'`).join(', ');
  return `${column} IN (${list})`;
}
