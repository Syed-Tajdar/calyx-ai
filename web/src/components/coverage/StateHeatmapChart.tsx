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
import type { HeatmapStateRow } from '@/lib/api';

interface StateHeatmapChartProps {
  data: HeatmapStateRow[];
}

export function StateHeatmapChart({ data }: StateHeatmapChartProps): JSX.Element {
  const chartData = data.slice(0, 15).map((row) => ({
    state: row.state,
    pings: row.ping_count,
    revenue: row.revenue,
  }));

  const showTop15Label = data.length > 15;

  return (
    <div className="calyx-card p-4">
      <p className="text-[10px] font-bold uppercase tracking-wider text-calyx-accent">
        {showTop15Label ? 'Demand by State — Top 15' : 'Demand by State'}
      </p>
      <p className="mt-1 text-[11px] text-calyx-muted">
        {showTop15Label
          ? 'Top 15 by volume · see map for all states'
          : 'By ping volume · see map for all states'}
      </p>
      {chartData.length === 0 ? (
        <p className="py-8 text-center text-sm text-calyx-muted">
          No state data yet. Pings with a state field will appear here.
        </p>
      ) : (
        <div className="mt-4">
          <ResponsiveContainer width="100%" height={Math.max(200, chartData.length * 28)}>
            <BarChart data={chartData} layout="vertical" margin={{ left: 10, right: 20 }}>
              <CartesianGrid strokeDasharray="3 3" stroke="var(--calyx-chart-grid)" horizontal={false} />
              <XAxis type="number" tick={{ fill: 'var(--calyx-chart-tick)', fontSize: 10 }} />
              <YAxis
                type="category"
                dataKey="state"
                width={36}
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
              />
              <Bar dataKey="pings" fill="var(--calyx-teal)" radius={[0, 4, 4, 0]} name="Pings" />
            </BarChart>
          </ResponsiveContainer>
        </div>
      )}
    </div>
  );
}
