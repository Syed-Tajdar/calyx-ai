import { ConflictException, ForbiddenException, HttpException, NotFoundException } from '@nestjs/common';
import { isFeatureOpen } from '../common/feature-gate';
import { decryptCredentialJson } from '../integrations/credential-crypto';
import { HttpRecordingClient, RecordingClient, RecordingFetch, WorkerStatus } from './recording.client';
import { Candidate, TranscriptionCandidatesService } from './transcription-candidates.service';
import { TranscriptionController } from './transcription.controller';
import { TranscriptionEnqueuerService } from './transcription-enqueuer.service';
import { TranscriptionService } from './transcription.service';

// Fake Postgres, fake ClickHouse, fake platform-sync. No real database, recording or provider.

const TENANT = '11111111-1111-1111-1111-111111111111';
const ADMIN = { sub: 'a0000000-0000-4000-8000-000000000001', tenant_id: TENANT, email: 'a@x.io', role: 'admin' };
const VIEWER = { ...ADMIN, sub: 'a0000000-0000-4000-8000-000000000002', role: 'viewer' };
const RB_INT = 'b0000000-0000-4000-8000-00000000000b';
const RT_INT = 'b0000000-0000-4000-8000-00000000000a';

beforeAll(() => {
  process.env.CREDENTIAL_ENCRYPTION_KEY = '0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef';
});
afterEach(() => {
  delete process.env.FEATURE_TRANSCRIPTION;
  delete process.env.TRANSCRIBE_SHOULD_CONVERT;
  delete process.env.TRANSCRIBE_AUTO;
  delete process.env.TRANSCRIBE_BACKFILL;
  delete process.env.V1_ONLY;
});

interface Job {
  id: string;
  platform: string;
  external_call_id: string;
  reason: string;
  state: string;
  integration_id: string;
}

function fakePg() {
  const s = {
    jobs: [] as Job[],
    audit: [] as Array<{ action: string; outcome: string; user: string }>,
    integrations: [
      { id: RB_INT, platform: 'ringba', recording_access_blob: null as Buffer | null },
      { id: RT_INT, platform: 'retreaver', recording_access_blob: null as Buffer | null },
    ],
    archiveAll: false,
  };
  const client = {
    async query(sql: string, params: unknown[] = []) {
      const has = (t: string) => sql.includes(t);
      if (has('SELECT DISTINCT ON (platform) id::text, platform FROM integrations')) {
        return { rows: s.integrations.map((i) => ({ id: i.id, platform: i.platform })), rowCount: s.integrations.length };
      }
      if (has('INSERT INTO transcription_jobs')) {
        const [, integ, platform, callId, reason] = params as string[];
        const exists = s.jobs.some((j) => j.platform === platform && j.external_call_id === callId);
        if (has('ON CONFLICT') && exists) return { rows: [], rowCount: 0 };
        const job = { id: `job-${s.jobs.length + 1}`, platform, external_call_id: callId, integration_id: integ,
          reason: has("'on_demand'") ? 'on_demand' : reason, state: 'queued', campaign_name: params[7] as string | null };
        s.jobs.push(job);
        return { rows: [], rowCount: 1 };
      }
      if (has('SELECT id::text, state FROM transcription_jobs')) {
        const j = s.jobs.find((x) => x.platform === params[0] && x.external_call_id === params[1]);
        return { rows: j ? [{ id: j.id, state: j.state }] : [], rowCount: j ? 1 : 0 };
      }
      if (has("UPDATE transcription_jobs SET state = 'queued'")) {
        const j = s.jobs.find((x) => x.id === params[0]);
        if (j) {
          j.state = 'queued';
          j.reason = 'on_demand';
        }
        return { rows: [], rowCount: j ? 1 : 0 };
      }
      if (has('SELECT id::text FROM integrations WHERE deleted_at IS NULL AND is_active')) {
        const i = s.integrations.find((x) => x.platform === params[0]);
        return { rows: i ? [{ id: i.id }] : [], rowCount: i ? 1 : 0 };
      }
      if (has("SELECT platform || '|' || external_call_id AS k FROM transcription_jobs")) {
        return { rows: s.jobs.map((j) => ({ k: `${j.platform}|${j.external_call_id}` })), rowCount: s.jobs.length };
      }
      if (has('UPDATE integrations SET recording_access_blob')) {
        const i = s.integrations.find((x) => x.id === params[0] && x.platform === 'ringba');
        if (i) i.recording_access_blob = params[1] as Buffer | null;
        return { rows: [], rowCount: i ? 1 : 0 };
      }
      if (has('SELECT id::text FROM call_recordings')) return { rows: [{ id: 'c0000000-0000-4000-8000-000000000001' }], rowCount: 1 };
      if (has('INSERT INTO recording_access_audit')) {
        const refused = has("'refused'");
        s.audit.push({ action: String(refused ? params[4] : params[5]), outcome: refused ? 'refused' : String(params[6]), user: String(params[1]) });
        return { rows: [], rowCount: 1 };
      }
      if (has('SELECT archive_all FROM transcription_settings')) return { rows: [{ archive_all: s.archiveAll }], rowCount: 1 };
      if (has('INSERT INTO transcription_settings')) {
        s.archiveAll = params[1] as boolean;
        return { rows: [], rowCount: 1 };
      }
      throw new Error(`unexpected SQL in test: ${sql.slice(0, 80)}`);
    },
  };
  return {
    state: s,
    withTenant: async <T>(_t: string, fn: (c: typeof client) => Promise<T>) => fn(client),
  };
}

