'use client';

import { useCallback, useEffect, useState } from 'react';
import { toast } from 'sonner';
import { Button } from '@/components/ui/button';
import {
  ApiError,
  BackfillEstimate,
  fetchBackfillEstimate,
  fetchTranscriptionUsage,
  setTranscriptionArchiveAll,
  startBackfill,
  TranscriptionUsage,
} from '@/lib/api';
import { getToken } from '@/lib/auth';
import { formatBytes, formatUsd } from '@/lib/transcript';

const plural = (n: number, word: string): string => `${n} ${word}${n === 1 ? '' : 's'}`;

function YesNo({ ok, yes = 'Configured', no = 'Not configured' }: { ok: boolean; yes?: string; no?: string }): JSX.Element {
  return (
    <span className={`rounded-full px-2 py-0.5 text-[10px] font-medium ${ok ? 'bg-emerald-500/15 text-emerald-400' : 'bg-white/5 text-calyx-muted'}`}>
      {ok ? yes : no}
    </span>
  );
}

/**
 * v2 Phase 2B — Settings → Providers: Deepgram and the recording archive. Shows configured yes/no (never
 * a key), this month's minutes and estimated $, the daily cap, the archive size and $, and the queue.
 * Admins switch "Archive all recorded calls" and start a backfill after seeing its estimate.
 */
