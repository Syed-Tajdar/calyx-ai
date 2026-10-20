import { BadRequestException, Inject, Injectable, Logger, NotFoundException } from '@nestjs/common';
import { PostgresService } from '../database/postgres.service';
import { EMAIL_SENDER, EmailSender } from './email.sender';
import {
  ALERT_COLUMNS,
  AlertRow,
  INTEGRATION_COLUMNS,
  IntegrationRow,
  isLive,
  PAUSE_COLUMNS,
  PauseRow,
  RULE_COLUMNS,
  TenantRunner,
} from './workflow-db';
import { WorkflowActionsService } from './workflow-actions.service';
import {
  EvaluationResult,
  evaluate,
  insideActiveHours,
  ruleWindow,
  WorkflowMetricsService,
} from './workflow-metrics.service';
import {
  actionsEnabled,
  autoModeBlockReason,
  isQaTrigger,
  maxAutoPausesDaily,
  pauseWarning,
  Platform,
  RINGBA_PUBLISHERS_URL,
  ringbaPauseVerified,
  RuleInput,
  RuleUpdate,
  validateRuleSemantics,
  WorkflowRule,
} from './workflow-types';

export interface RuleRecord extends WorkflowRule {
  /** Why Auto cannot be selected now (null = Auto available). */
  auto_block_reason: string | null;
  /** Suggested pauses this rule raised in the last 7 days ("what it would have paused"). */
  suggested_last_7d: number;
}

export interface DryRunResult extends EvaluationResult {
  window: { from: string; to: string; timezone: string };
}

export interface AlertRecord extends AlertRow {
  rule_name: string | null;
  /** Ringba while the pause is unverified: link instead of a Pause button. */
  ringba_link: string | null;
  can_pause: boolean;
}

export interface WorkflowStatus {
  actions_enabled: boolean;
  ringba_pause_verified: boolean;
  max_auto_pauses_daily: number;
  auto_pauses_today: number;
  email_enabled: boolean;
  ringba_publishers_url: string;
  open_alerts: number;
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
    platform: (row.platform as Platform | null) ?? null,
    notify_email: Boolean(row.notify_email),
    action_mode: row.action_mode as WorkflowRule['action_mode'],
    suggest_since: (row.suggest_since as string | null) ?? null,
    pause_hours: row.pause_hours === null || row.pause_hours === undefined ? null : Number(row.pause_hours),
    cooldown_hours: Number(row.cooldown_hours),
    is_active: Boolean(row.is_active),
    created_at: row.created_at as string | undefined,
    updated_at: row.updated_at as string | undefined,
  };
}

/** Rules, alert centre, protected publishers, the per-integration toggle and the audit view. */
@Injectable()
export class WorkflowsService {
  private readonly logger = new Logger(WorkflowsService.name);

  constructor(
    @Inject(PostgresService) private readonly pg: TenantRunner,
    private readonly metrics: WorkflowMetricsService,
    private readonly actions: WorkflowActionsService,
    @Inject(EMAIL_SENDER) private readonly email: EmailSender,
  ) {}

  // ---- rules ---------------------------------------------------------------------------------

  private semantic(r: RuleInput): Record<string, unknown> {
    const v = validateRuleSemantics(r);
    if (!v.ok) throw new BadRequestException(v.reason);
    return v.params;
  }

  async listRules(tenantId: string, now = new Date()): Promise<RuleRecord[]> {
    return this.pg.withTenant(tenantId, async (c) => {
      const r = await c.query(
        `SELECT ${RULE_COLUMNS},
                (SELECT COUNT(*)::int FROM alert_events a
                  WHERE a.rule_id = workflow_rules.id AND a.action = 'suggested'
                    AND a.created_at > NOW() - INTERVAL '7 days') AS suggested_last_7d
           FROM workflow_rules
          WHERE deleted_at IS NULL
          ORDER BY created_at DESC`,
      );
      return r.rows.map((row) => {
        const rule = toRule(row);
        return {
          ...rule,
          auto_block_reason: rule.scope === 'publisher' && rule.trigger_type !== 'integration_unhealthy'
            ? autoModeBlockReason(rule, now)
            : 'Auto pauses a publisher: needs scope publisher.',
          suggested_last_7d: Number(row.suggested_last_7d ?? 0),
        };
      });
    });
  }

