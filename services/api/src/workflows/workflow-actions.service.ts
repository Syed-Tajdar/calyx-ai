import {
  ConflictException,
  ForbiddenException,
  HttpException,
  HttpStatus,
  Inject,
  Injectable,
  Logger,
  NotFoundException,
  ServiceUnavailableException,
} from '@nestjs/common';
import { PostgresService } from '../database/postgres.service';
import {
  PUBLISHER_ACTION_CLIENT,
  PublisherActionClient,
  PublisherActionFailure,
  PublisherActionResult,
} from './platform-pause.client';
import {
  AlertRow,
  Db,
  EASTERN_TODAY_SQL,
  INTEGRATION_COLUMNS,
  IntegrationRow,
  isLive,
  PAUSE_COLUMNS,
  PauseRow,
  TenantRunner,
} from './workflow-db';
import {
  actionsEnabled,
  autoModeBlockReason,
  maxAutoPausesDaily,
  platformLabel,
  RINGBA_PUBLISHERS_URL,
  ringbaPauseVerified,
  WorkflowRule,
} from './workflow-types';

export type GuardDecision =
  | { kind: 'ok'; integration: IntegrationRow }
  | { kind: 'suggest'; reason: string; ringba_link?: string }
  | { kind: 'blocked'; reason: string; ringba_link?: string };

export interface ManualPauseResult {
  status: 'paused';
  pause_id: string | null;
  outcome: 'confirmed' | 'confirmed_by_response';
  read_back: 'ok' | 'none';
  message: string;
}

const KILL_SWITCH_MSG =
  'Publisher actions are switched off on the server (WORKFLOW_ACTIONS_ENABLED). Nothing was sent to the platform.';
const RINGBA_UNVERIFIED_MSG =
  'Ringba pause is not verified yet (WORKFLOW_RINGBA_PAUSE_VERIFIED). Pause this publisher in Ringba.';

/**
 * Pause / unpause with every API-side guardrail of docs/v2-2d-contract.md. Platform calls go to
 * platform-sync only, and never inside a DB transaction. Blocked attempts are audited here
 * (`publisher_action_audit` outcome `rejected`); platform-sync audits every attempt it receives.
 */
@Injectable()
export class WorkflowActionsService {
  private readonly logger = new Logger(WorkflowActionsService.name);

  constructor(
    @Inject(PostgresService) private readonly pg: TenantRunner,
    @Inject(PUBLISHER_ACTION_CLIENT) private readonly client: PublisherActionClient,
  ) {}

  // ---- reads ---------------------------------------------------------------------------------

  async integrationById(c: Db, id: string): Promise<IntegrationRow | null> {
    const r = await c.query(
      `SELECT ${INTEGRATION_COLUMNS} FROM integrations WHERE id = $1::uuid AND deleted_at IS NULL`,
      [id],
    );
    return (r.rows[0] as IntegrationRow) ?? null;
  }

  /**
   * The Live integration that owns a publisher. Pings carry no integration id, so the publisher
   * maps to the tenant's single Live integration for that platform. Zero or several → no pause.
   */
  async resolveLiveIntegration(
    c: Db,
    platform: string,
  ): Promise<{ integration: IntegrationRow | null; reason?: string }> {
    const r = await c.query(
      `SELECT ${INTEGRATION_COLUMNS} FROM integrations
        WHERE platform = $1 AND deleted_at IS NULL AND connection_mode = 'api_sync'
        ORDER BY created_at`,
      [platform],
    );
    const live = (r.rows as IntegrationRow[]).filter(isLive);
    if (live.length === 0) {
      return { integration: null, reason: `No Live ${platformLabel(platform)} integration (Test integrations are never paused).` };
    }
    if (live.length > 1) {
      return {
        integration: null,
        reason: `More than one Live ${platformLabel(platform)} integration: Calyx cannot tell which one owns this publisher.`,
      };
    }
    return { integration: live[0] };
  }

  async isProtected(c: Db, platform: string, publisherId: string): Promise<boolean> {
    const r = await c.query(
      `SELECT 1 FROM protected_publishers WHERE platform = $1 AND publisher_id = $2`,
      [platform, publisherId],
    );
    return r.rows.length > 0;
  }

