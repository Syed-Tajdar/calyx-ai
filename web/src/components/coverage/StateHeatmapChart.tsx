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
import { countryForRegion } from '@/lib/na-regions';

interface StateHeatmapChartProps {
  data: HeatmapStateRow[];
}

export function StateHeatmapChart({ data }: StateHeatmapChartProps): JSX.Element {
  const chartData = data.slice(0, 15).map((row) => {
    const country = row.country || countryForRegion(row.state) || '';
    return {
      state: country ? `${row.state} · ${country}` : row.state,
      pings: row.ping_count,
      revenue: row.revenue,
    };
  });

  const showTop15Label = data.length > 15;

  return (
    <div className="calyx-card p-4">
      <p className="text-[10px] font-bold uppercase tracking-wider text-calyx-accent">
        {showTop15Label ? 'Demand by Region — Top 15' : 'Demand by Region'}
      </p>
      <p className="mt-1 text-[11px] text-calyx-muted">
        {showTop15Label
          ? 'Top 15 by volume · US + Canada · see map for all regions'
          : 'By ping volume · US + Canada · see map for all regions'}
      </p>
      {chartData.length === 0 ? (
        <p className="py-8 text-center text-sm text-calyx-muted">
          No region data yet. Pings with a US or Canadian region code will appear here.
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
                width={72}
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
