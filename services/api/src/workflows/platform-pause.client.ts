/** Request body of platform-sync POST /publisher/pause|unpause (docs/v2-2d-contract.md, strict). */
export interface PublisherActionRequest {
  tenant_id: string;
  integration_id: string;
  publisher_id: string;
  user_id: string | null;
  mode: 'manual' | 'auto' | 'timer';
  rule_id: string | null;
  alert_event_id: string | null;
  numbers: Record<string, unknown>;
}

export interface PublisherActionSuccess {
  ok: true;
  outcome: 'confirmed' | 'confirmed_by_response';
  paused: boolean;
  read_back: 'ok' | 'none';
  message: string;
  audit_id: string | null;
}

export interface PublisherActionFailure {
  ok: false;
  /** HTTP status from platform-sync, or 0 when it could not be reached / misconfigured. */
  status: number;
  outcome: 'rejected' | 'failed';
  message: string;
  audit_id: string | null;
}

export type PublisherActionResult = PublisherActionSuccess | PublisherActionFailure;

export interface PublisherActionClient {
  call(action: 'pause' | 'unpause', req: PublisherActionRequest): Promise<PublisherActionResult>;
}

export const PUBLISHER_ACTION_CLIENT = 'PUBLISHER_ACTION_CLIENT';

/** Keep messages short and free of URLs/query strings (platform-sync already redacts). */
export function safeMessage(m: unknown, fallback: string): string {
  const s = typeof m === 'string' && m.trim() ? m.trim() : fallback;
  return s.replace(/https?:\/\/\S+/g, '[url]').slice(0, 300);
}

/**
 * HTTP client for platform-sync's internal publisher endpoints, Bearer WRITEBACK_INTERNAL_TOKEN
 * (same as write-back). Never throws: every outcome comes back as a result the caller records.
 * The token is never logged or put in a message.
 */
export class HttpPublisherActionClient implements PublisherActionClient {
  constructor(private readonly fetchImpl: typeof fetch = fetch) {}

  async call(action: 'pause' | 'unpause', req: PublisherActionRequest): Promise<PublisherActionResult> {
    const token = process.env.WRITEBACK_INTERNAL_TOKEN ?? '';
    if (!token) {
      return { ok: false, status: 0, outcome: 'failed', message: 'Internal token is not configured', audit_id: null };
    }
    const base = process.env.PLATFORM_SYNC_URL ?? 'http://localhost:8088';
    let res: Response;
    try {
      res = await this.fetchImpl(`${base}/publisher/${action}`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
        body: JSON.stringify(req),
        signal: AbortSignal.timeout(60_000),
      });
    } catch {
      return { ok: false, status: 0, outcome: 'failed', message: 'Platform sync is unreachable', audit_id: null };
    }
    const body = (await res.json().catch(() => ({}))) as Record<string, unknown>;
    const auditId = typeof body.audit_id === 'string' ? body.audit_id : null;
    if (res.ok) {
      const outcome = body.outcome === 'confirmed_by_response' ? 'confirmed_by_response' : 'confirmed';
      return {
        ok: true,
        outcome,
        paused: body.paused === true,
        read_back: body.read_back === 'ok' ? 'ok' : 'none',
        message: safeMessage(body.message, `${action} confirmed`),
        audit_id: auditId,
      };
    }
    return {
      ok: false,
      status: res.status,
      outcome: body.outcome === 'rejected' || res.status === 403 ? 'rejected' : 'failed',
      message: safeMessage(body.message, `Platform sync ${action} failed (${res.status})`),
      audit_id: auditId,
    };
  }
}
