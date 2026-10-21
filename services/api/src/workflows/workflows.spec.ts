import {
  BadRequestException,
  ConflictException,
  ForbiddenException,
  HttpException,
  NotFoundException,
} from '@nestjs/common';
import { FEATURE_GATE_KEY, isFeatureOpen } from '../common/feature-gate';
import { assertProductionSecrets, EnvConfig } from '../config/env';
import { EmailSender, MailTransport, smtpConfig } from './email.sender';
import {
  HttpPublisherActionClient,
  PublisherActionClient,
  PublisherActionRequest,
  PublisherActionResult,
} from './platform-pause.client';
import { mapFailure, WorkflowActionsService } from './workflow-actions.service';
import { AlertRow, IntegrationRow } from './workflow-db';
import { alertEmail, EVALUATOR_LOCK_KEY, WorkflowEvaluatorService } from './workflow-evaluator.service';
import {
  EntityCounts,
  evaluate,
  insideActiveHours,
  WorkflowMetricsService,
} from './workflow-metrics.service';
import {
  autoModeBlockReason,
  createRuleSchema,
  RINGBA_PUBLISHERS_URL,
  validateRuleSemantics,
  WorkflowRule,
} from './workflow-types';
import { WorkflowsController } from './workflows.controller';
import { WorkflowsService } from './workflows.service';

const TENANT = '11111111-1111-1111-1111-111111111111';
const ADMIN = { sub: 'a0000000-0000-4000-8000-000000000001', tenant_id: TENANT, email: 'a@x.io', role: 'admin' };
const VIEWER = { ...ADMIN, sub: 'a0000000-0000-4000-8000-000000000002', role: 'viewer' };
const RET_INT = 'b0000000-0000-4000-8000-00000000000a';
const RB_INT = 'b0000000-0000-4000-8000-00000000000b';
const NOW = new Date('2026-10-20T15:00:00Z'); // 11:00 EDT

const ENV_KEYS = [
  'FEATURE_WORKFLOWS',
  'V1_ONLY',
  'WORKFLOW_ACTIONS_ENABLED',
  'WORKFLOW_RINGBA_PAUSE_VERIFIED',
  'WORKFLOW_MAX_AUTO_PAUSES_DAILY',
  'SMTP_HOST',
  'SMTP_PASSWORD',
  'ALERT_EMAIL_TO',
  'WRITEBACK_INTERNAL_TOKEN',
  'PLATFORM_SYNC_URL',
];
const saved: Record<string, string | undefined> = {};
beforeAll(() => ENV_KEYS.forEach((k) => (saved[k] = process.env[k])));
beforeEach(() => ENV_KEYS.forEach((k) => delete process.env[k]));
afterAll(() =>
  ENV_KEYS.forEach((k) => (saved[k] === undefined ? delete process.env[k] : (process.env[k] = saved[k]))),
);

// ---- fakes -------------------------------------------------------------------------------------

function integration(over: Partial<IntegrationRow> = {}): IntegrationRow {
  return {
    id: RET_INT,
    name: 'Retreaver Live',
    platform: 'retreaver',
    connection_mode: 'api_sync',
    sync_mode: 'real',
    is_active: true,
    sync_status: 'ok',
    last_sync_at: null,
    allow_publisher_pause: true,
    ...over,
  };
}

interface State {
  integrations: IntegrationRow[];
  protectedPubs: Set<string>;
  autoToday: number;
  alerts: Array<AlertRow & { pause_hours?: number | null }>;
  pauses: Array<Record<string, unknown>>;
  audits: Array<Record<string, unknown>>;
  rules: Array<Record<string, unknown>>;
  cooldownHit: boolean;
  statements: Array<{ sql: string; params: unknown[] }>;
}

let idSeq = 0;
const nextId = () => `c0000000-0000-4000-8000-${String(++idSeq).padStart(12, '0')}`;

function fakePg(over: Partial<State> = {}) {
  const s: State = {
    integrations: [integration(), integration({ id: RB_INT, name: 'Ringba Live', platform: 'ringba' })],
    protectedPubs: new Set(),
    autoToday: 0,
    alerts: [],
    pauses: [],
    audits: [],
    rules: [],
    cooldownHit: false,
    statements: [],
    ...over,
  };
  const query = async (sql: string, params: unknown[] = []) => {
    s.statements.push({ sql, params });
    const has = (t: string) => sql.includes(t);
    if (has('FROM integrations') && has('WHERE id = $1')) {
      return { rows: s.integrations.filter((i) => i.id === params[0]) };
    }
    if (has('UPDATE integrations SET allow_publisher_pause')) {
      const i = s.integrations.find((x) => x.id === params[0]);
      if (i) i.allow_publisher_pause = params[1] as boolean;
      return { rows: i ? [i] : [], rowCount: i ? 1 : 0 };
    }
    if (has('FROM integrations') && has("connection_mode = 'api_sync'") && has('($1::text IS NULL OR platform = $1)')) {
      // integration_unhealthy: the SQL returns every API integration; the rule (status or stale) is in TS.
      return { rows: s.integrations.filter((i) => !params[0] || i.platform === params[0]) };
    }
    if (has('FROM integrations') && has('WHERE platform = $1')) {
      return { rows: s.integrations.filter((i) => i.platform === params[0]) };
    }
    if (has('FROM protected_publishers WHERE platform')) {
      return { rows: s.protectedPubs.has(`${params[0]}:${params[1]}`) ? [{ '?column?': 1 }] : [] };
    }
    if (has('COUNT(*)::int AS n FROM publisher_pauses')) return { rows: [{ n: s.autoToday }] };
    if (has("COUNT(*)::int AS n FROM alert_events WHERE status = 'open'")) {
      return { rows: [{ n: s.alerts.filter((a) => a.status === 'open').length }] };
    }
    if (has('FROM publisher_pauses') && has('WHERE platform = $1 AND publisher_id = $2')) {
      return { rows: s.pauses.filter((p) => p.platform === params[0] && p.publisher_id === params[1] && !p.unpaused_at) };
    }
    if (has('FROM publisher_pauses WHERE id = $1')) {
      return { rows: s.pauses.filter((p) => p.id === params[0] && !p.unpaused_at) };
    }
    if (has('FROM publisher_pauses') && has('unpause_at <= NOW()')) {
      return { rows: s.pauses.filter((p) => !p.unpaused_at && p.unpause_at && new Date(String(p.unpause_at)) <= NOW) };
    }
    if (has('INSERT INTO publisher_action_audit')) {
      s.audits.push({
        user_id: params[0],
        integration_id: params[1],
        platform: params[2],
        publisher_id: params[3],
        action: params[4],
        mode: params[5],
        rule_id: params[6],
        alert_event_id: params[7],
        message: params[8],
        outcome: 'rejected',
      });
      return { rows: [] };
    }
    if (has('UPDATE alert_events') && has('SET action = $2')) {
      const a = s.alerts.find((x) => x.id === params[0]);
      if (a) {
        a.action = params[1] as AlertRow['action'];
        a.action_detail = { ...a.action_detail, ...JSON.parse(String(params[2])) };
        if (params[3]) a.status = params[3] as AlertRow['status'];
      }
      return { rows: [] };
    }
    if (has('UPDATE alert_events SET emailed')) {
      const a = s.alerts.find((x) => x.id === params[0]);
      if (a) a.emailed = true;
      return { rows: [] };
    }
    if (has('INSERT INTO publisher_pauses')) {
      const id = nextId();
      s.pauses.push({
        id,
        integration_id: params[0],
        platform: params[1],
        publisher_id: params[2],
        publisher_name: params[3],
        mode: params[4],
        alert_event_id: params[5],
        rule_id: params[6],
        paused_by: params[7],
        pause_hours: params[8],
        confirmation: params[9],
      });
      return { rows: [{ id }] };
    }
    if (has('UPDATE publisher_pauses SET unpaused_at')) {
      const p = s.pauses.find((x) => x.id === params[0]);
      if (p) {
        p.unpaused_at = NOW.toISOString();
        p.unpaused_by = params[1];
      }
      return { rows: [], rowCount: p ? 1 : 0 };
    }
    if (has('FROM alert_events') && has('WHERE rule_id = $1::uuid AND entity_id = $2')) {
      return { rows: s.cooldownHit ? [{ '?column?': 1 }] : [] };
    }
    if (has('INSERT INTO alert_events')) {
      const row: AlertRow = {
        id: nextId(),
        rule_id: params[0] as string,
        trigger_type: params[1] as string,
        scope: params[2] as string,
        platform: params[3] as string,
        entity_id: params[4] as string,
        entity_name: params[5] as string,
        integration_id: params[6] as string | null,
        numbers: JSON.parse(String(params[7])),
        window_from: params[8] as string,
        window_to: params[9] as string,
        status: 'open',
        action: 'none',
        action_detail: {},
        emailed: false,
        acknowledged_at: null,
        created_at: NOW.toISOString(),
      };
      s.alerts.push(row);
      return { rows: [row] };
    }
    if (has('FROM alert_events a LEFT JOIN workflow_rules w') && has('WHERE a.id = $1')) {
      return { rows: s.alerts.filter((a) => a.id === params[0]) };
    }
    if (has('FROM workflow_rules WHERE is_active')) return { rows: s.rules };
    if (has('FROM workflow_rules WHERE id = $1')) return { rows: s.rules.filter((r) => r.id === params[0]) };
    if (has('INSERT INTO workflow_rules')) {
      const row = {
        id: nextId(),
        tenant_id: TENANT,
        name: params[0],
        trigger_type: params[1],
        params: JSON.parse(String(params[2])),
        window_hours: params[3],
        min_calls: params[4],
        scope: params[5],
        platform: params[6],
        notify_email: params[7],
        action_mode: params[8],
        suggest_since: params[8] === 'suggest' ? NOW.toISOString() : null,
        pause_hours: params[9],
        cooldown_hours: params[10],
        is_active: params[11],
      };
      s.rules.push(row);
      return { rows: [row] };
    }
    if (has('UPDATE workflow_rules') && has('SET name = $2')) {
      const r = s.rules.find((x) => x.id === params[0]);
      if (!r) return { rows: [] };
      Object.assign(r, {
        name: params[1],
        trigger_type: params[2],
        params: JSON.parse(String(params[3])),
        window_hours: params[4],
        min_calls: params[5],
        scope: params[6],
        platform: params[7],
        notify_email: params[8],
        action_mode: params[9],
        suggest_since: params[10],
        pause_hours: params[11],
        cooldown_hours: params[12],
        is_active: params[13],
      });
      return { rows: [r] };
    }
    return { rows: [], rowCount: 0 };
  };
  const pg = {
    state: s,
    withTenant: async <T>(tenant: string, fn: (c: { query: typeof query }) => Promise<T>) => {
      expect(tenant).toBe(TENANT);
      return fn({ query });
    },
    lockGranted: true,
    lockCalls: [] as string[],
    getPool: () => ({
      connect: async () => ({
        query: async (sql: string, params: unknown[] = []) => {
          pg.lockCalls.push(sql);
          if (sql.includes('pg_try_advisory_lock')) {
            expect(params).toEqual([EVALUATOR_LOCK_KEY]);
            return { rows: [{ locked: pg.lockGranted }] };
          }
          if (sql.includes('list_workflow_tenants')) return { rows: [{ tenant_id: TENANT }] };
          return { rows: [] };
        },
        release: () => undefined,
      }),
    }),
  };
  return pg;
}

