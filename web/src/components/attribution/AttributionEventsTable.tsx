import type { AttributionEventRecord } from '@/lib/api';
import { Badge } from '@/components/ui/badge';
import { DataTable } from '@/components/ui/data-table';
import { TableCell, TableRow } from '@/components/ui/table';

interface AttributionEventsTableProps {
  events: AttributionEventRecord[];
}

export function AttributionEventsTable({ events }: AttributionEventsTableProps): JSX.Element {
  return (
    <DataTable
      columns={['Platform', 'Status', 'Revenue', 'HTTP', 'Fired']}
      emptyTitle="No attribution events"
      emptyDescription="Conversion events appear when pings include revenue."
      isEmpty={events.length === 0}
    >
      {events.map((row) => (
        <TableRow key={`${row.event_id}-${row.fired_at}`}>
          <TableCell className="capitalize">{row.platform}</TableCell>
          <TableCell>
            <Badge variant={row.status === 'fired' ? 'success' : 'danger'}>{row.status}</Badge>
          </TableCell>
          <TableCell>${Number(row.revenue).toFixed(2)}</TableCell>
          <TableCell>{row.http_status}</TableCell>
          <TableCell className="text-calyx-muted">
            {new Date(row.fired_at).toLocaleString()}
          </TableCell>
        </TableRow>
      ))}
    </DataTable>
  );
}
