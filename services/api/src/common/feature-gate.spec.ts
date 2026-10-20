import { INestApplication, Module } from '@nestjs/common';
import { APP_GUARD, NestFactory } from '@nestjs/core';
import { JwtService } from '@nestjs/jwt';
import { PassportModule } from '@nestjs/passport';
import cookieParser from 'cookie-parser';
import { AddressInfo } from 'net';

// JwtStrategy / CsrfGuard read env at construction time.
process.env.APP_ENV = 'development';
process.env.DATABASE_URL ??= 'postgres://localhost:5432/callloom';
process.env.JWT_SECRET ??= '0123456789abcdef0123456789abcdef';

import { AlertsController } from '../alerts/alerts.controller';
import { AlertsService } from '../alerts/alerts.service';
import { AnalyticsController } from '../analytics/analytics.controller';
import { AnalyticsService } from '../analytics/analytics.service';
import { AskAiController } from '../ask-ai/ask-ai.controller';
import { AskAiService } from '../ask-ai/ask-ai.service';
import { AttributionController } from '../attribution/attribution.controller';
import { AttributionService } from '../attribution/attribution.service';
import { AuthSecurityService } from '../auth/auth-security.service';
import { CsrfGuard } from '../auth/csrf.guard';
import { JwtStrategy } from '../auth/jwt.strategy';
import { ComplianceController } from '../compliance/compliance.controller';
import { ComplianceService } from '../compliance/compliance.service';
import { EnrichmentController } from '../enrichment/enrichment.controller';
import { EnrichmentApiService } from '../enrichment/enrichment.service';
import { KafkaService } from '../kafka/kafka.service';
import { ProvidersController } from '../providers/providers.controller';
import { ProvidersService } from '../providers/providers.service';
import { ScorecardsController } from '../scorecards/scorecards.controller';
import { ScorecardsService } from '../scorecards/scorecards.service';
import { ScoringController } from '../scoring/scoring.controller';
import { ScoringService } from '../scoring/scoring.service';
import { WorkflowActionsService } from '../workflows/workflow-actions.service';
import { WorkflowsController } from '../workflows/workflows.controller';
import { WorkflowsService } from '../workflows/workflows.service';
import { GlobalExceptionFilter } from './global-exception.filter';
import {
  FEATURE_FLAG_ENV,
  FEATURES,
  Feature,
  FeatureGateGuard,
  isFeatureOpen,
} from './feature-gate';

const TENANT = '11111111-1111-1111-1111-111111111111';

/** Any method call resolves to a small object; lifecycle hooks / thenable checks stay undefined. */
function stub(): unknown {
  return new Proxy(
    {},
    {
      get: (_t, prop) => {
        if (typeof prop === 'symbol' || prop === 'then' || /^(on|before)[A-Z]/.test(prop)) {
          return undefined;
        }
        return async () => ({ ok: true });
      },
    },
  );
}

/** Mirrors the real AuthModule: the real CsrfGuard registered as a global APP_GUARD. */
@Module({
  providers: [
    { provide: AuthSecurityService, useValue: new AuthSecurityService({} as never) },
    CsrfGuard,
    { provide: APP_GUARD, useClass: CsrfGuard },
  ],
})
class CsrfStubModule {}

/** Mirrors the real AppModule: FeatureGateGuard in the ROOT module, CSRF in an imported one. */
@Module({
  imports: [CsrfStubModule, PassportModule],
  controllers: [
    AlertsController,
    AttributionController,
    ComplianceController,
    EnrichmentController,
    ProvidersController,
    AskAiController,
    ScoringController,
    ScorecardsController,
    AnalyticsController,
    WorkflowsController,
  ],
  providers: [
    JwtStrategy,
    { provide: APP_GUARD, useClass: FeatureGateGuard },
    ...[
      AlertsService,
      AttributionService,
      ComplianceService,
      EnrichmentApiService,
      ProvidersService,
      AskAiService,
      ScoringService,
      KafkaService,
      ScorecardsService,
      AnalyticsService,
      WorkflowsService,
      WorkflowActionsService,
    ].map((provide) => ({ provide, useValue: stub() })),
  ],
})
class GateTestModule {}

type Route = { method: string; path: string; body?: unknown; feature: Feature };

