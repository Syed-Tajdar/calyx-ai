import type { RiskScoreRecord } from '@/lib/api';

interface RiskScoresTableProps {
  scores: RiskScoreRecord[];
}

function riskColor(score: number): string {
  if (score >= 70) return 'text-red-400';
  if (score >= 40) return 'text-amber-400';
  return 'text-emerald-400';
}

export function RiskScoresTable({ scores }: RiskScoresTableProps): JSX.Element {
  if (scores.length === 0) {
    return (
      <div className="loom-card p-4 text-sm text-loom-muted">
        No risk scores yet. Compliance runs automatically on each ping.
      </div>
    );
  }

  return (
    <div className="loom-card overflow-hidden">
      <table className="w-full text-left text-xs">
        <thead className="border-b border-loom-border bg-loom-surface/50 text-loom-muted">
          <tr>
            <th className="px-3 py-2 font-medium">Call</th>
            <th className="px-3 py-2 font-medium">State</th>
            <th className="px-3 py-2 font-medium">Risk</th>
            <th className="px-3 py-2 font-medium">Attest</th>
            <th className="px-3 py-2 font-medium">Exposure</th>
          </tr>
        </thead>
        <tbody>
          {scores.map((row) => (
            <tr key={`${row.call_id}-${row.created_at}`} className="border-b border-loom-border/50">
              <td className="max-w-[120px] truncate px-3 py-2 font-mono text-white">
                {row.call_id.slice(0, 8)}…
              </td>
              <td className="px-3 py-2 text-white">{row.state || '—'}</td>
              <td className={`px-3 py-2 font-semibold ${riskColor(row.risk_score)}`}>
                {row.risk_score}
              </td>
              <td className="px-3 py-2 text-white">{row.attestation_level}</td>
              <td className="px-3 py-2 text-white">${row.exposure_usd.toFixed(2)}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
