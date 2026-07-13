import type { PingRecord } from '@/lib/api';
import { formatPlatformLabel } from '@/lib/platform-options';
import { pingEventAt } from '@/lib/ping-event-time';

export interface DashboardStats {
  totalPings: number;
  uniquePings: number;
  duplicatePings: number;
  uniqueRate: number;
  totalRevenue: number;
  timeline: Array<{ date: string; count: number }>;
  platforms: Array<{ name: string; value: number; percent: number }>;
  publishers: Array<{
    name: string;
    uniqueRate: number;
    duplicateRate: number;
    avgRevenue: number;
    total: number;
  }>;
}

export function computeDashboardStats(
  pings: PingRecord[],
  total: number,
): DashboardStats {
  const duplicatePings = pings.filter((p) => p.is_duplicate === 1).length;
  const uniquePings = pings.length - duplicatePings;
  const uniqueRate = pings.length > 0 ? (uniquePings / pings.length) * 100 : 0;
  const totalRevenue = pings.reduce((sum, p) => sum + p.revenue, 0);

  const byDate = new Map<string, number>();
  for (const ping of pings) {
    const date = pingEventAt(ping).slice(0, 10);
    byDate.set(date, (byDate.get(date) ?? 0) + 1);
  }
  const timeline = Array.from(byDate.entries())
    .sort(([a], [b]) => a.localeCompare(b))
    .map(([date, count]) => ({ date, count }));

  const byPlatform = new Map<string, number>();
  for (const ping of pings) {
    if (ping.platform === 'mock') continue;
    byPlatform.set(ping.platform, (byPlatform.get(ping.platform) ?? 0) + 1);
  }
  const platformTotal = Array.from(byPlatform.values()).reduce((a, b) => a + b, 0) || 1;
  const platforms = Array.from(byPlatform.entries()).map(([name, value]) => ({
    name: formatPlatformLabel(name),
    value,
    percent: Math.round((value / platformTotal) * 100),
  }));

  const byPublisher = new Map<string, PingRecord[]>();
  for (const ping of pings) {
    const key = ping.publisher_id || ping.campaign_id || 'Unknown';
    const list = byPublisher.get(key) ?? [];
    list.push(ping);
    byPublisher.set(key, list);
  }
  const publishers = Array.from(byPublisher.entries())
    .map(([name, rows]) => {
      const dupes = rows.filter((r) => r.is_duplicate === 1).length;
      const unique = rows.length - dupes;
      const revenue = rows.reduce((s, r) => s + r.revenue, 0);
      return {
        name: name.length > 16 ? `${name.slice(0, 14)}…` : name,
        uniqueRate: rows.length > 0 ? (unique / rows.length) * 100 : 0,
        duplicateRate: rows.length > 0 ? (dupes / rows.length) * 100 : 0,
        avgRevenue: rows.length > 0 ? revenue / rows.length : 0,
        total: rows.length,
      };
    })
    .sort((a, b) => b.total - a.total)
    .slice(0, 5);

  return {
    totalPings: total,
    uniquePings,
    duplicatePings,
    uniqueRate,
    totalRevenue,
    timeline,
    platforms,
    publishers,
  };
}
