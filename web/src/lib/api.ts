import { clearToken, getCsrfToken, isCookieSession } from './auth';

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? 'http://localhost:3100';

export class ApiError extends Error {
  constructor(
    message: string,
    readonly status: number,
  ) {
    super(message);
    this.name = 'ApiError';
  }
}

export interface PingRecord {
  tenant_id: string;
  platform: string;
  external_call_id: string;
  ani_hash: string;
  campaign_id: string;
  publisher_id: string;
  target_id: string;
  state: string;
  event_type: string;
  revenue: number;
  payout: number;
  is_duplicate: number;
  /** Platform-reported conversion state. ClickHouse default is 0, so false also means "not reported". */
  has_converted?: boolean;
  /** Connected talk time in seconds. */
  duration_seconds?: number;
  /** Buyer's qualifying threshold when the platform reports one. */
  required_duration_seconds?: number | null;
  /** Times this call has been written (re-syncs). 1 unless the call was updated. */
  sync_count?: number;
  /**
   * Whether this platform reports conversion state at all. Stamped by the API from its single
   * source of truth, so the UI never re-implements the rule (Stage 4 R1 / C6).
   */
  conversion_reported?: boolean;
  /**
   * Stamped by the API. The UI displays this and does not recompute it.
   * converted | converted_zero | not_converted | not_reported
   */
  conversion_state?: 'converted' | 'converted_zero' | 'not_converted' | 'not_reported';
  received_at: string;
  /** Source call time when known; otherwise equals received_at. Prefer for display/filters. */
  occurred_at?: string;
}

/** Conversion-state filter for the call list. Mirrors the API's `conversion` query param. */
export type PingConversionFilter = 'converted' | 'converted_zero' | 'unconverted' | 'should_convert';

export interface PingListResponse {
  data: PingRecord[];
  /** Raw event rows (pagination). */
  total: number;
  /** Distinct calls in the filter window — Ping Feed headline. */
  distinct_total: number;
  limit: number;
  offset: number;
}

export interface ScoreRecord {
  tenant_id: string;
  call_id: string;
  platform: string;
  overall_score: number;
  sentiment: string;
  rules_passed: number;
  rules_failed: number;
  created_at: string;
}

export interface TranscriptRecord {
  tenant_id: string;
  call_id: string;
  transcript_text: string;
  duration_seconds: number;
  word_count: number;
  created_at: string;
}

export interface EnrichmentAttribute {
  attribute_key: string;
  attribute_value: string;
  provider: string;
}

export interface MetricsResult {
  total_pings: number;
  total_revenue: number;
  total_payout: number;
  cpa: number;
  rpc: number;
  roi: number;
  duplicate_rate: number;
}

export interface PnLResult {
  revenue: number;
  payout: number;
  ad_spend: number;
  profit: number;
}

export interface PlatformMixRow {
  platform: string;
  count: number;
  percent: number;
}

export interface AlertRule {
  id: string;
  tenant_id: string;
  name: string;
  condition: Record<string, unknown>;
  threshold: number;
  channels: string[];
  actions: Record<string, unknown>[];
  is_active: boolean;
}

export interface IntegrationRecord {
  id: string;
  platform: string;
  name: string;
  connection_mode: 'webhook' | 'api_sync';
  sync_mode: 'mock' | 'real' | null;
  webhook_token: string | null;
  is_active: boolean;
  last_sync_at: string | null;
  sync_status: string;
  last_sync_error: string | null;
  created_at: string;
  supports_api_sync: boolean;
  supports_webhook: boolean;
  initial_lookback_days: number | null;
  /** Per-tenant write-back gate (default false). Drives whether Convert / Cancel are offered. */
  writeback_enabled?: boolean;
  /** Backend write-back registry (Stage 4b): Ringba and Retreaver. */
  supports_writeback?: boolean;
}

export interface ApiSyncCredentialField {
  key: string;
  label: string;
  secret?: boolean;
  placeholder?: string;
}

export interface PlatformConnectionOption {
  platform: string;
  supports_webhook: boolean;
  supports_api_sync: boolean;
  /** Backend write-back registry (Stage 4b): Ringba and Retreaver. */
  supports_writeback?: boolean;
  api_sync_fields?: ApiSyncCredentialField[];
}

