'use client';

import Link from 'next/link';
import type { ScoreRecord } from '@/lib/api';
import { Badge } from '@/components/ui/badge';
import { DataTable } from '@/components/ui/data-table';
import { TableCell, TableRow } from '@/components/ui/table';

interface ScoresTableProps {
  scores: ScoreRecord[];
  compact?: boolean;
}

function formatTime(iso: string): string {
  const d = new Date(iso);
  return d.toLocaleString('en-US', {
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  });
}

function sentimentVariant(sentiment: string): 'success' | 'warning' | 'danger' | 'secondary' {
  const s = sentiment.toLowerCase();
  if (s === 'positive') return 'success';
  if (s === 'negative') return 'danger';
  if (s === 'neutral') return 'secondary';
  return 'warning';
}

function ScoreBar({ score }: { score: number }): JSX.Element {
  const pct = Math.min(100, Math.max(0, score));
  const color =
    score >= 70 ? 'bg-calyx-success' : score >= 50 ? 'bg-calyx-warning' : 'bg-calyx-danger';
  return (
    <div className="flex items-center gap-2">
      <div className="h-1.5 w-16 overflow-hidden rounded-full bg-calyx-surface">
        <div className={`h-full ${color}`} style={{ width: `${pct}%` }} />
      </div>
      <span className="font-semibold tabular-nums">{score.toFixed(1)}</span>
    </div>
  );
}

export function ScoresTable({ scores, compact = false }: ScoresTableProps): JSX.Element {
  return (
    <DataTable
      columns={['Time', 'Call ID', 'Platform', 'Score', 'Sentiment']}
      emptyTitle={compact ? 'No recent scores' : 'No scored calls yet'}
      emptyDescription="Send a ping with revenue to trigger attribution events."
      isEmpty={scores.length === 0}
    >
      {scores.map((s) => (
        <TableRow key={`${s.call_id}-${s.created_at}`}>
          <TableCell className="text-calyx-muted">{formatTime(s.created_at)}</TableCell>
          <TableCell>
            <Link
              href={`/calls/${encodeURIComponent(s.call_id)}`}
              className="font-mono text-xs text-calyx-accent hover:underline"
            >
              {s.call_id.slice(0, 12)}…
            </Link>
          </TableCell>
          <TableCell className="capitalize text-calyx-muted">{s.platform}</TableCell>
          <TableCell>
            <ScoreBar score={s.overall_score} />
          </TableCell>
          <TableCell>
            <Badge variant={sentimentVariant(s.sentiment)} className="capitalize">
              {s.sentiment}
            </Badge>
          </TableCell>
        </TableRow>
      ))}
    </DataTable>
  );
}
