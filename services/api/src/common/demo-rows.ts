/**
 * Read-time filter for synthetic rows. Rows stay in the database.
 *
 * Naming convention — synthetic or script-generated rows must use one of:
 *   - `smoke-`   verify / smoke scripts
 *   - `apisync-` mock API-sync adapter
 * Never a bare UUID. Fixture rows from ClickHouse `000006_demo_fixture` use `demo-`.
 *
 * Default KPI / list reads exclude all three. `show_demo=true` includes all three.
 */

export const API_SYNC_ID_PREFIX = 'apisync-';
export const DEMO_FIXTURE_ID_PREFIX = 'demo-';
export const SMOKE_ID_PREFIX = 'smoke-';

export const TEST_DATA_ID_PREFIXES = [
  API_SYNC_ID_PREFIX,
  DEMO_FIXTURE_ID_PREFIX,
  SMOKE_ID_PREFIX,
] as const;

/** @deprecated Use API_SYNC_ID_PREFIX — kept for callers that still say "demo" for apisync rows. */
export const DEMO_EXTERNAL_ID_PREFIX = API_SYNC_ID_PREFIX;

const COLUMN_RE = /^[a-zA-Z_][a-zA-Z0-9_]*(\.[a-zA-Z_][a-zA-Z0-9_]*)?$/;

/** Production never shows demo rows. Local default is on. Rows are not deleted. */
export function isDemoModeEnabled(): boolean {
  if (process.env.APP_ENV === 'production') return false;
  const value = process.env.DEMO_MODE_ENABLED;
  if (value === undefined || value === '') return true;
  return value !== 'false' && value !== '0';
}

/**
 * ClickHouse SQL fragment. Empty when includeTestData is true (show_demo=true).
 * @param column typically `external_call_id` (pings) or `call_id` (scores / compliance)
 */
export function testDataExclusionSql(column: string, includeTestData?: boolean): string {
  if (isDemoModeEnabled() && includeTestData) return '';
  if (!COLUMN_RE.test(column)) {
    throw new Error(`invalid test-data exclusion column: ${column}`);
  }
  return TEST_DATA_ID_PREFIXES.map(
    (prefix) => `AND ${column} NOT LIKE '${prefix}%'`,
  ).join(' ');
}

/** @deprecated Use testDataExclusionSql — now excludes apisync-, demo-, and smoke-. */
export function apiSyncExclusionSql(column: string, includeApiSync?: boolean): string {
  return testDataExclusionSql(column, includeApiSync);
}

/** @deprecated Use testDataExclusionSql. */
export function demoExclusionSql(column: string, showDemo?: boolean): string {
  return testDataExclusionSql(column, showDemo);
}

export function isTestDataExternalId(id: string): boolean {
  return TEST_DATA_ID_PREFIXES.some((prefix) => id.startsWith(prefix));
}

/** @deprecated Use isTestDataExternalId */
export function isApiSyncExternalId(id: string): boolean {
  return isTestDataExternalId(id);
}

/** @deprecated Use isTestDataExternalId */
export function isDemoExternalId(id: string): boolean {
  return isTestDataExternalId(id);
}