function fakeClient(result: Partial<PublisherActionResult> & { ok: boolean } = { ok: true }) {
  const calls: Array<{ action: string; req: PublisherActionRequest }> = [];
  const client: PublisherActionClient & { calls: typeof calls } = {
    calls,
    call: async (action, req) => {
      calls.push({ action, req });
      if (result.ok) {
        return {
          outcome: 'confirmed',
          paused: action === 'pause',
          read_back: 'ok',
          message: 'ok',
          audit_id: 'd0000000-0000-4000-8000-000000000001',
          ...result,
        } as PublisherActionResult;
      }
      return { status: 502, outcome: 'failed', message: 'platform error', audit_id: null, ...result } as PublisherActionResult;
    },
  };
  return client;
}

function fakeEmail(enabled = false) {
  const sent: Array<{ subject: string; text: string }> = [];
  return {
    sent,
    enabled: () => enabled,
    send: async (m: { subject: string; text: string }) => {
      sent.push(m);
      return true;
    },
  } as unknown as EmailSender & { sent: typeof sent };
}

function fakeMetrics(rows: EntityCounts[] = []) {
  const calls: unknown[][] = [];
  return {
    calls,
    entityCounts: async (...args: unknown[]) => {
      calls.push(args);
      return rows;
    },
    publisherImpact: async () => ({ calls: 120, revenue: 900.5, publisher_name: 'Pub A', from: 'f', to: 't' }),
  } as unknown as WorkflowMetricsService & { calls: unknown[][] };
}

function build(opts: { pg?: ReturnType<typeof fakePg>; client?: ReturnType<typeof fakeClient>; email?: ReturnType<typeof fakeEmail>; rows?: EntityCounts[] } = {}) {
  const pg = opts.pg ?? fakePg();
  const client = opts.client ?? fakeClient();
  const email = opts.email ?? fakeEmail();
  const metrics = fakeMetrics(opts.rows);
  const actions = new WorkflowActionsService(pg as never, client);
  const workflows = new WorkflowsService(pg as never, metrics, actions, email);
  const evaluator = new WorkflowEvaluatorService(pg as never, workflows, actions, email);
  return { pg, client, email, metrics, actions, workflows, evaluator };
}

function rule(over: Partial<WorkflowRule> = {}): WorkflowRule {
  return {
    id: 'e0000000-0000-4000-8000-000000000001',
    tenant_id: TENANT,
    name: 'Low conversion',
    trigger_type: 'conversion_rate_below',
    params: { threshold_pct: 10 },
    window_hours: 24,
    min_calls: 20,
    scope: 'publisher',
    platform: null,
    notify_email: false,
    action_mode: 'notify',
    suggest_since: null,
    pause_hours: null,
    cooldown_hours: 6,
    is_active: true,
    ...over,
  };
}

const longSuggest = new Date(NOW.getTime() - 8 * 86_400_000).toISOString();
const shortSuggest = new Date(NOW.getTime() - 2 * 86_400_000).toISOString();

function counts(over: Partial<EntityCounts> = {}): EntityCounts {
  return {
    platform: 'retreaver',
    entity_id: 'ID30',
    entity_name: 'Pub A',
    calls: 50,
    converted: 2,
    short_calls: 0,
    repeat_calls: 0,
    revenue: 0,
    calls_prev_week: 0,
    revenue_prev_week: 0,
    baseline_calls_7: 0,
    calls_7d_before: 0,
    ...over,
  };
}

const match = (over: Partial<EntityCounts> = {}) => {
  const c = counts(over);
  return { platform: c.platform, entity_id: c.entity_id, entity_name: c.entity_name, numbers: { calls: c.calls } };
};
const WIN = { from: '2026-10-19T15:00:00.000Z', to: NOW.toISOString() };

// ---- triggers (pure) ---------------------------------------------------------------------------

