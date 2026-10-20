/**
 * Pure helpers for v2 Phase 2D Workflows (alerts → publisher pause). No React, no `@/` imports, so
 * `node --test` can load them directly. The API decides every guardrail; these only explain them.
 */

export type TriggerType =
  | 'conversion_rate_below'
  | 'short_call_share_above'
  | 'repeat_caller_share_above'
  | 'volume_change'
  | 'revenue_drop'
  | 'no_calls'
  | 'integration_unhealthy'
  | 'qa_flag_rate_above'
  | 'qa_score_below'
  | 'should_convert_share_above'
  | 'do_not_call_requests';

export type ActionMode = 'notify' | 'suggest' | 'auto';
export type RuleScope = 'publisher' | 'campaign' | 'tenant';
export type AlertAction = 'none' | 'suggested' | 'paused' | 'pause_blocked' | 'pause_failed' | 'unpaused';
export type AlertStatus = 'open' | 'acknowledged' | 'actioned' | 'resolved';

export interface TriggerOption {
  value: TriggerType;
  label: string;
  /** Param fields the form shows for this trigger. */
  params: Array<'threshold_pct' | 'short_seconds' | 'direction' | 'active_hours' | 'threshold' | 'count'>;
  /** QA triggers are saved but only evaluated once QA (Phase 2C) is live. */
  qa?: boolean;
}

export const TRIGGER_OPTIONS: TriggerOption[] = [
  { value: 'conversion_rate_below', label: 'Conversion rate below X%', params: ['threshold_pct'] },
  { value: 'short_call_share_above', label: 'Short calls (under N seconds) above X%', params: ['threshold_pct', 'short_seconds'] },
  { value: 'repeat_caller_share_above', label: 'Repeat callers above X%', params: ['threshold_pct'] },
  { value: 'volume_change', label: 'Call volume up/down X% vs 7-day average (same hours)', params: ['threshold_pct', 'direction'] },
  { value: 'revenue_drop', label: 'Revenue down X% vs same window last week', params: ['threshold_pct'] },
  { value: 'no_calls', label: 'Campaign has no calls for the window', params: ['active_hours'] },
  { value: 'integration_unhealthy', label: 'Integration sync failing or degraded (notify only)', params: [] },
  { value: 'qa_flag_rate_above', label: 'QA flag rate above X% (after QA)', params: ['threshold_pct'], qa: true },
  { value: 'qa_score_below', label: 'Average QA score below X (after QA)', params: ['threshold'], qa: true },
  { value: 'should_convert_share_above', label: 'Should-have-converted above X% (after QA)', params: ['threshold_pct'], qa: true },
  { value: 'do_not_call_requests', label: 'Do-not-call requests: N calls (after QA)', params: ['count'], qa: true },
];

export function triggerLabel(t: string): string {
  return TRIGGER_OPTIONS.find((o) => o.value === t)?.label ?? t;
}

export const MODE_LABELS: Record<ActionMode, string> = {
  notify: 'Notify only',
  suggest: 'Suggest (Pause button, admin confirms)',
  auto: 'Auto (Calyx pauses, then notifies)',
};

export interface ModeAvailability {
  mode: ActionMode;
  enabled: boolean;
  reason: string | null;
}

const DAY_MS = 86_400_000;

/**
 * Which modes the rule form may offer. Mirrors the API checks so the form explains them;
 * the API refuses anything invalid regardless.
 */
export function modeAvailability(input: {
  trigger: string;
  scope: RuleScope;
  platform: 'ringba' | 'retreaver' | null;
  savedMode: ActionMode | null;
  suggestSince: string | null;
  ringbaVerified: boolean;
  now?: Date;
}): ModeAvailability[] {
  const now = input.now ?? new Date();
  const notifyOnly = input.trigger === 'integration_unhealthy';
  const needsPublisher = input.scope !== 'publisher';
  const pauseReason = notifyOnly
    ? 'This trigger is notify only (it never pauses).'
    : needsPublisher
      ? 'Pausing works per publisher: set scope to Publisher.'
      : null;
  let autoReason: string | null = pauseReason;
  if (!autoReason && input.platform === 'ringba' && !input.ringbaVerified) {
    autoReason = 'Auto is off for Ringba until the Ringba pause is verified.';
  }
  if (!autoReason) {
    const since = input.savedMode && input.savedMode !== 'notify' ? input.suggestSince : null;
    if (!since || now.getTime() - new Date(since).getTime() < 7 * DAY_MS) {
      const left = since ? Math.ceil((7 * DAY_MS - (now.getTime() - new Date(since).getTime())) / DAY_MS) : 7;
      autoReason = since
        ? `Auto unlocks after 7 days in Suggest (${left} day${left === 1 ? '' : 's'} left).`
        : 'Auto needs 7 days in Suggest mode first. Save the rule in Suggest.';
    }
  }
  return [
    { mode: 'notify', enabled: true, reason: null },
    { mode: 'suggest', enabled: pauseReason === null, reason: pauseReason },
    { mode: 'auto', enabled: autoReason === null, reason: autoReason },
  ];
}

