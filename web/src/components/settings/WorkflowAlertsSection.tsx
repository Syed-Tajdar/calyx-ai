'use client';

import { FormEvent, useCallback, useEffect, useMemo, useState } from 'react';
import { Pencil, Trash2 } from 'lucide-react';
import { toast } from 'sonner';
import { Button } from '@/components/ui/button';
import { ErrorBanner, PageLoader } from '@/components/ui/PageStatus';
import {
  addProtectedPublisher,
  ApiError,
  createWorkflowRule,
  deleteWorkflowRule,
  dryRunWorkflowRule,
  fetchProtectedPublishers,
  fetchPublisherAudit,
  fetchWorkflowIntegrations,
  fetchWorkflowRules,
  fetchWorkflowStatus,
  ProtectedPublisher,
  PublisherActionAudit,
  removeProtectedPublisher,
  setAllowPublisherPause,
  updateWorkflowRule,
  WorkflowDryRun,
  WorkflowIntegration,
  WorkflowRuleInput,
  WorkflowRuleRecord,
  WorkflowStatus,
} from '@/lib/api';
import { getToken } from '@/lib/auth';
import {
  ActionMode,
  MODE_LABELS,
  modeAvailability,
  numberLabel,
  platformName,
  RuleScope,
  TRIGGER_OPTIONS,
  triggerLabel,
} from '@/lib/workflows';

interface FormState {
  name: string;
  trigger_type: string;
  threshold_pct: string;
  short_seconds: string;
  direction: 'up' | 'down' | 'both';
  active_start: string;
  active_end: string;
  threshold: string;
  count: string;
  window_hours: string;
  min_calls: string;
  scope: RuleScope;
  platform: '' | 'ringba' | 'retreaver';
  notify_email: boolean;
  action_mode: ActionMode;
  pause_hours: string;
  cooldown_hours: string;
  is_active: boolean;
}

const EMPTY: FormState = {
  name: '',
  trigger_type: 'conversion_rate_below',
  threshold_pct: '5',
  short_seconds: '20',
  direction: 'both',
  active_start: '',
  active_end: '',
  threshold: '60',
  count: '3',
  window_hours: '24',
  min_calls: '20',
  scope: 'publisher',
  platform: '',
  notify_email: false,
  action_mode: 'notify',
  pause_hours: '',
  cooldown_hours: '6',
  is_active: true,
};

function fromRule(r: WorkflowRuleRecord): FormState {
  const p = r.params as Record<string, unknown>;
  const ah = p.active_hours as { start: number; end: number } | undefined;
  return {
    ...EMPTY,
    name: r.name,
    trigger_type: r.trigger_type,
    threshold_pct: p.threshold_pct !== undefined ? String(p.threshold_pct) : EMPTY.threshold_pct,
    short_seconds: p.short_seconds !== undefined ? String(p.short_seconds) : EMPTY.short_seconds,
    direction: (p.direction as FormState['direction']) ?? 'both',
    active_start: ah ? String(ah.start) : '',
    active_end: ah ? String(ah.end) : '',
    threshold: p.threshold !== undefined ? String(p.threshold) : EMPTY.threshold,
    count: p.count !== undefined ? String(p.count) : EMPTY.count,
    window_hours: String(r.window_hours),
    min_calls: String(r.min_calls),
    scope: r.scope,
    platform: r.platform ?? '',
    notify_email: r.notify_email,
    action_mode: r.action_mode,
    pause_hours: r.pause_hours === null ? '' : String(r.pause_hours),
    cooldown_hours: String(r.cooldown_hours),
    is_active: r.is_active,
  };
}

