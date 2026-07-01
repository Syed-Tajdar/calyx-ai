import type { DemoPublisherRow } from '@/lib/demo-publishers';
import { QualityBadge } from '@/components/ui/StatusPill';

interface PerformanceTableProps {
  publishers: DemoPublisherRow[];
}

export function PerformanceTable({ publishers }: PerformanceTableProps): JSX.Element {
  return (
    <div className="calyx-card flex h-full flex-1 flex-col p-4">
      <p className="text-[10px] font-bold uppercase tracking-wider text-calyx-accent">
        Publisher Performance
      </p>
      <h3 className="mt-0.5 text-sm font-semibold text-calyx-text">Ping by Publisher</h3>
      <div className="mt-3 flex-1 overflow-x-auto">
        <table className="w-full min-w-[320px] text-left text-xs">
          <thead>
            <tr className="border-b border-calyx-border text-[10px] uppercase tracking-wider text-calyx-muted">
              <th className="pb-2 pr-2 font-medium">Publisher</th>
              <th className="pb-2 pr-2 font-medium">Unique %</th>
              <th className="pb-2 pr-2 font-medium">Dup %</th>
              <th className="pb-2 pr-2 font-medium">Avg Rev</th>
              <th className="pb-2 font-medium">Quality</th>
            </tr>
          </thead>
          <tbody>
            {publishers.map((row) => (
              <tr key={row.name} className="border-b border-calyx-border/40">
                <td className="py-2.5 pr-2 font-medium text-calyx-text">{row.name}</td>
                <td className="py-2.5 pr-2 text-emerald-400">{Math.round(row.uniqueRate)}%</td>
                <td className="py-2.5 pr-2 text-red-400">{Math.round(row.duplicateRate)}%</td>
                <td className="py-2.5 pr-2 text-calyx-muted">${row.avgRevenue.toFixed(2)}</td>
                <td className="py-2.5">
                  <QualityBadge quality={row.quality} />
                </td>
              </tr>
            ))}
            {publishers.length === 0 && (
              <tr>
                <td colSpan={5} className="py-8 text-center text-calyx-muted">
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