describe('triggers on deduplicated per-entity counts', () => {
  const base = { window_hours: 24, min_calls: 20 };

  it('conversion_rate_below fires under the threshold, not at or above it', () => {
    const r = evaluate({ ...base, trigger_type: 'conversion_rate_below', params: { threshold_pct: 10 } }, [
      counts({ entity_id: 'low', calls: 50, converted: 4 }), // 8%
      counts({ entity_id: 'edge', calls: 50, converted: 5 }), // 10%
    ]);
    expect(r.matches.map((m) => m.entity_id)).toEqual(['low']);
    expect(r.matches[0].numbers).toMatchObject({ calls: 50, converted: 4, conversion_rate_pct: 8, threshold_pct: 10, min_calls: 20 });
  });

  it('minimum sample: a bad window below min_calls never fires', () => {
    const r = evaluate({ ...base, trigger_type: 'conversion_rate_below', params: { threshold_pct: 10 } }, [
      counts({ calls: 3, converted: 0 }),
    ]);
    expect(r.matches).toHaveLength(0);
    expect(r.below_min_calls).toBe(1);
  });

  it('short_call_share_above', () => {
    const r = evaluate({ ...base, trigger_type: 'short_call_share_above', params: { threshold_pct: 30, short_seconds: 20 } }, [
      counts({ calls: 40, short_calls: 20 }),
      counts({ entity_id: 'ok', calls: 40, short_calls: 10 }),
    ]);
    expect(r.matches).toHaveLength(1);
    expect(r.matches[0].numbers).toMatchObject({ short_share_pct: 50, short_seconds: 20 });
  });

  it('repeat_caller_share_above', () => {
    const r = evaluate({ ...base, trigger_type: 'repeat_caller_share_above', params: { threshold_pct: 15 } }, [
      counts({ calls: 40, repeat_calls: 8 }),
    ]);
    expect(r.matches[0].numbers).toMatchObject({ repeat_calls: 8, repeat_share_pct: 20 });
  });

  it('volume_change compares with the 7-day average of the same hours, both directions', () => {
    const rows = [
      counts({ entity_id: 'down', calls: 10, baseline_calls_7: 350 }), // avg 50, -80%
      counts({ entity_id: 'up', calls: 120, baseline_calls_7: 350 }), // +140%
      counts({ entity_id: 'flat', calls: 52, baseline_calls_7: 350 }),
    ];
    const both = evaluate({ ...base, trigger_type: 'volume_change', params: { threshold_pct: 50, direction: 'both' } }, rows);
    expect(both.matches.map((m) => m.entity_id).sort()).toEqual(['down', 'up']);
    const down = evaluate({ ...base, trigger_type: 'volume_change', params: { threshold_pct: 50, direction: 'down' } }, rows);
    expect(down.matches.map((m) => m.entity_id)).toEqual(['down']);
    expect(down.matches[0].numbers).toMatchObject({ baseline_avg_calls: 50, change_pct: -80 });
  });

  it('revenue_drop vs the same window 7 days earlier', () => {
    const r = evaluate({ ...base, trigger_type: 'revenue_drop', params: { threshold_pct: 30 } }, [
      counts({ calls: 30, revenue: 400, revenue_prev_week: 1000, calls_prev_week: 60 }),
      counts({ entity_id: 'tiny', calls: 2, revenue: 0, revenue_prev_week: 50, calls_prev_week: 3 }),
    ]);
    expect(r.matches.map((m) => m.entity_id)).toEqual(['ID30']);
    expect(r.matches[0].numbers).toMatchObject({ drop_pct: 60, revenue: 400, revenue_prev_week: 1000 });
    expect(r.below_min_calls).toBe(1);
  });

  it('no_calls: zero calls now, enough calls in the 7 days before', () => {
    const r = evaluate({ ...base, trigger_type: 'no_calls', params: {} }, [
      counts({ entity_id: 'silent', calls: 0, calls_7d_before: 140 }),
      counts({ entity_id: 'rare', calls: 0, calls_7d_before: 3 }),
      counts({ entity_id: 'busy', calls: 9, calls_7d_before: 140 }),
    ]);
    expect(r.matches.map((m) => m.entity_id)).toEqual(['silent']);
  });

  it('active hours are Eastern time, across DST', () => {
    const active = { start: 9, end: 17 };
    // 2026-10-20 15:00Z = 11:00 EDT
    expect(insideActiveHours(NOW, 2, active)).toBe(true);
    expect(insideActiveHours(NOW, 3, active)).toBe(false); // window starts 08:00 ET
    // 2026-12-15 14:00Z = 09:00 EST (UTC-5)
    expect(insideActiveHours(new Date('2026-12-15T14:00:00Z'), 1, active)).toBe(false);
    expect(insideActiveHours(new Date('2026-12-15T15:00:00Z'), 1, active)).toBe(true);
    // 22:00Z = 17:00 EST: end of active hours
    expect(insideActiveHours(new Date('2026-12-15T22:00:00Z'), 1, active)).toBe(false);
    expect(insideActiveHours(NOW, 3, undefined)).toBe(true);
  });
});

describe('ClickHouse queries: dedup per call, params only', () => {
  function capture() {
    const queries: Array<{ query: string; query_params: Record<string, unknown> }> = [];
    const ch = {
      getClient: () => ({
        query: async (q: { query: string; query_params: Record<string, unknown> }) => {
          queries.push(q);
          return { json: async () => [{ ent_platform: 'retreaver', ent_id: 'ID1', ent_name: 'Pub', calls: '50', converted: '4', short_calls: '1', repeat_calls: '2', rev_cur: '120.5', calls_prev_week: '40', rev_prev_week: '300', baseline_calls_7: '350', calls_7d_before: '300' }] };
        },
      }),
    };
    return { queries, svc: new WorkflowMetricsService(ch as never) };
  }

  it('uses the dashboard dedup and passes the window as UTC unix seconds', async () => {
    const { queries, svc } = capture();
    const from = new Date('2026-10-19T15:00:00Z');
    const rows = await svc.entityCounts(TENANT, 'publisher', 'retreaver', { from, to: NOW }, 20);
    const q = queries[0];
    expect(q.query).toContain('argMax(revenue, received_at) AS rev');
    expect(q.query).toContain('argMax(platform_duplicate, received_at) AS dup');
    expect(q.query).toContain('GROUP BY platform, external_call_id');
    expect(q.query).toContain("platform IN ('ringba', 'retreaver')");
    expect(q.query).toContain("NOT LIKE 'smoke-%'");
    expect(q.query).toContain('sumIf(rev, in_cur AND dup = 0)');
    expect(q.query).toContain('conv = 1 AND rev > 0');
    // The alias trap: no aggregate alias reuses a per-call column name.
    const outerAliases = [...q.query.split('FROM (')[0].matchAll(/ AS (\w+)/g)].map((m) => m[1]);
    const perCallColumns = ['rev', 'pay', 'dup', 'conv', 'dur', 'ani', 'revenue', 'payout', 'event_at', 'publisher_id', 'campaign_id'];
    expect(outerAliases.filter((a) => perCallColumns.includes(a))).toEqual([]);
    expect(outerAliases).toContain('rev_cur');
    expect(q.query_params).toEqual({
      tenant_id: TENANT,
      platform: 'retreaver',
      from: Math.floor(from.getTime() / 1000),
      to: Math.floor(NOW.getTime() / 1000),
      scan_from: Math.floor(from.getTime() / 1000) - 7 * 86400,
      short_seconds: 20,
    });
    expect(rows[0]).toMatchObject({ entity_id: 'ID1', calls: 50, revenue: 120.5, baseline_calls_7: 350 });
  });

  it('never interpolates a user value into SQL', async () => {
    const { queries, svc } = capture();
    const evil = "x' OR 1=1 --";
    await svc.publisherImpact(TENANT, 'ringba', evil, NOW);
    expect(queries[0].query).not.toContain(evil);
    expect(queries[0].query).toContain('{publisher_id:String}');
    expect(queries[0].query_params.publisher_id).toBe(evil);
  });

  it('tenant scope has no entity HAVING; publisher scope drops empty ids', async () => {
    const { queries, svc } = capture();
    await svc.entityCounts(TENANT, 'tenant', null, { from: NOW, to: NOW }, 0);
    await svc.entityCounts(TENANT, 'publisher', null, { from: NOW, to: NOW }, 0);
    expect(queries[0].query).not.toContain('HAVING');
    expect(queries[1].query).toContain("HAVING ent_id != ''");
    expect(queries[0].query_params.platform).toBe('all');
  });
});

