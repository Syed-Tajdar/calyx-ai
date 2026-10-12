'use client';

import { History, Pause, Play, RefreshCw, Trash2 } from 'lucide-react';
import { toast } from 'sonner';
import { Button } from '@/components/ui/button';
import type { IntegrationRecord } from '@/lib/api';
import {
  ApiError,
  backfillIntegration,
  deleteIntegration,
  pauseIntegration,
  resumeIntegration,
  syncIntegration,
  updateIntegrationSyncMode,
} from '@/lib/api';
import { getToken } from '@/lib/auth';
import { useDateFilter } from '@/lib/date-filter';
import { formatPlatformLabel } from '@/lib/platform-options';
import { useState } from 'react';

interface IntegrationsListProps {
  integrations: IntegrationRecord[];
  onDeleted: (id: string) => void;
  onUpdated: () => void;
}

function maskToken(token: string | null): string {
  if (!token) return '—';
  if (token.length <= 8) return '••••••••';
  return `${token.slice(0, 4)}…${token.slice(-4)}`;
}

/**
 * Stage 4 R1-fix F1. `degraded` means the last window came back short of the platform's own
 * row count, so the cursor is deliberately held and the sync is not making progress. It has to
 * read differently from `ok` at a glance — a stuck sync that looks healthy is the failure this
 * whole stage is about. Hover shows `last_sync_error`, which names the window and the counts.
 */
function syncStatusStyle(status: string): string {
  switch (status) {
    case 'ok':
      return 'bg-emerald-500/15 text-emerald-400';
    case 'degraded':
      return 'bg-amber-500/15 text-amber-400';
    case 'error':
      return 'bg-red-500/15 text-red-400';
    case 'syncing':
      return 'bg-blue-500/15 text-blue-400';
    default:
      return 'bg-white/5 text-calyx-muted';
  }
}

function syncStatusLabel(status: string): string {
  return status === 'degraded' ? 'Degraded — sync held' : status;
}