export interface CreateIntegrationInput {
  platform: string;
  name: string;
  connection_mode: 'webhook' | 'api_sync';
  sync_mode?: 'mock' | 'real';
  webhook_secret?: string;
  api_key?: string;
  api_token?: string;
  account_id?: string;
  company_id?: string;
  subdomain?: string;
  public_key?: string;
  private_key?: string;
  auth_token?: string;
}

export interface HeatmapStateRow {
  state: string;
  country: 'US' | 'CA' | '';
  ping_count: number;
  revenue: number;
}

export interface HeatmapResult {
  data: HeatmapStateRow[];
  total_pings: number;
  us_regions: number;
  ca_regions: number;
  unmapped_pings?: number;
}

function authHeaders(token: string, init?: RequestInit): HeadersInit {
  const headers: Record<string, string> = {
    ...(init?.body ? { 'Content-Type': 'application/json' } : {}),
  };
  const method = (init?.method ?? 'GET').toUpperCase();
  if (method !== 'GET' && method !== 'HEAD') {
    const csrf = getCsrfToken();
    if (csrf) headers['X-CSRF-Token'] = csrf;
  }
  if (token && !isCookieSession(token)) {
    headers.Authorization = `Bearer ${token}`;
  }
  return { ...(init?.headers ?? {}), ...headers };
}

async function fetchWithAuth<T>(token: string, path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(`${API_URL}${path}`, {
    ...init,
    credentials: 'include',
    headers: authHeaders(token, init),
  });
  if (res.status === 401) {
    clearToken();
    throw new ApiError('Session expired', 401);
  }
  if (!res.ok) {
    throw new ApiError(await errorMessage(res, path), res.status);
  }
  if (res.status === 204) {
    return null as T;
  }
  const text = await res.text();
  if (!text) {
    return null as T;
  }
  return JSON.parse(text) as T;
}

/**
 * Surface the API's own message (Nest puts it in `message`) so operator-facing failures read as
 * "call already converted" rather than "Request failed: /integrations/…". Falls back to the path
 * when the body is empty or not JSON.
 */
async function errorMessage(res: Response, path: string): Promise<string> {
  try {
    const body: unknown = JSON.parse(await res.text());
    if (body && typeof body === 'object' && 'message' in body) {
      const m = (body as { message: unknown }).message;
      if (typeof m === 'string' && m.trim()) return m;
      if (Array.isArray(m) && typeof m[0] === 'string') return m[0];
    }
  } catch {
    // fall through to the generic message
  }
  return `Request failed: ${path}`;
}

async function fetchWithAuthNoContent(
  token: string,
  path: string,
  init?: RequestInit,
): Promise<void> {
  const res = await fetch(`${API_URL}${path}`, {
    ...init,
    credentials: 'include',
    headers: authHeaders(token, init),
  });
  if (res.status === 401) {
    clearToken();
    throw new ApiError('Session expired', 401);
  }
  if (!res.ok) {
    throw new ApiError(`Request failed: ${path}`, res.status);
  }
}

export async function login(email: string, password: string): Promise<string> {
  const res = await fetch(`${API_URL}/auth/login`, {
    method: 'POST',
    credentials: 'include',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ email, password }),
  });
  if (res.status === 429) {
    throw new Error('Too many attempts');
  }
  if (!res.ok) {
    throw new Error('Login failed');
  }
  // JWT is httpOnly — return sentinel for callers that still expect a string.
  return 'cookie-session';
}

export async function logout(): Promise<void> {
  const csrf = getCsrfToken();
  await fetch(`${API_URL}/auth/logout`, {
    method: 'POST',
    credentials: 'include',
    headers: csrf ? { 'X-CSRF-Token': csrf } : {},
  }).catch(() => undefined);
  clearToken();
}

export interface MeResult {
  email: string;
  tenant_id: string;
  tenant_name: string;
  role: string;
}

export async function fetchMe(token: string): Promise<MeResult> {
  return fetchWithAuth<MeResult>(token, '/auth/me');
}

export interface ProviderCatalogEntry {
  id: string;
  domain: string;
  display_name: string;
  required_secrets: string[];
  optional_config: string[];
  go_adapter: string;
}

export interface ProviderBinding {
  id: string;
  tenant_id: string;
  domain: string;
  provider_id: string;
  config_json: Record<string, unknown>;
  is_active: boolean;
  priority: number;
  created_at: string;
  updated_at: string;
}

