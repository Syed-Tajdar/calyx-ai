import {
  BadRequestException,
  ForbiddenException,
  HttpException,
  ServiceUnavailableException,
} from '@nestjs/common';
import { Request } from 'express';
import { AuthSecurityService } from '../auth/auth-security.service';
import { JwtPayload } from '../auth/jwt-payload.interface';
import { supportsWriteback, WRITEBACK_PLATFORMS } from '../common/platform-capabilities';
import { IntegrationsController } from './integrations.controller';
import { IntegrationsService } from './integrations.service';

// Stage 4b — Retreaver write-back through the API. The shared rules (admin-only, CSRF, kill
// switch, tenant flag, 409 lock) are platform-agnostic here; these tests prove them for a
// Retreaver integration and pin the forwarded body. writeback.routes.spec.ts is unchanged.

const TENANT = '11111111-1111-1111-1111-111111111111';
const RETREAVER_INTEGRATION = '33333333-3333-3333-3333-333333333333';
const RETREAVER_CALL = 'addcf985-017e-4962-be34-cf5d55e74afc';

function user(role: string): JwtPayload {
  return { sub: 'user-1', email: 'u@example.com', tenant_id: TENANT, role } as JwtPayload;
}

describe('write-back platform registry', () => {
  it('lists Ringba and Retreaver, matching the platform-sync registry', () => {
    expect([...WRITEBACK_PLATFORMS]).toEqual(['ringba', 'retreaver']);
  });

  it('supportsWriteback is case-insensitive and false for the other platforms', () => {
    expect(supportsWriteback('retreaver')).toBe(true);
    expect(supportsWriteback('Retreaver')).toBe(true);
    expect(supportsWriteback('ringba')).toBe(true);
    expect(supportsWriteback('trackdrive')).toBe(false);
    expect(supportsWriteback('callgrid')).toBe(false);
    expect(supportsWriteback('')).toBe(false);
  });

  it('platform-options reports supports_writeback per platform', () => {
    const svc = new IntegrationsService({} as never);
    const byPlatform = Object.fromEntries(
      svc.listPlatformOptions().map((o) => [o.platform, o.supports_writeback]),
    );
    expect(byPlatform.ringba).toBe(true);
    expect(byPlatform.retreaver).toBe(true);
    for (const p of Object.keys(byPlatform).filter((k) => k !== 'ringba' && k !== 'retreaver')) {
      expect(byPlatform[p]).toBe(false);
    }
  });

  it('stamps supports_writeback on every integration record', async () => {
    const rows = ['retreaver', 'ringba', 'trackdrive', 'callgrid'].map((platform, i) => ({
      id: `id-${i}`,
      platform,
      name: platform,
      connection_mode: 'api_sync',
      sync_mode: 'real',
      webhook_token: null,
      is_active: true,
      last_sync_at: null,
      sync_status: 'idle',
      last_sync_error: null,
      last_success_at: null,
      created_at: '2026-10-12',
      initial_lookback_days: 30,
      writeback_enabled: false,
    }));
    const postgres = {
      withTenant: async (_t: string, fn: (c: unknown) => unknown) =>
        fn({ query: async () => ({ rows, rowCount: rows.length }) }),
    };
    const svc = new IntegrationsService(postgres as never);
    const out = await svc.list(TENANT);
    expect(out.map((r) => [r.platform, r.supports_writeback, r.writeback_enabled])).toEqual([
      ['retreaver', true, false],
      ['ringba', true, false],
      ['trackdrive', false, false],
      ['callgrid', false, false],
    ]);
  });
});

describe('Retreaver convert / void routes', () => {
  it('returns 403 for a non-admin and never reaches the service', async () => {
    const ctrl = new IntegrationsController({
      convertWriteback: () => Promise.reject(new Error('should not run')),
      voidWriteback: () => Promise.reject(new Error('should not run')),
    } as never);
    for (const role of ['viewer', 'analyst', 'member']) {
      const u = user(role);
      await expect(
        ctrl.convertWriteback(TENANT, u, RETREAVER_INTEGRATION, {
          inbound_call_id: RETREAVER_CALL,
          amount: 1,
        }),
      ).rejects.toBeInstanceOf(ForbiddenException);
      await expect(
        ctrl.voidWriteback(TENANT, u, RETREAVER_INTEGRATION, { inbound_call_id: RETREAVER_CALL }),
      ).rejects.toBeInstanceOf(ForbiddenException);
    }
  });

  it('cookie session without a csrf token is 403 on the Retreaver integration paths', () => {
    process.env.APP_ENV = 'development';
    process.env.DATABASE_URL ??= 'postgres://localhost:5432/callloom';
    process.env.JWT_SECRET ??= '0123456789abcdef0123456789abcdef';
    const svc = new AuthSecurityService({} as never);
    for (const action of ['convert', 'void']) {
      const path = `/integrations/${RETREAVER_INTEGRATION}/writeback/${action}`;
      const req = {
        method: 'POST',
        path,
        url: path,
        headers: { origin: 'http://localhost:3101' },
        cookies: { calyx_session: 'present' },
      } as unknown as Request;
      expect(svc.assertCsrf(req)).toEqual({ ok: false, status: 403, message: 'csrf required' });
    }
  });

  it('accepts a Retreaver UUID and rejects a non-positive amount before forwarding', async () => {
    const calls: unknown[] = [];
    const ctrl = new IntegrationsController({
      convertWriteback: (...args: unknown[]) => {
        calls.push(args);
        return Promise.resolve({ outcome: 'confirmed' });
      },
    } as never);
    const admin = user('admin');
    await expect(
      ctrl.convertWriteback(TENANT, admin, RETREAVER_INTEGRATION, {
        inbound_call_id: RETREAVER_CALL,
        amount: 0,
      }),
    ).rejects.toBeInstanceOf(BadRequestException);
    await expect(
      ctrl.convertWriteback(TENANT, admin, RETREAVER_INTEGRATION, {
        inbound_call_id: RETREAVER_CALL,
        amount: 1000,
      }),
    ).resolves.toEqual({ outcome: 'confirmed' });
    expect(calls).toEqual([[TENANT, 'user-1', RETREAVER_INTEGRATION, RETREAVER_CALL, 1000]]);
  });
});

