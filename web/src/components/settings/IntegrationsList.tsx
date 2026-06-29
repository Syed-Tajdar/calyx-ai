'use client';

import { Trash2 } from 'lucide-react';
import { toast } from 'sonner';
import { Button } from '@/components/ui/button';
import type { IntegrationRecord } from '@/lib/api';
import { ApiError, deleteIntegration } from '@/lib/api';
import { getToken } from '@/lib/auth';
import { useState } from 'react';

interface IntegrationsListProps {
  integrations: IntegrationRecord[];
  onDeleted: (id: string) => void;
}

function maskToken(token: string): string {
  if (token.length <= 8) return '••••••••';
  return `${token.slice(0, 4)}…${token.slice(-4)}`;
}

export function IntegrationsList({
  integrations,
  onDeleted,
}: IntegrationsListProps): JSX.Element {
  const [deletingId, setDeletingId] = useState<string | null>(null);

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

  return (
    <div className="calyx-glass calyx-glass-lit overflow-hidden">
      <div className="border-b border-calyx-border/60 px-4 py-3">
        <p className="text-[10px] font-bold uppercase tracking-wider text-calyx-teal">
          Integrations
        </p>
        <h3 className="font-heading text-sm font-semibold text-calyx-text">Connected Platforms</h3>
      </div>
      <div className="overflow-x-auto">
        <table className="w-full min-w-[400px] text-left text-xs">
          <thead>
            <tr className="border-b border-calyx-border text-[10px] uppercase text-calyx-muted">
              <th className="px-4 py-3 font-medium">Platform</th>
              <th className="px-4 py-3 font-medium">Name</th>
              <th className="px-4 py-3 font-medium">Token</th>
              <th className="px-4 py-3 font-medium">Status</th>
              <th className="px-4 py-3 font-medium" />
            </tr>
          </thead>
          <tbody>
            {integrations.map((item) => (
              <tr
                key={item.id}
                className="border-b border-calyx-border/40 hover:bg-white/[0.03]"
              >
                <td className="px-4 py-3 capitalize text-calyx-text">{item.platform}</td>
                <td className="px-4 py-3 text-calyx-muted">{item.name}</td>
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
                <td className="px-4 py-3 text-right">
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
                </td>
              </tr>
            ))}
            {integrations.length === 0 && (
              <tr>
                <td colSpan={5} className="px-4 py-12 text-center text-calyx-muted">
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
