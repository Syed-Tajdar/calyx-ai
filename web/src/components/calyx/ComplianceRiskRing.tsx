import { useId } from 'react';

interface ComplianceRiskRingProps {
  score: number;
  exposureUsd: number;
  size?: 'sm' | 'md';
  align?: 'center' | 'start';
}

const SIZE_MAP = {
  sm: { ring: 64, stroke: 5, text: 'text-xs' },
  md: { ring: 80, stroke: 6, text: 'text-sm' },
};

function formatRiskScore(score: number): string {
  if (!Number.isFinite(score)) return '—';
  return `${Math.round(score)}/100`;
}

export function ComplianceRiskRing({
  score,
  exposureUsd,
  size = 'md',
  align = 'center',
}: ComplianceRiskRingProps): JSX.Element {
  const gradientId = useId().replace(/:/g, '');
  const { ring, stroke, text } = SIZE_MAP[size];
  const clampedScore = Math.min(100, Math.max(0, score));
  const radius = (ring - stroke) / 2;
  const circumference = 2 * Math.PI * radius;
  const offset = circumference - (clampedScore / 100) * circumference;
  const displayScore = formatRiskScore(clampedScore);
  const aligned = align === 'start';

  return (
    <div className={aligned ? 'flex flex-col items-start gap-2' : 'flex flex-col items-center gap-2'}>
      <div className="relative" style={{ width: ring, height: ring }}>
        <svg width={ring} height={ring} className="-rotate-90">
          <defs>
            <linearGradient id={gradientId} x1="0%" y1="0%" x2="100%" y2="100%">
              <stop offset="0%" stopColor="var(--calyx-teal)" />
              <stop offset="100%" stopColor="var(--aurora-orb-1)" />
            </linearGradient>
          </defs>
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
            stroke={`url(#${gradientId})`}
            strokeWidth={stroke}
            strokeDasharray={circumference}
            strokeDashoffset={offset}
            strokeLinecap="round"
          />
        </svg>
        <div className="absolute inset-0 flex items-center justify-center px-1">
          <span
            className={`max-w-full text-center font-bold leading-none tabular-nums text-calyx-text ${text}`}
          >
            {displayScore}
          </span>
        </div>
      </div>
      <p className={aligned ? 'text-left text-[10px] text-calyx-muted' : 'text-center text-[10px] text-calyx-muted'}>
        Exposure{' '}
        <span className="font-semibold tabular-nums text-calyx-text">
          ${exposureUsd.toFixed(2)}
        </span>
      </p>
    </div>
  );
}
