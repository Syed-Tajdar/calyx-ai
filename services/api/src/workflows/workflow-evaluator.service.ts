import { Inject, Injectable, Logger, OnModuleDestroy, OnModuleInit } from '@nestjs/common';
import { PostgresService } from '../database/postgres.service';
import { EMAIL_SENDER, EmailSender } from './email.sender';
import { ALERT_COLUMNS, AlertRow, PauseRow, PAUSE_COLUMNS, RULE_COLUMNS, TenantRunner } from './workflow-db';
import { WorkflowActionsService } from './workflow-actions.service';
import { Match } from './workflow-metrics.service';
import { WorkflowsService } from './workflows.service';
import {
  isQaTrigger,
  platformLabel,
  RINGBA_PUBLISHERS_URL,
  ringbaPauseVerified,
  workflowsEnabled,
  WorkflowRule,
} from './workflow-types';

export const EVALUATOR_INTERVAL_MS = 15 * 60 * 1000;
/** pg advisory lock key: one API instance evaluates at a time ("calyx workflows" → stable int). */
export const EVALUATOR_LOCK_KEY = 728_340_215;

/** Pool surface for the session-level advisory lock (held on one dedicated connection). */
export interface LockPool {
  connect(): Promise<{ query: (sql: string, params?: unknown[]) => Promise<{ rows: Array<Record<string, unknown>> }>; release: () => void }>;
}

export interface TickSummary {
  ran: boolean;
  reason?: string;
  tenants: number;
  rules: number;
  alerts: number;
  unpaused: number;
}

function toRule(row: Record<string, unknown>): WorkflowRule {
  return {
    id: String(row.id),
    tenant_id: String(row.tenant_id),
    name: String(row.name),
    trigger_type: row.trigger_type as WorkflowRule['trigger_type'],
    params: (row.params as Record<string, unknown>) ?? {},
    window_hours: Number(row.window_hours),
    min_calls: Number(row.min_calls),
    scope: row.scope as WorkflowRule['scope'],
    platform: (row.platform as WorkflowRule['platform']) ?? null,
    notify_email: Boolean(row.notify_email),
    action_mode: row.action_mode as WorkflowRule['action_mode'],
    suggest_since: (row.suggest_since as string | null) ?? null,
    pause_hours: row.pause_hours == null ? null : Number(row.pause_hours),
    cooldown_hours: Number(row.cooldown_hours),
    is_active: Boolean(row.is_active),
  };
}

export function alertEmail(rule: WorkflowRule, alert: Pick<AlertRow, 'entity_id' | 'entity_name' | 'platform' | 'numbers' | 'window_from' | 'window_to'>): { subject: string; text: string } {
  const who = alert.entity_name || alert.entity_id || 'tenant';
  const where = alert.platform ? ` (${platformLabel(alert.platform)})` : '';
  const lines = Object.entries(alert.numbers ?? {}).map(([k, v]) => `  ${k}: ${v}`);
  return {
    subject: `[Calyx] ${rule.name}: ${who}${where}`,
    text: [
      `Rule: ${rule.name} (${rule.trigger_type})`,
      `Entity: ${who}${where}`,
      `Window: ${alert.window_from ?? ''} to ${alert.window_to ?? ''} (UTC)`,
      'Numbers used:',
      ...lines,
      '',
      'Open Calyx > Alerts to review.',
    ].join('\n'),
  };
}

/**
 * Scheduled evaluator (every 15 minutes, Eastern-time rules, dashboard dedup data).
 * Runs only when FEATURE_WORKFLOWS is exactly "true", and only in the API instance that wins
 * `pg_try_advisory_lock`. Each tick: due unpause timers first, then every active rule.
 */
@Injectable()
export class WorkflowEvaluatorService implements OnModuleInit, OnModuleDestroy {
  private readonly logger = new Logger(WorkflowEvaluatorService.name);
  private timer: NodeJS.Timeout | null = null;
  private firstRun: NodeJS.Timeout | null = null;
  private running = false;

  constructor(
    @Inject(PostgresService) private readonly pg: TenantRunner & { getPool?: () => LockPool },
    private readonly workflows: WorkflowsService,
    private readonly actions: WorkflowActionsService,
    @Inject(EMAIL_SENDER) private readonly email: EmailSender,
  ) {}

  onModuleInit(): void {
    if (process.env.NODE_ENV === 'test') return;
    this.timer = setInterval(() => void this.tick(), EVALUATOR_INTERVAL_MS);
    this.timer.unref();
    this.firstRun = setTimeout(() => void this.tick(), 60_000);
    this.firstRun.unref();
  }

  onModuleDestroy(): void {
    if (this.timer) clearInterval(this.timer);
    if (this.firstRun) clearTimeout(this.firstRun);
  }

  async tick(now = new Date()): Promise<TickSummary> {
    const summary: TickSummary = { ran: false, tenants: 0, rules: 0, alerts: 0, unpaused: 0 };
    if (!workflowsEnabled()) return { ...summary, reason: 'FEATURE_WORKFLOWS is off' };
    if (this.running) return { ...summary, reason: 'previous tick still running' };
    const pool = this.pg.getPool?.();
    if (!pool) return { ...summary, reason: 'no pool' };
    this.running = true;
    const lockClient = await pool.connect();
    try {
      const got = await lockClient.query('SELECT pg_try_advisory_lock($1) AS locked', [EVALUATOR_LOCK_KEY]);
      if (!got.rows[0]?.locked) return { ...summary, reason: 'another instance holds the lock' };
      try {
        summary.ran = true;
        const tenants = await lockClient.query('SELECT tenant_id::text AS tenant_id FROM list_workflow_tenants()');
        for (const t of tenants.rows) {
          const tenantId = String(t.tenant_id);
          summary.tenants++;
          try {
            summary.unpaused += await this.runTimers(tenantId);
            const r = await this.runRules(tenantId, now);
            summary.rules += r.rules;
            summary.alerts += r.alerts;
          } catch (err: unknown) {
            this.logger.error(`workflow tick failed for tenant ${tenantId}: ${(err as Error).message}`);
          }
        }
      } finally {
        await lockClient.query('SELECT pg_advisory_unlock($1)', [EVALUATOR_LOCK_KEY]);
      }
      this.logger.log(
        `workflow tick: tenants=${summary.tenants} rules=${summary.rules} alerts=${summary.alerts} unpaused=${summary.unpaused}`,
      );
      return summary;
    } finally {
      lockClient.release();
      this.running = false;
    }
  }

