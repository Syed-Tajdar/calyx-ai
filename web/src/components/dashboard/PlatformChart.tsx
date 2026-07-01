'use client';

import { formatPlatformColor, formatPlatformLabel } from '@/lib/platform-options';

interface PlatformChartProps {
  data: Array<{ name: string; slug?: string; value: number; percent: number }>;
}

export function PlatformChart({ data }: PlatformChartProps): JSX.Element {
  const rows = data
    .filter((row) => row.slug !== 'mock' && row.name.toLowerCase() !== 'mock')
    .filter((row) => row.value > 0 || row.percent > 0);

  const displayRows =
    rows.length > 0 ? rows : [{ name: 'No data', slug: '', value: 0, percent: 0 }];

  return (
    <div className="calyx-card flex h-full flex-col p-5">
      <p className="text-[10px] font-bold uppercase tracking-wider text-calyx-muted">
        Call Source Breakdown
      </p>
      <h3 className="mt-0.5 text-sm font-semibold text-calyx-text">Platform mix</h3>
      <div className="mt-4 flex flex-col gap-3.5">
        {displayRows.map((row) => {
          const slug = row.slug ?? row.name.toLowerCase();
          const colors = formatPlatformColor(slug);
          const label = row.slug ? formatPlatformLabel(row.slug) : row.name;
          const isEmpty = row.name === 'No data';

          return (
            <div key={`${slug}-${row.name}`}>
              <div className="mb-1.5 flex items-center justify-between text-xs">
                <span className="flex items-center gap-2 text-calyx-muted">
                  {!isEmpty && (
                    <span
                      className="h-2 w-2 shrink-0 rounded-full"
                      style={{ backgroundColor: colors.dot }}
                      aria-hidden
                    />
                  )}
                  {label}
                </span>
                <span className="font-semibold tabular-nums text-calyx-text">
                  {Math.round(row.percent)}%
                </span>
              </div>
              <div className="h-2 overflow-hidden rounded-full bg-calyx-surface">
                <div
                  className="h-full rounded-full transition-all"
                  style={{
                    width: `${Math.max(row.percent, isEmpty ? 0 : 4)}%`,
                    backgroundColor: isEmpty ? 'var(--calyx-muted)' : colors.dot,
                  }}
                />
              </div>
            </div>
          );
        })}
      </div>
    </div>
  );
}
