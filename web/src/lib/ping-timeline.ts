import type { PingRecord } from '@/lib/api';
import { pingEventAt } from '@/lib/ping-event-time';

export interface PingTimelinePoint {
  date: string;
  count: number;
}

/** Bucket distinct calls by day (dedupe platform+external_call_id) and fill the window. */
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

  // Latest row per (platform, external_call_id) — matches analytics distinct-call counts.
  const latestByCall = new Map<string, PingRecord>();
  for (const ping of pings) {
    if (!ping.external_call_id) continue;
    const id = `${ping.platform}|${ping.external_call_id}`;
    const prev = latestByCall.get(id);
    if (!prev || pingEventAt(ping) >= pingEventAt(prev)) {
      latestByCall.set(id, ping);
    }
  }

  for (const ping of Array.from(latestByCall.values())) {
    const key = pingEventAt(ping).slice(0, 10);
    if (buckets.has(key)) {
      buckets.set(key, (buckets.get(key) ?? 0) + 1);
    }
  }

  return dayKeys.map((date) => ({ date, count: buckets.get(date) ?? 0 }));
}

export function chartFiltersForLastDays(
  days: number,
  platform?: string,
  showDemo = false,
): { from: string; platform?: string; show_demo?: boolean } {
  const from = new Date();
  from.setHours(0, 0, 0, 0);
  from.setDate(from.getDate() - (days - 1));
  return {
    from: from.toISOString(),
    ...(platform ? { platform } : {}),
    ...(showDemo ? { show_demo: true } : {}),
  };
}