export function alertStatusLabel(s: string): string {
  switch (s) {
    case 'open':
      return 'Open';
    case 'acknowledged':
      return 'Acknowledged';
    case 'actioned':
      return 'Actioned';
    case 'resolved':
      return 'Resolved';
    default:
      return s;
  }
}

/** Label for the alert's action, including Q10: Retreaver confirmed by its own PUT response. */
export function alertActionLabel(
  action: string,
  platform: string | null,
  detail: Record<string, unknown> = {},
): string {
  switch (action) {
    case 'none':
      return 'Alert only';
    case 'suggested':
      return 'Pause suggested';
    case 'paused':
      if (detail.confirmation === 'response' || detail.outcome === 'confirmed_by_response') {
        return platform === 'retreaver'
          ? "Paused (confirmed by Retreaver's response)"
          : 'Paused (confirmed by the platform response)';
      }
      return 'Paused (read back confirmed)';
    case 'pause_blocked':
      return 'Pause blocked';
    case 'pause_failed':
      return 'Pause failed';
    case 'unpaused':
      return 'Unpaused';
    default:
      return action;
  }
}

export function platformName(p: string | null | undefined): string {
  if (p === 'ringba') return 'Ringba';
  if (p === 'retreaver') return 'Retreaver';
  return p ?? '';
}

/** The bold sentence in the confirm popup (the pause is global for the publisher). */
export function pauseWarningText(platform: string): string {
  return `Pausing stops this publisher on ALL campaigns on ${platformName(platform)}.`;
}

export type AlertButton = 'pause' | 'ringba_link' | 'unpause' | null;

/** Which action control an alert row shows. */
export function alertButton(a: {
  scope: string;
  platform: string | null;
  action: string;
  ringba_link?: string | null;
  pause_id?: string | null;
}): AlertButton {
  if (a.action === 'paused' && a.pause_id) return 'unpause';
  if (a.scope !== 'publisher') return null;
  if (!['suggested', 'pause_blocked', 'pause_failed'].includes(a.action)) return null;
  if (a.platform === 'ringba' && a.ringba_link) return 'ringba_link';
  return 'pause';
}

/**
 * Link to the Pings list for this alert, filtered to the alert's publisher (or campaign) and
 * platform. Tenant-wide alerts open the list unfiltered.
 */
export function pingsLinkFor(alert: { entity_id: string; platform: string | null; scope?: string }): string {
  const params = new URLSearchParams();
  if (alert.entity_id) params.set(alert.scope === 'campaign' ? 'campaign' : 'publisher', alert.entity_id);
  if (alert.platform) params.set('platform', alert.platform);
  const qs = params.toString();
  return qs ? `/pings?${qs}` : '/pings';
}

/** Human-readable key for a numbers object ("conversion_rate_pct" → "Conversion rate %"). */
export function numberLabel(key: string): string {
  const map: Record<string, string> = {
    calls: 'Calls',
    converted: 'Converted',
    conversion_rate_pct: 'Conversion rate %',
    threshold_pct: 'Threshold %',
    short_calls: 'Short calls',
    short_share_pct: 'Short share %',
    short_seconds: 'Short = under (s)',
    repeat_calls: 'Repeat calls',
    repeat_share_pct: 'Repeat share %',
    baseline_avg_calls: '7-day avg calls',
    change_pct: 'Change %',
    direction: 'Direction',
    revenue: 'Revenue',
    revenue_prev_week: 'Revenue last week',
    calls_prev_week: 'Calls last week',
    drop_pct: 'Drop %',
    calls_7d_before: 'Calls in prior 7 days',
    min_calls: 'Min calls',
    window_hours: 'Window (h)',
    sync_status: 'Sync status',
    last_sync_at: 'Last sync',
  };
  return map[key] ?? key;
}

export function bellCountLabel(n: number): string {
  if (n <= 0) return '';
  return n > 99 ? '99+' : String(n);
}
