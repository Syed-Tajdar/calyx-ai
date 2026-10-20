'use client';

import Link from 'next/link';
import { useCallback, useEffect, useState } from 'react';
import { useRouter } from 'next/navigation';
import { ExternalLink } from 'lucide-react';
import { toast } from 'sonner';
import { Button } from '@/components/ui/button';
import { ConfirmDialog } from '@/components/ui/ConfirmDialog';
import { ErrorBanner, PageLoader } from '@/components/ui/PageStatus';
import {
  acknowledgeWorkflowAlert,
  ApiError,
  fetchPublisherImpact,
  fetchWorkflowAlerts,
  fetchWorkflowStatus,
  pauseFromAlert,
  PublisherImpact,
  unpausePublisher,
  WorkflowAlert,
  WorkflowStatus,
} from '@/lib/api';
import { getToken } from '@/lib/auth';
import {
  alertActionLabel,
  alertButton,
  alertStatusLabel,
  numberLabel,
  pauseWarningText,
  pingsLinkFor,
  platformName,
  triggerLabel,
} from '@/lib/workflows';

const FILTERS: Array<{ value: string; label: string }> = [
  { value: '', label: 'All' },
  { value: 'open', label: 'Open' },
  { value: 'acknowledged', label: 'Acknowledged' },
  { value: 'actioned', label: 'Actioned' },
];

const STATUS_CLASS: Record<string, string> = {
  open: 'bg-amber-500/15 text-amber-300',
  acknowledged: 'bg-sky-500/15 text-sky-300',
  actioned: 'bg-emerald-500/15 text-emerald-300',
  resolved: 'bg-white/10 text-calyx-muted',
};

function fmtTime(iso: string | null): string {
  if (!iso) return '';
  const d = new Date(iso.replace(' ', 'T'));
  if (Number.isNaN(d.getTime())) return iso;
  return d.toLocaleString('en-US', { timeZone: 'America/New_York', dateStyle: 'medium', timeStyle: 'short' }) + ' ET';
}

