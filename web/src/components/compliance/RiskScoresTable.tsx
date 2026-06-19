import Link from 'next/link';
import type { RiskScoreRecord } from '@/lib/api';
import { DataTable } from '@/components/ui/data-table';
import { RiskBadge } from '@/components/ui/RiskBadge';
import { TableCell, TableRow } from '@/components/ui/table';

interface RiskScoresTableProps {
  scores: RiskScoreRecord[];
}

export function RiskScoresTable({ scores }: RiskScoresTableProps): JSX.Element {
  return (
    <DataTable
      columns={['Call', 'State', 'Risk', 'Attest', 'Exposure']}
      emptyTitle="No risk scores yet"
      emptyDescription="Compliance runs automatically on each ping."
      isEmpty={scores.length === 0}
    >
      {scores.map((row) => (
        <TableRow key={`${row.call_id}-${row.created_at}`}>
          <TableCell className="max-w-[120px] truncate font-mono">
            <Link
              href={`/calls/${encodeURIComponent(row.call_id)}`}
              className="text-calyx-accent hover:underline"
            >
              {row.call_id.slice(0, 8)}…
            </Link>
          </TableCell>
          <TableCell>{row.state || '—'}</TableCell>
          <TableCell>
            <RiskBadge score={row.risk_score} />
          </TableCell>
          <TableCell>{row.attestation_level}</TableCell>
          <TableCell>${row.exposure_usd.toFixed(2)}</TableCell>
        </TableRow>
      ))}
    </DataTable>
  );
}
