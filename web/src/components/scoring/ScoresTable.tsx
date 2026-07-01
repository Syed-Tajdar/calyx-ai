'use client';

import Link from 'next/link';
import type { ScoreRecord } from '@/lib/api';
import { Badge } from '@/components/ui/badge';
import { DataTable } from '@/components/ui/data-table';
import { PlatformBadge } from '@/components/ui/PlatformBadge';
import { ScoreBar } from '@/components/ui/ScoreBar';
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

const COMPACT_COLUMN_CLASS_NAMES = [
  'min-w-[8.5rem] whitespace-nowrap',
  undefined,
  'w-[1%] whitespace-nowrap',
  undefined,
  undefined,
] as const;

export function ScoresTable({ scores, compact = false }: ScoresTableProps): JSX.Element {
  return (
    <DataTable
      columns={['Time', 'Call ID', 'Platform', 'Score', 'Sentiment']}
      columnClassNames={compact ? [...COMPACT_COLUMN_CLASS_NAMES] : undefined}
      emptyTitle={compact ? 'No recent scores' : 'No scored calls yet'}
      emptyDescription="Send a ping with revenue to trigger attribution events."
      isEmpty={scores.length === 0}
      className={compact ? 'flex h-full flex-1 flex-col' : undefined}
    >
      {scores.map((s) => (
        <TableRow key={`${s.call_id}-${s.created_at}`} className={compact ? '[&>td]:py-2.5' : undefined}>
          <TableCell
            className={
              compact
                ? 'min-w-[8.5rem] whitespace-nowrap text-xs text-calyx-muted'
                : 'text-calyx-muted'
            }
          >
            {formatTime(s.created_at)}
          </TableCell>
          <TableCell>
            <Link
              href={`/calls/${encodeURIComponent(s.call_id)}`}
              className="truncate font-mono text-xs text-calyx-accent hover:underline"
            >
              {s.call_id.slice(0, 12)}…
            </Link>
          </TableCell>
          <TableCell className={compact ? 'w-[1%] whitespace-nowrap' : undefined}>
            <PlatformBadge platform={s.platform} />
          </TableCell>
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