function fakeCandidates(rows: Candidate[]) {
  const calls: Array<Record<string, unknown>> = [];
  return {
    calls,
    find: async (q: Record<string, unknown>) => {
      calls.push(q);
      return rows;
    },
    one: async (_t: string, platform: string, id: string) =>
      id === 'RGB1' ? { platform, external_call_id: id, occurred_at: '2026-10-20T15:00:00.000Z', duration_seconds: 240, campaign_name: 'Medicare Español' } : null,
  } as unknown as TranscriptionCandidatesService & { calls: Array<Record<string, unknown>> };
}

function fakeRecordings(result: Partial<RecordingFetch> = {}): RecordingClient & { asked: string[] } {
  const asked: string[] = [];
  return {
    asked,
    fetch: async (kind) => {
      asked.push(kind);
      return { ok: true, status: 200, contentType: 'audio/mpeg', body: Buffer.from('mp3'), sha256: 'a'.repeat(64), ...result };
    },
    status: async (): Promise<WorkerStatus> => ({
      reachable: true, enabled: true, deepgram_key_set: true, archive_configured: true, daily_cap_minutes: 800, retention_months: 24,
    }),
  };
}

const cand = (id: string, over: Partial<Candidate> = {}): Candidate => ({
  platform: 'retreaver', external_call_id: id, reason: 'converted', occurred_at: '2026-10-21T14:00:00.000Z',
  duration_seconds: 300, campaign_name: 'Final Expense', ...over,
});

function build(rows: Candidate[] = [], rec = fakeRecordings()) {
  const pg = fakePg();
  const cands = fakeCandidates(rows);
  const svc = new TranscriptionService(pg as never, cands, rec);
  return { pg, cands, rec, svc, ctl: new TranscriptionController(svc) };
}

function fakeRes() {
  const headers: Record<string, string> = {};
  let body: Buffer | null = null;
  return {
    headers,
    get body() {
      return body;
    },
    setHeader: (k: string, v: string) => (headers[k.toLowerCase()] = v),
    end: (b: Buffer) => (body = b),
  };
}

describe('feature switch', () => {
  it('transcription is closed unless FEATURE_TRANSCRIPTION=true', () => {
    expect(isFeatureOpen('transcription', {})).toBe(false);
    expect(isFeatureOpen('transcription', { FEATURE_TRANSCRIPTION: 'false' })).toBe(false);
    expect(isFeatureOpen('transcription', { FEATURE_TRANSCRIPTION: 'true' })).toBe(true);
  });

  it('the enqueue job is idle while the flag is off', async () => {
    const e = new TranscriptionEnqueuerService({ getPool: () => { throw new Error('must not touch the DB'); } } as never,
      fakeCandidates([]), {} as never);
    expect((await e.tick()).reason).toBe('FEATURE_TRANSCRIPTION is off');
  });
});

