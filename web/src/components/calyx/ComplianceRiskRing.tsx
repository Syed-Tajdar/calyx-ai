import { riskLevelFromScore } from '@/components/ui/RiskBadge';

interface ComplianceRiskRingProps {
  score: number;
  exposureUsd: number;
  size?: 'sm' | 'md';
}

const SIZE_MAP = {
  sm: { ring: 64, stroke: 5, text: 'text-xs' },
  md: { ring: 80, stroke: 6, text: 'text-sm' },
};

const COLOR_MAP = {
  low: 'var(--calyx-success)',
  medium: 'var(--calyx-warning)',
  high: 'var(--calyx-danger)',
};

function formatRiskScore(score: number): string {
  if (!Number.isFinite(score)) return '—';
  return `${Math.round(score)}/100`;
}

export function ComplianceRiskRing({
  score,
  exposureUsd,
  size = 'md',
}: ComplianceRiskRingProps): JSX.Element {
  const level = riskLevelFromScore(score);
  const { ring, stroke, text } = SIZE_MAP[size];
  const clampedScore = Math.min(100, Math.max(0, score));
  const radius = (ring - stroke) / 2;
  const circumference = 2 * Math.PI * radius;
  const offset = circumference - (clampedScore / 100) * circumference;
  const color = COLOR_MAP[level];
  const displayScore = formatRiskScore(clampedScore);

  return (
    <div className="flex flex-col items-center gap-2">
      <div className="relative" style={{ width: ring, height: ring }}>
        <svg width={ring} height={ring} className="-rotate-90">
          <circle
            cx={ring / 2}
            cy={ring / 2}
            r={radius}
            fill="none"
            stroke="var(--calyx-border)"
            strokeWidth={stroke}
          />
          <circle
            cx={ring / 2}
            cy={ring / 2}
            r={radius}
            fill="none"
            stroke={color}
            strokeWidth={stroke}
            strokeDasharray={circumference}
            strokeDashoffset={offset}
            strokeLinecap="round"
          />
        </svg>
        <div className="absolute inset-0 flex items-center justify-center px-1">
          <span className={`max-w-full text-center font-bold leading-none tabular-nums text-calyx-text ${text}`}>
            {displayScore}
          </span>
        </div>
      </div>
      <p className="text-center text-[10px] text-calyx-muted">
        Exposure{' '}
        <span className="font-semibold tabular-nums text-calyx-text">
          ${exposureUsd.toFixed(2)}
        </span>
      </p>
    </div>
  );
}
