import type { AttributionEventRecord } from '@/lib/api';

interface AttributionEventsTableProps {
  events: AttributionEventRecord[];
}

export function AttributionEventsTable({ events }: AttributionEventsTableProps): JSX.Element {
  if (events.length === 0) {
    return (
      <div className="loom-card p-4 text-sm text-loom-muted">No attribution events logged yet.</div>
    );
  }

  return (
    <div className="loom-card overflow-hidden">
      <table className="w-full text-left text-xs">
        <thead className="border-b border-loom-border bg-loom-surface/50 text-loom-muted">
          <tr>
            <th className="px-3 py-2 font-medium">Platform</th>
            <th className="px-3 py-2 font-medium">Status</th>
            <th className="px-3 py-2 font-medium">Revenue</th>
            <th className="px-3 py-2 font-medium">HTTP</th>
            <th className="px-3 py-2 font-medium">Fired</th>
          </tr>
        </thead>
        <tbody>
          {events.map((row) => (
            <tr key={`${row.event_id}-${row.fired_at}`} className="border-b border-loom-border/50">
              <td className="px-3 py-2 capitalize text-white">{row.platform}</td>
              <td className="px-3 py-2">
                <span
                  className={
                    row.status === 'fired' ? 'text-emerald-400' : 'text-red-400'
                  }
                >
                  {row.status}
                </span>
              </td>
              <td className="px-3 py-2 text-white">${Number(row.revenue).toFixed(2)}</td>
              <td className="px-3 py-2 text-white">{row.http_status}</td>
              <td className="px-3 py-2 text-loom-muted">
                {new Date(row.fired_at).toLocaleString()}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