  /** Unpause every open pause whose timer passed (platform-sync mode "timer"). */
  async runTimers(tenantId: string): Promise<number> {
    const due = await this.pg.withTenant(tenantId, async (c) => {
      const r = await c.query(
        `SELECT ${PAUSE_COLUMNS} FROM publisher_pauses
          WHERE unpaused_at IS NULL AND unpause_at IS NOT NULL AND unpause_at <= NOW()`,
      );
      return r.rows as PauseRow[];
    });
    let n = 0;
    for (const p of due) {
      try {
        await this.actions.unpause(tenantId, p.id, 'timer', null);
        n++;
      } catch (err: unknown) {
        this.logger.warn(`timer unpause skipped (${p.platform}): ${(err as Error).message}`);
      }
    }
    return n;
  }

  async runRules(tenantId: string, now = new Date()): Promise<{ rules: number; alerts: number }> {
    const rules = await this.pg.withTenant(tenantId, async (c) => {
      const r = await c.query(`SELECT ${RULE_COLUMNS} FROM workflow_rules WHERE is_active AND deleted_at IS NULL`);
      return r.rows.map(toRule);
    });
    let alerts = 0;
    for (const rule of rules) {
      if (isQaTrigger(rule.trigger_type)) {
        this.logger.log(`rule ${rule.id} skipped: QA trigger ${rule.trigger_type} waits for Phase 2C`);
        continue;
      }
      try {
        const result = await this.workflows.evaluateRule(tenantId, rule, now);
        for (const m of result.matches) {
          if (await this.fire(tenantId, rule, m, result.window, now)) alerts++;
        }
      } catch (err: unknown) {
        this.logger.error(`rule ${rule.id} evaluation failed: ${(err as Error).message}`);
      }
    }
    return { rules: rules.length, alerts };
  }

  /** Cooldown → insert alert → email → action by mode. Returns false when cooled down. */
  async fire(
    tenantId: string,
    rule: WorkflowRule,
    m: Match,
    window: { from: string; to: string },
    now = new Date(),
  ): Promise<boolean> {
    const platform = m.platform || rule.platform || null;
    const alert = await this.pg.withTenant(tenantId, async (c) => {
      const cool = await c.query(
        `SELECT 1 FROM alert_events
          WHERE rule_id = $1::uuid AND entity_id = $2 AND COALESCE(platform, '') = COALESCE($3, '')
            AND created_at > NOW() - make_interval(hours => $4::int)
          LIMIT 1`,
        [rule.id, m.entity_id, platform, rule.cooldown_hours],
      );
      if (cool.rows.length) return null;
      let integrationId: string | null = null;
      if (rule.trigger_type === 'integration_unhealthy') integrationId = m.entity_id;
      else if (rule.scope === 'publisher' && platform) {
        integrationId = (await this.actions.resolveLiveIntegration(c, platform)).integration?.id ?? null;
      }
      const r = await c.query(
        `INSERT INTO alert_events AS a
           (tenant_id, rule_id, trigger_type, scope, platform, entity_id, entity_name, integration_id, numbers,
            window_from, window_to)
         VALUES (current_setting('app.current_tenant')::uuid, $1::uuid, $2, $3, $4, $5, $6, $7::uuid, $8::jsonb,
                 $9::timestamptz, $10::timestamptz)
         RETURNING ${ALERT_COLUMNS}`,
        [
          rule.id,
          rule.trigger_type,
          rule.trigger_type === 'integration_unhealthy' ? 'tenant' : rule.scope,
          platform,
          m.entity_id,
          m.entity_name,
          integrationId,
          JSON.stringify(m.numbers),
          window.from,
          window.to,
        ],
      );
      return r.rows[0] as AlertRow;
    });
    if (!alert) return false;

    if (rule.notify_email && this.email.enabled()) {
      const sent = await this.email.send(alertEmail(rule, alert));
      if (sent) {
        await this.pg.withTenant(tenantId, (c) =>
          c.query(`UPDATE alert_events SET emailed = TRUE WHERE id = $1::uuid`, [alert.id]),
        );
      }
    }

    // integration_unhealthy and non-publisher scopes never pause, whatever the mode.
    const canPause = rule.scope === 'publisher' && rule.trigger_type !== 'integration_unhealthy' && !!platform;
    if (!canPause || rule.action_mode === 'notify') return true;
    if (rule.action_mode === 'suggest') {
      const detail: Record<string, unknown> = { mode: 'suggest' };
      if (platform === 'ringba' && !ringbaPauseVerified()) {
        detail.ringba_link = RINGBA_PUBLISHERS_URL;
        detail.reason = 'Ringba pause is not verified yet: pause it in Ringba.';
      }
      await this.pg.withTenant(tenantId, (c) => this.actions.setAlertAction(c, alert.id, 'suggested', detail));
      return true;
    }
    await this.actions.autoPause(tenantId, alert, rule, now);
    return true;
  }
}
