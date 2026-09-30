import { shouldHaveConverted, type PingRecord } from '@/lib/api';
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

/**
 * Conversion state for one call.
 *
 * `Review` is the actionable state — the platform has not marked the call converted, but it either
 * earned revenue or met the buyer's qualifying duration. Matches the server-side `should_convert`
 * filter via the shared `shouldHaveConverted` helper, so badge and filter cannot drift apart.
 */
function ConversionCell({ ping }: { ping: PingRecord }): JSX.Element {
  // C6: this platform never reports conversion state, so neither "Converted" nor "Review"
  // would be a truthful claim.
  if (ping.conversion_reported === false) {
    return (
      <span className="text-calyx-muted" title="This platform does not report conversion state">
        n/a
      </span>
    );
  }
  if (ping.has_converted) {
    return <Badge variant="teal">Converted</Badge>;
  }
  if (shouldHaveConverted(ping)) {
    return (
      <Badge variant="warning" title="Not converted, but it earned revenue or met the required duration">
        Review
      </Badge>
    );
  }
  return <span className="text-calyx-muted">—</span>;
}

export function PingsTable({
  pings,
  linkable = false,
  hideHeader = false,
  highlightKeys,
}: PingsTableProps): JSX.Element {
  const table = (
    <DataTable
      columns={['Time', 'Platform', 'Call ID', 'State', 'Event', 'Revenue', 'Status', 'Syncs']}
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
            <ConversionCell ping={p} />
          </TableCell>
          <TableCell>
            {/*
              The list now returns one row per call, so this is how many times the call has been
              written — a re-sync means the platform changed something (revenue, conversion,
              payout adjustment). "1" is the ordinary case and stays quiet.
            */}
            {(p.sync_count ?? 1) > 1 ? (
              <Badge
                variant="secondary"
                title="Updated by a later sync — the values shown are the most recent"
              >
                {p.sync_count}×
              </Badge>
            ) : (
              <span className="text-calyx-muted">1</span>
            )}
          </TableCell>
        </TableRow>
      ))}
    </DataTable>
  );

  if (hideHeader) return table;

  return table;
}
