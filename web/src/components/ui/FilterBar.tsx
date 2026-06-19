'use client';

import { useDateFilter, type DateRangePreset } from '@/lib/date-filter';

const PRESETS: { value: DateRangePreset; label: string }[] = [
  { value: '7d', label: '7 days' },
  { value: '30d', label: '30 days' },
  { value: 'all', label: 'All time' },
];

interface FilterBarProps {
  showPlatform?: boolean;
  platforms?: string[];
}

export function FilterBar({ showPlatform = false, platforms = [] }: FilterBarProps): JSX.Element {
  const { preset, platform, setPreset, setPlatform } = useDateFilter();

  return (
    <div className="flex flex-wrap items-center gap-2">
      <span className="text-xs font-medium text-calyx-muted">Period</span>
      {PRESETS.map((p) => (
        <button
          key={p.value}
          type="button"
          onClick={() => setPreset(p.value)}
          className={`rounded-md px-2.5 py-1 text-xs font-medium transition ${
            preset === p.value
              ? 'bg-calyx-accent-muted text-calyx-accent'
              : 'text-calyx-muted hover:bg-calyx-surface hover:text-calyx-text'
          }`}
        >
          {p.label}
        </button>
      ))}
      {showPlatform && platforms.length > 0 && (
        <>
          <span className="ml-2 text-xs font-medium text-calyx-muted">Platform</span>
          <select
            value={platform}
            onChange={(e) => setPlatform(e.target.value)}
            className="rounded-md border border-calyx-border bg-calyx-surface px-2 py-1 text-xs text-calyx-text"
          >
            <option value="">All</option>
            {platforms.map((pl) => (
              <option key={pl} value={pl}>
                {pl}
              </option>
            ))}
          </select>
        </>
      )}
    </div>
  );
}
