const API_URL = process.env.NEXT_PUBLIC_API_URL ?? 'http://localhost:3000';

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
  const res = await fetch(`${API_URL}/pings?${params.toString()}`, {
    headers: { Authorization: `Bearer ${token}` },
  });
  if (!res.ok) {
    throw new Error('Failed to load pings');
  }
  return res.json() as Promise<PingListResponse>;
}