describe('Retreaver forwarding to platform-sync', () => {
  const realFetch = global.fetch;
  let sent: { url: string; init: RequestInit }[] = [];
  let reply: { status: number; body: unknown } = { status: 200, body: {} };

  beforeEach(() => {
    sent = [];
    process.env.WRITEBACK_INTERNAL_TOKEN = 'internal-test-token';
    process.env.PLATFORM_SYNC_URL = 'http://platform-sync.test';
    global.fetch = (async (url: string, init: RequestInit) => {
      sent.push({ url, init });
      return {
        status: reply.status,
        ok: reply.status >= 200 && reply.status < 300,
        json: async () => reply.body,
      };
    }) as never;
  });

  afterEach(() => {
    global.fetch = realFetch;
  });

  const svc = () => new IntegrationsService({} as never);

  it('forwards convert with the amount and nothing else — no payout, no session_notes', async () => {
    reply = { status: 200, body: { outcome: 'confirmed', platform: 'retreaver' } };
    const out = await svc().convertWriteback(TENANT, 'user-1', RETREAVER_INTEGRATION, RETREAVER_CALL, 1);
    expect(out).toEqual({ outcome: 'confirmed', platform: 'retreaver' });
    expect(sent).toHaveLength(1);
    expect(sent[0].url).toBe('http://platform-sync.test/writeback/convert');
    const body = JSON.parse(String(sent[0].init.body));
    expect(Object.keys(body).sort()).toEqual(
      ['amount', 'inbound_call_id', 'integration_id', 'tenant_id', 'user_id'].sort(),
    );
    expect(body).toEqual({
      integration_id: RETREAVER_INTEGRATION,
      tenant_id: TENANT,
      user_id: 'user-1',
      inbound_call_id: RETREAVER_CALL,
      amount: 1,
    });
    expect((sent[0].init.headers as Record<string, string>).Authorization).toBe(
      'Bearer internal-test-token',
    );
  });

  it('forwards void without an amount', async () => {
    reply = { status: 200, body: { outcome: 'confirmed' } };
    await svc().voidWriteback(TENANT, 'user-1', RETREAVER_INTEGRATION, RETREAVER_CALL);
    expect(sent[0].url).toBe('http://platform-sync.test/writeback/void');
    const body = JSON.parse(String(sent[0].init.body));
    expect('amount' in body).toBe(false);
    expect('payout' in body || 'session_notes' in body).toBe(false);
  });

  it('forwards even when the kill switch is off, so platform-sync audits the refusal (403)', async () => {
    reply = { status: 403, body: { message: 'writeback: write-back is disabled' } };
    await expect(
      svc().convertWriteback(TENANT, 'user-1', RETREAVER_INTEGRATION, RETREAVER_CALL, 1),
    ).rejects.toBeInstanceOf(ForbiddenException);
    expect(sent).toHaveLength(1);
  });

  it('maps the tenant flag refusal to 403', async () => {
    reply = {
      status: 403,
      body: { message: 'writeback: writeback_enabled is false for this integration' },
    };
    await expect(
      svc().voidWriteback(TENANT, 'user-1', RETREAVER_INTEGRATION, RETREAVER_CALL),
    ).rejects.toBeInstanceOf(ForbiddenException);
  });

  it('maps the idempotency lock to 409', async () => {
    reply = { status: 409, body: { message: 'writeback: convert already in progress for this call' } };
    const err = await svc()
      .convertWriteback(TENANT, 'user-1', RETREAVER_INTEGRATION, RETREAVER_CALL, 1)
      .catch((e: unknown) => e);
    expect(err).toBeInstanceOf(HttpException);
    expect((err as HttpException).getStatus()).toBe(409);
  });

  it('passes a pending (unconfirmed) result through as 202 with the outcome intact', async () => {
    reply = {
      status: 202,
      body: {
        outcome: 'pending',
        platform: 'retreaver',
        message: 'write accepted; read-back not yet matching after 20s (unconfirmed)',
      },
    };
    const err = await svc()
      .convertWriteback(TENANT, 'user-1', RETREAVER_INTEGRATION, RETREAVER_CALL, 1)
      .catch((e: unknown) => e);
    expect(err).toBeInstanceOf(HttpException);
    expect((err as HttpException).getStatus()).toBe(202);
    expect((err as HttpException).getResponse()).toMatchObject({ outcome: 'pending' });
  });

  it('maps a contradicting read-back (502) to 503 with the platform-sync message', async () => {
    reply = { status: 502, body: { message: 'writeback: read-back contradicts the write: …' } };
    const err = await svc()
      .convertWriteback(TENANT, 'user-1', RETREAVER_INTEGRATION, RETREAVER_CALL, 1)
      .catch((e: unknown) => e);
    expect(err).toBeInstanceOf(ServiceUnavailableException);
    expect((err as HttpException).message).toContain('read-back contradicts the write');
  });

  it('refuses with 503 when the internal token is missing, without calling platform-sync', async () => {
    process.env.WRITEBACK_INTERNAL_TOKEN = '';
    await expect(
      svc().convertWriteback(TENANT, 'user-1', RETREAVER_INTEGRATION, RETREAVER_CALL, 1),
    ).rejects.toBeInstanceOf(ServiceUnavailableException);
    expect(sent).toHaveLength(0);
  });
});
