'use client';

import { FormEvent, useState } from 'react';
import { Trash2 } from 'lucide-react';
import { toast } from 'sonner';
import { Button } from '@/components/ui/button';
import { ChannelBadge } from '@/components/ui/StatusPill';
import type { AlertRule } from '@/lib/api';
import { ApiError, createAlertRule, deleteAlertRule } from '@/lib/api';
import { getToken } from '@/lib/auth';

interface AlertRulesSectionProps {
  rules: AlertRule[];
  onCreated: (rule: AlertRule) => void;
  onDeleted: (id: string) => void;
}

export function AlertRulesSection({
  rules,
  onCreated,
  onDeleted,
}: AlertRulesSectionProps): JSX.Element {
  const [name, setName] = useState('');
  const [threshold, setThreshold] = useState('70');
  const [submitting, setSubmitting] = useState(false);
  const [deletingId, setDeletingId] = useState<string | null>(null);
  const [error, setError] = useState('');

  async function handleSubmit(e: FormEvent): Promise<void> {
    e.preventDefault();
    const token = getToken();
    if (!token) return;
    setSubmitting(true);
    setError('');
    try {
      const rule = await createAlertRule(token, {
        name,
        threshold: Number(threshold),
        condition: { field: 'overall_score', op: 'lt' },
      });
      onCreated(rule);
      setName('');
      setThreshold('70');
      toast.success('Alert rule created');
    } catch {
      setError('Failed to create alert rule');
    } finally {
      setSubmitting(false);
    }
  }

  async function handleDelete(id: string, ruleName: string): Promise<void> {
    if (!window.confirm(`Remove alert rule "${ruleName}"?`)) return;
    const token = getToken();
    if (!token) return;
    setDeletingId(id);
    try {
      await deleteAlertRule(token, id);
      onDeleted(id);
      toast.success('Alert rule removed');
    } catch (err: unknown) {
      if (err instanceof ApiError && err.status === 404) {
        onDeleted(id);
        toast.message('Rule was already removed');
      } else {
        toast.error('Failed to delete rule');
      }
    } finally {
      setDeletingId(null);
    }
  }

  return (
    <div className="space-y-4">
      <div className="calyx-glass calyx-glass-lit overflow-hidden">
        <div className="border-b border-calyx-border/60 px-4 py-3">
          <p className="text-[10px] font-bold uppercase tracking-wider text-calyx-teal">Alerts</p>
          <h3 className="font-heading text-sm font-semibold text-calyx-text">Active Rules</h3>
        </div>
        <div className="overflow-x-auto">
          <table className="w-full text-left text-xs">
            <thead>
              <tr className="border-b border-calyx-border text-[10px] uppercase text-calyx-muted">
                <th className="px-4 py-3 font-medium">Name</th>
                <th className="px-4 py-3 font-medium">Condition</th>
                <th className="px-4 py-3 font-medium numeric">Threshold</th>
                <th className="px-4 py-3 font-medium">Channels</th>
                <th className="px-4 py-3 font-medium" />
              </tr>
            </thead>
            <tbody>
              {rules.map((rule) => (
                <tr
                  key={rule.id}
                  className="border-b border-calyx-border/40 hover:bg-white/[0.03]"
                >
                  <td className="px-4 py-3 text-calyx-text">{rule.name}</td>
                  <td className="px-4 py-3 font-mono text-[10px] text-calyx-muted">
                    score &lt; {rule.threshold}
                  </td>
                  <td className="px-4 py-3 tabular-nums text-calyx-muted">{rule.threshold}</td>
                  <td className="px-4 py-3">
                    <div className="flex flex-wrap gap-1">
                      {rule.channels.map((ch) => (
                        <ChannelBadge key={ch} channel={ch} />
                      ))}
                    </div>
                  </td>
                  <td className="px-4 py-3 text-right">
                    <Button
                      type="button"
                      variant="ghost"
                      size="icon"
                      className="h-8 w-8 text-calyx-danger"
                      disabled={deletingId === rule.id}
                      onClick={() => void handleDelete(rule.id, rule.name)}
                      aria-label={`Delete ${rule.name}`}
                    >
                      <Trash2 className="h-4 w-4" />
                    </Button>
                  </td>
                </tr>
              ))}
              {rules.length === 0 && (
                <tr>
                  <td colSpan={5} className="px-4 py-8 text-center text-calyx-muted">
                    No alert rules yet.
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        </div>
      </div>

      <form onSubmit={handleSubmit} className="calyx-glass calyx-glass-lit space-y-4 p-4">
        <p className="font-heading text-sm font-semibold text-calyx-text">Create Alert Rule</p>
        <p className="text-xs text-calyx-muted">
          Fires when call overall_score is below threshold.
        </p>
        <div>
          <label htmlFor="rule-name" className="mb-1 block text-xs text-calyx-muted">
            Rule name
          </label>
          <input
            id="rule-name"
            type="text"
            className="calyx-input text-sm"
            value={name}
            onChange={(e) => setName(e.target.value)}
            required
            placeholder="Low score alert"
          />
        </div>
        <div>
          <label htmlFor="rule-threshold" className="mb-1 block text-xs text-calyx-muted">
            Score threshold (alert if below)
          </label>
          <input
            id="rule-threshold"
            type="number"
            min={0}
            max={100}
            step={0.5}
            className="calyx-input text-sm tabular-nums"
            value={threshold}
            onChange={(e) => setThreshold(e.target.value)}
            required
          />
        </div>
        {error && <p className="text-sm text-red-300">{error}</p>}
        <button type="submit" disabled={submitting} className="calyx-btn-primary text-sm">
          {submitting ? 'Creating…' : 'Create rule'}
        </button>
      </form>
    </div>
  );
}
