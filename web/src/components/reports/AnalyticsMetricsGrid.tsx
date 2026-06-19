import { DollarSign, Phone, TrendingUp, Wallet } from 'lucide-react';
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
          <div key={i} className="calyx-metric-card h-24 animate-pulse bg-calyx-card/50" />
        ))}
      </div>
    );
  }

  return (
    <section className="grid grid-cols-2 gap-3 md:grid-cols-3">
      <MetricCard
        label="Total Pings"
        value={metrics.total_pings.toLocaleString()}
        icon={Phone}
      />
      <MetricCard
        label="Revenue"
        value={`$${metrics.total_revenue.toFixed(0)}`}
        icon={DollarSign}
      />
      <MetricCard
        label="Payout"
        value={`$${metrics.total_payout.toFixed(0)}`}
        icon={Wallet}
      />
      <MetricCard label="CPA" value={`$${metrics.cpa.toFixed(2)}`} icon={DollarSign} />
      <MetricCard label="RPC" value={`$${metrics.rpc.toFixed(2)}`} icon={TrendingUp} />
      <MetricCard
        label="ROI"
        value={`${metrics.roi.toFixed(1)}%`}
        trendUp={metrics.roi >= 0}
        icon={TrendingUp}
      />
    </section>
  );
}