// ---- rules: validation and mode guardrails ----------------------------------------------------

describe('rule validation', () => {
  const input = (over: Record<string, unknown> = {}) =>
    createRuleSchema.parse({ name: 'r', trigger_type: 'conversion_rate_below', params: { threshold_pct: 5 }, ...over });

  it('params are strict per trigger type', () => {
    expect(validateRuleSemantics(input()).ok).toBe(true);
    expect(validateRuleSemantics(input({ params: { threshold_pct: 5, extra: 1 } })).ok).toBe(false);
    expect(validateRuleSemantics(input({ trigger_type: 'short_call_share_above' })).ok).toBe(false); // needs short_seconds
    expect(() => createRuleSchema.parse({ name: 'r', trigger_type: 'conversion_rate_below', bogus: true })).toThrow();
  });

  it('integration_unhealthy is notify only; suggest/auto need publisher scope; no_calls needs campaign scope', () => {
    expect(validateRuleSemantics(input({ trigger_type: 'integration_unhealthy', params: {}, action_mode: 'suggest' })).ok).toBe(false);
    expect(validateRuleSemantics(input({ scope: 'campaign', action_mode: 'suggest' })).ok).toBe(false);
    expect(validateRuleSemantics(input({ trigger_type: 'no_calls', params: {}, scope: 'publisher' })).ok).toBe(false);
    expect(
      validateRuleSemantics(input({ trigger_type: 'no_calls', scope: 'campaign', window_hours: 3, params: { active_hours: { start: 9, end: 17 } } })).ok,
    ).toBe(true);
  });

  it('QA triggers are accepted by the schema', () => {
    expect(validateRuleSemantics(input({ trigger_type: 'qa_score_below', params: { threshold: 60 } })).ok).toBe(true);
  });

  it('auto needs 7 days of suggest, and Ringba needs the verified flag', () => {
    expect(autoModeBlockReason({ platform: 'retreaver', suggest_since: null }, NOW)).toMatch(/7 days/);
    expect(autoModeBlockReason({ platform: 'retreaver', suggest_since: shortSuggest }, NOW)).toMatch(/7 days/);
    expect(autoModeBlockReason({ platform: 'retreaver', suggest_since: longSuggest }, NOW)).toBeNull();
    expect(autoModeBlockReason({ platform: 'ringba', suggest_since: longSuggest }, NOW)).toMatch(/Ringba/);
    expect(autoModeBlockReason({ platform: 'ringba', suggest_since: longSuggest }, NOW, { WORKFLOW_RINGBA_PAUSE_VERIFIED: 'true' })).toBeNull();
  });

  it('API refuses to save auto on create, and before 7 days of suggest on update', async () => {
    const { workflows, pg } = build();
    await expect(workflows.createRule(TENANT, ADMIN.sub, input({ action_mode: 'auto' }))).rejects.toBeInstanceOf(BadRequestException);
    const created = await workflows.createRule(TENANT, ADMIN.sub, input({ action_mode: 'suggest', platform: 'retreaver' }));
    expect(created.suggest_since).not.toBeNull();
    await expect(workflows.updateRule(TENANT, created.id, { action_mode: 'auto' }, NOW)).rejects.toThrow(/7 days/);
    pg.state.rules[0].suggest_since = longSuggest;
    const auto = await workflows.updateRule(TENANT, created.id, { action_mode: 'auto' }, NOW);
    expect(auto.action_mode).toBe('auto');
    expect(auto.suggest_since).toBe(longSuggest); // kept through Auto
    const back = await workflows.updateRule(TENANT, created.id, { action_mode: 'notify' }, NOW);
    expect(back.suggest_since).toBeNull();
  });

  it('API refuses auto for a Ringba rule while the Ringba pause is unverified', async () => {
    const { workflows, pg } = build();
    const created = await workflows.createRule(TENANT, ADMIN.sub, input({ action_mode: 'suggest', platform: 'ringba' }));
    pg.state.rules[0].suggest_since = longSuggest;
    await expect(workflows.updateRule(TENANT, created.id, { action_mode: 'auto' }, NOW)).rejects.toThrow(/Ringba/);
  });

  it('dry run returns matches and numbers without writing anything', async () => {
    const { workflows, pg, metrics } = build({ rows: [counts({ calls: 50, converted: 2 })] });
    const res = await workflows.dryRunUnsaved(TENANT, input({ params: { threshold_pct: 10 } }));
    expect(res.matches).toHaveLength(1);
    expect(res.matches[0].numbers.conversion_rate_pct).toBe(4);
    expect(res.window.timezone).toBe('America/New_York');
    expect(metrics.calls).toHaveLength(1);
    expect(pg.state.statements.filter((s) => /INSERT|UPDATE|DELETE/.test(s.sql))).toHaveLength(0);
  });

  it('QA triggers are skipped by the evaluator (log only)', async () => {
    const { workflows, metrics } = build();
    const res = await workflows.evaluateRule(TENANT, rule({ trigger_type: 'qa_flag_rate_above', params: { threshold_pct: 5 } }), NOW);
    expect(res.skipped_reason).toMatch(/2C/);
    expect(metrics.calls).toHaveLength(0);
  });

  it('no_calls outside Eastern active hours does not query or fire', async () => {
    const { workflows, metrics } = build();
    const res = await workflows.evaluateRule(
      TENANT,
      rule({ trigger_type: 'no_calls', scope: 'campaign', window_hours: 3, params: { active_hours: { start: 9, end: 17 } } }),
      NOW, // 11:00 ET, window from 08:00
    );
    expect(res.skipped_reason).toMatch(/active hours/);
    expect(metrics.calls).toHaveLength(0);
  });
});

// ---- firing and actions -----------------------------------------------------------------------

