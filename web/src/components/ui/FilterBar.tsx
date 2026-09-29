'use client';

import {
  useDateFilter,
  type ConversionSelection,
  type DateRangePreset,
} from '@/lib/date-filter';
import { formatPlatformLabel } from '@/lib/platform-options';

const PRESETS: { value: DateRangePreset; label: string }[] = [
  { value: '7d', label: '7 days' },
  { value: '30d', label: '30 days' },
  { value: 'all', label: 'All time' },
];

const CONVERSIONS: { value: ConversionSelection; label: string; title?: string }[] = [
  { value: '', label: 'All' },
  { value: 'converted', label: 'Converted' },
  { value: 'unconverted', label: 'Not converted' },
  {
    value: 'should_convert',
    label: 'Should have converted',
    title:
      'Not converted, but the call either earned revenue or talked at least as long as the required duration',
  },
];

interface FilterBarProps {
  showPlatform?: boolean;
  platforms?: string[];
  /** Show the demo-rows toggle (default true). */
  showDemoToggle?: boolean;
  /** Show the conversion-state filter (call list only). */
  showConversion?: boolean;
}

export function FilterBar({
  showPlatform = false,
  platforms = [],
  showDemoToggle = true,
  showConversion = false,
}: FilterBarProps): JSX.Element {
  const {
    preset,
    platform,
    showDemo,
    conversion,
    setPreset,
    setPlatform,
    setShowDemo,
    setConversion,
  } = useDateFilter();

  return (
    <div className="mb-4 flex flex-wrap items-center gap-x-5 gap-y-2">
      <div className="flex flex-wrap items-center gap-2">
        <span className="filter-group-label">Period</span>
        {PRESETS.map((p) => (
          <button
            key={p.value}
            type="button"
            onClick={() => setPreset(p.value)}
            className={`inline-flex h-8 items-center rounded-md px-2.5 text-xs font-medium leading-none transition ${
              preset === p.value
                ? 'bg-calyx-accent-muted text-calyx-accent'
                : 'text-calyx-muted hover:bg-calyx-surface hover:text-calyx-text'
            }`}
          >
            {p.label}
          </button>
        ))}
      </div>
      {showPlatform && (
        <div className="flex flex-wrap items-center gap-2">
          <span className="filter-group-label">Platform</span>
          <select
            value={platform}
            onChange={(e) => setPlatform(e.target.value)}
            className="inline-flex h-8 items-center rounded-md border border-calyx-border bg-calyx-surface px-2.5 text-xs leading-none text-calyx-text"
          >
            <option value="">All</option>
            {platforms.map((pl) => (
              <option key={pl} value={pl}>
                {formatPlatformLabel(pl)}
              </option>
            ))}
          </select>
        </div>
      )}
      {showConversion && (
        <div className="flex flex-wrap items-center gap-2">
          <span className="filter-group-label">Status</span>
          {CONVERSIONS.map((c) => (
            <button
              key={c.value || 'all'}
              type="button"
              title={c.title}
              onClick={() => setConversion(c.value)}
              className={`inline-flex h-8 items-center rounded-md px-2.5 text-xs font-medium leading-none transition ${
                conversion === c.value
                  ? 'bg-calyx-accent-muted text-calyx-accent'
                  : 'text-calyx-muted hover:bg-calyx-surface hover:text-calyx-text'
              }`}
            >
              {c.label}
            </button>
          ))}
        </div>
      )}
      {showDemoToggle && (
        <label className="flex cursor-pointer items-center gap-2 text-xs text-calyx-muted">
          <input
            type="checkbox"
            checked={showDemo}
            onChange={(e) => setShowDemo(e.target.checked)}
            className="h-3.5 w-3.5 rounded border-calyx-border accent-calyx-accent"
          />
          Show demo
          {showDemo && (
            <span className="rounded bg-amber-500/15 px-1.5 py-0.5 text-[10px] font-semibold uppercase tracking-wide text-amber-400">
              Demo included
            </span>
          )}
        </label>
      )}
    </div>
  );
}