const CLOSED_ROUTES: Route[] = [
  { method: 'GET', path: '/alerts/rules', feature: 'workflows' },
  { method: 'DELETE', path: '/alerts/rules/r1', feature: 'workflows' },
  { method: 'GET', path: '/workflows/rules', feature: 'workflows' },
  { method: 'GET', path: '/workflows/alerts', feature: 'workflows' },
  {
    method: 'POST',
    path: '/workflows/rules/dry-run',
    body: { name: 'r', trigger_type: 'conversion_rate_below', params: { threshold_pct: 5 } },
    feature: 'workflows',
  },
  {
    method: 'POST',
    path: '/workflows/alerts/c0000000-0000-4000-8000-000000000001/pause',
    feature: 'workflows',
  },
  {
    method: 'PATCH',
    path: '/workflows/integrations/c0000000-0000-4000-8000-000000000001/allow-pause',
    body: { allow: true },
    feature: 'workflows',
  },
  { method: 'GET', path: '/attribution/events', feature: 'attribution' },
  { method: 'GET', path: '/attribution/status', feature: 'attribution' },
  { method: 'GET', path: '/compliance/risk-scores', feature: 'compliance' },
  { method: 'GET', path: '/compliance/violations', feature: 'compliance' },
  { method: 'GET', path: '/compliance/exposure', feature: 'compliance' },
  { method: 'GET', path: '/enrichment/abc123', feature: 'enrichment' },
  { method: 'GET', path: '/providers/catalog', feature: 'providers' },
  { method: 'GET', path: '/providers/bindings', feature: 'providers' },
  {
    method: 'POST',
    path: '/providers/bindings',
    body: { provider: 'x', domain: 'enrichment' },
    feature: 'providers',
  },
  { method: 'PATCH', path: '/providers/bindings/b1', body: {}, feature: 'providers' },
  { method: 'DELETE', path: '/providers/bindings/b1', feature: 'providers' },
  { method: 'POST', path: '/providers/bindings/b1/test', feature: 'providers' },
  { method: 'POST', path: '/providers/resolve', body: { domain: 'enrichment' }, feature: 'providers' },
  { method: 'POST', path: '/ask', body: { question: 'how many calls?' }, feature: 'ask_ai' },
  { method: 'GET', path: '/ask/history', feature: 'ask_ai' },
  { method: 'GET', path: '/scoring/scores', feature: 'qa' },
  { method: 'GET', path: '/scoring/transcript?callId=c1', feature: 'transcription' },
  {
    method: 'POST',
    path: '/scoring/deepgram/callback',
    body: { tenant_id: TENANT, call_id: 'c1', transcript_text: 'hello' },
    feature: 'transcription',
  },
  { method: 'GET', path: '/scorecards', feature: 'qa' },
  { method: 'GET', path: '/scorecards/s1', feature: 'qa' },
  { method: 'DELETE', path: '/scorecards/s1', feature: 'qa' },
];

const FLAG_VARS = ['V1_ONLY', ...Object.values(FEATURE_FLAG_ENV).filter(Boolean)] as string[];

describe('feature gate (HTTP)', () => {
  let app: INestApplication;
  let base: string;
  let token: string;
  const saved: Record<string, string | undefined> = {};

  beforeAll(async () => {
    for (const k of FLAG_VARS) saved[k] = process.env[k];
    app = await NestFactory.create(GateTestModule, { logger: false });
    app.use(cookieParser());
    app.useGlobalFilters(new GlobalExceptionFilter());
    await app.listen(0, '127.0.0.1');
    const { port } = app.getHttpServer().address() as AddressInfo;
    base = `http://127.0.0.1:${port}`;
    token = new JwtService({ secret: process.env.JWT_SECRET }).sign({
      sub: 'u1',
      email: 'admin@example.com',
      tenant_id: TENANT,
      role: 'admin',
    });
  });

  afterAll(async () => {
    await app.close();
  });

  beforeEach(() => {
    for (const k of FLAG_VARS) delete process.env[k];
  });

  afterEach(() => {
    for (const k of FLAG_VARS) {
      if (saved[k] === undefined) delete process.env[k];
      else process.env[k] = saved[k];
    }
  });

  async function call(
    route: Route,
    opts: { auth?: boolean; headers?: Record<string, string> } = {},
  ): Promise<number> {
    const headers: Record<string, string> = { ...(opts.headers ?? {}) };
    if (opts.auth !== false) headers.authorization = `Bearer ${token}`;
    if (route.body !== undefined) headers['content-type'] = 'application/json';
    const res = await fetch(`${base}${route.path}`, {
      method: route.method,
      headers,
      body: route.body === undefined ? undefined : JSON.stringify(route.body),
    });
    return res.status;
  }

  describe.each([
    ['unset', undefined],
    ['true', 'true'],
  ])('V1_ONLY %s', (_label, value) => {
    beforeEach(() => {
      if (value !== undefined) process.env.V1_ONLY = value;
    });

    it.each(CLOSED_ROUTES.map((r) => [`${r.method} ${r.path}`, r] as const))(
      '%s -> 404 (authenticated)',
      async (_name, route) => {
        expect(await call(route)).toBe(404);
      },
    );

    it.each(CLOSED_ROUTES.map((r) => [`${r.method} ${r.path}`, r] as const))(
      '%s -> 404 (anonymous, never 401)',
      async (_name, route) => {
        expect(await call(route, { auth: false })).toBe(404);
      },
    );

    it('v1 route /analytics/metrics is unaffected', async () => {
      expect(await call({ method: 'GET', path: '/analytics/metrics', feature: 'qa' })).toBe(200);
      expect(
        await call({ method: 'GET', path: '/analytics/metrics', feature: 'qa' }, { auth: false }),
      ).toBe(401);
    });

    it('404 body matches the shape of an unknown route', async () => {
      const res = await fetch(`${base}/compliance/exposure`);
      const body = (await res.json()) as Record<string, unknown>;
      expect(res.status).toBe(404);
      expect(body).toMatchObject({
        statusCode: 404,
        error: 'Not Found',
        message: 'Cannot GET /compliance/exposure',
      });
    });
  });

  describe('guard order: feature gate runs before CsrfGuard and AuthGuard', () => {
    const csrfReject = {
      method: 'POST',
      path: '/scoring/deepgram/callback',
      body: { tenant_id: TENANT, call_id: 'c1', transcript_text: 'hi' },
      feature: 'transcription' as Feature,
    };
    // Cookie session + no Origin is rejected by the real CsrfGuard with 403.
    const cookieNoOrigin = { auth: false, headers: { cookie: 'calyx_session=x' } };

    it('closed: a request CSRF would reject still gets 404', async () => {
      expect(await call(csrfReject, cookieNoOrigin)).toBe(404);
    });

    it('open: the same request reaches CsrfGuard (403)', async () => {
      process.env.V1_ONLY = 'false';
      expect(await call(csrfReject, cookieNoOrigin)).toBe(403);
    });

    it('open: anonymous request reaches AuthGuard (401)', async () => {
      process.env.V1_ONLY = 'false';
      expect(await call({ method: 'GET', path: '/scorecards', feature: 'qa' }, { auth: false })).toBe(
        401,
      );
    });
  });

  describe('V1_ONLY=false re-opens everything', () => {
    it.each(CLOSED_ROUTES.map((r) => [`${r.method} ${r.path}`, r] as const))(
      '%s -> not 404',
      async (_name, route) => {
        process.env.V1_ONLY = 'false';
        const status = await call(route);
        expect(status).not.toBe(404);
        expect(status).toBeLessThan(500);
      },
    );
  });

  describe('a v2 flag re-opens only its own routes', () => {
    const flagged: Array<[string, Feature]> = [
      ['FEATURE_ASK_AI', 'ask_ai'],
      ['FEATURE_TRANSCRIPTION', 'transcription'],
      ['FEATURE_QA', 'qa'],
      ['FEATURE_WORKFLOWS', 'workflows'],
    ];

    it.each(flagged)('%s=true', async (flag, feature) => {
      process.env[flag] = 'true';
      for (const route of CLOSED_ROUTES) {
        const status = await call(route);
        if (route.feature === feature) expect([route.path, status]).not.toEqual([route.path, 404]);
        else expect([route.path, status]).toEqual([route.path, 404]);
      }
    });

    it('FEATURE_ASK_AI=true opens /ask and /ask/history only', async () => {
      process.env.FEATURE_ASK_AI = 'true';
      expect(await call({ method: 'GET', path: '/ask/history', feature: 'ask_ai' })).toBe(200);
      expect(await call({ method: 'GET', path: '/scoring/scores', feature: 'qa' })).toBe(404);
      expect(await call({ method: 'GET', path: '/compliance/exposure', feature: 'compliance' })).toBe(
        404,
      );
    });
  });
});

