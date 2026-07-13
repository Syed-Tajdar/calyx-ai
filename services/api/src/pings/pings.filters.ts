import { EVENT_AT_EXPR } from '../common/ping-event-time';

export interface PingListFilters {
  platform?: string;
  from?: string;
  to?: string;
  search?: string;
}

export function buildPingWhereClause(filters: PingListFilters): {
  clause: string;
  params: Record<string, string | number>;
} {
  const parts: string[] = [];
  const params: Record<string, string | number> = {};

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
