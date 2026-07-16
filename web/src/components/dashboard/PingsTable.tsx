import type { PingRecord } from '@/lib/api';
import { CallIdCell } from '@/components/dashboard/CallIdCell';
import { DataTable } from '@/components/ui/data-table';
import { Badge } from '@/components/ui/badge';
import { PlatformBadge } from '@/components/ui/PlatformBadge';
import { TableCell, TableRow } from '@/components/ui/table';
import { pingRowKey } from '@/lib/use-ping-feed';
import { pingEventAt } from '@/lib/ping-event-time';
import { cn } from '@/lib/utils';

interface PingsTableProps {
  pings: PingRecord[];
  linkable?: boolean;
  hideHeader?: boolean;
  highlightKeys?: Set<string>;
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

export function PingsTable({
  pings,
  linkable = false,
  hideHeader = false,
  highlightKeys,
}: PingsTableProps): JSX.Element {
  const table = (
    <DataTable
      columns={['Time', 'Platform', 'Call ID', 'State', 'Event', 'Revenue', 'Dup']}
      emptyTitle="No pings yet"
      emptyDescription="Connect Ringba to start receiving data."
      isEmpty={pings.length === 0}
      className={hideHeader ? 'border-0 shadow-none' : undefined}
    >
      {pings.map((p) => (
        <TableRow
          key={pingRowKey(p)}
          className={cn(highlightKeys?.has(pingRowKey(p)) && 'calyx-row-new')}
        >
          <TableCell className="text-calyx-muted">{formatTime(pingEventAt(p))}</TableCell>
          <TableCell>
            <PlatformBadge platform={p.platform} />
          </TableCell>
          <TableCell>
            <CallIdCell
              callId={p.external_call_id}
              href={
                linkable
                  ? `/calls/${encodeURIComponent(p.external_call_id)}?platform=${encodeURIComponent(p.platform)}`
                  : undefined
              }
            />
          </TableCell>
          <TableCell>{p.state || '—'}</TableCell>
          <TableCell className="capitalize text-calyx-muted">{p.event_type}</TableCell>
          <TableCell className="font-semibold text-calyx-success">${p.revenue.toFixed(2)}</TableCell>
          <TableCell>
            {p.is_duplicate ? (
              <Badge variant="warning">Yes</Badge>
            ) : (
              <span className="text-calyx-muted">No</span>
            )}
          </TableCell>
        </TableRow>
      ))}
    </DataTable>
  );

  if (hideHeader) return table;

  return table;
}
