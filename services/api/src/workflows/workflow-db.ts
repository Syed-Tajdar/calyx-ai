/** Shared row shapes and SQL helpers for the workflows module. Every value is a $n parameter. */

export interface Db {
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  query: (sql: string, params?: unknown[]) => Promise<{ rows: any[]; rowCount?: number | null }>;
}

export interface TenantRunner {
  withTenant<T>(tenantId: string, fn: (c: Db) => Promise<T>): Promise<T>;
}

export interface AlertRow {
  id: string;
  rule_id: string | null;
  rule_name?: string | null;
  trigger_type: string;
  scope: string;
  platform: string | null;
  entity_id: string;
  entity_name: string;
  integration_id: string | null;
  numbers: Record<string, unknown>;
  window_from: string | null;
  window_to: string | null;
  status: 'open' | 'acknowledged' | 'actioned' | 'resolved';
  action: 'none' | 'suggested' | 'paused' | 'pause_blocked' | 'pause_failed' | 'unpaused';
  action_detail: Record<string, unknown>;
  emailed: boolean;
  acknowledged_at: string | null;
  created_at: string;
  pause_id?: string | null;
  pause_confirmation?: string | null;
}

export interface PauseRow {
  id: string;
  integration_id: string;
  platform: 'ringba' | 'retreaver';
  publisher_id: string;
  publisher_name: string;
  mode: 'manual' | 'auto';
  alert_event_id: string | null;
  rule_id: string | null;
  paused_by: string | null;
  paused_at: string;
  unpause_at: string | null;
  confirmation: 'read_back' | 'response';
}

export interface IntegrationRow {
  id: string;
  name: string;
  platform: string;
  connection_mode: string;
  sync_mode: string | null;
  is_active: boolean;
  sync_status: string;
  last_sync_at: string | null;
  /** v1 sync health (migration 000034): last fully successful sync; null until the first one. */
  last_success_at?: string | null;
  created_at?: string | null;
  allow_publisher_pause: boolean;
}

export const RULE_COLUMNS = `id::text, tenant_id::text, name, trigger_type, params, window_hours, min_calls, scope,
  platform, notify_email, action_mode, suggest_since::text, pause_hours, cooldown_hours, is_active,
  created_at::text, updated_at::text`;

export const ALERT_COLUMNS = `a.id::text, a.rule_id::text, a.trigger_type, a.scope, a.platform, a.entity_id, a.entity_name,
  a.integration_id::text, a.numbers, a.window_from::text, a.window_to::text, a.status, a.action, a.action_detail,
  a.emailed, a.acknowledged_at::text, a.created_at::text`;

export const PAUSE_COLUMNS = `id::text, integration_id::text, platform, publisher_id, publisher_name, mode,
  alert_event_id::text, rule_id::text, paused_by::text, paused_at::text, unpause_at::text, confirmation`;

export const INTEGRATION_COLUMNS = `id::text, name, platform, connection_mode, sync_mode, is_active, sync_status,
  last_sync_at::text, to_json(last_success_at) #>> '{}' AS last_success_at, to_json(created_at) #>> '{}' AS created_at,
  COALESCE(allow_publisher_pause, FALSE) AS allow_publisher_pause`;

/**
 * integration_unhealthy (v1 sync health). A sync is unhealthy when its status says so — any failure is
 * `degraded`, repeated failures `failed` (`error` is the pre-000034 status) — or when it has not succeeded
 * for 30 minutes whatever the status says: from 2026-10-18 to 2026-10-21 every sync failed behind an `ok`.
 * Same 30-minute rule as the Settings badge (web/src/lib/sync-health.ts).
 */
export const UNHEALTHY_SYNC_STATUSES = ['error', 'degraded', 'failed'];
export const SYNC_STALE_AFTER_MS = 30 * 60 * 1000;

export function unhealthyReason(
  i: Pick<IntegrationRow, 'sync_status' | 'is_active' | 'last_success_at' | 'last_sync_at' | 'created_at'>,
  now: Date,
): 'status' | 'stale' | null {
  if (UNHEALTHY_SYNC_STATUSES.includes(i.sync_status)) return 'status';
  if (!i.is_active) return null; // paused: not expected to sync
  // last_success_at is null until the first success after migration 000034; last_sync_at only ever moved on
  // success before it, and a brand-new integration gets 30 minutes from created_at.
  const ref = i.last_success_at ?? i.last_sync_at ?? i.created_at ?? null;
  const t = ref ? Date.parse(ref) : NaN;
  if (Number.isNaN(t)) return null;
  return now.getTime() - t > SYNC_STALE_AFTER_MS ? 'stale' : null;
}

/** Live = API sync with real credentials, active, Ringba or Retreaver. Test (mock) is never Live. */
export function isLive(i: Pick<IntegrationRow, 'connection_mode' | 'sync_mode' | 'is_active' | 'platform'>): boolean {
  return (
    i.connection_mode === 'api_sync' &&
    i.sync_mode === 'real' &&
    i.is_active === true &&
    (i.platform === 'ringba' || i.platform === 'retreaver')
  );
}

/** One Eastern day, for the daily auto-pause limit. */
export const EASTERN_TODAY_SQL = `(NOW() AT TIME ZONE 'America/New_York')::date`;