describe('queue', () => {
  it('a call is queued once, whatever enqueues it again', async () => {
    const { svc, pg } = build();
    expect(await svc.enqueue(TENANT, [cand('c1'), cand('c2')])).toBe(2);
    expect(await svc.enqueue(TENANT, [cand('c1'), cand('c3')])).toBe(1); // c1 converted later: still once
    expect(pg.state.jobs.map((j) => j.external_call_id)).toEqual(['c1', 'c2', 'c3']);
  });

  it('with the flag on but TRANSCRIBE_AUTO off (the default) the job queues nothing; the button still works', async () => {
    process.env.FEATURE_TRANSCRIPTION = 'true';
    const { svc, ctl, pg } = build();
    const e = new TranscriptionEnqueuerService({ getPool: () => { throw new Error('must not touch the DB'); } } as never,
      fakeCandidates([cand('c1')]), svc);
    expect((await e.tick()).reason).toBe('TRANSCRIBE_AUTO is off (button only)');
    await expect(ctl.transcribe(TENANT, ADMIN as never, 'ringba', 'RGB1')).resolves.toEqual({ state: 'queued', retried: false });
    expect(pg.state.jobs).toHaveLength(1);
  });

  it('the enqueue job queues the candidates of every tenant, should-convert off by default', async () => {
    process.env.FEATURE_TRANSCRIPTION = 'true';
    process.env.TRANSCRIBE_AUTO = 'true';
    const { svc, pg } = build();
    const cands = fakeCandidates([cand('c1'), cand('c2', { platform: 'ringba' })]);
    const lock = { query: async (sql: string) => (sql.includes('list_transcription_tenants') ? { rows: [{ tenant_id: TENANT }] } : { rows: [{ locked: true }] }), release: () => undefined };
    const e = new TranscriptionEnqueuerService({ getPool: () => ({ connect: async () => lock }) } as never, cands, svc);
    const r = await e.tick();
    expect(r).toMatchObject({ ran: true, tenants: 1, candidates: 2, queued: 2 });
    expect(cands.calls[0]).toMatchObject({ shouldConvert: false, minSeconds: 20, archiveAll: false });
    expect(pg.state.jobs.find((j) => j.platform === 'ringba')?.integration_id).toBe(RB_INT);
  });

  it('TRANSCRIBE_SHOULD_CONVERT=true widens the scan', async () => {
    process.env.FEATURE_TRANSCRIPTION = 'true';
    process.env.TRANSCRIBE_AUTO = 'true';
    process.env.TRANSCRIBE_SHOULD_CONVERT = 'true';
    const { svc } = build();
    const cands = fakeCandidates([]);
    const lock = { query: async (sql: string) => (sql.includes('list_transcription_tenants') ? { rows: [{ tenant_id: TENANT }] } : { rows: [{ locked: true }] }), release: () => undefined };
    await new TranscriptionEnqueuerService({ getPool: () => ({ connect: async () => lock }) } as never, cands, svc).tick();
    expect(cands.calls[0].shouldConvert).toBe(true);
  });
});

describe('on demand', () => {
  it('admins only; a transcribed call is refused; a failed one is retried', async () => {
    const { ctl, pg } = build();
    expect(() => ctl.transcribe(TENANT, VIEWER as never, 'ringba', 'RGB1')).toThrow(ForbiddenException);
    await expect(ctl.transcribe(TENANT, ADMIN as never, 'ringba', 'RGB1')).resolves.toEqual({ state: 'queued', retried: false });
    // the button carries the call's campaign (the Spanish rule) like an automatic job
    expect(pg.state.jobs[0]).toMatchObject({ reason: 'on_demand', integration_id: RB_INT, campaign_name: 'Medicare Español' });
    await expect(ctl.transcribe(TENANT, ADMIN as never, 'ringba', 'RGB1')).rejects.toThrow(ConflictException);
    pg.state.jobs[0].state = 'done';
    await expect(ctl.transcribe(TENANT, ADMIN as never, 'ringba', 'RGB1')).rejects.toThrow('already transcribed');
    pg.state.jobs[0].state = 'failed';
    await expect(ctl.transcribe(TENANT, ADMIN as never, 'ringba', 'RGB1')).resolves.toEqual({ state: 'queued', retried: true });
    expect(() => ctl.transcribe(TENANT, ADMIN as never, 'trackdrive', 'x')).toThrow('platform must be ringba or retreaver');
  });
});

