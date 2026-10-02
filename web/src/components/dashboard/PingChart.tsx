'use client';

import {
  Area,
  AreaChart,
  CartesianGrid,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from 'recharts';

interface PingChartProps {
  data: Array<{ date: string; count: number }>;
  periodLabel: string;
}

const TEAL = '#1D9E75';

export function PingChart({ data, periodLabel }: PingChartProps): JSX.Element {
  const chartData = data.length > 0 ? data : [{ date: '—', count: 0 }];

  return (
    <div className="calyx-card p-4">
      <div className="mb-3 flex items-center justify-between">
        <div>
          <p className="text-[10px] font-bold uppercase tracking-wider text-calyx-muted">
            Inbound Ping Overview
          </p>
        </div>
        <span className="rounded-md bg-calyx-surface px-2 py-0.5 text-[10px] text-calyx-muted">
          {periodLabel}
        </span>
      </div>
      <div className="h-48 w-full sm:h-56">
        <ResponsiveContainer width="100%" height="100%">
          <AreaChart data={chartData} margin={{ top: 8, right: 8, left: 0, bottom: 0 }}>
            <defs>
              <linearGradient id="pingGradient" x1="0" y1="0" x2="0" y2="1">
                <stop offset="0%" stopColor={TEAL} stopOpacity={0.5} />
                <stop offset="55%" stopColor={TEAL} stopOpacity={0.12} />
                <stop offset="100%" stopColor={TEAL} stopOpacity={0} />
              </linearGradient>
            </defs>
            <CartesianGrid
              stroke="var(--calyx-chart-grid)"
              strokeDasharray="3 3"
              vertical={false}
            />
            <XAxis
              dataKey="date"
              tick={{ fill: 'var(--calyx-chart-tick)', fontSize: 10 }}
              axisLine={false}
              tickLine={false}
              tickFormatter={(v: string) => v.slice(5)}
              interval={0}
              minTickGap={4}
            />
            <YAxis
              tick={{ fill: 'var(--calyx-chart-tick)', fontSize: 10 }}
              axisLine={false}
              tickLine={false}
              allowDecimals={false}
              width={28}
              domain={[0, 'auto']}
            />
            <Tooltip
              contentStyle={{
                background: 'var(--calyx-chart-tooltip-bg)',
                border: '1px solid var(--calyx-chart-tooltip-border)',
                borderRadius: '8px',
                color: 'var(--calyx-text)',
                fontSize: '12px',
              }}
              labelFormatter={(label: string) => {
                const d = new Date(`${label}T12:00:00`);
                return d.toLocaleDateString('en-US', { month: 'short', day: 'numeric' });
              }}
              formatter={(value: number) => [`${value} pings`, 'Volume']}
            />
            <Area
              type="monotone"
              dataKey="count"
              stroke={TEAL}
              strokeWidth={2.5}
              fill="url(#pingGradient)"
              dot={{
                r: 4,
                fill: TEAL,
                stroke: 'var(--calyx-card)',
                strokeWidth: 2,
              }}
              activeDot={{
                r: 6,
                fill: TEAL,
                stroke: 'var(--calyx-card)',
                strokeWidth: 2,
              }}
              connectNulls
            />
          </AreaChart>
        </ResponsiveContainer>
      </div>
    </div>
  );
}