export async function fetchProvidersCatalog(
  token: string,
): Promise<{ version: string; providers: ProviderCatalogEntry[] }> {
  return fetchWithAuth<{ version: string; providers: ProviderCatalogEntry[] }>(
    token,
    '/providers/catalog',
  );
}

export async function fetchProviderBindings(
  token: string,
): Promise<{ data: ProviderBinding[] }> {
  return fetchWithAuth<{ data: ProviderBinding[] }>(token, '/providers/bindings');
}

export async function createProviderBinding(
  token: string,
  body: {
    domain: string;
    provider_id: string;
    config_json?: Record<string, unknown>;
    is_active?: boolean;
    priority?: number;
  },
): Promise<ProviderBinding> {
  return fetchWithAuth<ProviderBinding>(token, '/providers/bindings', {
    method: 'POST',
    body: JSON.stringify(body),
  });
}

export interface PingFetchFilters {
  platform?: string;
  from?: string;
  to?: string;
  search?: string;
  /** Include mock `apisync-%` rows. Default false (API hides them). */
  show_demo?: boolean;
  /** Conversion state. Omitted = no conversion filtering. */
  conversion?: PingConversionFilter;
  /** Raw event stream (one row per sync) instead of one row per call. */
  raw?: boolean;
  /** v1 All: ringba + retreaver only. */
  v1_platforms?: boolean;
}

export async function fetchPings(
  token: string,
  limit = 50,
  offset = 0,
  filters: PingFetchFilters = {},
): Promise<PingListResponse> {
  const params = new URLSearchParams({
    limit: String(limit),
    offset: String(offset),
  });
  if (filters.platform) params.set('platform', filters.platform);
  if (filters.from) params.set('from', filters.from);
  if (filters.to) params.set('to', filters.to);
  if (filters.search) params.set('search', filters.search);
  if (filters.show_demo) params.set('show_demo', 'true');
  if (filters.conversion) params.set('conversion', filters.conversion);
  if (filters.raw) params.set('raw', 'true');
  if (filters.v1_platforms) params.set('v1_platforms', 'true');
  return fetchWithAuth<PingListResponse>(token, `/pings?${params.toString()}`);
}

export async function downloadPingsExport(
  token: string,
  filters: PingFetchFilters = {},
): Promise<void> {
  const params = new URLSearchParams();
  if (filters.platform) params.set('platform', filters.platform);
  if (filters.from) params.set('from', filters.from);
  if (filters.to) params.set('to', filters.to);
  if (filters.search) params.set('search', filters.search);
  if (filters.show_demo) params.set('show_demo', 'true');
  if (filters.conversion) params.set('conversion', filters.conversion);
  if (filters.raw) params.set('raw', 'true');
  if (filters.v1_platforms) params.set('v1_platforms', 'true');
  const qs = params.toString();
  const res = await fetch(`${API_URL}/pings/export${qs ? `?${qs}` : ''}`, {
    credentials: 'include',
    headers: authHeaders(token),
  });
  if (res.status === 401) {
    clearToken();
    throw new ApiError('Session expired', 401);
  }
  if (!res.ok) {
    throw new ApiError('Export failed', res.status);
  }
  const blob = await res.blob();
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = `pings-${Date.now()}.csv`;
  a.click();
  URL.revokeObjectURL(url);
}

export interface FetchScoresOptions {
  callId?: string;
  limit?: number;
  offset?: number;
  show_demo?: boolean;
}

export async function fetchScores(
  token: string,
  options: FetchScoresOptions = {},
): Promise<ScoreRecord | null | { data: ScoreRecord[] }> {
  const params = new URLSearchParams();
  if (options.callId) params.set('callId', options.callId);
  if (options.limit !== undefined) params.set('limit', String(options.limit));
  if (options.offset !== undefined) params.set('offset', String(options.offset));
  if (options.show_demo) params.set('show_demo', 'true');
  const qs = params.toString();
  return fetchWithAuth<ScoreRecord | null | { data: ScoreRecord[] }>(
    token,
    `/scoring/scores${qs ? `?${qs}` : ''}`,
  );
}

export async function fetchTranscript(
  token: string,
  callId: string,
): Promise<TranscriptRecord | null> {
  const params = new URLSearchParams({ callId });
  return fetchWithAuth<TranscriptRecord | null>(token, `/scoring/transcript?${params.toString()}`);
}

