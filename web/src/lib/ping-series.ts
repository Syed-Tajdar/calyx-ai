export interface PingSeriesPoint {
  date: string;
  count: number;
}

export type PingSeriesGrain = 'day' | 'week' | 'month';

/** Badge text for the dashboard period. The chart uses this instead of a fixed "Last 7 days". */
export function periodBadge(preset: '7d' | '30d' | 'all'): string {
  if (preset === '7d') return '7 days';
  if (preset === '30d') return '30 days';
  return 'All time';
}

/** Daily under 60 points; otherwise weekly, then monthly, still under ~60 points. */
export function grainForSpan(dayCount: number): PingSeriesGrain {
  if (dayCount <= 60) return 'day';
  if (Math.ceil(dayCount / 7) <= 60) return 'week';
  return 'month';
}

function weekKey(isoDay: string): string {
  const d = new Date(`${isoDay}T12:00:00Z`);
  const diff = (d.getUTCDay() + 6) % 7;
  d.setUTCDate(d.getUTCDate() - diff);
  return d.toISOString().slice(0, 10);
}

/** Roll daily counts up. The sum of counts is unchanged. */
export function rollupDaily(points: PingSeriesPoint[], grain: PingSeriesGrain): PingSeriesPoint[] {
  if (grain === 'day') return points;
  const buckets = new Map<string, number>();
  for (const point of points) {
    const key = grain === 'month' ? point.date.slice(0, 7) : weekKey(point.date);
    buckets.set(key, (buckets.get(key) ?? 0) + point.count);
  }
  return Array.from(buckets.entries())
    .sort(([a], [b]) => a.localeCompare(b))
    .map(([date, count]) => ({ date, count }));
}

export function spanDays(points: PingSeriesPoint[]): number {
  if (points.length === 0) return 0;
  const first = new Date(`${points[0].date}T12:00:00Z`).getTime();
  const last = new Date(`${points[points.length - 1].date}T12:00:00Z`).getTime();
  return Math.floor((last - first) / 86_400_000) + 1;
}

export function chartSeriesForPreset(
  points: PingSeriesPoint[],
  preset: '7d' | '30d' | 'all',
): PingSeriesPoint[] {
  const grain = preset === 'all' ? grainForSpan(spanDays(points)) : 'day';
  return rollupDaily(points, grain);
}