  async getRule(tenantId: string, id: string): Promise<WorkflowRule> {
    return this.pg.withTenant(tenantId, async (c) => {
      const r = await c.query(`SELECT ${RULE_COLUMNS} FROM workflow_rules WHERE id = $1::uuid AND deleted_at IS NULL`, [id]);
      if (!r.rows[0]) throw new NotFoundException('Rule not found');
      return toRule(r.rows[0]);
    });
  }

  async createRule(tenantId: string, userId: string | null, input: RuleInput): Promise<WorkflowRule> {
    const params = this.semantic(input);
    if (input.action_mode === 'auto') {
      throw new BadRequestException('Auto needs 7 days in Suggest mode first. Save the rule in Suggest.');
    }
    return this.pg.withTenant(tenantId, async (c) => {
      const r = await c.query(
        `INSERT INTO workflow_rules
           (tenant_id, name, trigger_type, params, window_hours, min_calls, scope, platform, notify_email,
            action_mode, suggest_since, pause_hours, cooldown_hours, is_active, created_by)
         VALUES (current_setting('app.current_tenant')::uuid, $1, $2, $3::jsonb, $4, $5, $6, $7, $8, $9,
                 CASE WHEN $9 = 'suggest' THEN NOW() ELSE NULL END, $10, $11, $12, $13::uuid)
         RETURNING ${RULE_COLUMNS}`,
        [
          input.name,
          input.trigger_type,
          JSON.stringify(params),
          input.window_hours,
          input.min_calls,
          input.scope,
          input.platform,
          input.notify_email,
          input.action_mode,
          input.pause_hours,
          input.cooldown_hours,
          input.is_active,
          userId,
        ],
      );
      return toRule(r.rows[0]);
    });
  }

  async updateRule(tenantId: string, id: string, patch: RuleUpdate, now = new Date()): Promise<WorkflowRule> {
    return this.pg.withTenant(tenantId, async (c) => {
      const cur = await c.query(`SELECT ${RULE_COLUMNS} FROM workflow_rules WHERE id = $1::uuid AND deleted_at IS NULL`, [id]);
      if (!cur.rows[0]) throw new NotFoundException('Rule not found');
      const existing = toRule(cur.rows[0]);
      const merged: RuleInput = {
        name: patch.name ?? existing.name,
        trigger_type: patch.trigger_type ?? existing.trigger_type,
        params: patch.params ?? existing.params,
        window_hours: patch.window_hours ?? existing.window_hours,
        min_calls: patch.min_calls ?? existing.min_calls,
        scope: patch.scope ?? existing.scope,
        platform: patch.platform !== undefined ? patch.platform : existing.platform,
        notify_email: patch.notify_email ?? existing.notify_email,
        action_mode: patch.action_mode ?? existing.action_mode,
        pause_hours: patch.pause_hours !== undefined ? patch.pause_hours : existing.pause_hours,
        cooldown_hours: patch.cooldown_hours ?? existing.cooldown_hours,
        is_active: patch.is_active ?? existing.is_active,
      };
      const params = this.semantic(merged);
      // suggest_since: set when the rule enters Suggest, kept through Auto, cleared on Notify.
      let suggestSince: string | null = existing.suggest_since;
      if (merged.action_mode === 'notify') suggestSince = null;
      else if (merged.action_mode === 'suggest' && existing.action_mode !== 'suggest') suggestSince = now.toISOString();
      if (merged.action_mode === 'auto') {
        const reason = autoModeBlockReason({ platform: merged.platform, suggest_since: suggestSince }, now);
        if (reason) throw new BadRequestException(reason);
      }
      const r = await c.query(
        `UPDATE workflow_rules
            SET name = $2, trigger_type = $3, params = $4::jsonb, window_hours = $5, min_calls = $6, scope = $7,
                platform = $8, notify_email = $9, action_mode = $10, suggest_since = $11::timestamptz,
                pause_hours = $12, cooldown_hours = $13, is_active = $14, updated_at = NOW()
          WHERE id = $1::uuid AND deleted_at IS NULL
          RETURNING ${RULE_COLUMNS}`,
        [
          id,
          merged.name,
          merged.trigger_type,
          JSON.stringify(params),
          merged.window_hours,
          merged.min_calls,
          merged.scope,
          merged.platform,
          merged.notify_email,
          merged.action_mode,
          suggestSince,
          merged.pause_hours,
          merged.cooldown_hours,
          merged.is_active,
        ],
      );
      return toRule(r.rows[0]);
    });
  }

