interface ScoreBarProps {
  score: number;
}

export function ScoreBar({ score }: ScoreBarProps): JSX.Element {
  const pct = Math.min(100, Math.max(0, score));
  const fillColor = score >= 70 ? '#1D9E75' : '#EF9F27';

  return (
    <div className="flex items-center gap-2">
      <div className="h-1.5 w-16 overflow-hidden rounded-full bg-calyx-surface">
        <div className="h-full" style={{ width: `${pct}%`, backgroundColor: fillColor }} />
      </div>
      <span className="font-medium tabular-nums">{score.toFixed(1)}</span>
    </div>
  );
}
