'use client';

import type { AskHistoryRecord, AskStatus, AskToolData } from '@/lib/api';
import { Badge } from '@/components/ui/badge';
import { Input } from '@/components/ui/input';
import { Button } from '@/components/ui/button';

interface ChatWindowProps {
  history: AskHistoryRecord[];
  onAsk: (question: string) => Promise<unknown>;
  loading: boolean;
}

const STATUS_LABEL: Record<AskStatus, string> = {
  answered: 'From Calyx data',
  no_data: 'No data',
  unverified: 'Numbers only',
  budget: 'Daily budget reached',
  error: 'Error',
};

/** Columns shown for each tool, in order. Values are printed exactly as the API returned them. */
const COLUMNS: Record<AskToolData['tool'], Array<{ key: string; label: string }>> = {
  metrics: [
    { key: 'group', label: 'Group' },
    { key: 'calls', label: 'Calls' },
    { key: 'converted', label: 'Converted' },
    { key: 'conversion_rate_pct', label: 'Conv. %' },
    { key: 'revenue', label: 'Revenue' },
    { key: 'payout', label: 'Payout' },
    { key: 'profit', label: 'Profit' },
  ],
  compare_periods: [
    { key: 'group', label: 'Group' },
    { key: 'current.calls', label: 'Calls (now)' },
    { key: 'previous.calls', label: 'Calls (before)' },
    { key: 'calls_change_pct', label: 'Calls Δ %' },
    { key: 'current.revenue', label: 'Revenue (now)' },
    { key: 'previous.revenue', label: 'Revenue (before)' },
    { key: 'revenue_change_pct', label: 'Revenue Δ %' },
    { key: 'conversion_rate_change', label: 'Conv. Δ pts' },
  ],
  list_values: [
    { key: 'value', label: 'Value' },
    { key: 'calls', label: 'Calls' },
  ],
};

const MONEY = new Set(['revenue', 'payout', 'profit', 'current.revenue', 'previous.revenue']);

function cell(row: Record<string, unknown>, key: string): string {
  const v = key.split('.').reduce<unknown>((o, k) => (o && typeof o === 'object' ? (o as Record<string, unknown>)[k] : undefined), row);
  if (v === null || v === undefined) return '—';
  if (typeof v === 'number') {
    return MONEY.has(key)
      ? `$${v.toLocaleString('en-US', { minimumFractionDigits: 2, maximumFractionDigits: 2 })}`
      : v.toLocaleString('en-US');
  }
  return String(v);
}

function periodText(args: Record<string, unknown>): string {
  const cur = args.current as { from?: string; to?: string } | undefined;
  const prev = args.previous as { from?: string; to?: string } | undefined;
  if (cur && prev) return `${cur.from} → ${cur.to} vs ${prev.from} → ${prev.to}`;
  return `${String(args.from ?? '')} → ${String(args.to ?? '')}`;
}

export function NumbersUsed({ numbers }: { numbers: AskToolData[] }): JSX.Element | null {
  if (!numbers?.length) return null;
  return (
    <details className="mt-2 rounded border border-calyx-border bg-calyx-surface" open>
      <summary className="cursor-pointer px-2 py-1 text-xs font-semibold text-calyx-muted">
        Numbers used ({numbers.length} {numbers.length === 1 ? 'query' : 'queries'})
      </summary>
      <div className="space-y-3 p-2">
        {numbers.map((d, i) => (
          <div key={`${d.tool}-${i}`} className="overflow-x-auto">
            <p className="mb-1 text-[11px] text-calyx-muted">
              {d.tool.replace('_', ' ')} · {periodText(d.args)} (Eastern)
              {d.args.group_by && d.args.group_by !== 'none' ? ` · by ${String(d.args.group_by)}` : ''}
              {d.args.platform && d.args.platform !== 'all' ? ` · ${String(d.args.platform)}` : ''}
            </p>
            {d.rows.length === 0 ? (
              <p className="text-xs text-calyx-muted">No rows.</p>
            ) : (
              <table className="w-full text-left text-xs">
                <thead>
                  <tr className="text-calyx-muted">
                    {COLUMNS[d.tool].map((c) => (
                      <th key={c.key} className="px-1 py-0.5 font-medium">
                        {c.label}
                      </th>
                    ))}
                  </tr>
                </thead>
                <tbody>
                  {d.rows.map((r, ri) => (
                    <tr key={ri} className="border-t border-calyx-border text-calyx-text">
                      {COLUMNS[d.tool].map((c) => (
                        <td key={c.key} className="px-1 py-0.5 tabular-nums">
                          {cell(r, c.key)}
                        </td>
                      ))}
                    </tr>
                  ))}
                </tbody>
              </table>
            )}
          </div>
        ))}
      </div>
    </details>
  );
}

export function ChatWindow({ history, onAsk, loading }: ChatWindowProps): JSX.Element {
  async function handleSubmit(e: React.FormEvent<HTMLFormElement>): Promise<void> {
    e.preventDefault();
    const form = e.currentTarget;
    const input = form.elements.namedItem('question') as HTMLInputElement;
    const trimmed = input.value.trim();
    if (!trimmed || loading) return;
    input.value = '';
    await onAsk(trimmed).catch(() => undefined);
  }

  const items = history.slice().reverse();

  return (
    <div className="calyx-card flex flex-col overflow-hidden">
      <div className="max-h-[36rem] space-y-3 overflow-y-auto p-4" aria-live="polite">
        {items.length === 0 && (
          <p className="text-sm text-calyx-muted">
            Ask about calls, revenue, conversion, publishers, campaigns, buyers or states. Answers use only
            Calyx data, in Eastern time.
          </p>
        )}
        {items.map((h) => (
          <div key={h.id} className="space-y-2">
            <div className="ml-auto max-w-[85%] rounded-md bg-calyx-accent-muted px-3 py-2 text-sm text-calyx-text">
              {h.question}
            </div>
            <div className="max-w-[95%] rounded-md border border-calyx-border px-3 py-2 text-sm text-calyx-text">
              <p className="whitespace-pre-wrap">{h.answer}</p>
              <div className="mt-2 flex flex-wrap items-center gap-2 text-[11px] text-calyx-muted">
                <Badge variant={h.status === 'answered' ? 'default' : 'secondary'}>{STATUS_LABEL[h.status] ?? h.status}</Badge>
                {h.date_from && h.date_to && (
                  <span>
                    {h.date_from === h.date_to ? h.date_from : `${h.date_from} → ${h.date_to}`} · Eastern
                  </span>
                )}
              </div>
              <NumbersUsed numbers={h.numbers} />
            </div>
          </div>
        ))}
      </div>
      <form onSubmit={handleSubmit} className="flex gap-2 border-t border-calyx-border p-3">
        <Input name="question" placeholder="e.g. Which publisher had the lowest conversion yesterday?" maxLength={1000} disabled={loading} />
        <Button type="submit" disabled={loading}>
          {loading ? 'Asking…' : 'Ask'}
        </Button>
      </form>
    </div>
  );
}