  async deleteRule(tenantId: string, id: string): Promise<void> {
    await this.pg.withTenant(tenantId, async (c) => {
      const r = await c.query(
        `UPDATE workflow_rules SET deleted_at = NOW(), is_active = FALSE, updated_at = NOW()
          WHERE id = $1::uuid AND deleted_at IS NULL`,
        [id],
      );
      if (!r.rowCount) throw new NotFoundException('Rule not found');
    });
  }

  // ---- evaluation (shared by dry run and the evaluator) ---------------------------------------

  /** Which entities match the rule now, with their numbers. No side effects. */
  async evaluateRule(
    tenantId: string,
    rule: Pick<WorkflowRule, 'trigger_type' | 'params' | 'window_hours' | 'min_calls' | 'scope' | 'platform'>,
    now = new Date(),
  ): Promise<DryRunResult> {
    const win = ruleWindow(rule, now);
    const window = { from: win.from.toISOString(), to: win.to.toISOString(), timezone: 'America/New_York' };
    if (isQaTrigger(rule.trigger_type)) {
      return { window, matches: [], below_min_calls: 0, evaluated: 0, skipped_reason: 'QA triggers run once QA (Phase 2C) is live.' };
    }
    if (rule.trigger_type === 'integration_unhealthy') {
      const integrations = await this.pg.withTenant(tenantId, async (c) => {
        const r = await c.query(
          `SELECT ${INTEGRATION_COLUMNS} FROM integrations
            WHERE deleted_at IS NULL AND connection_mode = 'api_sync' AND sync_status IN ('error', 'degraded')
              AND ($1::text IS NULL OR platform = $1)`,
          [rule.platform],
        );
        return (r.rows as IntegrationRow[]).filter(isLive);
      });
      return {
        window,
        evaluated: integrations.length,
        below_min_calls: 0,
        matches: integrations.map((i) => ({
          platform: i.platform,
          entity_id: i.id,
          entity_name: i.name,
          numbers: { sync_status: i.sync_status, last_sync_at: i.last_sync_at },
        })),
      };
    }
    if (rule.trigger_type === 'no_calls') {
      const active = (rule.params as { active_hours?: { start: number; end: number } }).active_hours;
      if (!insideActiveHours(now, rule.window_hours, active)) {
        return { window, matches: [], below_min_calls: 0, evaluated: 0, skipped_reason: 'Outside the active hours (Eastern time).' };
      }
    }
    const shortSeconds = Number((rule.params as { short_seconds?: number }).short_seconds ?? 0);
    const rows = await this.metrics.entityCounts(tenantId, rule.scope, rule.platform, win, shortSeconds);
    return { window, ...evaluate(rule, rows) };
  }

  async dryRunSaved(tenantId: string, id: string): Promise<DryRunResult> {
    const rule = await this.getRule(tenantId, id);
    return this.evaluateRule(tenantId, rule);
  }