  async autoPausesToday(c: Db): Promise<number> {
    const r = await c.query(
      `SELECT COUNT(*)::int AS n FROM publisher_pauses
        WHERE mode = 'auto' AND (paused_at AT TIME ZONE 'America/New_York')::date = ${EASTERN_TODAY_SQL}`,
    );
    return Number(r.rows[0]?.n ?? 0);
  }

  async openPause(c: Db, platform: string, publisherId: string): Promise<PauseRow | null> {
    const r = await c.query(
      `SELECT ${PAUSE_COLUMNS} FROM publisher_pauses
        WHERE platform = $1 AND publisher_id = $2 AND unpaused_at IS NULL`,
      [platform, publisherId],
    );
    return (r.rows[0] as PauseRow) ?? null;
  }

  private async liveIntegrationFor(c: Db, alert: AlertRow): Promise<{ integration: IntegrationRow | null; reason?: string }> {
    if (alert.integration_id) {
      const i = await this.integrationById(c, alert.integration_id);
      if (i && isLive(i)) return { integration: i };
    }
    return this.resolveLiveIntegration(c, alert.platform ?? '');
  }

  private integrationBlock(i: IntegrationRow | null, reason?: string): string | null {
    if (!i) return reason ?? 'No Live integration for this publisher.';
    if (!isLive(i)) return 'The integration is not Live (Test, paused or not API sync). Calyx never pauses through it.';
    if (!i.allow_publisher_pause) return '"Allow Calyx to pause publishers" is off for this integration.';
    return null;
  }

  // ---- guardrails ----------------------------------------------------------------------------

  /** Auto mode, in contract order. Suggest = downgrade (admin can still confirm); blocked = no pause. */
  async decideAuto(c: Db, alert: AlertRow, rule: WorkflowRule, now: Date): Promise<GuardDecision> {
    const platform = alert.platform ?? '';
    if (!actionsEnabled()) return { kind: 'blocked', reason: KILL_SWITCH_MSG };
    if (await this.isProtected(c, platform, alert.entity_id)) {
      return { kind: 'suggest', reason: 'Protected publisher: never auto-paused (Suggest only).' };
    }
    const max = maxAutoPausesDaily();
    if ((await this.autoPausesToday(c)) >= max) {
      return { kind: 'suggest', reason: `Daily limit of ${max} automatic pauses reached (Eastern day). Suggest only.` };
    }
    const autoBlock = autoModeBlockReason(rule, now);
    if (autoBlock) return { kind: 'suggest', reason: autoBlock };
    if (platform === 'ringba' && !ringbaPauseVerified()) {
      return { kind: 'suggest', reason: RINGBA_UNVERIFIED_MSG, ringba_link: RINGBA_PUBLISHERS_URL };
    }
    const { integration, reason } = await this.liveIntegrationFor(c, alert);
    const block = this.integrationBlock(integration, reason);
    if (block) return { kind: 'blocked', reason: block };
    return { kind: 'ok', integration: integration as IntegrationRow };
  }

  /** Manual (Suggest confirm). Protected publishers and the daily limit apply to Auto only. */
  async decideManual(c: Db, alert: AlertRow): Promise<GuardDecision> {
    if (!actionsEnabled()) return { kind: 'blocked', reason: KILL_SWITCH_MSG };
    if (alert.platform === 'ringba' && !ringbaPauseVerified()) {
      return { kind: 'blocked', reason: RINGBA_UNVERIFIED_MSG, ringba_link: RINGBA_PUBLISHERS_URL };
    }
    const { integration, reason } = await this.liveIntegrationFor(c, alert);
    const block = this.integrationBlock(integration, reason);
    if (block) return { kind: 'blocked', reason: block };
    return { kind: 'ok', integration: integration as IntegrationRow };
  }

  // ---- writes --------------------------------------------------------------------------------