describe('evaluator firing: notify / suggest / auto and guardrails', () => {
  it('notify: alert with the numbers and window, no platform call', async () => {
    const { evaluator, pg, client } = build();
    expect(await evaluator.fire(TENANT, rule(), match(), WIN, NOW)).toBe(true);
    expect(pg.state.alerts).toHaveLength(1);
    expect(pg.state.alerts[0]).toMatchObject({ action: 'none', entity_id: 'ID30', platform: 'retreaver', integration_id: RET_INT });
    expect(pg.state.alerts[0].numbers).toEqual({ calls: 50 });
    expect(pg.state.alerts[0].window_from).toBe(WIN.from);
    expect(client.calls).toHaveLength(0);
  });

  it('cooldown: one alert per rule x entity per cooldown_hours', async () => {
    const pg = fakePg({ cooldownHit: true });
    const { evaluator } = build({ pg });
    expect(await evaluator.fire(TENANT, rule(), match(), WIN, NOW)).toBe(false);
    expect(pg.state.alerts).toHaveLength(0);
    const cool = pg.state.statements.find((s) => s.sql.includes('make_interval(hours => $4::int)'));
    expect(cool?.params).toEqual([rule().id, 'ID30', 'retreaver', 6]);
  });

  it('suggest: action suggested, no platform call', async () => {
    const { evaluator, pg, client } = build();
    await evaluator.fire(TENANT, rule({ action_mode: 'suggest', suggest_since: shortSuggest }), match(), WIN, NOW);
    expect(pg.state.alerts[0].action).toBe('suggested');
    expect(pg.state.alerts[0].action_detail.ringba_link).toBeUndefined();
    expect(client.calls).toHaveLength(0);
  });

  it('suggest on Ringba while unverified: link to Ringba, no call', async () => {
    const { evaluator, pg, client } = build();
    await evaluator.fire(TENANT, rule({ action_mode: 'suggest' }), match({ platform: 'ringba', entity_id: 'PUB1' }), WIN, NOW);
    expect(pg.state.alerts[0].action).toBe('suggested');
    expect(pg.state.alerts[0].action_detail.ringba_link).toBe(RINGBA_PUBLISHERS_URL);
    expect(client.calls).toHaveLength(0);
  });

  it('auto with every guardrail passed: platform-sync pause (mode auto), pause row with timer, alert paused', async () => {
    process.env.WORKFLOW_ACTIONS_ENABLED = 'true';
    const { evaluator, pg, client } = build();
    await evaluator.fire(TENANT, rule({ action_mode: 'auto', suggest_since: longSuggest, pause_hours: 4 }), match(), WIN, NOW);
    expect(client.calls).toHaveLength(1);
    expect(client.calls[0]).toMatchObject({
      action: 'pause',
      req: { tenant_id: TENANT, integration_id: RET_INT, publisher_id: 'ID30', user_id: null, mode: 'auto', rule_id: rule().id },
    });
    expect(client.calls[0].req.alert_event_id).toBe(pg.state.alerts[0].id);
    expect(pg.state.pauses[0]).toMatchObject({ mode: 'auto', pause_hours: 4, confirmation: 'read_back', paused_by: null });
    expect(pg.state.alerts[0]).toMatchObject({ action: 'paused', status: 'actioned' });
    expect(pg.state.audits).toHaveLength(0); // platform-sync audits the attempt itself
  });

  it('Retreaver confirmed by response is recorded as confirmation "response"', async () => {
    process.env.WORKFLOW_ACTIONS_ENABLED = 'true';
    const client = fakeClient({ ok: true, outcome: 'confirmed_by_response', read_back: 'none' });
    const { evaluator, pg } = build({ client });
    await evaluator.fire(TENANT, rule({ action_mode: 'auto', suggest_since: longSuggest }), match(), WIN, NOW);
    expect(pg.state.pauses[0].confirmation).toBe('response');
    expect(pg.state.alerts[0].action_detail).toMatchObject({ outcome: 'confirmed_by_response', read_back: 'none' });
  });

  it('kill switch off: alert only, no platform call, audit rejected', async () => {
    const { evaluator, pg, client } = build();
    await evaluator.fire(TENANT, rule({ action_mode: 'auto', suggest_since: longSuggest }), match(), WIN, NOW);
    expect(client.calls).toHaveLength(0);
    expect(pg.state.alerts[0].action).toBe('pause_blocked');
    expect(pg.state.audits).toEqual([expect.objectContaining({ outcome: 'rejected', mode: 'auto', action: 'pause', user_id: null })]);
    expect(String(pg.state.audits[0].message)).toMatch(/WORKFLOW_ACTIONS_ENABLED/);
  });

  it('auto refused before 7 days of suggest (evaluator re-check): suggested + audit', async () => {
    process.env.WORKFLOW_ACTIONS_ENABLED = 'true';
    const { evaluator, pg, client } = build();
    await evaluator.fire(TENANT, rule({ action_mode: 'auto', suggest_since: shortSuggest }), match(), WIN, NOW);
    expect(client.calls).toHaveLength(0);
    expect(pg.state.alerts[0].action).toBe('suggested');
    expect(pg.state.audits).toHaveLength(1);
  });

  it('integration toggle off: blocked, audited, no call', async () => {
    process.env.WORKFLOW_ACTIONS_ENABLED = 'true';
    const pg = fakePg({ integrations: [integration({ allow_publisher_pause: false })] });
    const { evaluator, client } = build({ pg });
    await evaluator.fire(TENANT, rule({ action_mode: 'auto', suggest_since: longSuggest }), match(), WIN, NOW);
    expect(client.calls).toHaveLength(0);
    expect(pg.state.alerts[0].action).toBe('pause_blocked');
    expect(String(pg.state.audits[0].message)).toMatch(/Allow Calyx to pause/);
  });

  it('Test integration (sync_mode mock) is never paused, even with the toggle on', async () => {
    process.env.WORKFLOW_ACTIONS_ENABLED = 'true';
    const pg = fakePg({ integrations: [integration({ sync_mode: 'mock', allow_publisher_pause: true })] });
    const { evaluator, client } = build({ pg });
    await evaluator.fire(TENANT, rule({ action_mode: 'auto', suggest_since: longSuggest }), match(), WIN, NOW);
    expect(client.calls).toHaveLength(0);
    expect(pg.state.alerts[0].action).toBe('pause_blocked');
    expect(pg.state.alerts[0].integration_id).toBeNull();
  });

  it('daily auto limit: Suggest only after the limit', async () => {
    process.env.WORKFLOW_ACTIONS_ENABLED = 'true';
    process.env.WORKFLOW_MAX_AUTO_PAUSES_DAILY = '2';
    const pg = fakePg({ autoToday: 2 });
    const { evaluator, client } = build({ pg });
    await evaluator.fire(TENANT, rule({ action_mode: 'auto', suggest_since: longSuggest }), match(), WIN, NOW);
    expect(client.calls).toHaveLength(0);
    expect(pg.state.alerts[0].action).toBe('suggested');
    expect(String(pg.state.audits[0].message)).toMatch(/Daily limit of 2/);
    const countSql = pg.state.statements.find((s) => s.sql.includes("mode = 'auto'"));
    expect(countSql?.sql).toContain("AT TIME ZONE 'America/New_York'");
  });

  it('protected publisher: never auto-paused, Suggest instead', async () => {
    process.env.WORKFLOW_ACTIONS_ENABLED = 'true';
    const pg = fakePg({ protectedPubs: new Set(['retreaver:ID30']) });
    const { evaluator, client } = build({ pg });
    await evaluator.fire(TENANT, rule({ action_mode: 'auto', suggest_since: longSuggest }), match(), WIN, NOW);
    expect(client.calls).toHaveLength(0);
    expect(pg.state.alerts[0].action).toBe('suggested');
    expect(String(pg.state.audits[0].message)).toMatch(/Protected/);
  });

  it('auto on a both-platform rule: Ringba match falls back to Suggest with link while unverified', async () => {
    process.env.WORKFLOW_ACTIONS_ENABLED = 'true';
    const { evaluator, pg, client } = build();
    await evaluator.fire(TENANT, rule({ action_mode: 'auto', suggest_since: longSuggest }), match({ platform: 'ringba', entity_id: 'P9' }), WIN, NOW);
    expect(client.calls).toHaveLength(0);
    expect(pg.state.alerts[0].action).toBe('suggested');
    expect(pg.state.alerts[0].action_detail.ringba_link).toBe(RINGBA_PUBLISHERS_URL);
  });

  it('auto platform failure (502) → pause_failed; platform 403 → pause_blocked', async () => {
    process.env.WORKFLOW_ACTIONS_ENABLED = 'true';
    const a = build({ client: fakeClient({ ok: false, status: 502 }) });
    await a.evaluator.fire(TENANT, rule({ action_mode: 'auto', suggest_since: longSuggest }), match(), WIN, NOW);
    expect(a.pg.state.alerts[0].action).toBe('pause_failed');
    const b = build({ client: fakeClient({ ok: false, status: 403, outcome: 'rejected' }) });
    await b.evaluator.fire(TENANT, rule({ action_mode: 'auto', suggest_since: longSuggest }), match(), WIN, NOW);
    expect(b.pg.state.alerts[0].action).toBe('pause_blocked');
    expect(b.pg.state.pauses).toHaveLength(0);
  });

  it('integration_unhealthy never pauses, even if a rule row says auto', async () => {
    process.env.WORKFLOW_ACTIONS_ENABLED = 'true';
    const pg = fakePg({ integrations: [integration({ sync_status: 'error' }), integration({ id: RB_INT, platform: 'ringba', sync_status: 'degraded' }), integration({ id: 'b0000000-0000-4000-8000-0000000000cc', sync_mode: 'mock', sync_status: 'error' })] });
    const { evaluator, workflows, client } = build({ pg });
    const r = rule({ trigger_type: 'integration_unhealthy', params: {}, action_mode: 'auto', suggest_since: longSuggest, scope: 'tenant' });
    const res = await workflows.evaluateRule(TENANT, r, NOW);
    expect(res.matches.map((m) => m.entity_id).sort()).toEqual([RET_INT, RB_INT].sort()); // Test integration ignored
    for (const m of res.matches) await evaluator.fire(TENANT, r, m, res.window, NOW);
    expect(client.calls).toHaveLength(0);
    expect(pg.state.alerts.every((a) => a.action === 'none')).toBe(true);
  });

  it('integration_unhealthy also fires on failed and on a stale last success (v1 sync health)', async () => {
    const ago = (min: number) => new Date(NOW.getTime() - min * 60_000).toISOString();
    const id = (n: number) => `b0000000-0000-4000-8000-0000000001${String(n).padStart(2, '0')}`;
    const pg = fakePg({
      integrations: [
        integration({ id: id(1), name: 'failed', sync_status: 'failed', last_success_at: ago(5) }),
        integration({ id: id(2), name: 'ok but stale', sync_status: 'ok', last_success_at: ago(45) }),
        integration({ id: id(3), name: 'ok and fresh', sync_status: 'ok', last_success_at: ago(5) }),
        integration({ id: id(4), name: 'syncing, never succeeded since 000034, old last_sync_at', sync_status: 'syncing', last_success_at: null, last_sync_at: ago(90) }),
        integration({ id: id(5), name: 'no success yet, last_sync_at fresh (just after the deploy)', sync_status: 'ok', last_success_at: null, last_sync_at: ago(3) }),
        integration({ id: id(6), name: 'brand new, 10 min old', sync_status: 'idle', last_success_at: null, last_sync_at: null, created_at: ago(10) }),
        integration({ id: id(7), name: 'never synced, 40 min old', sync_status: 'idle', last_success_at: null, last_sync_at: null, created_at: ago(40) }),
        integration({ id: id(8), name: 'paused and stale', sync_status: 'ok', is_active: false, last_success_at: ago(600) }),
        integration({ id: id(9), name: 'Test (mock) and stale', sync_mode: 'mock', sync_status: 'ok', last_success_at: ago(600) }),
      ],
    });
    const { workflows } = build({ pg });
    const res = await workflows.evaluateRule(TENANT, rule({ trigger_type: 'integration_unhealthy', params: {}, action_mode: 'notify', scope: 'tenant' }), NOW);
    const got = Object.fromEntries(res.matches.map((m) => [m.entity_name, m.numbers.reason]));
    expect(got).toEqual({
      failed: 'sync_status failed',
      'ok but stale': 'no successful sync for 30+ minutes',
      'syncing, never succeeded since 000034, old last_sync_at': 'no successful sync for 30+ minutes',
      'never synced, 40 min old': 'no successful sync for 30+ minutes',
    });
    expect(res.matches.find((m) => m.entity_name === 'ok but stale')?.numbers.last_success_at).toBe(ago(45));
  });

  it('email: skipped when SMTP is off, sent and flagged when on', async () => {
    const off = build({ email: fakeEmail(false) });
    await off.evaluator.fire(TENANT, rule({ notify_email: true }), match(), WIN, NOW);
    expect(off.email.sent).toHaveLength(0);
    expect(off.pg.state.alerts[0].emailed).toBe(false);
    const on = build({ email: fakeEmail(true) });
    await on.evaluator.fire(TENANT, rule({ notify_email: true }), match(), WIN, NOW);
    expect(on.email.sent).toHaveLength(1);
    expect(on.email.sent[0].subject).toContain('Low conversion');
    expect(on.pg.state.alerts[0].emailed).toBe(true);
  });
});