  async dryRunUnsaved(tenantId: string, input: RuleInput): Promise<DryRunResult> {
    const params = this.semantic(input);
    return this.evaluateRule(tenantId, { ...input, params });
  }

  // ---- alert centre --------------------------------------------------------------------------

  private toAlertRecord(row: AlertRow & { rule_name?: string | null }): AlertRecord {
    const ringbaLink =
      row.platform === 'ringba' && !ringbaPauseVerified() && row.scope === 'publisher' ? RINGBA_PUBLISHERS_URL : null;
    return {
      ...row,
      rule_name: row.rule_name ?? null,
      ringba_link: ringbaLink,
      can_pause:
        row.scope === 'publisher' &&
        !ringbaLink &&
        ['suggested', 'pause_blocked', 'pause_failed'].includes(row.action),
    };
  }

  async listAlerts(tenantId: string, status?: string, limit = 100): Promise<{ data: AlertRecord[]; open_count: number }> {
    return this.pg.withTenant(tenantId, async (c) => {
      const r = await c.query(
        `SELECT ${ALERT_COLUMNS}, w.name AS rule_name, p.id::text AS pause_id, p.confirmation AS pause_confirmation
           FROM alert_events a
           LEFT JOIN workflow_rules w ON w.id = a.rule_id
           LEFT JOIN publisher_pauses p ON p.alert_event_id = a.id AND p.unpaused_at IS NULL
          WHERE ($1::text IS NULL OR a.status = $1)
          ORDER BY a.created_at DESC
          LIMIT $2`,
        [status ?? null, Math.min(Math.max(limit, 1), 200)],
      );
      const open = await c.query(`SELECT COUNT(*)::int AS n FROM alert_events WHERE status = 'open'`);
      return { data: r.rows.map((row) => this.toAlertRecord(row)), open_count: Number(open.rows[0]?.n ?? 0) };
    });
  }

  async acknowledge(tenantId: string, userId: string, id: string): Promise<{ id: string; status: string }> {
    return this.pg.withTenant(tenantId, async (c) => {
      const r = await c.query(
        `UPDATE alert_events
            SET status = CASE WHEN status = 'open' THEN 'acknowledged' ELSE status END,
                acknowledged_by = COALESCE(acknowledged_by, $2::uuid),
                acknowledged_at = COALESCE(acknowledged_at, NOW())
          WHERE id = $1::uuid
          RETURNING id::text, status`,
        [id, userId],
      );
      if (!r.rows[0]) throw new NotFoundException('Alert not found');
      return r.rows[0] as { id: string; status: string };
    });
  }

  async listOpenPauses(tenantId: string): Promise<PauseRow[]> {
    return this.pg.withTenant(tenantId, async (c) => {
      const r = await c.query(`SELECT ${PAUSE_COLUMNS} FROM publisher_pauses WHERE unpaused_at IS NULL ORDER BY paused_at DESC`);
      return r.rows as PauseRow[];
    });
  }

  async publisherImpact(tenantId: string, platform: Platform, publisherId: string) {
    const impact = await this.metrics.publisherImpact(tenantId, platform, publisherId);
    return { platform, publisher_id: publisherId, ...impact, warning: pauseWarning(platform) };
  }

  async listAudit(tenantId: string): Promise<unknown[]> {
    return this.pg.withTenant(tenantId, async (c) => {
      const r = await c.query(
        `SELECT id::text, user_id::text, integration_id::text, platform, publisher_id, action, mode, rule_id::text,
                alert_event_id::text, outcome, read_back, message, numbers, created_at::text
           FROM publisher_action_audit ORDER BY created_at DESC LIMIT 50`,
      );
      return r.rows;
    });
  }

