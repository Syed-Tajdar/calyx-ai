'use client';

import {
  Bar,
  BarChart,
  CartesianGrid,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from 'recharts';
import type { PnLResult } from '@/lib/api';
import { V1_ONLY } from '@/lib/v1-features';

interface PnLSummaryProps {
  pnl: PnLResult | null;
}

export function PnLSummary({ pnl }: PnLSummaryProps): JSX.Element {
  if (!pnl) {
    return <div className="calyx-card h-48 animate-pulse bg-calyx-card/50" />;
  }

  const grossProfit = pnl.revenue - pnl.payout;
  const profit = V1_ONLY ? grossProfit : pnl.profit;
  const chartData = V1_ONLY
    ? [
        { name: 'Revenue', value: pnl.revenue },
        { name: 'Payout', value: pnl.payout },
        { name: 'Profit (gross)', value: profit },
      ]
    : [
        { name: 'Revenue', value: pnl.revenue },
        { name: 'Payout', value: pnl.payout },
        { name: 'Ad Spend', value: pnl.ad_spend },
        { name: 'Profit', value: profit },
      ];

  return (
    <div className="space-y-4">
      <section className={`grid grid-cols-2 gap-3 ${V1_ONLY ? 'md:grid-cols-3' : 'md:grid-cols-4'}`}>
        <div className="calyx-metric-card">
          <p className="text-[10px] font-semibold uppercase text-calyx-muted">Revenue</p>
          <p className="mt-2 text-xl font-bold tabular-nums text-calyx-success">${pnl.revenue.toFixed(0)}</p>
        </div>
        <div className="calyx-metric-card">
          <p className="text-[10px] font-semibold uppercase text-calyx-muted">Payout</p>
          <p className="mt-2 text-xl font-bold text-calyx-text">${pnl.payout.toFixed(0)}</p>
        </div>
        {!V1_ONLY && (
          <div className="calyx-metric-card">
            <p className="text-[10px] font-semibold uppercase text-calyx-muted">Ad Spend</p>
            <p className="mt-2 text-xl font-bold tabular-nums text-calyx-warning">${pnl.ad_spend.toFixed(0)}</p>
          </div>
        )}
        <div className="calyx-metric-card">
          <p className="text-[10px] font-semibold uppercase text-calyx-muted">
            {V1_ONLY ? 'Profit (gross)' : 'Profit'}
          </p>
          <p
            className={`mt-2 text-xl font-bold tabular-nums ${profit >= 0 ? 'text-calyx-success' : 'text-calyx-danger'}`}
          >
            ${profit.toFixed(0)}
          </p>
        </div>
      </section>

      <div className="calyx-card p-4">
        <p className="mb-4 text-[10px] font-bold uppercase tracking-wider text-calyx-accent">
          P&amp;L Breakdown
        </p>
        <ResponsiveContainer width="100%" height={200}>
          <BarChart data={chartData} layout="vertical" margin={{ left: 20, right: 20 }}>
            <CartesianGrid strokeDasharray="3 3" stroke="var(--calyx-chart-grid)" horizontal={false} />
            <XAxis type="number" tick={{ fill: 'var(--calyx-chart-tick)', fontSize: 10 }} />
            <YAxis
              type="category"
              dataKey="name"
              width={70}
              tick={{ fill: 'var(--calyx-chart-tick)', fontSize: 10 }}
            />
            <Tooltip
              contentStyle={{
                background: 'var(--calyx-chart-tooltip-bg)',
                border: '1px solid var(--calyx-chart-tooltip-border)',
                borderRadius: 8,
                fontSize: 12,
                color: 'var(--calyx-text)',
              }}
              formatter={(value: number) => [`$${value.toFixed(2)}`, '']}
            />
            <Bar dataKey="value" fill="var(--calyx-teal)" radius={[0, 4, 4, 0]} />
          </BarChart>
        </ResponsiveContainer>
      </div>
    </div>
  );
}