describe('manual pause (Suggest confirm) and unpause', () => {
  async function suggested(over: Partial<State> = {}, platform = 'retreaver', client = fakeClient()) {
    const pg = fakePg(over);
    const ctx = build({ pg, client });
    await ctx.evaluator.fire(TENANT, rule({ action_mode: 'suggest', pause_hours: 2 }), match({ platform }), WIN, NOW);
    return { ...ctx, alertId: pg.state.alerts[0].id };
  }

  it('confirmed pause calls platform-sync with mode manual and the admin as user', async () => {
    process.env.WORKFLOW_ACTIONS_ENABLED = 'true';
    const { actions, pg, client, alertId } = await suggested();
    const res = await actions.manualPause(TENANT, ADMIN.sub, alertId);
    expect(res).toMatchObject({ status: 'paused', outcome: 'confirmed' });
    expect(client.calls[0].req).toMatchObject({ mode: 'manual', user_id: ADMIN.sub, integration_id: RET_INT });
    expect(pg.state.pauses[0]).toMatchObject({ mode: 'manual', paused_by: ADMIN.sub });
  });

  it('kill switch off → 403, audit rejected, no call', async () => {
    const { actions, pg, client, alertId } = await suggested();
    await expect(actions.manualPause(TENANT, ADMIN.sub, alertId)).rejects.toBeInstanceOf(ForbiddenException);
    expect(client.calls).toHaveLength(0);
    expect(pg.state.audits[0]).toMatchObject({ mode: 'manual', user_id: ADMIN.sub, outcome: 'rejected' });
    expect(pg.state.alerts[0].action).toBe('pause_blocked');
  });

  it('Ringba unverified → 403 with the link, no call', async () => {
    process.env.WORKFLOW_ACTIONS_ENABLED = 'true';
    const { actions, pg, client, alertId } = await suggested({}, 'ringba');
    await expect(actions.manualPause(TENANT, ADMIN.sub, alertId)).rejects.toBeInstanceOf(ForbiddenException);
    expect(client.calls).toHaveLength(0);
    expect(pg.state.alerts[0].action_detail.ringba_link).toBe(RINGBA_PUBLISHERS_URL);
  });

  it('protected publishers can still be paused by hand', async () => {
    process.env.WORKFLOW_ACTIONS_ENABLED = 'true';
    const { actions, client, alertId } = await suggested({ protectedPubs: new Set(['retreaver:ID30']) });
    await actions.manualPause(TENANT, ADMIN.sub, alertId);
    expect(client.calls).toHaveLength(1);
  });

  it('platform 409 → Conflict, 502 → Bad Gateway, alert pause_failed', async () => {
    process.env.WORKFLOW_ACTIONS_ENABLED = 'true';
    const c409 = await suggested({}, 'retreaver', fakeClient({ ok: false, status: 409, message: 'in progress' }));
    await expect(c409.actions.manualPause(TENANT, ADMIN.sub, c409.alertId)).rejects.toBeInstanceOf(ConflictException);
    expect(c409.pg.state.alerts[0].action).toBe('pause_failed');
    const c502 = await suggested({}, 'retreaver', fakeClient({ ok: false, status: 502 }));
    const err = await c502.actions.manualPause(TENANT, ADMIN.sub, c502.alertId).catch((e: unknown) => e);
    expect(err).toBeInstanceOf(HttpException);
    expect((err as HttpException).getStatus()).toBe(502);
    expect(mapFailure({ ok: false, status: 404, outcome: 'failed', message: 'x', audit_id: null })).toBeInstanceOf(NotFoundException);
  });

  it('already paused → 409 without a platform call', async () => {
    process.env.WORKFLOW_ACTIONS_ENABLED = 'true';
    const { actions, pg, client, alertId } = await suggested();
    pg.state.pauses.push({ id: 'p1', platform: 'retreaver', publisher_id: 'ID30' });
    await expect(actions.manualPause(TENANT, ADMIN.sub, alertId)).rejects.toBeInstanceOf(ConflictException);
    expect(client.calls).toHaveLength(0);
  });

  it('manual unpause calls platform-sync unpause (mode manual) and closes the pause', async () => {
    process.env.WORKFLOW_ACTIONS_ENABLED = 'true';
    const { actions, pg, client, alertId } = await suggested();
    const p = await actions.manualPause(TENANT, ADMIN.sub, alertId);
    await actions.unpause(TENANT, String(p.pause_id), 'manual', ADMIN.sub);
    expect(client.calls[1]).toMatchObject({ action: 'unpause', req: { mode: 'manual', user_id: ADMIN.sub, publisher_id: 'ID30' } });
    expect(pg.state.pauses[0].unpaused_at).toBeTruthy();
    expect(pg.state.alerts[0].action).toBe('unpaused');
  });

  it('manual unpause with kill switch off → 403 + audit; timer skips quietly', async () => {
    const pg = fakePg();
    pg.state.pauses.push({ id: 'p1', integration_id: RET_INT, platform: 'retreaver', publisher_id: 'ID30', unpause_at: '2026-10-20T10:00:00Z' });
    const { actions, evaluator, client } = build({ pg });
    await expect(actions.unpause(TENANT, 'p1', 'manual', ADMIN.sub)).rejects.toBeInstanceOf(ForbiddenException);
    expect(pg.state.audits[0]).toMatchObject({ action: 'unpause', mode: 'manual' });
    expect(await evaluator.runTimers(TENANT)).toBe(0);
    expect(client.calls).toHaveLength(0);
    expect(pg.state.audits).toHaveLength(1);
  });

  it('timer: due pauses are unpaused with mode timer and no user', async () => {
    process.env.WORKFLOW_ACTIONS_ENABLED = 'true';
    const pg = fakePg();
    pg.state.pauses.push({ id: 'p1', integration_id: RET_INT, platform: 'retreaver', publisher_id: 'ID30', unpause_at: '2026-10-20T10:00:00Z' });
    pg.state.pauses.push({ id: 'p2', integration_id: RET_INT, platform: 'retreaver', publisher_id: 'ID31', unpause_at: '2026-10-21T10:00:00Z' });
    const { evaluator, client } = build({ pg });
    expect(await evaluator.runTimers(TENANT)).toBe(1);
    expect(client.calls).toEqual([expect.objectContaining({ action: 'unpause', req: expect.objectContaining({ mode: 'timer', user_id: null, publisher_id: 'ID30' }) })]);
    expect(pg.state.pauses[0].unpaused_at).toBeTruthy();
    expect(pg.state.pauses[1].unpaused_at).toBeUndefined();
  });
});

