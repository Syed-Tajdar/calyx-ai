import { EVENT_AT_EXPR } from '../common/ping-event-time';
import { apiSyncExclusionSql } from '../common/demo-rows';
import { conversionFilterSql } from '../common/platform-capabilities';

/**
 * Conversion state filter for the call list.
 *
 * `should_convert` is the revenue-leak view: the platform has not marked the call converted, but
 * either it already earned revenue, or it talked at least as long as the buyer's own qualifying
 * threshold. Deliberately a wide net — see `docs/convert-cancel-screens-plan.md` decision D1.
 */
export const PING_CONVERSION_FILTERS = [
  'converted',
  'unconverted',
  'should_convert',
] as const;

export type PingConversionFilter = (typeof PING_CONVERSION_FILTERS)[number];

export function isPingConversionFilter(v: unknown): v is PingConversionFilter {
  return (
    typeof v === 'string' &&
    (PING_CONVERSION_FILTERS as readonly string[]).includes(v)
  );
}

export interface PingListFilters {
  platform?: string;
  from?: string;
  to?: string;
  search?: string;
  /** When true, include mock `apisync-%` external_call_id rows. Default: hide. */
  show_demo?: boolean;
  /** Conversion state. Omitted = no conversion filtering. */
  conversion?: PingConversionFilter;
  /**
   * When true, return the raw event stream — one row per sync, so a call that was re-synced
   * appears several times. Default (false) collapses to one row per call.
   */
  raw?: boolean;
}

export function buildPingWhereClause(filters: PingListFilters): {
  clause: string;
  params: Record<string, string | number>;
} {
  const parts: string[] = [];
  const params: Record<string, string | number> = {};

  const demoClause = apiSyncExclusionSql('external_call_id', filters.show_demo);
  if (demoClause) parts.push(demoClause);

  if (filters.platform) {
    parts.push('AND platform = {platform:String}');
    params.platform = filters.platform;
  }
  if (filters.from) {
    parts.push(
      `AND ${EVENT_AT_EXPR} >= parseDateTimeBestEffort({from:String})`,
    );
    params.from = filters.from;
  }
  if (filters.to) {
    parts.push(
      `AND ${EVENT_AT_EXPR} < parseDateTimeBestEffort({to:String})`,
    );
    params.to = filters.to;
  }
  if (filters.search) {
    // Exact match first, then prefix (Retreaver Call Log shows truncated UUID).
    parts.push(
      `AND (
          external_call_id = {search:String}
          OR startsWith(lowerUTF8(external_call_id), lowerUTF8({search:String}))
        )`,
    );
    params.search = filters.search;
  }

  return { clause: parts.join('\n        '), params };
}

/** Predicate for HAVING (latest row) or an extra AND (raw rows). Empty when no conversion filter. */
export function pingConversionPredicate(filters: PingListFilters, mode: 'raw' | 'latest'): string {
  if (!filters.conversion) return '';
  return conversionFilterSql(filters.conversion, mode);
}