  async auditRejected(
    c: Db,
    a: {
      userId: string | null;
      integrationId: string | null;
      platform: string;
      publisherId: string;
      action: 'pause' | 'unpause';
      mode: 'manual' | 'auto' | 'timer';
      ruleId: string | null;
      alertId: string | null;
      message: string;
      numbers: Record<string, unknown>;
    },
  ): Promise<void> {
    await c.query(
      `INSERT INTO publisher_action_audit
         (tenant_id, user_id, integration_id, platform, publisher_id, action, mode, rule_id, alert_event_id,
          outcome, read_back, message, request_redacted, numbers)
       VALUES (current_setting('app.current_tenant')::uuid, $1::uuid, $2::uuid, $3, $4, $5, $6, $7::uuid, $8::uuid,
               'rejected', 'none', $9, NULL, $10::jsonb)`,
      [
        a.userId,
        a.integrationId,
        a.platform,
        a.publisherId,
        a.action,
        a.mode,
        a.ruleId,
        a.alertId,
        a.message,
        JSON.stringify(a.numbers ?? {}),
      ],
    );
  }

  async setAlertAction(
    c: Db,
    alertId: string,
    action: AlertRow['action'],
    detail: Record<string, unknown>,
    status?: AlertRow['status'],
  ): Promise<void> {
    await c.query(
      `UPDATE alert_events
          SET action = $2, action_detail = action_detail || $3::jsonb,
              status = COALESCE($4, status)
        WHERE id = $1::uuid`,
      [alertId, action, JSON.stringify(detail), status ?? null],
    );
  }

  private async recordPause(
    c: Db,
    alert: AlertRow,
    rule: Pick<WorkflowRule, 'pause_hours'> | null,
    integrationId: string,
    mode: 'manual' | 'auto',
    userId: string | null,
    res: Extract<PublisherActionResult, { ok: true }>,
  ): Promise<string | null> {
    const confirmation = res.outcome === 'confirmed_by_response' ? 'response' : 'read_back';
    const r = await c.query(
      `INSERT INTO publisher_pauses
         (tenant_id, integration_id, platform, publisher_id, publisher_name, mode, alert_event_id, rule_id,
          paused_by, unpause_at, confirmation)
       VALUES (current_setting('app.current_tenant')::uuid, $1::uuid, $2, $3, $4, $5, $6::uuid, $7::uuid, $8::uuid,
               CASE WHEN $9::int IS NULL THEN NULL ELSE NOW() + make_interval(hours => $9::int) END, $10)
       ON CONFLICT (tenant_id, platform, publisher_id) WHERE unpaused_at IS NULL DO NOTHING
       RETURNING id::text`,
      [
        integrationId,
        alert.platform,
        alert.entity_id,
        alert.entity_name ?? '',
        mode,
        alert.id,
        alert.rule_id,
        userId,
        rule?.pause_hours ?? null,
        confirmation,
      ],
    );
    const pauseId = (r.rows[0]?.id as string) ?? null;
    await this.setAlertAction(
      c,
      alert.id,
      'paused',
      {
        mode,
        outcome: res.outcome,
        read_back: res.read_back,
        confirmation,
        audit_id: res.audit_id,
        message: res.message,
        pause_id: pauseId,
      },
      'actioned',
    );
    return pauseId;
  }

  private async recordFailure(c: Db, alert: AlertRow, mode: 'manual' | 'auto', res: PublisherActionFailure): Promise<void> {
    // 403 from platform-sync = refused before any platform call (it wrote its own audit row).
    const action = res.status === 403 ? 'pause_blocked' : 'pause_failed';
    await this.setAlertAction(c, alert.id, action, {
      mode,
      http_status: res.status,
      outcome: res.outcome,
      message: res.message,
      audit_id: res.audit_id,
    });
  }