export function IntegrationsList({
  integrations,
  onDeleted,
  onUpdated,
}: IntegrationsListProps): JSX.Element {
  const [deletingId, setDeletingId] = useState<string | null>(null);
  const [pausingId, setPausingId] = useState<string | null>(null);
  const [syncingId, setSyncingId] = useState<string | null>(null);
  const [backfillingId, setBackfillingId] = useState<string | null>(null);
  const { demoModeEnabled } = useDateFilter();
  const [updatingModeId, setUpdatingModeId] = useState<string | null>(null);

  async function handlePauseToggle(item: IntegrationRecord): Promise<void> {
    const pausing = item.is_active;
    const ok = window.confirm(
      pausing
        ? `Pause "${item.name}"? It stays in this list and stops syncing. Resume turns it back on without changing Test or Live.`
        : `Resume "${item.name}"? Sync mode stays as it is now.`,
    );
    if (!ok) return;
    const token = getToken();
    if (!token) return;
    setPausingId(item.id);
    try {
      if (pausing) await pauseIntegration(token, item.id);
      else await resumeIntegration(token, item.id);
      toast.success(pausing ? 'Integration paused' : 'Integration resumed');
      onUpdated();
    } catch {
      toast.error(pausing ? 'Failed to pause integration' : 'Failed to resume integration');
    } finally {
      setPausingId(null);
    }
  }

  async function handleDelete(id: string, name: string): Promise<void> {
    if (
      !window.confirm(
        `Delete "${name}"? This removes it from Settings. Pause is separate and keeps the row.`,
      )
    ) {
      return;
    }
    const token = getToken();
    if (!token) return;
    setDeletingId(id);
    try {
      await deleteIntegration(token, id);
      onDeleted(id);
      toast.success('Integration removed');
    } catch (err: unknown) {
      if (err instanceof ApiError && err.status === 404) {
        onDeleted(id);
        toast.message('Integration was already removed');
      } else {
        toast.error('Failed to delete integration');
      }
    } finally {
      setDeletingId(null);
    }
  }

  async function handleSync(id: string): Promise<void> {
    const token = getToken();
    if (!token) return;
    setSyncingId(id);
    try {
      await syncIntegration(token, id);
      toast.success('Sync started — refresh Pings in a few seconds');
      onUpdated();
    } catch {
      toast.error('Sync failed — is platform-sync running?');
    } finally {
      setSyncingId(null);
    }
  }

  async function handleBackfill(id: string, days: number): Promise<void> {
    if (
      !window.confirm(
        `Backfill last ${days} days? This clears the sync cursor and re-pulls from the lookback window.`,
      )
    ) {
      return;
    }
    const token = getToken();
    if (!token) return;
    setBackfillingId(id);
    try {
      await backfillIntegration(token, id);
      toast.success(`Backfill started (last ${days} days)`);
      onUpdated();
    } catch (err: unknown) {
      if (err instanceof ApiError && err.status === 429) {
        toast.error(err.message || 'Backfill rate limited — try again later');
      } else {
        toast.error('Backfill failed — is platform-sync running?');
      }
    } finally {
      setBackfillingId(null);
    }
  }

  async function handleSyncModeChange(
    id: string,
    nextMode: 'mock' | 'real',
  ): Promise<void> {
    const token = getToken();
    if (!token) return;
    setUpdatingModeId(id);
    try {
      await updateIntegrationSyncMode(token, id, nextMode);
      toast.success(nextMode === 'mock' ? 'Switched to Test mode' : 'Switched to Live mode');
      onUpdated();
    } catch (err: unknown) {
      if (err instanceof ApiError) {
        toast.error(err.message);
      } else {
        toast.error('Failed to update sync mode');
      }
    } finally {
      setUpdatingModeId(null);
    }
  }

  return (
    <div className="calyx-glass calyx-glass-lit overflow-hidden">
      <div className="border-b border-calyx-border/60 px-4 py-3">
        <p className="text-[10px] font-bold uppercase tracking-wider text-calyx-teal">
          Integrations
        </p>
        <h3 className="font-heading text-sm font-semibold text-calyx-text">Connected Platforms</h3>
      </div>
      <div className="overflow-x-auto">
        <table className="w-full min-w-[520px] text-left text-xs">
          <thead>
            <tr className="border-b border-calyx-border text-[10px] uppercase text-calyx-muted">
              <th className="px-4 py-3 font-medium">Platform</th>
              <th className="px-4 py-3 font-medium">Name</th>
              <th className="px-4 py-3 font-medium">Mode</th>
              <th className="px-4 py-3 font-medium">Data</th>
              <th className="px-4 py-3 font-medium">Lookback</th>
              <th className="px-4 py-3 font-medium">Token</th>
              <th className="px-4 py-3 font-medium">Status</th>
              <th className="px-4 py-3 font-medium">Sync</th>
              <th className="px-4 py-3 font-medium" />
            </tr>
          </thead>
          <tbody>
            {integrations.map((item) => {
              const lookbackDays = item.initial_lookback_days ?? 30;
              return (
              <tr
                key={item.id}
                className="border-b border-calyx-border/40 hover:bg-white/[0.03]"
              >
                <td className="px-4 py-3 text-calyx-text">{formatPlatformLabel(item.platform)}</td>
                <td className="px-4 py-3 text-calyx-muted">{item.name}</td>
                <td className="px-4 py-3 text-calyx-muted">
                  {item.connection_mode === 'api_sync' ? 'API key' : 'Webhook'}
                </td>
                <td className="px-4 py-3">
                  {item.connection_mode === 'api_sync' && item.is_active ? (
                    demoModeEnabled ? (
                    <div className="inline-flex rounded-md border border-calyx-border p-0.5">
                      <button
                        type="button"
                        disabled={updatingModeId === item.id}
                        className={`rounded px-2 py-0.5 text-[10px] font-medium transition-colors ${
                          (item.sync_mode ?? 'mock') === 'mock'
                            ? 'bg-calyx-teal/20 text-calyx-teal'
                            : 'text-calyx-muted hover:text-calyx-text'
                        }`}
                        onClick={() => void handleSyncModeChange(item.id, 'mock')}
                      >
                        Test
                      </button>
                      <button
                        type="button"
                        disabled={updatingModeId === item.id}
                        className={`rounded px-2 py-0.5 text-[10px] font-medium transition-colors ${
                          item.sync_mode === 'real'
                            ? 'bg-emerald-500/20 text-emerald-400'
                            : 'text-calyx-muted hover:text-calyx-text'
                        }`}
                        onClick={() => void handleSyncModeChange(item.id, 'real')}
                      >
                        Live
                      </button>
                    </div>
                    ) : (
                      <span className="text-calyx-muted">
                        {item.sync_mode === 'real' ? 'Live' : 'Test'}
                      </span>
                    )
                  ) : (
                    <span className="text-calyx-muted">
                      {item.is_active ? '—' : 'Paused'}
                    </span>
                  )}
                </td>
                <td className="px-4 py-3 text-calyx-muted">
                  {item.connection_mode === 'api_sync'
                    ? `Syncing last ${lookbackDays} days`
                    : '—'}
                </td>
                <td className="px-4 py-3 font-mono text-[10px] text-calyx-muted">
                  {maskToken(item.webhook_token)}
                </td>
                <td className="px-4 py-3">
                  <span
                    className={`rounded-full px-2 py-0.5 text-[10px] font-medium ${
                      item.is_active
                        ? 'bg-emerald-500/15 text-emerald-400'
                        : 'bg-red-500/15 text-red-400'
                    }`}
                  >
                    {item.is_active ? 'Active' : 'Paused'}
                  </span>
                </td>
                <td className="px-4 py-3 text-calyx-muted">
                  {item.connection_mode === 'api_sync' ? (
                    <span
                      className={`rounded-full px-2 py-0.5 text-[10px] font-medium ${syncStatusStyle(item.sync_status)}`}
                      title={item.last_sync_error ?? undefined}
                    >
                      {syncStatusLabel(item.sync_status)}
                    </span>
                  ) : (
                    '—'
                  )}
                </td>
                <td className="px-4 py-3 text-right">
                  <div className="flex justify-end gap-1">
                    {item.connection_mode === 'api_sync' && item.is_active && (
                      <>
                        <Button
                          type="button"
                          variant="ghost"
                          size="icon"
                          className="h-8 w-8"
                          disabled={syncingId === item.id || backfillingId === item.id}
                          onClick={() => void handleSync(item.id)}
                          aria-label={`Sync ${item.name}`}
                          title="Sync"
                        >
                          <RefreshCw className="h-4 w-4" />
                        </Button>
                        <Button
                          type="button"
                          variant="ghost"
                          size="icon"
                          className="h-8 w-8"
                          disabled={syncingId === item.id || backfillingId === item.id}
                          onClick={() => void handleBackfill(item.id, lookbackDays)}
                          aria-label={`Backfill ${item.name}`}
                          title={`Backfill last ${lookbackDays} days`}
                        >
                          <History className="h-4 w-4" />
                        </Button>
                      </>
                    )}
                    <Button
                      type="button"
                      variant="ghost"
                      size="icon"
                      className="h-8 w-8"
                      disabled={pausingId === item.id}
                      onClick={() => void handlePauseToggle(item)}
                      aria-label={item.is_active ? `Pause ${item.name}` : `Resume ${item.name}`}
                      title={item.is_active ? 'Pause' : 'Resume'}
                    >
                      {item.is_active ? <Pause className="h-4 w-4" /> : <Play className="h-4 w-4" />}
                    </Button>
                    <Button
                      type="button"
                      variant="ghost"
                      size="icon"
                      className="h-8 w-8 text-calyx-danger"
                      disabled={deletingId === item.id}
                      onClick={() => void handleDelete(item.id, item.name)}
                      aria-label={`Delete ${item.name}`}
                    >
                      <Trash2 className="h-4 w-4" />
                    </Button>
                  </div>
                </td>
              </tr>
              );
            })}
            {integrations.length === 0 && (
              <tr>
                <td colSpan={9} className="px-4 py-12 text-center text-calyx-muted">
                  No integrations configured.
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </div>
    </div>
  );
}
