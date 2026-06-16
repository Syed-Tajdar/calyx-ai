import type { MetricsResult } from '@/lib/api';
import { MetricCard } from '@/components/dashboard/MetricCard';

interface AnalyticsMetricsGridProps {
  metrics: MetricsResult | null;
}

export function AnalyticsMetricsGrid({ metrics }: AnalyticsMetricsGridProps): JSX.Element {
  if (!metrics) {
    return (
      <div className="grid grid-cols-2 gap-3 md:grid-cols-3">
        {Array.from({ length: 6 }).map((_, i) => (
          <div key={i} className="loom-metric-card h-24 animate-pulse bg-loom-card/50" />
        ))}
      </div>
    );
  }

  return (
    <section className="grid grid-cols-2 gap-3 md:grid-cols-3">
      <MetricCard
        label="Total Pings"
        value={metrics.total_pings.toLocaleString()}
        icon="calls"
      />
      <MetricCard
        label="Revenue"
        value={`$${metrics.total_revenue.toFixed(0)}`}
        icon="rate"
      />
      <MetricCard
        label="Payout"
        value={`$${metrics.total_payout.toFixed(0)}`}
        icon="missed"
      />
      <MetricCard label="CPA" value={`$${metrics.cpa.toFixed(2)}`} icon="answered" />
      <MetricCard label="RPC" value={`$${metrics.rpc.toFixed(2)}`} icon="rate" />
      <MetricCard
        label="ROI"
        value={`${metrics.roi.toFixed(1)}%`}
        trendUp={metrics.roi >= 0}
        icon="calls"
      />
    </section>
  );
}
