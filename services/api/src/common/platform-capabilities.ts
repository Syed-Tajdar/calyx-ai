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

/**
 * One conversion state, stamped by the API. The web only displays this value.
 *
 *   converted       reports AND flag AND revenue > 0
 *   converted_zero  reports AND flag AND revenue = 0
 *   not_converted   reports AND flag is false
 *   not_reported    platform does not report conversion state
 */
export const CONVERSION_STATES = [
  'converted',
  'converted_zero',
  'not_converted',
  'not_reported',
] as const;

export type ConversionState = (typeof CONVERSION_STATES)[number];

export function conversionState(input: {
  platform: string;
  hasConverted: boolean;
  revenue: number;
}): ConversionState {
  if (!reportsConversionState(input.platform)) return 'not_reported';
  if (!input.hasConverted) return 'not_converted';
  return input.revenue > 0 ? 'converted' : 'converted_zero';
}

/**
 * SQL for the call-list filters. Must stay in lockstep with `conversionState`.
 * `latest` uses argMax so a re-synced call is judged on its newest row.
 * `should_convert` is not a fifth state: it is `not_converted` plus revenue or duration.
 */
export function conversionFilterSql(
  filter: 'converted' | 'converted_zero' | 'unconverted' | 'should_convert',
  mode: 'raw' | 'latest',
): string {
  const platform = reportsConversionStateSql('platform');
  const hc = mode === 'latest' ? 'argMax(has_converted, received_at)' : 'has_converted';
  const rev = mode === 'latest' ? 'argMax(revenue, received_at)' : 'revenue';
  const dur = mode === 'latest' ? 'argMax(duration_seconds, received_at)' : 'duration_seconds';
  const req =
    mode === 'latest'
      ? 'argMax(required_duration_seconds, received_at)'
      : 'required_duration_seconds';
  if (filter === 'converted') {
    return `${platform} AND ${hc} = 1 AND ${rev} > 0`;
  }
  if (filter === 'converted_zero') {
    return `${platform} AND ${hc} = 1 AND ${rev} = 0`;
  }
  if (filter === 'unconverted') {
    return `${platform} AND ${hc} = 0`;
  }
  return `${platform} AND ${hc} = 0 AND (
    ${rev} > 0
    OR (
      ${req} IS NOT NULL
      AND ${dur} >= ${req}
    )
  )`;
}