export async function fetchEnrichment(
  token: string,
  aniHash: string,
  limit = 100,
): Promise<{ data: EnrichmentAttribute[]; total: number; limit?: number }> {
  const params = new URLSearchParams({ limit: String(limit) });
  return fetchWithAuth<{ data: EnrichmentAttribute[]; total: number; limit?: number }>(
    token,
    `/enrichment/${encodeURIComponent(aniHash)}?${params.toString()}`,
  );
}

export async function fetchAnalyticsMetrics(
  token: string,
  filters: { from?: string; to?: string; show_demo?: boolean; platform?: string; v1_platforms?: boolean } = {},
): Promise<MetricsResult> {
  const params = new URLSearchParams();
  if (filters.from) params.set('from', filters.from);
  if (filters.to) params.set('to', filters.to);
  if (filters.platform) params.set('platform', filters.platform);
  if (filters.show_demo) params.set('show_demo', 'true');
  if (filters.v1_platforms) params.set('v1_platforms', 'true');
  const qs = params.toString();
  return fetchWithAuth<MetricsResult>(token, `/analytics/metrics${qs ? `?${qs}` : ''}`);
}

export async function fetchPingSeries(
  token: string,
  filters: { from?: string; to?: string; show_demo?: boolean; platform?: string; v1_platforms?: boolean } = {},
): Promise<Array<{ date: string; count: number }>> {
  const params = new URLSearchParams();
  if (filters.from) params.set('from', filters.from);
  if (filters.to) params.set('to', filters.to);
  if (filters.platform) params.set('platform', filters.platform);
  if (filters.show_demo) params.set('show_demo', 'true');
  if (filters.v1_platforms) params.set('v1_platforms', 'true');
  const qs = params.toString();
  return fetchWithAuth(token, `/analytics/ping-series${qs ? `?${qs}` : ''}`);
}

export async function fetchPnL(
  token: string,
  filters: { from?: string; to?: string; show_demo?: boolean; platform?: string; v1_platforms?: boolean } = {},
): Promise<PnLResult> {
  const params = new URLSearchParams();
  if (filters.from) params.set('from', filters.from);
  if (filters.to) params.set('to', filters.to);
  if (filters.platform) params.set('platform', filters.platform);
  if (filters.show_demo) params.set('show_demo', 'true');
  if (filters.v1_platforms) params.set('v1_platforms', 'true');
  const qs = params.toString();
  return fetchWithAuth<PnLResult>(token, `/analytics/pnl${qs ? `?${qs}` : ''}`);
}

export async function fetchPlatformMix(
  token: string,
  filters: { from?: string; to?: string; platform?: string; show_demo?: boolean; v1_platforms?: boolean } = {},
): Promise<PlatformMixRow[]> {
  const params = new URLSearchParams();
  if (filters.from) params.set('from', filters.from);
  if (filters.to) params.set('to', filters.to);
  if (filters.platform) params.set('platform', filters.platform);
  if (filters.show_demo) params.set('show_demo', 'true');
  if (filters.v1_platforms) params.set('v1_platforms', 'true');
  const qs = params.toString();
  return fetchWithAuth<PlatformMixRow[]>(token, `/analytics/platform-mix${qs ? `?${qs}` : ''}`);
}

export async function fetchAlertRules(token: string): Promise<{ data: AlertRule[] }> {
  return fetchWithAuth<{ data: AlertRule[] }>(token, '/alerts/rules');
}

export async function createAlertRule(
  token: string,
  body: { name: string; condition: Record<string, unknown>; threshold: number },
): Promise<AlertRule> {
  return fetchWithAuth<AlertRule>(token, '/alerts/rules', {
    method: 'POST',
    body: JSON.stringify(body),
  });
}

export async function fetchIntegrations(token: string): Promise<IntegrationRecord[]> {
  return fetchWithAuth<IntegrationRecord[]>(token, '/integrations');
}

export async function fetchPlatformConnectionOptions(
  token: string,
): Promise<PlatformConnectionOption[]> {
  return fetchWithAuth<PlatformConnectionOption[]>(token, '/integrations/platform-options');
}

export async function createIntegration(
  token: string,
  body: CreateIntegrationInput,
): Promise<IntegrationRecord> {
  return fetchWithAuth<IntegrationRecord>(token, '/integrations', {
    method: 'POST',
    body: JSON.stringify(body),
  });
}

export async function testIntegration(
  token: string,
  id: string,
): Promise<{ ok: boolean; message: string }> {
  return fetchWithAuth<{ ok: boolean; message: string }>(
    token,
    `/integrations/${id}/test`,
    { method: 'POST' },
  );
}

