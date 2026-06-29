import type { LucideIcon } from 'lucide-react';
import { MiniSparkline } from '@/components/ui/MiniSparkline';

interface MetricCardProps {
  label: string;
  value: string;
  trend?: string;
  trendUp?: boolean;
  icon?: LucideIcon;
  sparklineData?: number[];
}

export function MetricCard({
  label,
  value,
  trend,
  trendUp = true,
  icon: Icon,
  sparklineData,
}: MetricCardProps): JSX.Element {
  return (
    <div className="calyx-metric-card flex flex-col">
      <div className="flex items-start justify-between">
        <p className="text-[10px] font-semibold uppercase leading-tight tracking-wide text-calyx-muted">
          {label}
        </p>
        {Icon && <Icon className="h-4 w-4 text-calyx-teal" aria-hidden />}
      </div>
      <p className="calyx-metric-value mt-2 text-2xl font-bold text-calyx-text">{value}</p>
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
      {sparklineData && sparklineData.length > 1 && (
        <div className="mt-auto flex justify-end pt-3">
          <MiniSparkline data={sparklineData} />
        </div>
      )}
    </div>
  );
}
