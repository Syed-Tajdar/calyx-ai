import type { LucideIcon } from 'lucide-react';
import type { ReactNode } from 'react';
import { MiniSparkline } from '@/components/ui/MiniSparkline';
import { sparklinePeriodDelta } from '@/lib/sparkline-utils';
import { cn } from '@/lib/utils';

interface MetricCardFrameProps {
  children: ReactNode;
  className?: string;
}

export function MetricCardFrame({ children, className }: MetricCardFrameProps): JSX.Element {
  return (
    <div
      className={cn(
        'calyx-metric-card relative flex flex-col pl-5 motion-safe:transition-shadow motion-safe:hover:shadow-[0_8px_32px_rgba(0,0,0,0.55)]',
        className,
      )}
    >
      <span
        className="absolute bottom-3 left-0 top-3 w-[3px] rounded-r"
        style={{
          background: 'linear-gradient(180deg, var(--calyx-teal), var(--aurora-orb-1))',
        }}
        aria-hidden
      />
      {children}
    </div>
  );
}

interface MetricCardProps {
  label: string;
  value: string;
  trend?: string;
  trendUp?: boolean;
  icon?: LucideIcon;
  sparklineData?: number[];
  /** Stagger icon pulse across a metric row (ms). */
  iconPulseDelayMs?: number;
}

export function MetricCard({
  label,
  value,
  trend,
  icon: Icon,
  sparklineData,
  iconPulseDelayMs,
}: MetricCardProps): JSX.Element {
  const delta = sparklineData ? sparklinePeriodDelta(sparklineData) : null;

  return (
    <MetricCardFrame>
      <div className="flex items-start justify-between">
        <p className="text-[10px] font-semibold uppercase leading-tight tracking-wide text-calyx-muted">
          {label}
        </p>
        {Icon && (
          <span
            className="calyx-metric-icon-pulse"
            style={
              iconPulseDelayMs !== undefined
                ? { animationDelay: `${iconPulseDelayMs}ms` }
                : undefined
            }
          >
            <Icon className="size-[18px] text-calyx-teal" strokeWidth={2} aria-hidden />
          </span>
        )}
      </div>
      <p className="calyx-metric-value mt-2 text-2xl font-bold tabular-nums text-calyx-text">
        {value}
      </p>
      {(trend || delta) && (
        <div className="mt-2 flex flex-wrap items-center gap-2">
          {trend && (
            <span className="text-[10px] font-medium text-calyx-muted">{trend}</span>
          )}
          {delta && (
            <span
              className={cn(
                'inline-flex items-center gap-0.5 rounded-full px-2 py-0.5 text-[11px] font-medium',
                delta.up
                  ? 'bg-calyx-teal/10 text-calyx-teal'
                  : 'bg-calyx-danger/15 text-calyx-danger',
              )}
            >
              {delta.up ? '↑' : '↓'} {delta.pct.toFixed(0)}%
            </span>
          )}
        </div>
      )}
      {sparklineData && sparklineData.length > 1 && (
        <div className="mt-auto flex justify-end pt-3">
          <MiniSparkline data={sparklineData} />
        </div>
      )}
    </MetricCardFrame>
  );
}
