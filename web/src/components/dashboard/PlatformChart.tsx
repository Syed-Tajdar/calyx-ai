'use client';

interface PlatformChartProps {
  data: Array<{ name: string; value: number; percent: number }>;
}

const BAR_COLORS = [
  'var(--calyx-accent)',
  'var(--calyx-teal)',
  'var(--calyx-warning)',
  'var(--calyx-success)',
  'var(--calyx-muted)',
];

export function PlatformChart({ data }: PlatformChartProps): JSX.Element {
  const rows = data.length > 0 ? data : [{ name: 'No data', value: 0, percent: 0 }];

  return (
    <div className="calyx-card flex h-full flex-col p-5">
      <p className="text-[10px] font-bold uppercase tracking-wider text-calyx-muted">
        Call Source Breakdown
      </p>
      <h3 className="mt-0.5 text-sm font-semibold text-calyx-text">Platform mix</h3>
      <div className="mt-4 flex flex-col gap-3.5">
        {rows.map((row, index) => (
          <div key={row.name}>
            <div className="mb-1.5 flex justify-between text-xs">
              <span className="text-calyx-muted">{row.name}</span>
              <span className="font-semibold tabular-nums text-calyx-text">{row.percent}%</span>
            </div>
            <div className="h-2 overflow-hidden rounded-full bg-calyx-surface">
              <div
                className="h-full rounded-full transition-all"
                style={{
                  width: `${Math.max(row.percent, row.name === 'No data' ? 0 : 4)}%`,
                  backgroundColor: BAR_COLORS[index % BAR_COLORS.length],
                }}
              />
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}
