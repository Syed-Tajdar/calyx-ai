import type { LucideIcon } from 'lucide-react';

interface MetricCardProps {
  label: string;
  value: string;
  trend?: string;
  trendUp?: boolean;
  icon?: LucideIcon;
}

export function MetricCard({
  label,
  value,
  trend,
  trendUp = true,
  icon: Icon,
}: MetricCardProps): JSX.Element {
  return (
    <div className="calyx-metric-card">
      <div className="flex items-start justify-between">
        <p className="text-[10px] font-semibold uppercase leading-tight tracking-wide text-calyx-muted">
          {label}
        </p>
        {Icon && <Icon className="h-4 w-4 text-calyx-teal" aria-hidden />}
      </div>
      <p className="mt-2 text-2xl font-bold tabular-nums text-calyx-text">{value}</p>
      {trend && (
        <div className="mt-2">
          <span
            className={`inline-flex items-center gap-0.5 rounded px-1.5 py-0.5 text-[10px] font-semibold ${
              trendUp
                ? 'bg-calyx-success/15 text-calyx-success'
                : 'bg-calyx-danger/15 text-calyx-danger'
            }`}
          >
            {trendUp ? '↑' : '↓'} {trend}
          </span>
        </div>
      )}
    </div>
  );
}
