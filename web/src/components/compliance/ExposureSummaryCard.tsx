import type { ExposureSummary } from '@/lib/api';

interface ExposureSummaryCardProps {
  exposure: ExposureSummary | null;
  loading?: boolean;
}

export function ExposureSummaryCard({ exposure, loading }: ExposureSummaryCardProps): JSX.Element {
  if (loading) {
    return (
      <div className="calyx-card p-4">
        <p className="text-sm text-calyx-muted">Loading exposure…</p>
      </div>
    );
  }

  if (!exposure) {
    return (
      <div className="calyx-card p-4">
        <p className="text-sm text-calyx-muted">No compliance exposure data yet.</p>
      </div>
    );
  }

  return (
    <div className="calyx-card p-4">
      <p className="text-[10px] font-bold uppercase tracking-wider text-calyx-accent">Liability Exposure</p>
      <p className="mt-2 text-3xl font-bold text-amber-400">
        ${exposure.total_exposure_usd.toFixed(2)}
      </p>
      <div className="mt-4 grid grid-cols-3 gap-2 text-center text-xs">
        <div>
          <p className="text-lg font-semibold text-calyx-text">{exposure.violation_count}</p>
          <p className="text-calyx-muted">High risk</p>
        </div>
        <div>
          <p className="text-lg font-semibold text-calyx-text">{exposure.avg_risk_score.toFixed(0)}</p>
          <p className="text-calyx-muted">Avg risk</p>
        </div>
        <div>
          <p className="text-lg font-semibold text-calyx-text">{exposure.dnc_match_count}</p>
          <p className="text-calyx-muted">DNC hits</p>
        </div>
      </div>
    </div>
  );
}
