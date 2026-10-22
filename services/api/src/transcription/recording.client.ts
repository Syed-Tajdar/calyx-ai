/**
 * Client for platform-sync's internal 2B routes (compose network, Bearer WRITEBACK_INTERNAL_TOKEN).
 * The archive lives behind platform-sync; the API only proxies the verified bytes to an admin. No
 * bucket URL, signed or not, ever reaches the browser. Never throws; never logs the token.
 */

export interface RecordingRef {
  tenant_id: string;
  platform: string;
  external_call_id: string;
}

export interface RecordingFetch {
  ok: boolean;
  status: number;
  contentType: string;
  body: Buffer;
  sha256: string;
  filename?: string;
  error?: string;
}

export interface WorkerStatus {
  reachable: boolean;
  enabled: boolean;
  deepgram_key_set: boolean;
  archive_configured: boolean;
  daily_cap_minutes: number | null;
  retention_months: number | null;
}

export interface RecordingClient {
  fetch(kind: 'audio' | 'evidence', ref: RecordingRef): Promise<RecordingFetch>;
  status(): Promise<WorkerStatus>;
}

export const RECORDING_CLIENT = 'RECORDING_CLIENT';

export class HttpRecordingClient implements RecordingClient {
  constructor(private readonly fetchImpl: typeof fetch = fetch) {}

  private base(): string {
    return process.env.PLATFORM_SYNC_URL ?? 'http://localhost:8088';
  }

  private headers(): Record<string, string> | null {
    const token = process.env.WRITEBACK_INTERNAL_TOKEN ?? '';
    return token ? { Authorization: `Bearer ${token}` } : null;
  }

  async fetch(kind: 'audio' | 'evidence', ref: RecordingRef): Promise<RecordingFetch> {
    const headers = this.headers();
    const empty = { contentType: 'application/json', body: Buffer.alloc(0), sha256: '' };
    if (!headers) return { ok: false, status: 0, ...empty, error: 'Internal token is not configured' };
    const q = new URLSearchParams({
      tenant_id: ref.tenant_id,
      platform: ref.platform,
      external_call_id: ref.external_call_id,
    });
    let res: Response;
    try {
      res = await this.fetchImpl(`${this.base()}/internal/recordings/${kind}?${q.toString()}`, {
        headers,
        signal: AbortSignal.timeout(120_000),
      });
    } catch {
      return { ok: false, status: 0, ...empty, error: 'Platform sync is unreachable' };
    }
    const body = Buffer.from(await res.arrayBuffer());
    if (!res.ok) {
      let error = `Recording not available (${res.status})`;
      try {
        const j = JSON.parse(body.toString('utf8')) as { error?: string };
        if (typeof j.error === 'string') error = j.error.replace(/https?:\/\/\S+/g, '[url]').slice(0, 200);
      } catch {
        /* not JSON */
      }
      return { ok: false, status: res.status, ...empty, error };
    }
    const disposition = res.headers.get('content-disposition') ?? '';
    const m = /filename="([^"]+)"/.exec(disposition);
    return {
      ok: true,
      status: res.status,
      contentType: res.headers.get('content-type') ?? 'application/octet-stream',
      body,
      sha256: res.headers.get('x-recording-sha256') ?? '',
      filename: m?.[1],
    };
  }

  async status(): Promise<WorkerStatus> {
    const off: WorkerStatus = {
      reachable: false,
      enabled: false,
      deepgram_key_set: false,
      archive_configured: false,
      daily_cap_minutes: null,
      retention_months: null,
    };
    const headers = this.headers();
    if (!headers) return off;
    try {
      const res = await this.fetchImpl(`${this.base()}/internal/transcription/status`, {
        headers,
        signal: AbortSignal.timeout(5_000),
      });
      if (res.status === 404) return { ...off, reachable: true }; // worker off in platform-sync
      if (!res.ok) return off;
      const j = (await res.json()) as Partial<WorkerStatus>;
      return {
        reachable: true,
        enabled: j.enabled === true,
        deepgram_key_set: j.deepgram_key_set === true,
        archive_configured: j.archive_configured === true,
        daily_cap_minutes: typeof j.daily_cap_minutes === 'number' ? j.daily_cap_minutes : null,
        retention_months: typeof j.retention_months === 'number' ? j.retention_months : null,
      };
    } catch {
      return off;
    }
  }
}