export function TranscriptionProvidersSection(): JSX.Element {
  const [usage, setUsage] = useState<TranscriptionUsage | null>(null);
  const [error, setError] = useState('');
  const [days, setDays] = useState(7);
  const [estimate, setEstimate] = useState<BackfillEstimate | null>(null);
  const [busy, setBusy] = useState(false);

  const load = useCallback(async () => {
    const token = getToken();
    if (!token) return;
    try {
      setUsage(await fetchTranscriptionUsage(token));
      setError('');
    } catch (err: unknown) {
      setError(err instanceof ApiError && err.status === 403 ? 'Providers are for admins.' : 'Usage could not be loaded');
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  async function toggleArchiveAll(next: boolean): Promise<void> {
    const token = getToken();
    if (!token) return;
    setBusy(true);
    try {
      await setTranscriptionArchiveAll(token, next);
      toast.success(next ? 'Every recorded call will be archived' : 'Only transcribed calls are archived');
      await load();
    } catch {
      toast.error('Could not change the archive setting');
    } finally {
      setBusy(false);
    }
  }

  async function preview(): Promise<void> {
    const token = getToken();
    if (!token) return;
    setBusy(true);
    try {
      setEstimate(await fetchBackfillEstimate(token, days));
    } catch (err: unknown) {
      toast.error(err instanceof ApiError ? err.message : 'Estimate failed');
    } finally {
      setBusy(false);
    }
  }

  async function start(): Promise<void> {
    const token = getToken();
    if (!token || !estimate) return;
    if (!window.confirm(`Queue ${estimate.calls} calls (about ${estimate.minutes} minutes, ${formatUsd(estimate.estimated_usd)})? The daily cap still applies.`)) return;
    setBusy(true);
    try {
      const r = await startBackfill(token, estimate.days);
      toast.success(`${plural(r.queued, 'call')} queued`);
      setEstimate(null);
      await load();
    } catch (err: unknown) {
      toast.error(err instanceof ApiError ? err.message : 'Backfill failed');
    } finally {
      setBusy(false);
    }
  }

  if (error) return <div className="calyx-card p-4 text-sm text-calyx-muted">{error}</div>;
  if (!usage) return <div className="calyx-card p-4 text-sm text-calyx-muted">Loading…</div>;

  const capPct = usage.daily_cap_minutes > 0 ? Math.min(100, Math.round((usage.minutes_today / usage.daily_cap_minutes) * 100)) : 0;

  return (
    <div className="space-y-4" data-testid="transcription-providers">
      <div className="calyx-card p-4">
        <p className="text-[10px] font-bold uppercase tracking-wider text-calyx-teal">Transcription</p>
        <div className="mt-3 grid gap-3 text-sm sm:grid-cols-2">
          <div className="flex items-center justify-between gap-2">
            <span className="text-calyx-muted">Deepgram (Nova-3)</span>
            <YesNo ok={usage.worker.deepgram_key_set} />
          </div>
          <div className="flex items-center justify-between gap-2">
            <span className="text-calyx-muted">Worker</span>
            <YesNo ok={usage.worker.enabled} yes="Running" no={usage.worker.reachable ? 'Off' : 'Unreachable'} />
          </div>
          <div className="flex items-center justify-between gap-2">
            <span className="text-calyx-muted">This month ({usage.month})</span>
            <span>
              {usage.minutes_month} min · {plural(usage.calls_month, 'call')} · ≈ {formatUsd(usage.estimated_usd_month)}
            </span>
          </div>
          <div className="flex items-center justify-between gap-2">
            <span className="text-calyx-muted">Today (Eastern) / daily cap</span>
            <span>
              {usage.minutes_today} / {usage.daily_cap_minutes} min ({capPct}%)
            </span>
          </div>
          <div className="flex items-center justify-between gap-2">
            <span className="text-calyx-muted">Queue</span>
            <span className="text-xs">
              {Object.entries(usage.queue).length === 0
                ? 'empty'
                : Object.entries(usage.queue).map(([k, v]) => `${k} ${v}`).join(' · ')}
            </span>
          </div>
          <div className="flex items-center justify-between gap-2">
            <span className="text-calyx-muted">Which calls</span>
            <span className="text-xs" data-testid="transcription-auto">
              {usage.settings.auto
                ? `automatic: converted${usage.settings.should_convert ? ' + should have converted' : ''} · ≥ ${usage.settings.min_seconds}s · since ${
                    usage.settings.auto_since
                      ? new Date(usage.settings.auto_since).toLocaleString('en-US', { timeZone: 'America/New_York', month: 'short', day: 'numeric', hour: 'numeric', minute: '2-digit', timeZoneName: 'short' })
                      : 'the API start'
                  }`
                : 'Transcribe button only (automatic off)'}
            </span>
          </div>
        </div>
      </div>

      <div className="calyx-card p-4">
        <p className="text-[10px] font-bold uppercase tracking-wider text-calyx-teal">Recording archive</p>
        <div className="mt-3 grid gap-3 text-sm sm:grid-cols-2">
          <div className="flex items-center justify-between gap-2">
            <span className="text-calyx-muted">Private bucket (B2 / R2)</span>
            <YesNo ok={usage.worker.archive_configured} />
          </div>
          <div className="flex items-center justify-between gap-2">
            <span className="text-calyx-muted">Archived</span>
            <span>
              {plural(usage.archive.recordings, 'recording')} · {formatBytes(usage.archive.bytes)} · ≈ {formatUsd(usage.archive.estimated_usd_month)}/month
            </span>
          </div>
          <div className="flex items-center justify-between gap-2">
            <span className="text-calyx-muted">Not archived / expired</span>
            <span>
              {usage.archive.not_archived} / {usage.archive.expired}
              {usage.worker.retention_months ? ` · kept ${usage.worker.retention_months} months` : ''}
            </span>
          </div>
          <label className="flex items-center justify-between gap-2">
            <span className="text-calyx-muted">Archive all recorded calls</span>
            <input
              type="checkbox"
              aria-label="Archive all recorded calls"
              checked={usage.settings.archive_all}
              disabled={busy}
              onChange={(e) => void toggleArchiveAll(e.target.checked)}
            />
          </label>
        </div>
      </div>

      <div className="calyx-card p-4">
        <p className="text-[10px] font-bold uppercase tracking-wider text-calyx-teal">Backfill</p>
        <div className="mt-3 flex flex-wrap items-end gap-2 text-sm">
          <label className="flex flex-col gap-1">
            <span className="text-xs text-calyx-muted">Last N days</span>
            <input
              type="number"
              min={1}
              max={30}
              aria-label="Backfill days"
              className="calyx-input w-24"
              value={days}
              onChange={(e) => {
                setDays(Math.max(1, Math.min(30, Number(e.target.value) || 7)));
                setEstimate(null);
              }}
            />
          </label>
          <Button type="button" variant="outline" size="sm" disabled={busy} onClick={() => void preview()}>
            Show estimate
          </Button>
          {estimate && usage?.settings.backfill && (
            <Button type="button" size="sm" disabled={busy || estimate.calls === 0} onClick={() => void start()}>
              Start backfill
            </Button>
          )}
        </div>
        {usage && !usage.settings.backfill && (
          <p className="mt-2 text-xs text-calyx-muted" data-testid="backfill-off">
            Backfill is switched off on the server (TRANSCRIBE_BACKFILL). The estimate still works.
          </p>
        )}
        {estimate && (
          <p className="mt-3 text-sm" data-testid="backfill-estimate">
            {plural(estimate.calls, 'call')} · about {estimate.minutes} minutes · ≈ {formatUsd(estimate.estimated_usd)}
            {estimate.days_at_cap && estimate.days_at_cap > 1 ? ` · ${estimate.days_at_cap} days at the daily cap` : ''}
            {estimate.already_queued_or_done > 0 ? ` · ${estimate.already_queued_or_done} already queued or done` : ''}
          </p>
        )}
      </div>
    </div>
  );
}
