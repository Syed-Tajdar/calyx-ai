import type { PingRecord } from '@/lib/api';

export interface PingTimelinePoint {
  date: string;
  count: number;
}

/** Bucket pings by day and fill every day in the window (zeros included). */
export function buildFilledPingTimeline(
  pings: PingRecord[],
  days: number,
): PingTimelinePoint[] {
  const dayKeys: string[] = [];
  const buckets = new Map<string, number>();

  const start = new Date();
  start.setHours(0, 0, 0, 0);
  start.setDate(start.getDate() - (days - 1));

  for (let i = 0; i < days; i++) {
    const d = new Date(start);
    d.setDate(start.getDate() + i);
    const key = d.toISOString().slice(0, 10);
    dayKeys.push(key);
    buckets.set(key, 0);
  }

  for (const ping of pings) {
    const key = ping.received_at.slice(0, 10);
    if (buckets.has(key)) {
      buckets.set(key, (buckets.get(key) ?? 0) + 1);
    }
  }

  return dayKeys.map((date) => ({ date, count: buckets.get(date) ?? 0 }));
}

export function chartFiltersForLastDays(
  days: number,
  platform?: string,
): { from: string; platform?: string } {
  const from = new Date();
  from.setHours(0, 0, 0, 0);
  from.setDate(from.getDate() - (days - 1));
  return {
    from: from.toISOString(),
    ...(platform ? { platform } : {}),
  };
}
