import type { ScoreRecord } from '@/lib/api';

interface ScoreCardProps {
  score: ScoreRecord | null;
  loading?: boolean;
}

export function ScoreCard({ score, loading }: ScoreCardProps): JSX.Element {
  if (loading) {
    return (
      <div className="calyx-card p-4">
        <p className="text-sm text-calyx-muted">Loading score…</p>
      </div>
    );
  }

  if (!score) {
    return (
      <div className="calyx-card p-4">
        <p className="text-[10px] font-bold uppercase tracking-wider text-calyx-accent">Call Quality</p>
        <p className="mt-2 text-sm text-calyx-muted">
          Score not available yet. Scoring completes shortly after a ping is ingested.
        </p>
      </div>
    );
  }

  const scoreColor =
    score.overall_score >= 70
      ? 'text-emerald-400'
      : score.overall_score >= 50
        ? 'text-amber-400'
        : 'text-red-400';

  return (
    <div className="calyx-card p-4">
      <p className="text-[10px] font-bold uppercase tracking-wider text-calyx-accent">Call Quality</p>
      <div className="mt-3 flex items-end gap-4">
        <p className={`text-4xl font-bold ${scoreColor}`}>{score.overall_score.toFixed(1)}</p>
        <div className="mb-1">
          <p className="text-sm capitalize text-calyx-text">{score.sentiment}</p>
          <p className="text-xs text-calyx-muted">{score.platform}</p>
        </div>
      </div>
      <div className="mt-4 flex gap-4 text-xs">
        <span className="text-emerald-400">{score.rules_passed} passed</span>
        <span className="text-red-400">{score.rules_failed} failed</span>
      </div>
    </div>
  );
}
