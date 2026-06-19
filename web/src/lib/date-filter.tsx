'use client';

import { createContext, useContext, useMemo, useState, type ReactNode } from 'react';

export type DateRangePreset = '7d' | '30d' | 'all';

interface DateFilterState {
  preset: DateRangePreset;
  platform: string;
  setPreset: (preset: DateRangePreset) => void;
  setPlatform: (platform: string) => void;
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

  const value = useMemo(
    () => ({
      preset,
      platform,
      setPreset,
      setPlatform,
      startDate: presetToStart(preset),
    }),
    [preset, platform],
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

export function filterByDateAndPlatform<T extends { received_at?: string; platform?: string }>(
  rows: T[],
  startDate: Date | null,
  platform: string,
): T[] {
  return rows.filter((row) => {
    if (platform && row.platform && row.platform !== platform) return false;
    if (startDate && row.received_at) {
      const t = new Date(row.received_at).getTime();
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
}

export function dateFilterToQuery(
  startDate: Date | null,
  platform: string,
): DateFilterQuery {
  const query: DateFilterQuery = {};
  if (startDate) {
    query.from = startDate.toISOString();
  }
  if (platform) {
    query.platform = platform;
  }
  return query;
}
