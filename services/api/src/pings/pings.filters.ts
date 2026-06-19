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
    parts.push('AND received_at >= parseDateTimeBestEffort({from:String})');
    params.from = filters.from;
  }
  if (filters.to) {
    parts.push('AND received_at < parseDateTimeBestEffort({to:String})');
    params.to = filters.to;
  }
  if (filters.search) {
    parts.push(
      "AND positionCaseInsensitive(external_call_id, {search:String}) > 0",
    );
    params.search = filters.search;
  }

  return { clause: parts.join('\n        '), params };
}
