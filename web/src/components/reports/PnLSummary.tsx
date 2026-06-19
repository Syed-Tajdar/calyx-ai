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

interface PnLSummaryProps {
  pnl: PnLResult | null;
}

export function PnLSummary({ pnl }: PnLSummaryProps): JSX.Element {
  if (!pnl) {
    return <div className="calyx-card h-48 animate-pulse bg-calyx-card/50" />;
  }

  const chartData = [
    { name: 'Revenue', value: pnl.revenue },
    { name: 'Payout', value: pnl.payout },
    { name: 'Ad Spend', value: pnl.ad_spend },
    { name: 'Profit', value: pnl.profit },
  ];

  return (
    <div className="space-y-4">
      <section className="grid grid-cols-2 gap-3 md:grid-cols-4">
        <div className="calyx-metric-card">
          <p className="text-[10px] font-semibold uppercase text-calyx-muted">Revenue</p>
          <p className="mt-2 text-xl font-bold text-emerald-400">${pnl.revenue.toFixed(0)}</p>
        </div>
        <div className="calyx-metric-card">
          <p className="text-[10px] font-semibold uppercase text-calyx-muted">Payout</p>
          <p className="mt-2 text-xl font-bold text-calyx-text">${pnl.payout.toFixed(0)}</p>
        </div>
        <div className="calyx-metric-card">
          <p className="text-[10px] font-semibold uppercase text-calyx-muted">Ad Spend</p>
          <p className="mt-2 text-xl font-bold text-amber-400">${pnl.ad_spend.toFixed(0)}</p>
        </div>
        <div className="calyx-metric-card">
          <p className="text-[10px] font-semibold uppercase text-calyx-muted">Profit</p>
          <p
            className={`mt-2 text-xl font-bold ${pnl.profit >= 0 ? 'text-emerald-400' : 'text-red-400'}`}
          >
            ${pnl.profit.toFixed(0)}
          </p>
        </div>
      </section>

      <div className="calyx-card p-4">
        <p className="mb-4 text-[10px] font-bold uppercase tracking-wider text-calyx-accent">
          P&amp;L Breakdown
        </p>
        <ResponsiveContainer width="100%" height={200}>
          <BarChart data={chartData} layout="vertical" margin={{ left: 20, right: 20 }}>
            <CartesianGrid strokeDasharray="3 3" stroke="#1a4a47" horizontal={false} />
            <XAxis type="number" tick={{ fill: '#8ba8a6', fontSize: 10 }} />
            <YAxis
              type="category"
              dataKey="name"
              width={70}
              tick={{ fill: '#8ba8a6', fontSize: 10 }}
            />
            <Tooltip
              contentStyle={{
                background: '#0f2e2c',
                border: '1px solid #1a4a47',
                borderRadius: 8,
                fontSize: 12,
              }}
              formatter={(value: number) => [`$${value.toFixed(2)}`, '']}
            />
            <Bar dataKey="value" fill="#00c9b7" radius={[0, 4, 4, 0]} />
          </BarChart>
        </ResponsiveContainer>
      </div>
    </div>
  );
}