function toInput(f: FormState): WorkflowRuleInput {
  const opt = TRIGGER_OPTIONS.find((o) => o.value === f.trigger_type);
  const params: Record<string, unknown> = {};
  for (const k of opt?.params ?? []) {
    if (k === 'threshold_pct') params.threshold_pct = Number(f.threshold_pct);
    if (k === 'short_seconds') params.short_seconds = Number(f.short_seconds);
    if (k === 'direction') params.direction = f.direction;
    if (k === 'threshold') params.threshold = Number(f.threshold);
    if (k === 'count') params.count = Number(f.count);
    if (k === 'active_hours' && f.active_start !== '' && f.active_end !== '') {
      params.active_hours = { start: Number(f.active_start), end: Number(f.active_end) };
    }
  }
  return {
    name: f.name.trim(),
    trigger_type: f.trigger_type,
    params,
    window_hours: Number(f.window_hours),
    min_calls: Number(f.min_calls),
    scope: f.scope,
    platform: f.platform === '' ? null : f.platform,
    notify_email: f.notify_email,
    action_mode: f.action_mode,
    pause_hours: f.action_mode !== 'notify' && f.pause_hours !== '' ? Number(f.pause_hours) : null,
    cooldown_hours: Number(f.cooldown_hours),
    is_active: f.is_active,
  };
}

const card = 'calyx-glass calyx-glass-lit overflow-hidden';
const head = 'border-b border-calyx-border/60 px-4 py-3';
const label = 'mb-1 block text-xs text-calyx-muted';

