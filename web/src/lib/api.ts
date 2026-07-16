import { clearToken } from './auth';

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? 'http://localhost:3000';

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
  received_at: string;
  /** Source call time when known; otherwise equals received_at. Prefer for display/filters. */
  occurred_at?: string;
}

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
}

async function fetchWithAuth<T>(token: string, path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(`${API_URL}${path}`, {
    ...init,
    headers: {
      ...(init?.headers ?? {}),
      Authorization: `Bearer ${token}`,
      ...(init?.body ? { 'Content-Type': 'application/json' } : {}),
    },
  });
  if (res.status === 401) {
    clearToken();
    throw new ApiError('Session expired', 401);
  }
  if (!res.ok) {
    throw new ApiError(`Request failed: ${path}`, res.status);
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

async function fetchWithAuthNoContent(
  token: string,
  path: string,
  init?: RequestInit,
): Promise<void> {
  const res = await fetch(`${API_URL}${path}`, {
    ...init,
    headers: {
      ...(init?.headers ?? {}),
      Authorization: `Bearer ${token}`,
      ...(init?.body ? { 'Content-Type': 'application/json' } : {}),
    },
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
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ email, password }),
  });
  if (!res.ok) {
    throw new Error('Login failed');
  }
  const body = (await res.json()) as { access_token: string };
  return body.access_token;
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
  const qs = params.toString();
  const res = await fetch(`${API_URL}/pings/export${qs ? `?${qs}` : ''}`, {
    headers: { Authorization: `Bearer ${token}` },
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
}

export async function fetchScores(
  token: string,
  options: FetchScoresOptions = {},
): Promise<ScoreRecord | null | { data: ScoreRecord[] }> {
  const params = new URLSearchParams();
  if (options.callId) params.set('callId', options.callId);
  if (options.limit !== undefined) params.set('limit', String(options.limit));
  if (options.offset !== undefined) params.set('offset', String(options.offset));
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
): Promise<{ data: EnrichmentAttribute[]; total: number }> {
  const params = new URLSearchParams({ limit: String(limit) });
  return fetchWithAuth<{ data: EnrichmentAttribute[]; total: number }>(
    token,
    `/enrichment/${encodeURIComponent(aniHash)}?${params.toString()}`,
  );
}

export async function fetchAnalyticsMetrics(
  token: string,
  filters: { from?: string; to?: string } = {},
): Promise<MetricsResult> {
  const params = new URLSearchParams();
  if (filters.from) params.set('from', filters.from);
  if (filters.to) params.set('to', filters.to);
  const qs = params.toString();
  return fetchWithAuth<MetricsResult>(token, `/analytics/metrics${qs ? `?${qs}` : ''}`);
}

export async function fetchPnL(
  token: string,
  filters: { from?: string; to?: string } = {},
): Promise<PnLResult> {
  const params = new URLSearchParams();
  if (filters.from) params.set('from', filters.from);
  if (filters.to) params.set('to', filters.to);
  const qs = params.toString();
  return fetchWithAuth<PnLResult>(token, `/analytics/pnl${qs ? `?${qs}` : ''}`);
}

export async function fetchPlatformMix(
  token: string,
  filters: { from?: string; to?: string; platform?: string } = {},
): Promise<PlatformMixRow[]> {
  const params = new URLSearchParams();
  if (filters.from) params.set('from', filters.from);
  if (filters.to) params.set('to', filters.to);
  if (filters.platform) params.set('platform', filters.platform);
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

export async function fetchCoverageHeatmap(token: string): Promise<HeatmapResult> {
  return fetchWithAuth<HeatmapResult>(token, '/coverage/heatmap');
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
): Promise<{ data: RiskScoreRecord[] }> {
  const params = new URLSearchParams({ limit: String(limit) });
  return fetchWithAuth<{ data: RiskScoreRecord[] }>(
    token,
    `/compliance/risk-scores?${params.toString()}`,
  );
}

export async function fetchComplianceViolations(
  token: string,
  limit = 50,
): Promise<{ data: ViolationRecord[] }> {
  const params = new URLSearchParams({ limit: String(limit) });
  return fetchWithAuth<{ data: ViolationRecord[] }>(
    token,
    `/compliance/violations?${params.toString()}`,
  );
}

export async function fetchComplianceExposure(token: string): Promise<ExposureSummary> {
  return fetchWithAuth<ExposureSummary>(token, '/compliance/exposure');
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
    campaign_id: string;
    target_id: string;
    state: string;
    event_type: string;
    revenue: number;
    payout: number;
    ani_hash: string;
    received_at: string;
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