describe('isFeatureOpen', () => {
  it('everything is closed when V1_ONLY is unset or true', () => {
    for (const f of FEATURES) {
      expect(isFeatureOpen(f, {})).toBe(false);
      expect(isFeatureOpen(f, { V1_ONLY: 'true' })).toBe(false);
    }
  });

  it('V1_ONLY=false opens everything', () => {
    for (const f of FEATURES) expect(isFeatureOpen(f, { V1_ONLY: 'false' })).toBe(true);
  });

  it('each v2 flag opens exactly its feature', () => {
    for (const [feature, flag] of Object.entries(FEATURE_FLAG_ENV)) {
      if (!flag) continue;
      const open = FEATURES.filter((f) => isFeatureOpen(f, { [flag]: 'true' }));
      expect(open).toEqual([feature]);
    }
  });

  it('attribution, compliance and enrichment have no flag and no other flag opens them', () => {
    const allFlagsOn = Object.fromEntries(
      Object.values(FEATURE_FLAG_ENV)
        .filter((f): f is string => Boolean(f))
        .concat(['FEATURE_ASK_AI_TRANSCRIPTS', 'FEATURE_PROVIDERS'])
        .map((f) => [f, 'true']),
    );
    for (const f of ['attribution', 'compliance', 'enrichment', 'providers'] as Feature[]) {
      expect(FEATURE_FLAG_ENV[f]).toBeNull();
      expect(isFeatureOpen(f, allFlagsOn)).toBe(false);
    }
  });

  it('flags only count when exactly "true"', () => {
    expect(isFeatureOpen('ask_ai', { FEATURE_ASK_AI: '1' })).toBe(false);
    expect(isFeatureOpen('ask_ai', { FEATURE_ASK_AI: 'TRUE' })).toBe(false);
  });
});

describe('AppModule wiring', () => {
  it('registers FeatureGateGuard as APP_GUARD in the root module (scanned before AuthModule)', async () => {
    const { AppModule } = await import('../app.module');
    const providers = Reflect.getMetadata('providers', AppModule) as Array<{
      provide?: unknown;
      useClass?: unknown;
    }>;
    expect(providers).toContainEqual({ provide: APP_GUARD, useClass: FeatureGateGuard });
  });
});