/** Alert centre: history, numbers used, Pause (Suggest confirm) / Unpause, Ringba link. Admin only. */
export function AlertCenter(): JSX.Element {
  const router = useRouter();
  const [alerts, setAlerts] = useState<WorkflowAlert[]>([]);
  const [status, setStatus] = useState<WorkflowStatus | null>(null);
  const [filter, setFilter] = useState('');
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [busyId, setBusyId] = useState<string | null>(null);
  const [confirm, setConfirm] = useState<{ alert: WorkflowAlert; impact: PublisherImpact | null } | null>(null);

  const load = useCallback(async () => {
    const token = getToken();
    if (!token) return;
    try {
      const [list, st] = await Promise.all([fetchWorkflowAlerts(token, filter || undefined), fetchWorkflowStatus(token)]);
      setAlerts(list.data);
      setStatus(st);
      setError('');
    } catch (err: unknown) {
      if (err instanceof ApiError && err.status === 401) router.replace('/login');
      else if (err instanceof ApiError && err.status === 403) setError('The alert centre is for admins.');
      else setError('Failed to load alerts');
    } finally {
      setLoading(false);
    }
  }, [filter, router]);

  useEffect(() => {
    void load();
  }, [load]);

  async function ack(a: WorkflowAlert): Promise<void> {
    const token = getToken();
    if (!token) return;
    setBusyId(a.id);
    try {
      await acknowledgeWorkflowAlert(token, a.id);
      await load();
    } catch (err: unknown) {
      toast.error(err instanceof ApiError ? err.message : 'Failed to acknowledge');
    } finally {
      setBusyId(null);
    }
  }

  async function openPause(a: WorkflowAlert): Promise<void> {
    const token = getToken();
    if (!token || !a.platform) return;
    setConfirm({ alert: a, impact: null });
    try {
      const impact = await fetchPublisherImpact(token, a.platform, a.entity_id);
      setConfirm({ alert: a, impact });
    } catch {
      toast.error('Could not load the last 7 days for this publisher');
    }
  }

  async function doPause(): Promise<void> {
    const token = getToken();
    if (!token || !confirm) return;
    setBusyId(confirm.alert.id);
    try {
      const res = await pauseFromAlert(token, confirm.alert.id);
      toast.success(
        res.outcome === 'confirmed_by_response' && confirm.alert.platform === 'retreaver'
          ? "Paused (confirmed by Retreaver's response)"
          : 'Publisher paused',
      );
      setConfirm(null);
      await load();
    } catch (err: unknown) {
      toast.error(err instanceof ApiError ? err.message : 'Pause failed');
      setConfirm(null);
      await load();
    } finally {
      setBusyId(null);
    }
  }

  async function doUnpause(a: WorkflowAlert): Promise<void> {
    const token = getToken();
    if (!token || !a.pause_id) return;
    if (!window.confirm(`Unpause ${a.entity_name || a.entity_id} on ${platformName(a.platform)}?`)) return;
    setBusyId(a.id);
    try {
      await unpausePublisher(token, a.pause_id);
      toast.success('Publisher unpaused');
      await load();
    } catch (err: unknown) {
      toast.error(err instanceof ApiError ? err.message : 'Unpause failed');
    } finally {
      setBusyId(null);
    }
  }

  return (
    <div className="space-y-4">
      {status && !status.actions_enabled && (
        <div className="rounded-xl border border-amber-500/30 bg-amber-500/10 px-4 py-3 text-xs text-amber-200">
          Publisher actions are switched off on the server. Rules alert and suggest; nothing is paused on Ringba or
          Retreaver.
        </div>
      )}
      <div className="flex flex-wrap gap-2">
        {FILTERS.map((f) => (
          <Button
            key={f.value || 'all'}
            type="button"
            size="sm"
            variant={filter === f.value ? 'default' : 'outline'}
            onClick={() => setFilter(f.value)}
          >
            {f.label}
          </Button>
        ))}
      </div>

      {loading && <PageLoader />}
      {error && <ErrorBanner message={error} />}
      {!loading && !error && alerts.length === 0 && (
        <p className="calyx-glass p-6 text-center text-sm text-calyx-muted">No alerts.</p>
      )}

      <ul className="space-y-3">
        {alerts.map((a) => {
          const button = alertButton(a);
          return (
            <li key={a.id} className="calyx-glass calyx-glass-lit space-y-3 p-4" data-testid="alert-row">
              <div className="flex flex-wrap items-start justify-between gap-2">
                <div className="min-w-0">
                  <p className="font-heading text-sm font-semibold text-calyx-text">
                    {a.rule_name ?? triggerLabel(a.trigger_type)}
                  </p>
                  <p className="text-xs text-calyx-muted">
                    {triggerLabel(a.trigger_type)} · {a.scope}
                    {a.entity_id ? ` · ${a.entity_name || a.entity_id}` : ''}
                    {a.entity_name && a.entity_id ? ` (${a.entity_id})` : ''}
                    {a.platform ? ` · ${platformName(a.platform)}` : ''}
                  </p>
                  <p className="text-[11px] text-calyx-muted">
                    {fmtTime(a.created_at)} · window {fmtTime(a.window_from)} – {fmtTime(a.window_to)}
                  </p>
                </div>
                <div className="flex flex-wrap items-center gap-2">
                  <span className={`rounded-full px-2 py-0.5 text-[10px] font-semibold ${STATUS_CLASS[a.status] ?? ''}`}>
                    {alertStatusLabel(a.status)}
                  </span>
                  <span className="rounded-full bg-white/5 px-2 py-0.5 text-[10px] text-calyx-text">
                    {alertActionLabel(a.action, a.platform, { ...a.action_detail, confirmation: a.pause_confirmation ?? a.action_detail.confirmation })}
                  </span>
                </div>
              </div>

              <div className="overflow-x-auto">
                <table className="text-left text-xs">
                  <tbody>
                    {Object.entries(a.numbers ?? {}).map(([k, v]) => (
                      <tr key={k}>
                        <td className="pr-4 text-calyx-muted">{numberLabel(k)}</td>
                        <td className="tabular-nums text-calyx-text">{String(v)}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>

              {typeof a.action_detail.reason === 'string' && (
                <p className="text-xs text-calyx-muted">{a.action_detail.reason}</p>
              )}
              {typeof a.action_detail.message === 'string' && a.action !== 'paused' && (
                <p className="text-xs text-red-300">{a.action_detail.message}</p>
              )}

              <div className="flex flex-wrap items-center gap-2">
                <Link href={pingsLinkFor(a)} className="text-xs text-calyx-teal underline">
                  Open Pings
                </Link>
                {a.status === 'open' && (
                  <Button type="button" size="sm" variant="outline" disabled={busyId === a.id} onClick={() => void ack(a)}>
                    Acknowledge
                  </Button>
                )}
                {button === 'pause' && (
                  <Button type="button" size="sm" disabled={busyId === a.id} onClick={() => void openPause(a)}>
                    Pause publisher
                  </Button>
                )}
                {button === 'ringba_link' && a.ringba_link && (
                  <a
                    href={a.ringba_link}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="inline-flex items-center gap-1 text-xs text-calyx-teal underline"
                  >
                    Pause it in Ringba (publisher {a.entity_id}) <ExternalLink className="h-3 w-3" />
                  </a>
                )}
                {button === 'unpause' && (
                  <Button type="button" size="sm" variant="outline" disabled={busyId === a.id} onClick={() => void doUnpause(a)}>
                    Unpause
                  </Button>
                )}
              </div>
              {button === 'ringba_link' && (
                <p className="text-[11px] text-calyx-muted">
                  Ringba has no direct link to one publisher: the link opens the Publishers list. Find publisher id{' '}
                  {a.entity_id} there and use its pause button.
                </p>
              )}
            </li>
          );
        })}
      </ul>

      <ConfirmDialog
        open={confirm !== null}
        title="Pause publisher?"
        confirmLabel="Pause publisher"
        destructive
        busy={confirm !== null && busyId === confirm.alert.id}
        confirmDisabled={!confirm?.impact}
        onCancel={() => setConfirm(null)}
        onConfirm={() => void doPause()}
      >
        {confirm && (
          <>
            <p className="font-bold text-calyx-text">{pauseWarningText(confirm.alert.platform ?? '')}</p>
            <p>
              Publisher: <span className="text-calyx-text">{confirm.alert.entity_name || confirm.alert.entity_id}</span>{' '}
              ({confirm.alert.entity_id})
            </p>
            {confirm.impact ? (
              <p>
                Last 7 days: <span className="tabular-nums text-calyx-text">{confirm.impact.calls}</span> calls,{' '}
                <span className="tabular-nums text-calyx-text">
                  {confirm.impact.revenue.toLocaleString('en-US', { style: 'currency', currency: 'USD' })}
                </span>{' '}
                revenue. Those calls and that revenue stop while it is paused.
              </p>
            ) : (
              <p>Loading the last 7 days…</p>
            )}
          </>
        )}
      </ConfirmDialog>
    </div>
  );
}
