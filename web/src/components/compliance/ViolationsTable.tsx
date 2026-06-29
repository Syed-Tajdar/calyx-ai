import type { ViolationRecord } from '@/lib/api';
import Link from 'next/link';

interface ViolationsTableProps {
  violations: ViolationRecord[];
}

export function ViolationsTable({ violations }: ViolationsTableProps): JSX.Element {
  return (
    <div className="calyx-glass calyx-glass-lit overflow-hidden">
      <div className="border-b border-calyx-border/60 px-4 py-3">
        <h3 className="font-heading text-sm font-semibold text-calyx-text">Violations</h3>
      </div>
      <div className="overflow-x-auto">
        <table className="w-full text-left text-sm">
          <thead>
            <tr className="border-b border-calyx-border text-[10px] uppercase text-calyx-muted">
              <th className="calyx-table-th">Call ID</th>
              <th className="calyx-table-th">Type</th>
              <th className="calyx-table-th numeric">Risk</th>
              <th className="calyx-table-th numeric">Exposure</th>
              <th className="calyx-table-th">Detected</th>
            </tr>
          </thead>
          <tbody>
            {violations.map((v) => (
              <tr
                key={`${v.call_id}-${v.created_at}`}
                className="border-b border-calyx-border/40 hover:bg-white/[0.03]"
              >
                <td className="calyx-table-cell">
                  <Link
                    href={`/calls/${encodeURIComponent(v.call_id)}`}
                    className="font-mono text-xs text-calyx-teal hover:underline"
                  >
                    {v.call_id}
                  </Link>
                </td>
                <td className="calyx-table-cell capitalize">{v.violation_type.replace(/_/g, ' ')}</td>
                <td className="calyx-table-cell numeric tabular-nums">{v.risk_score.toFixed(1)}</td>
                <td className="calyx-table-cell numeric tabular-nums">
                  ${v.exposure_usd.toFixed(2)}
                </td>
                <td className="calyx-table-cell text-calyx-muted">
                  {new Date(v.created_at).toLocaleString()}
                </td>
              </tr>
            ))}
            {violations.length === 0 && (
              <tr>
                <td colSpan={5} className="px-4 py-8 text-center text-calyx-muted">
                  No violations detected in this period.
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </div>
    </div>
  );
}