describe('evaluator tick', () => {
  it('does nothing unless FEATURE_WORKFLOWS is exactly "true" (V1_ONLY=false is not enough)', async () => {
    process.env.V1_ONLY = 'false';
    const { evaluator, pg } = build();
    expect((await evaluator.tick(NOW)).ran).toBe(false);
    expect(pg.lockCalls).toHaveLength(0);
  });

  it('skips when another instance holds the advisory lock', async () => {
    process.env.FEATURE_WORKFLOWS = 'true';
    const pg = fakePg();
    pg.lockGranted = false;
    const { evaluator } = build({ pg });
    const res = await evaluator.tick(NOW);
    expect(res).toMatchObject({ ran: false, reason: 'another instance holds the lock' });
    expect(pg.lockCalls.some((s) => s.includes('list_workflow_tenants'))).toBe(false);
  });

  it('runs every active rule for each tenant, then unlocks', async () => {
    process.env.FEATURE_WORKFLOWS = 'true';
    const pg = fakePg({ rules: [rule() as unknown as Record<string, unknown>, rule({ id: 'e2', trigger_type: 'qa_score_below', params: { threshold: 50 } }) as unknown as Record<string, unknown>] });
    const { evaluator } = build({ pg, rows: [counts({ calls: 50, converted: 1 })] });
    const res = await evaluator.tick(NOW);
    expect(res).toMatchObject({ ran: true, tenants: 1, rules: 2, alerts: 1 });
    expect(pg.lockCalls.at(-1)).toContain('pg_advisory_unlock');
  });
});

// ---- controller: gate and admin ---------------------------------------------------------------

describe('Workflows controller', () => {
  it('is behind the workflows feature gate (404 while FEATURE_WORKFLOWS is off)', () => {
    expect(Reflect.getMetadata(FEATURE_GATE_KEY, WorkflowsController)).toBe('workflows');
    expect(isFeatureOpen('workflows', {})).toBe(false);
    expect(isFeatureOpen('workflows', { FEATURE_WORKFLOWS: 'true' })).toBe(true);
  });

  it('every route is admin only (403)', async () => {
    const { workflows, actions } = build();
    const ctrl = new WorkflowsController(workflows, actions);
    const id = 'e0000000-0000-4000-8000-000000000001';
    const calls: Array<() => unknown> = [
      () => ctrl.status(TENANT, VIEWER),
      () => ctrl.listRules(TENANT, VIEWER),
      () => ctrl.createRule(TENANT, VIEWER, {}),
      () => ctrl.dryRunUnsaved(TENANT, VIEWER, {}),
      () => ctrl.updateRule(TENANT, VIEWER, id, {}),
      () => ctrl.deleteRule(TENANT, VIEWER, id),
      () => ctrl.dryRunSaved(TENANT, VIEWER, id),
      () => ctrl.listAlerts(TENANT, VIEWER),
      () => ctrl.ack(TENANT, VIEWER, id),
      () => ctrl.pause(TENANT, VIEWER, id),
      () => ctrl.pauses(TENANT, VIEWER),
      () => ctrl.unpause(TENANT, VIEWER, id),
      () => ctrl.impact(TENANT, VIEWER, 'retreaver', 'x'),
      () => ctrl.audit(TENANT, VIEWER),
      () => ctrl.listProtected(TENANT, VIEWER),
      () => ctrl.addProtected(TENANT, VIEWER, {}),
      () => ctrl.removeProtected(TENANT, VIEWER, 'retreaver', 'x'),
      () => ctrl.integrations(TENANT, VIEWER),
      () => ctrl.allowPause(TENANT, VIEWER, id, { allow: true }),
    ];
    for (const call of calls) {
      await expect(Promise.resolve().then(call)).rejects.toBeInstanceOf(ForbiddenException);
    }
  });

  it('publisher impact carries the bold warning text for the confirm popup', async () => {
    const { workflows, actions } = build();
    const ctrl = new WorkflowsController(workflows, actions);
    const res = await ctrl.impact(TENANT, ADMIN, 'ringba', 'P1');
    expect(res).toMatchObject({ calls: 120, revenue: 900.5, warning: 'Pausing stops this publisher on ALL campaigns on Ringba.' });
  });

  it('integration toggle: only Live integrations can turn it on; off is always allowed', async () => {
    const pg = fakePg({ integrations: [integration({ sync_mode: 'mock', allow_publisher_pause: false })] });
    const { workflows, actions } = build({ pg });
    const ctrl = new WorkflowsController(workflows, actions);
    await expect(ctrl.allowPause(TENANT, ADMIN, RET_INT, { allow: true })).rejects.toBeInstanceOf(BadRequestException);
    await expect(ctrl.allowPause(TENANT, ADMIN, RET_INT, { allow: false })).resolves.toMatchObject({ allow_publisher_pause: false, live: false });
    pg.state.integrations[0].sync_mode = 'real';
    await expect(ctrl.allowPause(TENANT, ADMIN, RET_INT, { allow: true })).resolves.toMatchObject({ allow_publisher_pause: true, live: true });
  });

  it('alert list: Ringba alerts carry the link and no Pause button while unverified', async () => {
    const { workflows } = build();
    const rec = (workflows as unknown as { toAlertRecord: (r: unknown) => { ringba_link: string | null; can_pause: boolean } }).toAlertRecord({
      id: 'x', scope: 'publisher', platform: 'ringba', action: 'suggested',
    });
    expect(rec).toMatchObject({ ringba_link: RINGBA_PUBLISHERS_URL, can_pause: false });
    process.env.WORKFLOW_RINGBA_PAUSE_VERIFIED = 'true';
    const rec2 = (workflows as unknown as { toAlertRecord: (r: unknown) => { ringba_link: string | null; can_pause: boolean } }).toAlertRecord({
      id: 'x', scope: 'publisher', platform: 'ringba', action: 'suggested',
    });
    expect(rec2).toMatchObject({ ringba_link: null, can_pause: true });
  });

  it('every Postgres statement passes values as parameters', async () => {
    process.env.WORKFLOW_ACTIONS_ENABLED = 'true';
    const pg = fakePg();
    const { evaluator, actions } = build({ pg });
    const evil = "ID30'; DROP TABLE x; --";
    await evaluator.fire(TENANT, rule({ action_mode: 'suggest' }), { platform: 'retreaver', entity_id: evil, entity_name: evil, numbers: {} }, WIN, NOW);
    await actions.manualPause(TENANT, ADMIN.sub, pg.state.alerts[0].id);
    for (const s of pg.state.statements) expect(s.sql).not.toContain(evil);
    expect(pg.state.statements.some((s) => s.params.includes(evil))).toBe(true);
  });
});

