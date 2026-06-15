'use client';

import { Cell, Pie, PieChart, ResponsiveContainer, Tooltip } from 'recharts';

interface PlatformChartProps {
  data: Array<{ name: string; value: number; percent: number }>;
}

const COLORS = ['#00c9b7', '#14b8a6', '#2dd4bf', '#5eead4', '#99f6e4'];

export function PlatformChart({ data }: PlatformChartProps): JSX.Element {
  const chartData = data.length > 0 ? data : [{ name: 'No data', value: 1, percent: 0 }];

  return (
    <div className="loom-card flex h-full flex-col p-4">
      <p className="text-[10px] font-bold uppercase tracking-wider text-loom-muted">
        Call Source Breakdown
      </p>
      <h3 className="mt-0.5 text-sm font-semibold text-white">Ping Sources</h3>
      <div className="mt-3 flex flex-1 flex-col items-center gap-3 sm:flex-row">
        <div className="relative h-36 w-36 shrink-0">
          <ResponsiveContainer width="100%" height="100%">
            <PieChart>
              <Pie
                data={chartData}
                dataKey="value"
                nameKey="name"
                innerRadius={42}
                outerRadius={62}
                paddingAngle={3}
                strokeWidth={0}
              >
                {chartData.map((entry, index) => (
                  <Cell key={entry.name} fill={COLORS[index % COLORS.length]} />
                ))}
              </Pie>
              <Tooltip
                contentStyle={{
                  background: '#0f2e2c',
                  border: '1px solid #1a4a47',
                  borderRadius: '8px',
                  color: '#fff',
                  fontSize: '12px',
                }}
              />
            </PieChart>
          </ResponsiveContainer>
          <div className="pointer-events-none absolute inset-0 flex items-center justify-center">
            <span className="text-lg font-bold text-white">
              {chartData.reduce((s, d) => s + d.value, 0)}
            </span>
          </div>
        </div>
        <ul className="w-full space-y-2">
          {chartData.map((item, index) => (
            <li key={item.name} className="flex items-center justify-between text-xs">
              <span className="flex items-center gap-2 text-loom-muted">
                <span
                  className="h-2 w-2 rounded-full"
                  style={{ backgroundColor: COLORS[index % COLORS.length] }}
                />
                {item.name}
              </span>
              <span className="font-semibold text-white">{item.percent}%</span>
            </li>
          ))}
        </ul>
      </div>
    </div>
  );
}