describe('backfill', () => {
  it('is refused while TRANSCRIBE_BACKFILL is off (the default); the estimate still works', async () => {
    const { ctl, pg } = build([cand('c1', { duration_seconds: 600 })]);
    await expect(ctl.backfillEstimate(TENANT, ADMIN as never, '7')).resolves.toMatchObject({ calls: 1 });
    await expect(ctl.backfill(TENANT, ADMIN as never, { days: 7 })).rejects.toThrow('TRANSCRIBE_BACKFILL');
    expect(pg.state.jobs).toHaveLength(0);
  });

  it('shows calls, minutes and $ before anything is queued; then queues them', async () => {
    process.env.TRANSCRIBE_BACKFILL = 'true';
    const { ctl, pg } = build([cand('c1', { duration_seconds: 600 }), cand('c2', { duration_seconds: 0 }), cand('c3', { duration_seconds: 120 })]);
    pg.state.jobs.push({ id: 'old', platform: 'retreaver', external_call_id: 'c3', reason: 'converted', state: 'done', integration_id: RT_INT });
    const est = await ctl.backfillEstimate(TENANT, ADMIN as never, '7');
    expect(est).toMatchObject({ days: 7, calls: 2, already_queued_or_done: 1, minutes: 16, estimated_usd: 0.1, daily_cap_minutes: 800 });
    expect(pg.state.jobs).toHaveLength(1); // the estimate queued nothing
    const r = await ctl.backfill(TENANT, ADMIN as never, { days: 7 });
    expect(r).toEqual({ days: 7, queued: 2 });
    expect(pg.state.jobs.filter((j) => j.reason === 'backfill')).toHaveLength(2);
  });

  it('is limited to 30 days and admins', async () => {
    const { ctl } = build();
    expect(() => ctl.backfill(TENANT, ADMIN as never, { days: 31 })).toThrow(HttpException);
    expect(() => ctl.backfillEstimate(TENANT, VIEWER as never, '7')).toThrow(ForbiddenException);
  });
});

describe('playback and evidence (admin only, audited)', () => {
  it('a non-admin gets 403 and the attempt is audited as refused; the archive is not touched', async () => {
    const { ctl, pg, rec } = build();
    await expect(ctl.audio(TENANT, VIEWER as never, 'retreaver', 'c1', undefined, fakeRes() as never)).rejects.toThrow(ForbiddenException);
    await expect(ctl.evidence(TENANT, VIEWER as never, 'retreaver', 'c1', fakeRes() as never)).rejects.toThrow(ForbiddenException);
    expect(pg.state.audit).toEqual([
      { action: 'play', outcome: 'refused', user: VIEWER.sub },
      { action: 'evidence_pack', outcome: 'refused', user: VIEWER.sub },
    ]);
    expect(rec.asked).toEqual([]);
  });

  it('an admin play and download are served and audited', async () => {
    const { ctl, pg } = build();
    const res = fakeRes();
    await ctl.audio(TENANT, ADMIN as never, 'retreaver', 'c1', undefined, res as never);
    expect(res.body?.toString()).toBe('mp3');
    expect(res.headers['cache-control']).toBe('no-store');
    expect(res.headers['content-disposition']).toBeUndefined();
    const dl = fakeRes();
    await ctl.audio(TENANT, ADMIN as never, 'retreaver', 'c1', '1', dl as never);
    expect(dl.headers['content-disposition']).toContain('attachment');
    expect(pg.state.audit.map((a) => `${a.action}:${a.outcome}`)).toEqual(['play:served', 'download:served']);
  });

  it('a failed fetch is audited as failed and its message carries no URL', async () => {
    const { ctl, pg } = build([], fakeRecordings({ ok: false, status: 502, error: 'integrity check failed', body: Buffer.alloc(0) }));
    await expect(ctl.evidence(TENANT, ADMIN as never, 'retreaver', 'c1', fakeRes() as never)).rejects.toThrow('integrity check failed');
    expect(pg.state.audit).toEqual([{ action: 'evidence_pack', outcome: 'failed', user: ADMIN.sub }]);
  });
});

