import { z } from 'zod';

/** v2 Phase 2D — rule shapes, validation and the env switches. See docs/v2-2d-contract.md. */

export const WORKFLOW_TZ = 'America/New_York';

export const DATA_TRIGGERS = [
  'conversion_rate_below',
  'short_call_share_above',
  'repeat_caller_share_above',
  'volume_change',
  'revenue_drop',
  'no_calls',
  'integration_unhealthy',
] as const;

/** Accepted by the schema (and the DB CHECK); evaluated once Phase 2C (QA) exists. */
export const QA_TRIGGERS = [
  'qa_flag_rate_above',
  'qa_score_below',
  'should_convert_share_above',
  'do_not_call_requests',
] as const;

export const TRIGGER_TYPES = [...DATA_TRIGGERS, ...QA_TRIGGERS] as const;
export type TriggerType = (typeof TRIGGER_TYPES)[number];
export type Scope = 'publisher' | 'campaign' | 'tenant';
export type ActionMode = 'notify' | 'suggest' | 'auto';
export type Platform = 'ringba' | 'retreaver';

export function isQaTrigger(t: string): boolean {
  return (QA_TRIGGERS as readonly string[]).includes(t);
}

const pct = z.number().min(0).max(1000);

const activeHours = z
  .object({ start: z.number().int().min(0).max(23), end: z.number().int().min(1).max(24) })
  .strict()
  .refine((h) => h.end > h.start, { message: 'active_hours.end must be after start' });

/** Trigger parameters, strict per type. Thresholds are percentages (5 = 5%). */
export const TRIGGER_PARAM_SCHEMAS: Record<TriggerType, z.ZodTypeAny> = {
  conversion_rate_below: z.object({ threshold_pct: pct.max(100) }).strict(),
  short_call_share_above: z
    .object({ threshold_pct: pct.max(100), short_seconds: z.number().int().min(1).max(3600) })
    .strict(),
  repeat_caller_share_above: z.object({ threshold_pct: pct.max(100) }).strict(),
  volume_change: z
    .object({ threshold_pct: pct, direction: z.enum(['up', 'down', 'both']).default('both') })
    .strict(),
  revenue_drop: z.object({ threshold_pct: pct.max(100) }).strict(),
  no_calls: z.object({ active_hours: activeHours.optional() }).strict(),
  integration_unhealthy: z.object({}).strict(),
  qa_flag_rate_above: z.object({ threshold_pct: pct.max(100) }).strict(),
  qa_score_below: z.object({ threshold: z.number().min(0).max(100) }).strict(),
  should_convert_share_above: z.object({ threshold_pct: pct.max(100) }).strict(),
  do_not_call_requests: z.object({ count: z.number().int().min(1).max(10000) }).strict(),
};

const ruleShape = {
  name: z.string().trim().min(1).max(120),
  trigger_type: z.enum(TRIGGER_TYPES),
  params: z.record(z.unknown()).default({}),
  window_hours: z.number().int().min(1).max(720).default(24),
  min_calls: z.number().int().min(0).max(100000).default(20),
  scope: z.enum(['publisher', 'campaign', 'tenant']).default('publisher'),
  platform: z.enum(['ringba', 'retreaver']).nullable().default(null),
  notify_email: z.boolean().default(false),
  action_mode: z.enum(['notify', 'suggest', 'auto']).default('notify'),
  pause_hours: z.number().int().min(1).max(720).nullable().default(null),
  cooldown_hours: z.number().int().min(1).max(168).default(6),
  is_active: z.boolean().default(true),
};

export const createRuleSchema = z.object(ruleShape).strict();
export const updateRuleSchema = z
  .object({
    name: ruleShape.name.optional(),
    trigger_type: z.enum(TRIGGER_TYPES).optional(),
    params: z.record(z.unknown()).optional(),
    window_hours: z.number().int().min(1).max(720).optional(),
    min_calls: z.number().int().min(0).max(100000).optional(),
    scope: z.enum(['publisher', 'campaign', 'tenant']).optional(),
    platform: z.enum(['ringba', 'retreaver']).nullable().optional(),
    notify_email: z.boolean().optional(),
    action_mode: z.enum(['notify', 'suggest', 'auto']).optional(),
    pause_hours: z.number().int().min(1).max(720).nullable().optional(),
    cooldown_hours: z.number().int().min(1).max(168).optional(),
    is_active: z.boolean().optional(),
  })
  .strict();

export type RuleInput = z.output<typeof createRuleSchema>;
export type RuleUpdate = z.output<typeof updateRuleSchema>;

export interface WorkflowRule extends RuleInput {
  id: string;
  tenant_id: string;
  suggest_since: string | null;
  created_at?: string;
  updated_at?: string;
}