export async function syncIntegration(
  token: string,
  id: string,
): Promise<{ status: string }> {
  return fetchWithAuth<{ status: string }>(token, `/integrations/${id}/sync`, {
    method: 'POST',
  });
}

export async function backfillIntegration(
  token: string,
  id: string,
): Promise<{ status: string }> {
  return fetchWithAuth<{ status: string }>(token, `/integrations/${id}/backfill`, {
    method: 'POST',
  });
}

/**
 * Outcome of a write-back (Ringba or Retreaver). Mirrors the platform-sync `writeback.Result`.
 * The service re-reads the call from Ringba after writing (`WRITEBACK_SETTLE_SECONDS`, default 20s), so
 * `has_converted` / `conversion_amount` are what Ringba actually reports back — not what we sent.
 */
export interface WritebackResult {
  outcome: string;
  inbound_call_id: string;
  campaign_id?: string;
  webhook_name?: string;
  amount: number;
  has_converted: boolean;
  conversion_amount: number;
  audit_id?: string;
  message?: string;
  offline_conversion_uploaded?: string;
  ringba_response_ref?: string;
  void_reversible?: boolean;
  /** Set for every platform except Ringba (Stage 4b). */
  platform?: string;
  /** Retreaver read-back before/after the write. */
  read_back?: Record<string, unknown>;
}

/**
 * Mark a call converted in Ringba at `amount`.
 *
 * The amount is not a label: it is sent to Ringba as `call_revenue` and becomes that call's
 * conversion value, which syncs back into Calyx as the call's revenue.
 */
export async function convertCall(
  token: string,
  integrationId: string,
  inboundCallId: string,
  amount: number,
): Promise<WritebackResult> {
  return fetchWithAuth<WritebackResult>(
    token,
    `/integrations/${integrationId}/writeback/convert`,
    {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ inbound_call_id: inboundCallId, amount }),
    },
  );
}

/** Void (cancel) a conversion in Ringba. Reversible — the call can be converted again. */
export async function voidCall(
  token: string,
  integrationId: string,
  inboundCallId: string,
): Promise<WritebackResult> {
  return fetchWithAuth<WritebackResult>(
    token,
    `/integrations/${integrationId}/writeback/void`,
    {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ inbound_call_id: inboundCallId }),
    },
  );
}

/**
 * Pick the integration write-back should go through: a live Ringba API-key connection.
 * Returns null when none exists — callers hide the Convert / Cancel controls rather than
 * letting the operator click into a 403.
 */
export function resolveWritebackIntegration(
  integrations: IntegrationRecord[],
  platform: string,
): IntegrationRecord | null {
  return (
    integrations.find(
      (i) =>
        i.platform === platform &&
        i.connection_mode === 'api_sync' &&
        i.sync_mode === 'real',
    ) ?? null
  );
}

export async function updateIntegrationSyncMode(
  token: string,
  id: string,
  syncMode: 'mock' | 'real',
  initialLookbackDays?: number,
): Promise<IntegrationRecord> {
  const body: { sync_mode: 'mock' | 'real'; initial_lookback_days?: number } = {
    sync_mode: syncMode,
  };
  if (initialLookbackDays !== undefined) {
    body.initial_lookback_days = initialLookbackDays;
  }
  return fetchWithAuth<IntegrationRecord>(token, `/integrations/${id}/sync-mode`, {
    method: 'PATCH',
    body: JSON.stringify(body),
  });
}

export async function fetchCoverageHeatmap(
  token: string,
  options: { show_demo?: boolean; from?: string; to?: string; platform?: string; v1_platforms?: boolean } = {},
): Promise<HeatmapResult> {
  const params = new URLSearchParams();
  if (options.show_demo) params.set('show_demo', 'true');
  if (options.from) params.set('from', options.from);
  if (options.to) params.set('to', options.to);
  if (options.platform) params.set('platform', options.platform);
  if (options.v1_platforms) params.set('v1_platforms', 'true');
  const qs = params.toString();
  return fetchWithAuth<HeatmapResult>(token, `/coverage/heatmap${qs ? `?${qs}` : ''}`);
}

export interface RiskScoreRecord {
  tenant_id: string;
  call_id: string;
  publisher_id: string;
  state: string;
  risk_score: number;
  dnc_match: number;
  attestation_level: string;
  exposure_usd: number;
  created_at: string;
}

