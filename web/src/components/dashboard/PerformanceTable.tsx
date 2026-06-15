import type { DashboardStats } from '@/lib/dashboard-stats';

interface PerformanceTableProps {
  publishers: DashboardStats['publishers'];
}

export function PerformanceTable({ publishers }: PerformanceTableProps): JSX.Element {
  return (
    <div className="loom-card flex h-full flex-col p-4">
      <p className="text-[10px] font-bold uppercase tracking-wider text-loom-accent">
        Publisher Performance
      </p>
      <h3 className="mt-0.5 text-sm font-semibold text-white">Ping by Publisher</h3>
      <div className="mt-3 flex-1 overflow-x-auto">
        <table className="w-full min-w-[280px] text-left text-xs">
          <thead>
            <tr className="border-b border-loom-border text-[10px] uppercase tracking-wider text-loom-muted">
              <th className="pb-2 pr-2 font-medium">Publisher</th>
              <th className="pb-2 pr-2 font-medium">Unique %</th>
              <th className="pb-2 pr-2 font-medium">Dup %</th>
              <th className="pb-2 font-medium">Avg Rev</th>
            </tr>
          </thead>
          <tbody>
            {publishers.map((row) => (
              <tr key={row.name} className="border-b border-loom-border/40">
                <td className="py-2.5 pr-2 font-medium text-white">{row.name}</td>
                <td className="py-2.5 pr-2 text-emerald-400">{row.uniqueRate.toFixed(1)}%</td>
                <td className="py-2.5 pr-2 text-red-400">{row.duplicateRate.toFixed(1)}%</td>
                <td className="py-2.5 text-loom-muted">${row.avgRevenue.toFixed(2)}</td>
              </tr>
            ))}
            {publishers.length === 0 && (
              <tr>
                <td colSpan={4} className="py-8 text-center text-loom-muted">
                  No publisher data yet
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </div>
    </div>
  );
}
