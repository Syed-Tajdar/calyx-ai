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
}

export interface PingListResponse {
  data: PingRecord[];
  total: number;
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
}

export interface PnLResult {
  revenue: number;
  payout: number;
  ad_spend: number;
  profit: number;
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
  webhook_token: string;
  is_active: boolean;
  created_at: string;
}

export interface HeatmapStateRow {
  state: string;
  ping_count: number;
  revenue: number;
}

export interface HeatmapResult {
  data: HeatmapStateRow[];
  total_pings: number;
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
  return res.json() as Promise<T>;
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

export async function fetchPings(
  token: string,
  limit = 50,
  offset = 0,
): Promise<PingListResponse> {
  const params = new URLSearchParams({
    limit: String(limit),
    offset: String(offset),
  });
  return fetchWithAuth<PingListResponse>(token, `/pings?${params.toString()}`);
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

export async function fetchAnalyticsMetrics(token: string): Promise<MetricsResult> {
  return fetchWithAuth<MetricsResult>(token, '/analytics/metrics');
}

export async function fetchPnL(token: string): Promise<PnLResult> {
  return fetchWithAuth<PnLResult>(token, '/analytics/pnl');
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

export async function fetchCoverageHeatmap(token: string): Promise<HeatmapResult> {
  return fetchWithAuth<HeatmapResult>(token, '/coverage/heatmap');
}