export interface ViolationRecord {
  tenant_id: string;
  call_id: string;
  violation_type: string;
  risk_score: number;
  exposure_usd: number;
  created_at: string;
}

export interface ExposureSummary {
  total_exposure_usd: number;
  violation_count: number;
  avg_risk_score: number;
  dnc_match_count: number;
}

export async function fetchComplianceRiskScores(
  token: string,
  limit = 50,
  options: { show_demo?: boolean } = {},
): Promise<{ data: RiskScoreRecord[] }> {
  const params = new URLSearchParams({ limit: String(limit) });
  if (options.show_demo) params.set('show_demo', 'true');
  return fetchWithAuth<{ data: RiskScoreRecord[] }>(
    token,
    `/compliance/risk-scores?${params.toString()}`,
  );
}

export async function fetchComplianceViolations(
  token: string,
  limit = 50,
  options: { show_demo?: boolean } = {},
): Promise<{ data: ViolationRecord[] }> {
  const params = new URLSearchParams({ limit: String(limit) });
  if (options.show_demo) params.set('show_demo', 'true');
  return fetchWithAuth<{ data: ViolationRecord[] }>(
    token,
    `/compliance/violations?${params.toString()}`,
  );
}

export async function fetchComplianceExposure(
  token: string,
  options: { show_demo?: boolean } = {},
): Promise<ExposureSummary> {
  const params = new URLSearchParams();
  if (options.show_demo) params.set('show_demo', 'true');
  const qs = params.toString();
  return fetchWithAuth<ExposureSummary>(
    token,
    `/compliance/exposure${qs ? `?${qs}` : ''}`,
  );
}

export interface AskResponse {
  query_id: string;
  question: string;
  answer: string;
  intent: string;
  cache_hit: boolean;
  row_count: number;
  tokens_used: number;
}

export interface AskHistoryRecord {
  id: string;
  question: string;
  answer: string;
  intent: string;
  cache_hit: boolean;
  row_count: number;
  tokens_used: number;
  created_at: string;
}

export async function postAsk(token: string, question: string): Promise<AskResponse> {
  return fetchWithAuth<AskResponse>(token, '/ask', {
    method: 'POST',
    body: JSON.stringify({ question }),
  });
}

export async function fetchAskHistory(
  token: string,
  limit = 20,
): Promise<{ data: AskHistoryRecord[] }> {
  const params = new URLSearchParams({ limit: String(limit) });
  return fetchWithAuth<{ data: AskHistoryRecord[] }>(
    token,
    `/ask/history?${params.toString()}`,
  );
}

export interface AttributionEventRecord {
  tenant_id: string;
  call_id: string;
  event_id: string;
  platform: string;
  click_id: string;
  revenue: number;
  status: string;
  http_status: number;
  error_message: string;
  fired_at: string;
}

export interface PlatformStatusSummary {
  platform: string;
  fired_count: number;
  failed_count: number;
  last_fired_at: string | null;
}

export async function fetchAttributionEvents(
  token: string,
  limit = 50,
): Promise<{ data: AttributionEventRecord[] }> {
  const params = new URLSearchParams({ limit: String(limit) });
  return fetchWithAuth<{ data: AttributionEventRecord[] }>(
    token,
    `/attribution/events?${params.toString()}`,
  );
}

export async function fetchAttributionStatus(
  token: string,
): Promise<{ data: PlatformStatusSummary[] }> {
  return fetchWithAuth<{ data: PlatformStatusSummary[] }>(token, '/attribution/status');
}

export interface CallDeepView {
  call_id: string;
  tenant_id: string;
  ping: {
    platform: string;
    publisher_id: string;
    publisher_name?: string;
    campaign_id: string;
    campaign_name?: string;
    target_id: string;
    target_name?: string;
    buyer_id?: string;
    buyer_name?: string;
    state: string;
    event_type: string;
    revenue: number;
    payout: number;
    /** Null when the platform did not report profit. A real zero is 0. */
    profit_gross?: number | null;
    duration_seconds?: number;
    call_length_seconds?: number;
    has_recording?: boolean;
    recording_known?: boolean;
    has_converted?: boolean;
    /** Stamped by the API. Same four values as the call list. */
    conversion_state?: 'converted' | 'converted_zero' | 'not_converted' | 'not_reported';
    ani_hash: string;
    received_at: string;
    occurred_at?: string;
    sync_path?: string;
  } | null;
  scoring: {
    overall_score: number;
    sentiment: string;
    rules_passed: number;
    rules_failed: number;
    duration_seconds: number;
    word_count: number;
    scored_at: string;
  } | null;
  compliance: {
    risk_score: number;
    attestation_level: string;
    exposure_usd: number;
    dnc_match: number;
    scored_at: string;
  } | null;
  attribution: {
    platform: string;
    status: string;
    click_id: string;
    revenue: number;
    http_status: number;
    fired_at: string;
  } | null;
  enrichment: {
    attribute_count: number;
    sample_attributes: EnrichmentAttribute[];
  } | null;
}

