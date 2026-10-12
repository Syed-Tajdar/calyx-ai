'use client';

import { createContext, useContext, useEffect, useMemo, useState, type ReactNode } from 'react';
import { fetchMe, type PingConversionFilter } from '@/lib/api';
import { getToken } from '@/lib/auth';
import { V1_ONLY } from '@/lib/v1-features';

export type DateRangePreset = '7d' | '30d' | 'all';

/** `''` means no conversion filtering (all calls). */
export type ConversionSelection = PingConversionFilter | '';

interface DateFilterState {
  preset: DateRangePreset;
  platform: string;
  /** Include `apisync-%` demo rows. Default false. */
  showDemo: boolean;
  /** API + APP_ENV decide this. Production is always false. */
  demoModeEnabled: boolean;
  /** Conversion state filter. `''` = all. */
  conversion: ConversionSelection;
  setPreset: (preset: DateRangePreset) => void;
  setPlatform: (platform: string) => void;
  setShowDemo: (show: boolean) => void;
  setConversion: (conversion: ConversionSelection) => void;
  startDate: Date | null;
}

const DateFilterContext = createContext<DateFilterState | null>(null);

function presetToStart(preset: DateRangePreset): Date | null {
  if (preset === 'all') return null;
  const days = preset === '7d' ? 7 : 30;
  const d = new Date();
  d.setHours(0, 0, 0, 0);
  d.setDate(d.getDate() - (days - 1));
  return d;
}

export function DateFilterProvider({ children }: { children: ReactNode }): JSX.Element {
  const [preset, setPreset] = useState<DateRangePreset>('30d');
  const [platform, setPlatform] = useState('');
  const [showDemo, setShowDemoState] = useState(false);
  // Default false so production never flashes the Show-demo toggle before /auth/me.
  const [demoModeEnabled, setDemoModeEnabled] = useState(false);
  const [conversion, setConversion] = useState<ConversionSelection>('');

  useEffect(() => {
    const token = getToken();
    if (!token) return;
    fetchMe(token)
      .then((me) => {
        setDemoModeEnabled(me.demo_mode_enabled);
        if (!me.demo_mode_enabled) setShowDemoState(false);
      })
      .catch(() => undefined);
  }, []);

  function setShowDemo(show: boolean): void {
    if (!demoModeEnabled) {
      setShowDemoState(false);
      return;
    }
    setShowDemoState(show);
  }

  const value = useMemo(
    () => ({
      preset,
      platform,
      showDemo: demoModeEnabled && showDemo,
      demoModeEnabled,
      conversion,
      setPreset,
      setPlatform,
      setShowDemo,
      setConversion,
      startDate: presetToStart(preset),
    }),
    [preset, platform, showDemo, demoModeEnabled, conversion],
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
  conversion?: PingConversionFilter;
  /** v1 All: ask the API to count ringba + retreaver only. */
  v1_platforms?: boolean;
}

export function dateFilterToQuery(
  startDate: Date | null,
  platform: string,
  showDemo = false,
  conversion: ConversionSelection = '',
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
  if (conversion) {
    query.conversion = conversion;
  }
  if (V1_ONLY && !platform) {
    query.v1_platforms = true;
  }
  return query;
}
