import Link from 'next/link';
import type { PingRecord } from '@/lib/api';

interface PingsTableProps {
  pings: PingRecord[];
  linkable?: boolean;
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

export function PingsTable({ pings, linkable = false }: PingsTableProps): JSX.Element {
  return (
    <div className="loom-card overflow-hidden">
      <div className="border-b border-loom-border px-4 py-3">
        <p className="text-[10px] font-bold uppercase tracking-wider text-loom-accent">
          Live Feed
        </p>
        <h3 className="text-sm font-semibold text-white">Recent Pings</h3>
      </div>
      <div className="overflow-x-auto">
        <table className="w-full min-w-[600px] text-left text-xs">
          <thead>
            <tr className="border-b border-loom-border text-[10px] uppercase tracking-wider text-loom-muted">
              <th className="px-4 py-3 font-medium">Time</th>
              <th className="px-4 py-3 font-medium">Platform</th>
              <th className="px-4 py-3 font-medium">Call ID</th>
              <th className="px-4 py-3 font-medium">State</th>
              <th className="px-4 py-3 font-medium">Event</th>
              <th className="px-4 py-3 font-medium">Revenue</th>
              <th className="px-4 py-3 font-medium">Dup</th>
            </tr>
          </thead>
          <tbody>
            {pings.map((p) => (
              <tr
                key={`${p.external_call_id}-${p.received_at}`}
                className="border-b border-loom-border/40"
              >
                <td className="px-4 py-3 text-loom-muted">{formatTime(p.received_at)}</td>
                <td className="px-4 py-3">
                  <span className="rounded-full bg-loom-accent/10 px-2 py-0.5 text-[10px] font-medium capitalize text-loom-accent">
                    {p.platform}
                  </span>
                </td>
                <td className="px-4 py-3 font-mono text-[10px] text-white/80">
                  {linkable ? (
                    <Link
                      href={`/calls/${encodeURIComponent(p.external_call_id)}?aniHash=${encodeURIComponent(p.ani_hash)}`}
                      className="text-loom-accent hover:underline"
                    >
                      {p.external_call_id.slice(0, 10)}…
                    </Link>
                  ) : (
                    <>{p.external_call_id.slice(0, 10)}…</>
                  )}
                </td>
                <td className="px-4 py-3 text-white">{p.state || '—'}</td>
                <td className="px-4 py-3 capitalize text-loom-muted">{p.event_type}</td>
                <td className="px-4 py-3 font-semibold text-emerald-400">
                  ${p.revenue.toFixed(2)}
                </td>
                <td className="px-4 py-3">
                  {p.is_duplicate ? (
                    <span className="text-amber-400">Yes</span>
                  ) : (
                    <span className="text-loom-muted">No</span>
                  )}
                </td>
              </tr>
            ))}
            {pings.length === 0 && (
              <tr>
                <td colSpan={7} className="px-4 py-12 text-center text-loom-muted">
                  No pings yet. Connect Ringba to start receiving data.
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </div>
    </div>
  );
}
