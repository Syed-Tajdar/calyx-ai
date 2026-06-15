interface MetricCardProps {
  label: string;
  value: string;
  trend?: string;
  trendUp?: boolean;
  icon?: 'calls' | 'answered' | 'missed' | 'rate';
}

function MetricIcon({ type }: { type: MetricCardProps['icon'] }): JSX.Element {
  const cls = 'h-4 w-4 text-loom-accent/70';
  if (type === 'answered') {
    return (
      <svg className={cls} viewBox="0 0 24 24" fill="none" aria-hidden>
        <path d="M5 13l4 4L19 7" stroke="currentColor" strokeWidth="2" strokeLinecap="round" />
      </svg>
    );
  }
  if (type === 'missed') {
    return (
      <svg className={cls} viewBox="0 0 24 24" fill="none" aria-hidden>
        <path d="M6 6l12 12M18 6L6 18" stroke="currentColor" strokeWidth="2" strokeLinecap="round" />
      </svg>
    );
  }
  if (type === 'rate') {
    return (
      <svg className={cls} viewBox="0 0 24 24" fill="none" aria-hidden>
        <path d="M4 18V8M10 18V4M16 18v-6M22 18v-10" stroke="currentColor" strokeWidth="2" strokeLinecap="round" />
      </svg>
    );
  }
  return (
    <svg className={cls} viewBox="0 0 24 24" fill="none" aria-hidden>
      <path
        d="M6.5 4h3l1.5 5-2 1.5a11 11 0 005 5L17.5 14l5 1.5v3a2 2 0 01-2 2C10.5 20.5 3.5 13.5 4.5 6a2 2 0 012-2z"
        stroke="currentColor"
        strokeWidth="1.5"
      />
    </svg>
  );
}

export function MetricCard({
  label,
  value,
  trend,
  trendUp = true,
  icon = 'calls',
}: MetricCardProps): JSX.Element {
  return (
    <div className="loom-metric-card">
      <div className="flex items-start justify-between">
        <p className="text-[10px] font-semibold uppercase leading-tight tracking-wide text-loom-muted">
          {label}
        </p>
        <MetricIcon type={icon} />
      </div>
      <p className="mt-2 text-2xl font-bold text-white">{value}</p>
      {trend && (
        <div className="mt-2 flex items-center gap-1">
          <span
            className={`inline-flex items-center gap-0.5 rounded px-1.5 py-0.5 text-[10px] font-semibold ${
              trendUp
                ? 'bg-emerald-500/15 text-emerald-400'
                : 'bg-red-500/15 text-red-400'
            }`}
          >
            {trendUp ? '↑' : '↓'} {trend}
          </span>
        </div>
      )}
    </div>
  );
}