/** Settings → Alerts (v2 Phase 2D). Admin only; the API enforces every guardrail. */
export function WorkflowAlertsSection(): JSX.Element {
  const [rules, setRules] = useState<WorkflowRuleRecord[]>([]);
  const [status, setStatus] = useState<WorkflowStatus | null>(null);
  const [integrations, setIntegrations] = useState<WorkflowIntegration[]>([]);
  const [protectedPubs, setProtectedPubs] = useState<ProtectedPublisher[]>([]);
  const [audit, setAudit] = useState<PublisherActionAudit[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [form, setForm] = useState<FormState>(EMPTY);
  const [editing, setEditing] = useState<WorkflowRuleRecord | null>(null);
  const [saving, setSaving] = useState(false);
  const [dryRun, setDryRun] = useState<WorkflowDryRun | null>(null);
  const [dryRunning, setDryRunning] = useState(false);
  const [newProt, setNewProt] = useState<ProtectedPublisher>({ platform: 'retreaver', publisher_id: '', name: '' });

  const load = useCallback(async () => {
    const token = getToken();
    if (!token) return;
    try {
      const [r, s, i, p, a] = await Promise.all([
        fetchWorkflowRules(token),
        fetchWorkflowStatus(token),
        fetchWorkflowIntegrations(token),
        fetchProtectedPublishers(token),
        fetchPublisherAudit(token),
      ]);
      setRules(r.data);
      setStatus(s);
      setIntegrations(i.data);
      setProtectedPubs(p.data);
      setAudit(a.data);
      setError('');
    } catch (err: unknown) {
      setError(err instanceof ApiError && err.status === 403 ? 'Alerts settings are for admins.' : 'Failed to load alerts settings');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  const trigger = TRIGGER_OPTIONS.find((o) => o.value === form.trigger_type);
  const modes = useMemo(
    () =>
      modeAvailability({
        trigger: form.trigger_type,
        scope: form.scope,
        platform: form.platform === '' ? null : form.platform,
        savedMode: editing?.action_mode ?? null,
        suggestSince: editing?.suggest_since ?? null,
        ringbaVerified: status?.ringba_pause_verified ?? false,
      }),
    [form.trigger_type, form.scope, form.platform, editing, status],
  );

  function set<K extends keyof FormState>(k: K, v: FormState[K]): void {
    setForm((prev) => ({ ...prev, [k]: v }));
  }

  async function save(e: FormEvent): Promise<void> {
    e.preventDefault();
    const token = getToken();
    if (!token) return;
    setSaving(true);
    try {
      const input = toInput(form);
      if (editing) await updateWorkflowRule(token, editing.id, input);
      else await createWorkflowRule(token, input);
      toast.success(editing ? 'Rule updated' : 'Rule created');
      setForm(EMPTY);
      setEditing(null);
      setDryRun(null);
      await load();
    } catch (err: unknown) {
      toast.error(err instanceof ApiError ? err.message : 'Failed to save rule');
    } finally {
      setSaving(false);
    }
  }

  async function runDry(): Promise<void> {
    const token = getToken();
    if (!token) return;
    setDryRunning(true);
    try {
      setDryRun(await dryRunWorkflowRule(token, { ...toInput(form), action_mode: 'notify', pause_hours: null }));
    } catch (err: unknown) {
      toast.error(err instanceof ApiError ? err.message : 'Dry run failed');
    } finally {
      setDryRunning(false);
    }
  }

  async function remove(r: WorkflowRuleRecord): Promise<void> {
    if (!window.confirm(`Delete rule "${r.name}"?`)) return;
    const token = getToken();
    if (!token) return;
    try {
      await deleteWorkflowRule(token, r.id);
      await load();
    } catch {
      toast.error('Failed to delete rule');
    }
  }

  async function toggleAllow(i: WorkflowIntegration): Promise<void> {
    const token = getToken();
    if (!token) return;
    try {
      await setAllowPublisherPause(token, i.id, !i.allow_publisher_pause);
      await load();
    } catch (err: unknown) {
      toast.error(err instanceof ApiError ? err.message : 'Failed to change the setting');
    }
  }

  async function addProt(e: FormEvent): Promise<void> {
    e.preventDefault();
    const token = getToken();
    if (!token || !newProt.publisher_id.trim()) return;
    try {
      await addProtectedPublisher(token, { ...newProt, publisher_id: newProt.publisher_id.trim() });
      setNewProt({ platform: newProt.platform, publisher_id: '', name: '' });
      await load();
    } catch (err: unknown) {
      toast.error(err instanceof ApiError ? err.message : 'Failed to add');
    }
  }

  async function removeProt(p: ProtectedPublisher): Promise<void> {
    const token = getToken();
    if (!token) return;
    try {
      await removeProtectedPublisher(token, p.platform, p.publisher_id);
      await load();
    } catch {
      toast.error('Failed to remove');
    }
  }

  if (loading) return <PageLoader />;
  if (error) return <ErrorBanner message={error} />;

  return (
    <div className="space-y-6">
      {status && (
        <div className="calyx-glass p-4 text-xs text-calyx-muted">
          <p>
            Publisher actions on the server:{' '}
            <span className={status.actions_enabled ? 'text-emerald-300' : 'text-amber-300'}>
              {status.actions_enabled ? 'on' : 'off (alerts and suggestions only, nothing is paused)'}
            </span>
            {' · '}Ringba pause: {status.ringba_pause_verified ? 'verified' : 'not verified (Ringba gets a link, no pause)'}
            {' · '}Automatic pauses today: {status.auto_pauses_today} / {status.max_auto_pauses_daily}
            {' · '}Email: {status.email_enabled ? 'on' : 'off (bell only)'}
          </p>
        </div>
      )}

      <div className={card}>
        <div className={head}>
          <p className="text-[10px] font-bold uppercase tracking-wider text-calyx-teal">Workflows</p>
          <h3 className="font-heading text-sm font-semibold text-calyx-text">Rules</h3>
        </div>
        <div className="overflow-x-auto">
          <table className="w-full text-left text-xs">
            <thead>
              <tr className="border-b border-calyx-border text-[10px] uppercase text-calyx-muted">
                <th className="px-4 py-3 font-medium">Name</th>
                <th className="px-4 py-3 font-medium">Trigger</th>
                <th className="px-4 py-3 font-medium">Scope</th>
                <th className="px-4 py-3 font-medium">Window / min calls</th>
                <th className="px-4 py-3 font-medium">Mode</th>
                <th className="px-4 py-3 font-medium">Suggested (7d)</th>
                <th className="px-4 py-3 font-medium" />
              </tr>
            </thead>
            <tbody>
              {rules.map((r) => (
                <tr key={r.id} className="border-b border-calyx-border/40">
                  <td className="px-4 py-3 text-calyx-text">
                    {r.name}
                    {!r.is_active && <span className="ml-2 text-calyx-muted">(off)</span>}
                  </td>
                  <td className="px-4 py-3 text-calyx-muted">{triggerLabel(r.trigger_type)}</td>
                  <td className="px-4 py-3 text-calyx-muted">
                    {r.scope}
                    {r.platform ? ` · ${platformName(r.platform)}` : ' · both'}
                  </td>
                  <td className="px-4 py-3 tabular-nums text-calyx-muted">
                    {r.window_hours}h / {r.min_calls}
                  </td>
                  <td className="px-4 py-3 text-calyx-muted">{r.action_mode}</td>
                  <td className="px-4 py-3 tabular-nums text-calyx-muted">{r.suggested_last_7d ?? 0}</td>
                  <td className="whitespace-nowrap px-4 py-3 text-right">
                    <Button
                      type="button"
                      variant="ghost"
                      size="icon"
                      className="h-8 w-8"
                      aria-label={`Edit ${r.name}`}
                      onClick={() => {
                        setEditing(r);
                        setForm(fromRule(r));
                        setDryRun(null);
                      }}
                    >
                      <Pencil className="h-4 w-4" />
                    </Button>
                    <Button
                      type="button"
                      variant="ghost"
                      size="icon"
                      className="h-8 w-8 text-calyx-danger"
                      aria-label={`Delete ${r.name}`}
                      onClick={() => void remove(r)}
                    >
                      <Trash2 className="h-4 w-4" />
                    </Button>
                  </td>
                </tr>
              ))}
              {rules.length === 0 && (
                <tr>
                  <td colSpan={7} className="px-4 py-8 text-center text-calyx-muted">
                    No rules yet.
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        </div>
      </div>

      <form onSubmit={save} className="calyx-glass calyx-glass-lit space-y-4 p-4">
        <p className="font-heading text-sm font-semibold text-calyx-text">{editing ? `Edit rule: ${editing.name}` : 'New rule'}</p>
        <div className="grid gap-4 sm:grid-cols-2">
          <div>
            <label htmlFor="wf-name" className={label}>Name</label>
            <input id="wf-name" className="calyx-input text-sm" required value={form.name} onChange={(e) => set('name', e.target.value)} />
          </div>
          <div>
            <label htmlFor="wf-trigger" className={label}>Trigger</label>
            <select
              id="wf-trigger"
              className="calyx-input text-sm"
              value={form.trigger_type}
              onChange={(e) => {
                const t = e.target.value;
                setForm((prev) => ({
                  ...prev,
                  trigger_type: t,
                  scope: t === 'no_calls' ? 'campaign' : t === 'integration_unhealthy' ? 'tenant' : prev.scope,
                  action_mode: t === 'integration_unhealthy' ? 'notify' : prev.action_mode,
                }));
              }}
            >
              {TRIGGER_OPTIONS.map((o) => (
                <option key={o.value} value={o.value}>
                  {o.label}
                </option>
              ))}
            </select>
            {trigger?.qa && <p className="mt-1 text-[11px] text-amber-300">Saved now; evaluated once QA (Phase 2C) is live.</p>}
          </div>

          {trigger?.params.includes('threshold_pct') && (
            <div>
              <label htmlFor="wf-thr" className={label}>Threshold (%)</label>
              <input id="wf-thr" type="number" min={0} step={0.1} className="calyx-input text-sm" value={form.threshold_pct} onChange={(e) => set('threshold_pct', e.target.value)} />
            </div>
          )}
          {trigger?.params.includes('short_seconds') && (
            <div>
              <label htmlFor="wf-short" className={label}>Short call = under (seconds)</label>
              <input id="wf-short" type="number" min={1} className="calyx-input text-sm" value={form.short_seconds} onChange={(e) => set('short_seconds', e.target.value)} />
            </div>
          )}
          {trigger?.params.includes('direction') && (
            <div>
              <label htmlFor="wf-dir" className={label}>Direction</label>
              <select id="wf-dir" className="calyx-input text-sm" value={form.direction} onChange={(e) => set('direction', e.target.value as FormState['direction'])}>
                <option value="both">Up or down</option>
                <option value="down">Down</option>
                <option value="up">Up</option>
              </select>
            </div>
          )}
          {trigger?.params.includes('active_hours') && (
            <div>
              <span className={label}>Active hours, Eastern (optional, start–end)</span>
              <div className="flex gap-2">
                <input aria-label="Active hours start" type="number" min={0} max={23} placeholder="9" className="calyx-input text-sm" value={form.active_start} onChange={(e) => set('active_start', e.target.value)} />
                <input aria-label="Active hours end" type="number" min={1} max={24} placeholder="17" className="calyx-input text-sm" value={form.active_end} onChange={(e) => set('active_end', e.target.value)} />
              </div>
            </div>
          )}
          {trigger?.params.includes('threshold') && (
            <div>
              <label htmlFor="wf-score" className={label}>Score threshold</label>
              <input id="wf-score" type="number" min={0} max={100} className="calyx-input text-sm" value={form.threshold} onChange={(e) => set('threshold', e.target.value)} />
            </div>
          )}
          {trigger?.params.includes('count') && (
            <div>
              <label htmlFor="wf-count" className={label}>Calls</label>
              <input id="wf-count" type="number" min={1} className="calyx-input text-sm" value={form.count} onChange={(e) => set('count', e.target.value)} />
            </div>
          )}

          <div>
            <label htmlFor="wf-window" className={label}>Window (hours)</label>
            <input id="wf-window" type="number" min={1} max={720} className="calyx-input text-sm" value={form.window_hours} onChange={(e) => set('window_hours', e.target.value)} />
          </div>
          <div>
            <label htmlFor="wf-min" className={label}>Minimum calls in the window</label>
            <input id="wf-min" type="number" min={0} className="calyx-input text-sm" value={form.min_calls} onChange={(e) => set('min_calls', e.target.value)} />
          </div>
          <div>
            <label htmlFor="wf-scope" className={label}>Scope</label>
            <select id="wf-scope" className="calyx-input text-sm" value={form.scope} onChange={(e) => set('scope', e.target.value as RuleScope)}>
              <option value="publisher">Per publisher</option>
              <option value="campaign">Per campaign</option>
              <option value="tenant">Whole tenant</option>
            </select>
          </div>
          <div>
            <label htmlFor="wf-platform" className={label}>Platform</label>
            <select id="wf-platform" className="calyx-input text-sm" value={form.platform} onChange={(e) => set('platform', e.target.value as FormState['platform'])}>
              <option value="">Both</option>
              <option value="ringba">Ringba</option>
              <option value="retreaver">Retreaver</option>
            </select>
          </div>
          <div>
            <label htmlFor="wf-cool" className={label}>Cooldown (hours between alerts per entity)</label>
            <input id="wf-cool" type="number" min={1} max={168} className="calyx-input text-sm" value={form.cooldown_hours} onChange={(e) => set('cooldown_hours', e.target.value)} />
          </div>
          <div className="flex items-end gap-4">
            <label className="flex items-center gap-2 text-xs text-calyx-text">
              <input type="checkbox" checked={form.notify_email} onChange={(e) => set('notify_email', e.target.checked)} />
              Email admins{status && !status.email_enabled ? ' (email is off on the server)' : ''}
            </label>
            <label className="flex items-center gap-2 text-xs text-calyx-text">
              <input type="checkbox" checked={form.is_active} onChange={(e) => set('is_active', e.target.checked)} />
              Active
            </label>
          </div>
        </div>

        <fieldset className="space-y-2">
          <legend className={label}>Action</legend>
          {modes.map((m) => (
            <label key={m.mode} className={`flex items-start gap-2 text-xs ${m.enabled ? 'text-calyx-text' : 'text-calyx-muted'}`}>
              <input
                type="radio"
                name="wf-mode"
                value={m.mode}
                disabled={!m.enabled}
                checked={form.action_mode === m.mode}
                onChange={() => set('action_mode', m.mode)}
              />
              <span>
                {MODE_LABELS[m.mode]}
                {m.reason && <span className="block text-[11px] text-amber-300">{m.reason}</span>}
              </span>
            </label>
          ))}
        </fieldset>
        {form.action_mode !== 'notify' && (
          <div className="max-w-xs">
            <label htmlFor="wf-pause-h" className={label}>Pause for N hours, then unpause (optional)</label>
            <input id="wf-pause-h" type="number" min={1} max={720} className="calyx-input text-sm" value={form.pause_hours} onChange={(e) => set('pause_hours', e.target.value)} />
          </div>
        )}

        <div className="flex flex-wrap gap-2">
          <button type="submit" disabled={saving} className="calyx-btn-primary text-sm">
            {saving ? 'Saving…' : editing ? 'Save rule' : 'Create rule'}
          </button>
          <Button type="button" variant="outline" size="sm" disabled={dryRunning} onClick={() => void runDry()}>
            {dryRunning ? 'Running…' : 'Dry run (today, no action)'}
          </Button>
          {editing && (
            <Button type="button" variant="ghost" size="sm" onClick={() => { setEditing(null); setForm(EMPTY); setDryRun(null); }}>
              Cancel edit
            </Button>
          )}
        </div>

        {dryRun && (
          <div className="space-y-2 rounded-lg border border-calyx-border/60 p-3 text-xs" data-testid="dry-run">
            <p className="text-calyx-muted">
              Window {new Date(dryRun.window.from).toLocaleString('en-US', { timeZone: 'America/New_York' })} –{' '}
              {new Date(dryRun.window.to).toLocaleString('en-US', { timeZone: 'America/New_York' })} ET · {dryRun.evaluated} checked ·{' '}
              {dryRun.matches.length} would match · {dryRun.below_min_calls} below the minimum calls
            </p>
            {dryRun.skipped_reason && <p className="text-amber-300">{dryRun.skipped_reason}</p>}
            {dryRun.matches.length > 0 && (
              <div className="overflow-x-auto">
                <table className="w-full text-left">
                  <thead>
                    <tr className="text-[10px] uppercase text-calyx-muted">
                      <th className="py-1 pr-3">Entity</th>
                      <th className="py-1 pr-3">Platform</th>
                      <th className="py-1">Numbers</th>
                    </tr>
                  </thead>
                  <tbody>
                    {dryRun.matches.map((m) => (
                      <tr key={`${m.platform}:${m.entity_id}`} className="border-t border-calyx-border/40">
                        <td className="py-1 pr-3 text-calyx-text">{m.entity_name || m.entity_id || 'Tenant'}{m.entity_name ? ` (${m.entity_id})` : ''}</td>
                        <td className="py-1 pr-3 text-calyx-muted">{platformName(m.platform)}</td>
                        <td className="py-1 text-calyx-muted">
                          {Object.entries(m.numbers).map(([k, v]) => `${numberLabel(k)}: ${v}`).join(' · ')}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </div>
        )}
      </form>

      <div className={card}>
        <div className={head}>
          <h3 className="font-heading text-sm font-semibold text-calyx-text">Allow Calyx to pause publishers</h3>
          <p className="text-[11px] text-calyx-muted">Off by default. Live integrations only; Test integrations are never paused.</p>
        </div>
        <ul className="divide-y divide-calyx-border/40 text-xs">
          {integrations.map((i) => (
            <li key={i.id} className="flex items-center justify-between gap-3 px-4 py-3">
              <span className="text-calyx-text">
                {i.name} <span className="text-calyx-muted">· {platformName(i.platform)} · {i.live ? 'Live' : 'not Live'}</span>
              </span>
              <label className="flex items-center gap-2 text-calyx-text">
                <input
                  type="checkbox"
                  role="switch"
                  aria-label={`Allow Calyx to pause publishers on ${i.name}`}
                  checked={i.allow_publisher_pause}
                  disabled={!i.live && !i.allow_publisher_pause}
                  onChange={() => void toggleAllow(i)}
                />
                {i.allow_publisher_pause ? 'On' : 'Off'}
              </label>
            </li>
          ))}
          {integrations.length === 0 && <li className="px-4 py-6 text-center text-calyx-muted">No Ringba or Retreaver integrations.</li>}
        </ul>
      </div>

      <div className={card}>
        <div className={head}>
          <h3 className="font-heading text-sm font-semibold text-calyx-text">Protected publishers</h3>
          <p className="text-[11px] text-calyx-muted">Never paused automatically (Suggest only).</p>
        </div>
        <ul className="divide-y divide-calyx-border/40 text-xs">
          {protectedPubs.map((p) => (
            <li key={`${p.platform}:${p.publisher_id}`} className="flex items-center justify-between px-4 py-2">
              <span className="text-calyx-text">
                {p.name || p.publisher_id} <span className="text-calyx-muted">· {platformName(p.platform)} · {p.publisher_id}</span>
              </span>
              <Button type="button" variant="ghost" size="sm" className="text-calyx-danger" onClick={() => void removeProt(p)}>
                Remove
              </Button>
            </li>
          ))}
        </ul>
        <form onSubmit={addProt} className="flex flex-wrap items-end gap-2 border-t border-calyx-border/60 p-4">
          <select aria-label="Platform" className="calyx-input w-36 text-sm" value={newProt.platform} onChange={(e) => setNewProt({ ...newProt, platform: e.target.value as ProtectedPublisher['platform'] })}>
            <option value="retreaver">Retreaver</option>
            <option value="ringba">Ringba</option>
          </select>
          <input aria-label="Publisher id" placeholder="Publisher id" className="calyx-input w-44 text-sm" value={newProt.publisher_id} onChange={(e) => setNewProt({ ...newProt, publisher_id: e.target.value })} />
          <input aria-label="Name" placeholder="Name (optional)" className="calyx-input w-44 text-sm" value={newProt.name} onChange={(e) => setNewProt({ ...newProt, name: e.target.value })} />
          <Button type="submit" size="sm">Add</Button>
        </form>
      </div>

      <div className={card}>
        <div className={head}>
          <h3 className="font-heading text-sm font-semibold text-calyx-text">Recent publisher actions</h3>
          <p className="text-[11px] text-calyx-muted">Every pause and unpause attempt, including blocked ones (last 50).</p>
        </div>
        <div className="overflow-x-auto">
          <table className="w-full text-left text-xs">
            <thead>
              <tr className="border-b border-calyx-border text-[10px] uppercase text-calyx-muted">
                <th className="px-4 py-2 font-medium">When</th>
                <th className="px-4 py-2 font-medium">Publisher</th>
                <th className="px-4 py-2 font-medium">Action</th>
                <th className="px-4 py-2 font-medium">Mode</th>
                <th className="px-4 py-2 font-medium">Result</th>
                <th className="px-4 py-2 font-medium">Message</th>
              </tr>
            </thead>
            <tbody>
              {audit.map((a) => (
                <tr key={a.id} className="border-b border-calyx-border/40">
                  <td className="whitespace-nowrap px-4 py-2 text-calyx-muted">
                    {new Date(a.created_at.replace(' ', 'T')).toLocaleString('en-US', { timeZone: 'America/New_York' })}
                  </td>
                  <td className="px-4 py-2 text-calyx-text">{platformName(a.platform)} · {a.publisher_id}</td>
                  <td className="px-4 py-2 text-calyx-muted">{a.action}</td>
                  <td className="px-4 py-2 text-calyx-muted">{a.mode}{a.user_id ? '' : a.mode === 'manual' ? '' : ' (Calyx)'}</td>
                  <td className="px-4 py-2 text-calyx-muted">
                    {a.outcome === 'confirmed_by_response' && a.platform === 'retreaver'
                      ? "confirmed by Retreaver's response"
                      : a.outcome}
                  </td>
                  <td className="px-4 py-2 text-calyx-muted">{a.message ?? ''}</td>
                </tr>
              ))}
              {audit.length === 0 && (
                <tr>
                  <td colSpan={6} className="px-4 py-6 text-center text-calyx-muted">No publisher actions yet.</td>
                </tr>
              )}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  );
}
