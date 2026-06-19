import { riskLevelFromScore } from '@/components/ui/RiskBadge';

interface ComplianceRiskRingProps {
  score: number;
  exposureUsd: number;
  size?: 'sm' | 'md';
}

const SIZE_MAP = {
  sm: { ring: 64, stroke: 5, text: 'text-lg' },
  md: { ring: 80, stroke: 6, text: 'text-2xl' },
};

const COLOR_MAP = {
  low: '#10B981',
  medium: '#F59E0B',
  high: '#EF4444',
};

export function ComplianceRiskRing({
  score,
  exposureUsd,
  size = 'md',
}: ComplianceRiskRingProps): JSX.Element {
  const level = riskLevelFromScore(score);
  const { ring, stroke, text } = SIZE_MAP[size];
  const radius = (ring - stroke) / 2;
  const circumference = 2 * Math.PI * radius;
  const offset = circumference - (score / 100) * circumference;
  const color = COLOR_MAP[level];

  return (
    <div className="flex flex-col items-center gap-2">
      <div className="relative" style={{ width: ring, height: ring }}>
        <svg width={ring} height={ring} className="-rotate-90">
          <circle
            cx={ring / 2}
            cy={ring / 2}
            r={radius}
            fill="none"
            stroke="#1F2937"
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
        <div className="absolute inset-0 flex items-center justify-center">
          <span className={`font-bold tabular-nums text-calyx-text ${text}`}>{score}</span>
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
