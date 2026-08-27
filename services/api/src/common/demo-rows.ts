/**
 * Excludes platform-sync mock rows whose external_call_id / call_id starts with `apisync-`.
 *
 * Intentionally does NOT exclude ClickHouse fixture rows (`demo-call-*`, `demo-callloom-*`
 * from migration 000006_demo_fixture). Those remain counted so a cold-start dashboard is
 * not empty. See docs/part1-delta-report.md (D3 Option A).
 *
 * Naming convention for synthetic / test rows inserted by hand or by scripts:
 *   MUST use the `apisync-` prefix (e.g. `apisync-webhook-parity-…`) so this helper
 *   excludes them from headline KPIs by default. Do NOT invent a third prefix
 *   (e.g. `webhook-parity-…`) — those leak into production counts.
 */

export const API_SYNC_ID_PREFIX = 'apisync-';

/** @deprecated Use API_SYNC_ID_PREFIX — kept for callers that still say "demo" for apisync rows. */
export const DEMO_EXTERNAL_ID_PREFIX = API_SYNC_ID_PREFIX;

const COLUMN_RE = /^[a-zA-Z_][a-zA-Z0-9_]*(\.[a-zA-Z_][a-zA-Z0-9_]*)?$/;

/**
 * ClickHouse SQL fragment. Empty when includeApiSync is true.
 * @param column typically `external_call_id` (pings) or `call_id` (scores / compliance)
 * @param includeApiSync when true, do not filter (Show demo / show_demo=true)
 */
export function apiSyncExclusionSql(column: string, includeApiSync?: boolean): string {
  if (includeApiSync) return '';
  if (!COLUMN_RE.test(column)) {
    throw new Error(`invalid api-sync exclusion column: ${column}`);
  }
  return `AND ${column} NOT LIKE '${API_SYNC_ID_PREFIX}%'`;
}

/** @deprecated Use apiSyncExclusionSql — name was misleading (fixture demo-% still counted). */
export function demoExclusionSql(column: string, showDemo?: boolean): string {
  return apiSyncExclusionSql(column, showDemo);
}

export function isApiSyncExternalId(id: string): boolean {
  return id.startsWith(API_SYNC_ID_PREFIX);
}

/** @deprecated Use isApiSyncExternalId */
export function isDemoExternalId(id: string): boolean {
  return isApiSyncExternalId(id);
}