describe('Ringba Recording Access credentials', () => {
  it('are stored encrypted, never returned, and only on a Ringba integration', async () => {
    const { ctl, pg } = build();
    await expect(ctl.setRecordingAccess(TENANT, ADMIN as never, RB_INT, { username: 'calyxrecordings', password: 'pw-123' }))
      .resolves.toEqual({ recording_access_set: true });
    const blob = pg.state.integrations[0].recording_access_blob!;
    expect(blob.toString('utf8')).not.toContain('pw-123');
    expect(decryptCredentialJson(blob)).toEqual({ username: 'calyxrecordings', password: 'pw-123' });
    await expect(ctl.clearRecordingAccess(TENANT, ADMIN as never, RB_INT)).resolves.toEqual({ recording_access_set: false });
    expect(pg.state.integrations[0].recording_access_blob).toBeNull();
    await expect(ctl.setRecordingAccess(TENANT, ADMIN as never, RT_INT, { username: 'u', password: 'p' })).rejects.toThrow(NotFoundException);
    expect(() => ctl.setRecordingAccess(TENANT, VIEWER as never, RB_INT, { username: 'u', password: 'p' })).toThrow(ForbiddenException);
    expect(() => ctl.setRecordingAccess(TENANT, ADMIN as never, RB_INT, { username: 'u' })).toThrow(HttpException);
  });
});

describe('settings', () => {
  it('"Archive all recorded calls" is off by default and admin-only', async () => {
    const { svc, ctl } = build();
    expect(await svc.archiveAll(TENANT)).toBe(false);
    await expect(ctl.settings(TENANT, ADMIN as never, { archive_all: true })).resolves.toEqual({ archive_all: true });
    expect(await svc.archiveAll(TENANT)).toBe(true);
    expect(() => ctl.settings(TENANT, VIEWER as never, { archive_all: false })).toThrow(ForbiddenException);
  });
});

describe('platform-sync client', () => {
  const env = { ...process.env };
  afterEach(() => {
    process.env = { ...env };
  });

  it('refuses without the internal token, strips URLs from errors, reads the worker status', async () => {
    delete process.env.WRITEBACK_INTERNAL_TOKEN;
    expect((await new HttpRecordingClient(jest.fn()).fetch('audio', { tenant_id: TENANT, platform: 'ringba', external_call_id: 'x' })).error)
      .toBe('Internal token is not configured');
    process.env.WRITEBACK_INTERNAL_TOKEN = 'tok';
    const err = await new HttpRecordingClient((async () =>
      new Response(JSON.stringify({ error: 'archive get https://s3.example/recordings/abc failed' }), { status: 502 })) as never)
      .fetch('audio', { tenant_id: TENANT, platform: 'ringba', external_call_id: 'x' });
    expect(err.ok).toBe(false);
    expect(err.error).not.toContain('https://');
    const off = await new HttpRecordingClient((async () => new Response('', { status: 404 })) as never).status();
    expect(off).toMatchObject({ reachable: true, enabled: false });
    const on = await new HttpRecordingClient((async () => new Response(JSON.stringify({ enabled: true, deepgram_key_set: true, archive_configured: false, daily_cap_minutes: 800 }), { status: 200 })) as never).status();
    expect(on).toMatchObject({ reachable: true, enabled: true, deepgram_key_set: true, archive_configured: false, daily_cap_minutes: 800 });
  });
});
