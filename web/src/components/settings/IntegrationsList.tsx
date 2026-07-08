'use client';

import { RefreshCw, Trash2 } from 'lucide-react';
import { toast } from 'sonner';
import { Button } from '@/components/ui/button';
import type { IntegrationRecord } from '@/lib/api';
import { ApiError, deleteIntegration, syncIntegration, updateIntegrationSyncMode } from '@/lib/api';
import { getToken } from '@/lib/auth';
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

export function IntegrationsList({
  integrations,
  onDeleted,
  onUpdated,
}: IntegrationsListProps): JSX.Element {
  const [deletingId, setDeletingId] = useState<string | null>(null);
  const [syncingId, setSyncingId] = useState<string | null>(null);
  const [updatingModeId, setUpdatingModeId] = useState<string | null>(null);

  async function handleDelete(id: string, name: string): Promise<void> {
    if (!window.confirm(`Remove integration "${name}"?`)) return;
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
              <th className="px-4 py-3 font-medium">Token</th>
              <th className="px-4 py-3 font-medium">Status</th>
              <th className="px-4 py-3 font-medium">Sync</th>
              <th className="px-4 py-3 font-medium" />
            </tr>
          </thead>
          <tbody>
            {integrations.map((item) => (
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
                  {item.connection_mode === 'api_sync' ? (
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
                    <span className="text-calyx-muted">—</span>
                  )}
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
                    {item.is_active ? 'Active' : 'Inactive'}
                  </span>
                </td>
                <td className="px-4 py-3 text-calyx-muted">
                  {item.connection_mode === 'api_sync' ? (
                    <span title={item.last_sync_error ?? undefined}>
                      {item.sync_status}
                    </span>
                  ) : (
                    '—'
                  )}
                </td>
                <td className="px-4 py-3 text-right">
                  <div className="flex justify-end gap-1">
                    {item.connection_mode === 'api_sync' && (
                      <Button
                        type="button"
                        variant="ghost"
                        size="icon"
                        className="h-8 w-8"
                        disabled={syncingId === item.id}
                        onClick={() => void handleSync(item.id)}
                        aria-label={`Sync ${item.name}`}
                      >
                        <RefreshCw className="h-4 w-4" />
                      </Button>
                    )}
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
            ))}
            {integrations.length === 0 && (
              <tr>
                <td colSpan={8} className="px-4 py-12 text-center text-calyx-muted">
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
