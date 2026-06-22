'use client';

import { FormEvent, useState } from 'react';
import type { AlertRule } from '@/lib/api';
import { createAlertRule } from '@/lib/api';
import { getToken } from '@/lib/auth';

interface AlertRulesSectionProps {
  rules: AlertRule[];
  onCreated: (rule: AlertRule) => void;
}

export function AlertRulesSection({ rules, onCreated }: AlertRulesSectionProps): JSX.Element {
  const [name, setName] = useState('');
  const [threshold, setThreshold] = useState('70');
  const [submitting, setSubmitting] = useState(false);
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
    } catch {
      setError('Failed to create alert rule');
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <div className="space-y-4">
      <div className="calyx-card overflow-hidden">
        <div className="border-b border-calyx-border px-4 py-3">
          <p className="text-[10px] font-bold uppercase tracking-wider text-calyx-accent">Alerts</p>
          <h3 className="text-sm font-semibold text-calyx-text">Active Rules</h3>
        </div>
        <div className="overflow-x-auto">
          <table className="w-full text-left text-xs">
            <thead>
              <tr className="border-b border-calyx-border text-[10px] uppercase text-calyx-muted">
                <th className="px-4 py-3 font-medium">Name</th>
                <th className="px-4 py-3 font-medium">Condition</th>
                <th className="px-4 py-3 font-medium">Threshold</th>
                <th className="px-4 py-3 font-medium">Channels</th>
              </tr>
            </thead>
            <tbody>
              {rules.map((rule) => (
                <tr key={rule.id} className="border-b border-calyx-border/40">
                  <td className="px-4 py-3 text-calyx-text">{rule.name}</td>
                  <td className="px-4 py-3 font-mono text-[10px] text-calyx-muted">
                    score &lt; {rule.threshold}
                  </td>
                  <td className="px-4 py-3 text-calyx-muted">{rule.threshold}</td>
                  <td className="px-4 py-3 text-calyx-muted">{rule.channels.join(', ')}</td>
                </tr>
              ))}
              {rules.length === 0 && (
                <tr>
                  <td colSpan={4} className="px-4 py-8 text-center text-calyx-muted">
                    No alert rules yet.
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        </div>
      </div>

      <form onSubmit={handleSubmit} className="calyx-card space-y-4 p-4">
        <p className="text-sm font-semibold text-calyx-text">Create Alert Rule</p>
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
            className="calyx-input text-sm"
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
