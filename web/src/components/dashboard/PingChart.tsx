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
}

export function PingChart({ data }: PingChartProps): JSX.Element {
  const chartData = data.length > 0 ? data : [{ date: '—', count: 0 }];

  return (
    <div className="loom-card p-4">
      <div className="mb-3 flex items-center justify-between">
        <div>
          <p className="text-[10px] font-bold uppercase tracking-wider text-loom-muted">
            Inbound Ping Overview
          </p>
        </div>
        <span className="rounded-md bg-loom-surface px-2 py-0.5 text-[10px] text-loom-muted">
          Last 7 days
        </span>
      </div>
      <div className="h-48 w-full sm:h-56">
        <ResponsiveContainer width="100%" height="100%">
          <AreaChart data={chartData}>
            <defs>
              <linearGradient id="pingGradient" x1="0" y1="0" x2="0" y2="1">
                <stop offset="0%" stopColor="#00c9b7" stopOpacity={0.45} />
                <stop offset="100%" stopColor="#00c9b7" stopOpacity={0.02} />
              </linearGradient>
            </defs>
            <CartesianGrid stroke="#1a4a47" strokeDasharray="3 3" vertical={false} />
            <XAxis
              dataKey="date"
              tick={{ fill: '#8ba8a6', fontSize: 10 }}
              axisLine={false}
              tickLine={false}
              tickFormatter={(v: string) => v.slice(5)}
            />
            <YAxis
              tick={{ fill: '#8ba8a6', fontSize: 10 }}
              axisLine={false}
              tickLine={false}
              allowDecimals={false}
              width={30}
            />
            <Tooltip
              contentStyle={{
                background: '#0f2e2c',
                border: '1px solid #1a4a47',
                borderRadius: '8px',
                color: '#fff',
                fontSize: '12px',
              }}
            />
            <Area
              type="monotone"
              dataKey="count"
              stroke="#00c9b7"
              strokeWidth={2}
              fill="url(#pingGradient)"
            />
          </AreaChart>
        </ResponsiveContainer>
      </div>
    </div>
  );
}