// ---- HTTP client and email ---------------------------------------------------------------------

describe('platform-sync HTTP client', () => {
  it('sends Bearer token and the strict body; maps 200 and errors; never throws', async () => {
    process.env.WRITEBACK_INTERNAL_TOKEN = 'test-token-not-real';
    process.env.PLATFORM_SYNC_URL = 'http://sync.test';
    const seen: Array<{ url: string; init: RequestInit }> = [];
    const responses = [
      new Response(JSON.stringify({ outcome: 'confirmed_by_response', paused: true, read_back: 'none', message: 'ok', audit_id: 'a1' }), { status: 200 }),
      new Response(JSON.stringify({ outcome: 'failed', message: 'lock https://api.x.com/?api_key=secret', audit_id: null }), { status: 409 }),
    ];
    const client = new HttpPublisherActionClient((async (url: string, init: RequestInit) => {
      seen.push({ url, init });
      return responses.shift() as Response;
    }) as unknown as typeof fetch);
    const req: PublisherActionRequest = { tenant_id: TENANT, integration_id: RET_INT, publisher_id: 'ID30', user_id: null, mode: 'auto', rule_id: null, alert_event_id: null, numbers: {} };
    const ok = await client.call('pause', req);
    expect(ok).toMatchObject({ ok: true, outcome: 'confirmed_by_response', read_back: 'none' });
    expect(seen[0].url).toBe('http://sync.test/publisher/pause');
    expect((seen[0].init.headers as Record<string, string>).Authorization).toBe('Bearer test-token-not-real');
    expect(JSON.parse(String(seen[0].init.body))).toEqual(req);
    const bad = await client.call('unpause', req);
    expect(bad).toMatchObject({ ok: false, status: 409 });
    expect((bad as { message: string }).message).not.toContain('secret');
  });

  it('no token → failed result without a request', async () => {
    let called = false;
    const client = new HttpPublisherActionClient((async () => {
      called = true;
      return new Response('{}');
    }) as unknown as typeof fetch);
    const res = await client.call('pause', { tenant_id: TENANT, integration_id: RET_INT, publisher_id: 'x', user_id: null, mode: 'manual', rule_id: null, alert_event_id: null, numbers: {} });
    expect(res.ok).toBe(false);
    expect(called).toBe(false);
  });
});

describe('email sender', () => {
  it('is off when SMTP_HOST is empty', async () => {
    let built = 0;
    const sender = new EmailSender({ ALERT_EMAIL_TO: 'a@x.io' }, () => {
      built++;
      return { sendMail: async () => ({}) };
    });
    expect(sender.enabled()).toBe(false);
    expect(await sender.send({ subject: 's', text: 't' })).toBe(false);
    expect(built).toBe(0);
    expect(smtpConfig({ SMTP_HOST: 'smtp.x', ALERT_EMAIL_TO: '' })).toBeNull();
  });

  it('sends through the transport when configured', async () => {
    const sent: unknown[] = [];
    const transport: MailTransport = { sendMail: async (m) => sent.push(m) };
    const sender = new EmailSender(
      { SMTP_HOST: 'smtp.example.test', SMTP_PORT: '587', SMTP_USER: 'u', SMTP_PASSWORD: 'not-logged', SMTP_FROM: 'alerts@x.io', ALERT_EMAIL_TO: 'a@x.io, b@x.io' },
      () => transport,
    );
    expect(await sender.send(alertEmail(rule(), { entity_id: 'ID30', entity_name: 'Pub A', platform: 'retreaver', numbers: { calls: 50 }, window_from: WIN.from, window_to: WIN.to }))).toBe(true);
    expect(sent[0]).toMatchObject({ from: 'alerts@x.io', to: ['a@x.io', 'b@x.io'], subject: '[Calyx] Low conversion: Pub A (Retreaver)' });
    expect((sent[0] as { text: string }).text).toContain('calls: 50');
  });

  it('a transport error is reported as not sent, never thrown', async () => {
    const sender = new EmailSender({ SMTP_HOST: 'smtp.x', ALERT_EMAIL_TO: 'a@x.io' }, () => ({
      sendMail: async () => {
        throw Object.assign(new Error('auth'), { code: 'EAUTH' });
      },
    }));
    expect(await sender.send({ subject: 's', text: 't' })).toBe(false);
  });

  it('production refuses SMTP_HOST without a real SMTP_PASSWORD', () => {
    const env = {
      APP_ENV: 'production',
      JWT_SECRET: 'x'.repeat(40),
      CREDENTIAL_ENCRYPTION_KEY: 'f'.repeat(64),
      WRITEBACK_INTERNAL_TOKEN: 'prod-token-value',
      DATABASE_URL: 'postgres://u:p@h/db',
      CLICKHOUSE_URL: 'http://h:8123',
      DEEPGRAM_CALLBACK_SECRET: 'prod-deepgram',
      REDIS_URL: 'redis://:prodpass@h:6379',
    } as unknown as EnvConfig;
    const run = (extra: Partial<EnvConfig>) => {
      try {
        assertProductionSecrets({ ...env, ...extra } as EnvConfig);
        return '';
      } catch (e) {
        return (e as Error).message;
      }
    };
    expect(run({ SMTP_HOST: 'smtp.x', SMTP_PASSWORD: '' })).toContain('SMTP_PASSWORD');
    expect(run({ SMTP_HOST: 'smtp.x', SMTP_PASSWORD: 'changeme' })).toContain('SMTP_PASSWORD');
    expect(run({ SMTP_HOST: 'smtp.x', SMTP_PASSWORD: 'a-real-looking-secret' })).not.toContain('SMTP_PASSWORD');
    expect(run({})).not.toContain('SMTP_PASSWORD');
  });
});
