'use client';

import { MetricCardFrame } from '@/components/dashboard/MetricCard';
import type { HeatmapStateRow } from '@/lib/api';
import { stateNameFromCode } from '@/lib/us-states';
import { useCountUp, usePrefersReducedMotion } from '@/lib/use-count-up';
import { cn } from '@/lib/utils';

interface StateDetailPanelProps {
  selected: HeatmapStateRow | null;
  rank: number | null;
  totalPings: number;
  onClear: () => void;
  className?: string;
}

const PANEL_SHELL =
  'w-full self-start overflow-hidden rounded-xl p-3.5 pl-4 motion-safe:transition-shadow';

function formatShare(pings: number, total: number): string {
  if (total === 0) return '0%';
  return `${((pings / total) * 100).toFixed(1)}%`;
}

export function StateDetailPanel({
  selected,
  rank,
  totalPings,
  onClear,
  className,
}: StateDetailPanelProps): JSX.Element {
  const reducedMotion = usePrefersReducedMotion();
  const animate = !reducedMotion;

  const pings = selected?.ping_count ?? 0;
  const revenue = selected?.revenue ?? 0;
  const avgRpc = pings > 0 ? revenue / pings : 0;

  const displayPings = useCountUp(pings, { enabled: animate });
  const displayRevenue = useCountUp(revenue, { enabled: animate });

  if (!selected) {
    return (
      <MetricCardFrame className={cn(PANEL_SHELL, 'py-6', className)}>
        <p className="text-sm font-medium text-calyx-muted">Select a state</p>
        <p className="mt-1 text-xs text-calyx-dim">
          Click a state on the map to view demand details.
        </p>
      </MetricCardFrame>
    );
  }

  const name = stateNameFromCode(selected.state);

  return (
    <MetricCardFrame
      className={cn(
        PANEL_SHELL,
        'motion-safe:animate-[calyx-panel-in_280ms_ease-out]',
        className,
      )}
    >
      <div className="flex items-start justify-between gap-2">
        <div className="min-w-0">
          <p className="text-[10px] font-semibold uppercase tracking-wider text-calyx-muted">
            State detail
          </p>
          <h3 className="mt-0.5 truncate text-sm font-bold text-calyx-text">{name}</h3>
        </div>
        {rank !== null && (
          <span className="shrink-0 rounded-full bg-calyx-teal/10 px-2 py-0.5 text-[10px] font-semibold text-calyx-teal">
            #{rank}
          </span>
        )}
      </div>

      <dl className="mt-3 grid grid-cols-1 gap-2.5 text-xs">
        <div>
          <dt className="text-[10px] text-calyx-muted">Total pings</dt>
          <dd className="text-base font-bold tabular-nums leading-tight text-calyx-text">
            {Math.round(displayPings).toLocaleString()}
          </dd>
        </div>
        <div>
          <dt className="text-[10px] text-calyx-muted">Revenue</dt>
          <dd className="text-base font-bold tabular-nums leading-tight text-calyx-teal">
            ${displayRevenue.toFixed(2)}
          </dd>
        </div>
        <div className="grid grid-cols-2 gap-2 border-t border-calyx-border/40 pt-2.5">
          <div>
            <dt className="text-[10px] text-calyx-muted">Avg RPC</dt>
            <dd className="mt-0.5 font-semibold tabular-nums text-calyx-text">
              ${avgRpc.toFixed(2)}
            </dd>
          </div>
          <div>
            <dt className="text-[10px] text-calyx-muted">Share</dt>
            <dd className="mt-0.5 font-semibold tabular-nums text-calyx-text">
              {formatShare(pings, totalPings)}
            </dd>
          </div>
        </div>
      </dl>

      <button
        type="button"
        onClick={onClear}
        className="mt-3 text-[11px] font-medium text-calyx-accent hover:text-calyx-accent-hover motion-safe:transition-colors"
      >
        Clear selection
      </button>
    </MetricCardFrame>
  );
}