export async function fetchCallDeepView(token: string, callId: string): Promise<CallDeepView> {
  const params = new URLSearchParams({ callId });
  return fetchWithAuth<CallDeepView>(token, `/native/call-deep-view?${params.toString()}`);
}

export interface ScorecardRule {
  id: string;
  weight: number;
  question: string;
}

export interface ScorecardRecord {
  id: string;
  tenant_id: string;
  name: string;
  vertical: string;
  rules: ScorecardRule[];
  is_active: boolean;
  created_at: string;
  updated_at: string;
}

export async function fetchScorecards(
  token: string,
  limit = 25,
  offset = 0,
): Promise<{ data: ScorecardRecord[]; total: number }> {
  const params = new URLSearchParams({
    limit: String(limit),
    offset: String(offset),
  });
  return fetchWithAuth<{ data: ScorecardRecord[]; total: number }>(
    token,
    `/scorecards?${params.toString()}`,
  );
}

export async function fetchScorecard(token: string, id: string): Promise<ScorecardRecord> {
  return fetchWithAuth<ScorecardRecord>(token, `/scorecards/${id}`);
}

export async function createScorecard(
  token: string,
  body: {
    name: string;
    vertical: string;
    rules: ScorecardRule[];
    is_active?: boolean;
  },
): Promise<ScorecardRecord> {
  return fetchWithAuth<ScorecardRecord>(token, '/scorecards', {
    method: 'POST',
    body: JSON.stringify(body),
  });
}

export async function updateScorecard(
  token: string,
  id: string,
  body: Partial<{
    name: string;
    vertical: string;
    rules: ScorecardRule[];
    is_active: boolean;
  }>,
): Promise<ScorecardRecord> {
  return fetchWithAuth<ScorecardRecord>(token, `/scorecards/${id}`, {
    method: 'PATCH',
    body: JSON.stringify(body),
  });
}

export async function deleteScorecard(token: string, id: string): Promise<void> {
  return fetchWithAuthNoContent(token, `/scorecards/${id}`, { method: 'DELETE' });
}

export async function deleteAlertRule(token: string, id: string): Promise<void> {
  return fetchWithAuthNoContent(token, `/alerts/rules/${id}`, { method: 'DELETE' });
}

export async function pauseIntegration(
  token: string,
  id: string,
): Promise<IntegrationRecord> {
  return fetchWithAuth<IntegrationRecord>(token, `/integrations/${id}/pause`, { method: 'POST' });
}

export async function resumeIntegration(
  token: string,
  id: string,
): Promise<IntegrationRecord> {
  return fetchWithAuth<IntegrationRecord>(token, `/integrations/${id}/resume`, { method: 'POST' });
}

export async function deleteIntegration(token: string, id: string): Promise<void> {
  return fetchWithAuthNoContent(token, `/integrations/${id}`, { method: 'DELETE' });
}

export async function deleteProviderBinding(token: string, id: string): Promise<void> {
  return fetchWithAuthNoContent(token, `/providers/bindings/${id}`, { method: 'DELETE' });
}

export async function updateProviderBinding(
  token: string,
  id: string,
  body: {
    config_json?: Record<string, unknown>;
    is_active?: boolean;
    priority?: number;
  },
): Promise<ProviderBinding> {
  return fetchWithAuth<ProviderBinding>(token, `/providers/bindings/${id}`, {
    method: 'PATCH',
    body: JSON.stringify(body),
  });
}

export async function testProviderBinding(
  token: string,
  id: string,
  domain = 'attribution',
): Promise<Record<string, unknown>> {
  const params = new URLSearchParams({ domain });
  return fetchWithAuth<Record<string, unknown>>(
    token,
    `/providers/bindings/${id}/test?${params.toString()}`,
    { method: 'POST' },
  );
}