/**
 * Cross-field rules shared by create, update and dry run. Returns the parsed params, or a
 * human-readable reason. Mode-vs-time checks (7 days of Suggest, Ringba proof) are separate
 * (`autoModeBlockReason`) because they depend on the stored rule and the server switches.
 */
export function validateRuleSemantics(r: RuleInput): { ok: true; params: Record<string, unknown> } | { ok: false; reason: string } {
  const parsed = TRIGGER_PARAM_SCHEMAS[r.trigger_type].safeParse(r.params ?? {});
  if (!parsed.success) {
    const issue = parsed.error.issues[0];
    return { ok: false, reason: `params: ${issue.path.join('.') || '(root)'} ${issue.message}` };
  }
  const params = parsed.data as Record<string, unknown>;
  if (r.trigger_type === 'no_calls' && r.scope !== 'campaign') {
    return { ok: false, reason: 'no_calls works per campaign: set scope to campaign' };
  }
  if (r.trigger_type === 'no_calls') {
    const h = params.active_hours as { start: number; end: number } | undefined;
    if (h && r.window_hours > h.end - h.start) {
      return { ok: false, reason: 'window_hours must fit inside active_hours' };
    }
  }
  if (r.trigger_type === 'integration_unhealthy' && r.action_mode !== 'notify') {
    return { ok: false, reason: 'integration_unhealthy is notify only (it never pauses)' };
  }
  if (r.action_mode !== 'notify' && r.scope !== 'publisher') {
    return { ok: false, reason: 'Suggest and Auto pause a publisher: set scope to publisher' };
  }
  if (r.pause_hours !== null && r.action_mode === 'notify') {
    return { ok: false, reason: 'pause_hours needs action mode suggest or auto' };
  }
  return { ok: true, params };
}

export const SUGGEST_DAYS_BEFORE_AUTO = 7;

/** Why Auto cannot be on for this rule now, or null. */
export function autoModeBlockReason(
  rule: { platform: Platform | null; suggest_since: string | Date | null },
  now: Date = new Date(),
  env: Record<string, string | undefined> = process.env,
): string | null {
  // A rule on both platforms may be Auto: its Ringba matches fall back to Suggest-with-link in the
  // evaluator while the Ringba pause is unverified. A Ringba-only rule cannot be Auto at all.
  if (rule.platform === 'ringba' && !ringbaPauseVerified(env)) {
    return 'Auto is off for Ringba until the Ringba pause is verified (WORKFLOW_RINGBA_PAUSE_VERIFIED).';
  }
  if (!rule.suggest_since) return 'Auto needs 7 days in Suggest mode first.';
  const since = new Date(rule.suggest_since).getTime();
  if (!(now.getTime() - since >= SUGGEST_DAYS_BEFORE_AUTO * 86_400_000)) {
    return 'Auto needs 7 days in Suggest mode first.';
  }
  return null;
}

// ---- Server switches -------------------------------------------------------------------------

type Env = Record<string, string | undefined>;

/** Evaluator runs only when the feature flag is exactly "true" (V1_ONLY=false alone does not start it). */
export function workflowsEnabled(env: Env = process.env): boolean {
  return env.FEATURE_WORKFLOWS === 'true';
}

/** Kill switch. Off (default) → never call platform-sync pause/unpause. */
export function actionsEnabled(env: Env = process.env): boolean {
  return env.WORKFLOW_ACTIONS_ENABLED === 'true';
}

export function ringbaPauseVerified(env: Env = process.env): boolean {
  return env.WORKFLOW_RINGBA_PAUSE_VERIFIED === 'true';
}

export function maxAutoPausesDaily(env: Env = process.env): number {
  const raw = env.WORKFLOW_MAX_AUTO_PAUSES_DAILY;
  if (raw === undefined || raw === '') return 3;
  const n = Number(raw);
  return Number.isFinite(n) && n >= 0 ? Math.floor(n) : 3;
}

/**
 * Ringba has no documented per-publisher UI URL (repo: only the call-logs report page is known,
 * shared/providers/platform-deep-links.json). Link to the Publishers list; the UI shows the
 * publisher id to look for.
 */
export const RINGBA_PUBLISHERS_URL = 'https://app.ringba.com/#/dashboard/publishers';

export function platformLabel(p: string | null | undefined): string {
  if (p === 'ringba') return 'Ringba';
  if (p === 'retreaver') return 'Retreaver';
  return p ?? '';
}

export function pauseWarning(platform: string): string {
  return `Pausing stops this publisher on ALL campaigns on ${platformLabel(platform)}.`;
}