  async status(tenantId: string): Promise<WorkflowStatus> {
    return this.pg.withTenant(tenantId, async (c) => {
      const open = await c.query(`SELECT COUNT(*)::int AS n FROM alert_events WHERE status = 'open'`);
      return {
        actions_enabled: actionsEnabled(),
        ringba_pause_verified: ringbaPauseVerified(),
        max_auto_pauses_daily: maxAutoPausesDaily(),
        auto_pauses_today: await this.actions.autoPausesToday(c),
        email_enabled: this.email.enabled(),
        ringba_publishers_url: RINGBA_PUBLISHERS_URL,
        open_alerts: Number(open.rows[0]?.n ?? 0),
      };
    });
  }

  // ---- protected publishers ------------------------------------------------------------------

  async listProtected(tenantId: string): Promise<unknown[]> {
    return this.pg.withTenant(tenantId, async (c) => {
      const r = await c.query(
        `SELECT platform, publisher_id, name, created_at::text FROM protected_publishers ORDER BY created_at DESC`,
      );
      return r.rows;
    });
  }

  async addProtected(
    tenantId: string,
    userId: string | null,
    p: { platform: Platform; publisher_id: string; name: string },
  ): Promise<{ platform: string; publisher_id: string; name: string }> {
    await this.pg.withTenant(tenantId, async (c) => {
      await c.query(
        `INSERT INTO protected_publishers (tenant_id, platform, publisher_id, name, created_by)
         VALUES (current_setting('app.current_tenant')::uuid, $1, $2, $3, $4::uuid)
         ON CONFLICT (tenant_id, platform, publisher_id) DO NOTHING`, // the API role has no UPDATE on this table
        [p.platform, p.publisher_id, p.name, userId],
      );
    });
    return p;
  }

  async removeProtected(tenantId: string, platform: Platform, publisherId: string): Promise<void> {
    await this.pg.withTenant(tenantId, async (c) => {
      const r = await c.query(`DELETE FROM protected_publishers WHERE platform = $1 AND publisher_id = $2`, [
        platform,
        publisherId,
      ]);
      if (!r.rowCount) throw new NotFoundException('Protected publisher not found');
    });
  }

  // ---- per-integration toggle ----------------------------------------------------------------

  async listIntegrations(tenantId: string): Promise<Array<IntegrationRow & { live: boolean }>> {
    return this.pg.withTenant(tenantId, async (c) => {
      const r = await c.query(
        `SELECT ${INTEGRATION_COLUMNS} FROM integrations
          WHERE deleted_at IS NULL AND platform IN ('ringba', 'retreaver')
          ORDER BY created_at DESC`,
      );
      return (r.rows as IntegrationRow[]).map((i) => ({ ...i, live: isLive(i) }));
    });
  }

  /**
   * "Allow Calyx to pause publishers". Turning it on needs a Live integration; turning it off is
   * always allowed. Every change is audited in integration_settings_audit (allow_pause_on/off).
   */
  async setAllowPause(tenantId: string, userId: string, id: string, allow: boolean): Promise<IntegrationRow & { live: boolean }> {
    const row = await this.pg.withTenant(tenantId, async (c) => {
      const cur = await this.actions.integrationById(c, id);
      if (!cur) throw new NotFoundException('Integration not found');
      if (allow && !isLive(cur)) {
        throw new BadRequestException('Only Live integrations (Ringba or Retreaver API sync, real, active) can allow pauses.');
      }
      const r = await c.query(
        `UPDATE integrations SET allow_publisher_pause = $2, updated_at = NOW()
          WHERE id = $1::uuid AND deleted_at IS NULL
          RETURNING ${INTEGRATION_COLUMNS}`,
        [id, allow],
      );
      await c.query(
        `INSERT INTO integration_settings_audit (tenant_id, user_id, integration_id, action)
         VALUES ($1::uuid, $2::uuid, $3::uuid, $4)`,
        [tenantId, userId, id, allow ? 'allow_pause_on' : 'allow_pause_off'],
      );
      return r.rows[0] as IntegrationRow;
    });
    this.logger.log(`allow_publisher_pause=${allow} integration=${id} tenant=${tenantId}`);
    return { ...row, live: isLive(row) };
  }
}
