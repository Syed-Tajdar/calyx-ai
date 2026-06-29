'use client';

import { FormEvent, useEffect, useState } from 'react';
import { Button } from '@/components/ui/button';
import type { ScorecardRecord, ScorecardRule } from '@/lib/api';

interface ScorecardEditorModalProps {
  open: boolean;
  initial?: ScorecardRecord | null;
  onClose: () => void;
  onSave: (payload: {
    name: string;
    vertical: string;
    rules: ScorecardRule[];
    is_active?: boolean;
  }) => Promise<void>;
}

const EMPTY_RULE: ScorecardRule = { id: 'rule_1', weight: 1, question: '' };

export function ScorecardEditorModal({
  open,
  initial,
  onClose,
  onSave,
}: ScorecardEditorModalProps): JSX.Element | null {
  const [name, setName] = useState('');
  const [vertical, setVertical] = useState('general');
  const [rules, setRules] = useState<ScorecardRule[]>([EMPTY_RULE]);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState('');

  useEffect(() => {
    if (!open) return;
    if (initial) {
      setName(initial.name);
      setVertical(initial.vertical);
      setRules(initial.rules.length > 0 ? initial.rules : [EMPTY_RULE]);
    } else {
      setName('');
      setVertical('general');
      setRules([EMPTY_RULE]);
    }
    setError('');
  }, [open, initial]);

  if (!open) return null;

  async function handleSubmit(e: FormEvent): Promise<void> {
    e.preventDefault();
    setSaving(true);
    setError('');
    try {
      await onSave({ name, vertical, rules });
      onClose();
    } catch {
      setError('Failed to save scorecard');
    } finally {
      setSaving(false);
    }
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4">
      <form
        onSubmit={handleSubmit}
        className="calyx-glass calyx-glass-lit max-h-[90vh] w-full max-w-lg overflow-y-auto p-6"
      >
        <h2 className="font-heading text-lg font-semibold text-calyx-text">
          {initial ? 'Edit Scorecard' : 'New Scorecard'}
        </h2>
        <div className="mt-4 space-y-4">
          <div>
            <label className="mb-1 block text-xs text-calyx-muted">Name</label>
            <input
              className="calyx-input text-sm"
              value={name}
              onChange={(e) => setName(e.target.value)}
              required
            />
          </div>
          <div>
            <label className="mb-1 block text-xs text-calyx-muted">Vertical</label>
            <input
              className="calyx-input text-sm"
              value={vertical}
              onChange={(e) => setVertical(e.target.value)}
              required
            />
          </div>
          <div className="space-y-3">
            <p className="text-xs font-semibold uppercase text-calyx-muted">Rules</p>
            {rules.map((rule, index) => (
              <div key={`${rule.id}-${index}`} className="grid gap-2 rounded-lg border border-calyx-border/60 p-3">
                <input
                  className="calyx-input text-xs"
                  placeholder="Rule ID"
                  value={rule.id}
                  onChange={(e) => {
                    const next = [...rules];
                    next[index] = { ...rule, id: e.target.value };
                    setRules(next);
                  }}
                  required
                />
                <input
                  className="calyx-input text-xs"
                  placeholder="Question"
                  value={rule.question}
                  onChange={(e) => {
                    const next = [...rules];
                    next[index] = { ...rule, question: e.target.value };
                    setRules(next);
                  }}
                  required
                />
                <input
                  type="number"
                  min={0}
                  max={1}
                  step={0.1}
                  className="calyx-input text-xs tabular-nums"
                  placeholder="Weight 0–1"
                  value={rule.weight}
                  onChange={(e) => {
                    const next = [...rules];
                    next[index] = { ...rule, weight: Number(e.target.value) };
                    setRules(next);
                  }}
                  required
                />
              </div>
            ))}
            <Button
              type="button"
              variant="outline"
              size="sm"
              onClick={() => setRules((prev) => [...prev, { ...EMPTY_RULE, id: `rule_${prev.length + 1}` }])}
            >
              Add rule
            </Button>
          </div>
        </div>
        {error && <p className="mt-3 text-sm text-red-300">{error}</p>}
        <div className="mt-6 flex justify-end gap-2">
          <Button type="button" variant="ghost" onClick={onClose}>
            Cancel
          </Button>
          <Button type="submit" disabled={saving}>
            {saving ? 'Saving…' : 'Save'}
          </Button>
        </div>
      </form>
    </div>
  );
}
