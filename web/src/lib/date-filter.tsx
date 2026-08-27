'use client';

import { createContext, useContext, useMemo, useState, type ReactNode } from 'react';

export type DateRangePreset = '7d' | '30d' | 'all';

interface DateFilterState {
  preset: DateRangePreset;
  platform: string;
  /** Include `apisync-%` demo rows. Default false. */
  showDemo: boolean;
  setPreset: (preset: DateRangePreset) => void;
  setPlatform: (platform: string) => void;
  setShowDemo: (show: boolean) => void;
  startDate: Date | null;
}

const DateFilterContext = createContext<DateFilterState | null>(null);

function presetToStart(preset: DateRangePreset): Date | null {
  if (preset === 'all') return null;
  const days = preset === '7d' ? 7 : 30;
  const d = new Date();
  d.setDate(d.getDate() - days);
  d.setHours(0, 0, 0, 0);
  return d;
}

export function DateFilterProvider({ children }: { children: ReactNode }): JSX.Element {
  const [preset, setPreset] = useState<DateRangePreset>('30d');
  const [platform, setPlatform] = useState('');
  const [showDemo, setShowDemo] = useState(false);

  const value = useMemo(
    () => ({
      preset,
      platform,
      showDemo,
      setPreset,
      setPlatform,
      setShowDemo,
      startDate: presetToStart(preset),
    }),
    [preset, platform, showDemo],
  );

  return <DateFilterContext.Provider value={value}>{children}</DateFilterContext.Provider>;
}

export function useDateFilter(): DateFilterState {
  const ctx = useContext(DateFilterContext);
  if (!ctx) {
    throw new Error('useDateFilter must be used within DateFilterProvider');
  }
  return ctx;
}

export function filterByDateAndPlatform<
  T extends { received_at?: string; occurred_at?: string; platform?: string },
>(rows: T[], startDate: Date | null, platform: string): T[] {
  return rows.filter((row) => {
    if (platform && row.platform && row.platform !== platform) return false;
    const eventAt = row.occurred_at || row.received_at;
    if (startDate && eventAt) {
      const t = new Date(eventAt).getTime();
      if (t < startDate.getTime()) return false;
    }
    return true;
  });
}

export interface DateFilterQuery {
  from?: string;
  to?: string;
  platform?: string;
  search?: string;
  show_demo?: boolean;
}

export function dateFilterToQuery(
  startDate: Date | null,
  platform: string,
  showDemo = false,
): DateFilterQuery {
  const query: DateFilterQuery = {};
  if (startDate) {
    query.from = startDate.toISOString();
  }
  if (platform) {
    query.platform = platform;
  }
  if (showDemo) {
    query.show_demo = true;
  }
  return query;
}