  /**
   * Auto pause from the evaluator (alert already inserted). Never throws on a refusal: the outcome
   * is written on the alert. Returns the final alert action.
   */
  async autoPause(tenantId: string, alert: AlertRow, rule: WorkflowRule, now = new Date()): Promise<AlertRow['action']> {
    const decision = await this.pg.withTenant(tenantId, async (c) => {
      const d = await this.decideAuto(c, alert, rule, now);
      if (d.kind === 'ok' && (await this.openPause(c, alert.platform ?? '', alert.entity_id))) {
        await this.setAlertAction(c, alert.id, 'none', { note: 'Already paused by Calyx.' });
        return { kind: 'done' as const };
      }
      if (d.kind !== 'ok') {
        await this.auditRejected(c, {
          userId: null,
          integrationId: alert.integration_id,
          platform: alert.platform ?? '',
          publisherId: alert.entity_id,
          action: 'pause',
          mode: 'auto',
          ruleId: rule.id,
          alertId: alert.id,
          message: d.reason,
          numbers: alert.numbers,
        });
        await this.setAlertAction(c, alert.id, d.kind === 'suggest' ? 'suggested' : 'pause_blocked', {
          mode: 'auto',
          reason: d.reason,
          ...(d.ringba_link ? { ringba_link: d.ringba_link } : {}),
        });
      }
      return d;
    });
    if (decision.kind === 'done') return 'none';
    if (decision.kind === 'suggest') return 'suggested';
    if (decision.kind === 'blocked') return 'pause_blocked';

    const res = await this.client.call('pause', {
      tenant_id: tenantId,
      integration_id: decision.integration.id,
      publisher_id: alert.entity_id,
      user_id: null,
      mode: 'auto',
      rule_id: rule.id,
      alert_event_id: alert.id,
      numbers: alert.numbers,
    });
    return this.pg.withTenant(tenantId, async (c) => {
      if (res.ok) {
        await this.recordPause(c, alert, rule, decision.integration.id, 'auto', null, res);
        return 'paused' as const;
      }
      await this.recordFailure(c, alert, 'auto', res);
      return res.status === 403 ? ('pause_blocked' as const) : ('pause_failed' as const);
    });
  }

  /** Admin confirms a suggested pause. Throws the mapped HTTP error after recording it. */
  async manualPause(tenantId: string, userId: string, alertId: string): Promise<ManualPauseResult> {
    const prep = await this.pg.withTenant(tenantId, async (c) => {
      const r = await c.query(
        `SELECT a.id::text, a.rule_id::text, a.trigger_type, a.scope, a.platform, a.entity_id, a.entity_name,
                a.integration_id::text, a.numbers, a.status, a.action, a.action_detail,
                w.pause_hours
           FROM alert_events a LEFT JOIN workflow_rules w ON w.id = a.rule_id
          WHERE a.id = $1::uuid`,
        [alertId],
      );
      const alert = r.rows[0] as (AlertRow & { pause_hours: number | null }) | undefined;
      if (!alert) return { error: new NotFoundException('Alert not found') };
      if (alert.scope !== 'publisher' || !alert.platform || !alert.entity_id) {
        return { error: new ConflictException('Only publisher alerts can pause a publisher.') };
      }
      if (!['suggested', 'pause_blocked', 'pause_failed'].includes(alert.action)) {
        return { error: new ConflictException(`This alert cannot be paused (action is ${alert.action}).`) };
      }
      const d = await this.decideManual(c, alert);
      if (d.kind !== 'ok') {
        await this.auditRejected(c, {
          userId,
          integrationId: alert.integration_id,
          platform: alert.platform,
          publisherId: alert.entity_id,
          action: 'pause',
          mode: 'manual',
          ruleId: alert.rule_id,
          alertId: alert.id,
          message: d.reason,
          numbers: alert.numbers,
        });
        await this.setAlertAction(c, alert.id, 'pause_blocked', {
          mode: 'manual',
          reason: d.reason,
          ...(d.ringba_link ? { ringba_link: d.ringba_link } : {}),
        });
        return { error: new ForbiddenException(d.reason), committed: true };
      }
      if (await this.openPause(c, alert.platform, alert.entity_id)) {
        return { error: new ConflictException('This publisher is already paused by Calyx.') };
      }
      return { alert, integration: d.integration };
    });
    // Errors are thrown only after the transaction committed, so the audit row survives.
    if ('error' in prep && prep.error) throw prep.error;
    const { alert, integration } = prep as { alert: AlertRow & { pause_hours: number | null }; integration: IntegrationRow };

    const res = await this.client.call('pause', {
      tenant_id: tenantId,
      integration_id: integration.id,
      publisher_id: alert.entity_id,
      user_id: userId,
      mode: 'manual',
      rule_id: alert.rule_id,
      alert_event_id: alert.id,
      numbers: alert.numbers,
    });
    const pauseId = await this.pg.withTenant(tenantId, async (c) => {
      if (res.ok) return this.recordPause(c, alert, { pause_hours: alert.pause_hours }, integration.id, 'manual', userId, res);
      await this.recordFailure(c, alert, 'manual', res);
      return null;
    });
    if (!res.ok) throw mapFailure(res);
    return { status: 'paused', pause_id: pauseId, outcome: res.outcome, read_back: res.read_back, message: res.message };
  }

