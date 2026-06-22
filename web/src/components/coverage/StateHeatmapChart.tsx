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

  return (
    <div className="space-y-4">
      <div className="calyx-card p-4">
        <p className="mb-4 text-[10px] font-bold uppercase tracking-wider text-calyx-accent">
          Demand by State
        </p>
        {chartData.length === 0 ? (
          <p className="py-8 text-center text-sm text-calyx-muted">
            No state data yet. Pings with a state field will appear here.
          </p>
        ) : (
          <ResponsiveContainer width="100%" height={Math.max(200, chartData.length * 28)}>
            <BarChart data={chartData} layout="vertical" margin={{ left: 10, right: 20 }}>
              <CartesianGrid strokeDasharray="3 3" stroke="#1a4a47" horizontal={false} />
              <XAxis type="number" tick={{ fill: '#8ba8a6', fontSize: 10 }} />
              <YAxis
                type="category"
                dataKey="state"
                width={36}
                tick={{ fill: '#8ba8a6', fontSize: 10 }}
              />
              <Tooltip
                contentStyle={{
                  background: '#0f2e2c',
                  border: '1px solid #1a4a47',
                  borderRadius: 8,
                  fontSize: 12,
                }}
              />
              <Bar dataKey="pings" fill="#00c9b7" radius={[0, 4, 4, 0]} name="Pings" />
            </BarChart>
          </ResponsiveContainer>
        )}
      </div>

      {data.length > 0 && (
        <div className="calyx-card overflow-hidden">
          <table className="w-full text-left text-xs">
            <thead>
              <tr className="border-b border-calyx-border text-[10px] uppercase text-calyx-muted">
                <th className="px-4 py-3 font-medium">State</th>
                <th className="px-4 py-3 font-medium">Pings</th>
                <th className="px-4 py-3 font-medium">Revenue</th>
              </tr>
            </thead>
            <tbody>
              {data.map((row) => (
                <tr key={row.state} className="border-b border-calyx-border/40">
                  <td className="px-4 py-3 font-semibold text-calyx-text">{row.state}</td>
                  <td className="px-4 py-3 text-calyx-muted">{row.ping_count}</td>
                  <td className="px-4 py-3 text-emerald-400">${row.revenue.toFixed(2)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}