  /**
   * Unpause (admin, or the rule's timer). Timer runs skip quietly while the kill switch is off
   * (no attempt, so no audit row every 15 minutes); a manual unpause is refused and audited.
   */
  async unpause(
    tenantId: string,
    pauseId: string,
    mode: 'manual' | 'timer',
    userId: string | null,
  ): Promise<{ status: 'unpaused'; outcome: string; message: string }> {
    const prep = await this.pg.withTenant(tenantId, async (c) => {
      const r = await c.query(`SELECT ${PAUSE_COLUMNS} FROM publisher_pauses WHERE id = $1::uuid AND unpaused_at IS NULL`, [
        pauseId,
      ]);
      const pause = r.rows[0] as PauseRow | undefined;
      if (!pause) return { error: new NotFoundException('Open pause not found') };
      if (!actionsEnabled()) {
        if (mode === 'timer') return { error: new ServiceUnavailableException(KILL_SWITCH_MSG) };
        await this.auditRejected(c, {
          userId,
          integrationId: pause.integration_id,
          platform: pause.platform,
          publisherId: pause.publisher_id,
          action: 'unpause',
          mode,
          ruleId: pause.rule_id,
          alertId: pause.alert_event_id,
          message: KILL_SWITCH_MSG,
          numbers: {},
        });
        return { error: new ForbiddenException(`${KILL_SWITCH_MSG} Unpause it in ${platformLabel(pause.platform)}.`) };
      }
      return { pause };
    });
    if ('error' in prep && prep.error) throw prep.error;
    const pause = (prep as { pause: PauseRow }).pause;

    const res = await this.client.call('unpause', {
      tenant_id: tenantId,
      integration_id: pause.integration_id,
      publisher_id: pause.publisher_id,
      user_id: userId,
      mode,
      rule_id: pause.rule_id,
      alert_event_id: pause.alert_event_id,
      numbers: {},
    });
    if (!res.ok) {
      this.logger.warn(`unpause ${mode} failed: status=${res.status} outcome=${res.outcome}`);
      throw mapFailure(res);
    }
    await this.pg.withTenant(tenantId, async (c) => {
      await c.query(
        `UPDATE publisher_pauses SET unpaused_at = NOW(), unpaused_by = $2::uuid WHERE id = $1::uuid AND unpaused_at IS NULL`,
        [pause.id, userId],
      );
      if (pause.alert_event_id) {
        await this.setAlertAction(c, pause.alert_event_id, 'unpaused', {
          unpause_mode: mode,
          unpause_outcome: res.outcome,
          unpause_audit_id: res.audit_id,
        });
      }
    });
    return { status: 'unpaused', outcome: res.outcome, message: res.message };
  }
}

/** platform-sync error → HTTP error for the admin (contract: 403/404/409/502). */
export function mapFailure(res: PublisherActionFailure): HttpException {
  switch (res.status) {
    case 403:
      return new ForbiddenException(res.message);
    case 404:
      return new NotFoundException(res.message);
    case 409:
      return new ConflictException(res.message);
    case 502:
      return new HttpException(res.message, HttpStatus.BAD_GATEWAY);
    default:
      return new ServiceUnavailableException(res.message);
  }
}
