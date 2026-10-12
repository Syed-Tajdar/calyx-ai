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

function localDayKey(d: Date): string {
  const month = String(d.getMonth() + 1).padStart(2, '0');
  const day = String(d.getDate()).padStart(2, '0');
  return `${d.getFullYear()}-${month}-${day}`;
}

/** Inclusive local days from (today − N + 1) through today. */
export function periodDayKeys(days: number, today = new Date()): string[] {
  const start = new Date(today);
  start.setHours(12, 0, 0, 0);
  start.setDate(start.getDate() - (days - 1));
  const keys: string[] = [];
  for (let i = 0; i < days; i++) {
    const d = new Date(start);
    d.setDate(start.getDate() + i);
    keys.push(localDayKey(d));
  }
  return keys;
}

function countsByDate(points: PingSeriesPoint[]): Map<string, number> {
  const counts = new Map<string, number>();
  for (const point of points) {
    counts.set(point.date, (counts.get(point.date) ?? 0) + point.count);
  }
  return counts;
}

function fillDaily(points: PingSeriesPoint[], days: number, today: Date): PingSeriesPoint[] {
  const counts = countsByDate(points);
  return periodDayKeys(days, today).map((date) => ({ date, count: counts.get(date) ?? 0 }));
}

function fillThroughToday(points: PingSeriesPoint[], today: Date): PingSeriesPoint[] {
  const endKey = localDayKey(today);
  const startKey = points.length
    ? [...points].sort((a, b) => a.date.localeCompare(b.date))[0].date
    : endKey;
  const start = new Date(`${startKey}T12:00:00Z`);
  const end = new Date(`${endKey}T12:00:00Z`);
  if (end.getTime() < start.getTime()) {
    return rollupDaily(points, grainForSpan(spanDays(points)));
  }
  const daily: PingSeriesPoint[] = [];
  for (let t = start.getTime(); t <= end.getTime(); t += 86_400_000) {
    daily.push({ date: new Date(t).toISOString().slice(0, 10), count: 0 });
  }
  const counts = countsByDate(points);
  for (const point of daily) {
    point.count = counts.get(point.date) ?? 0;
  }
  return rollupDaily(daily, grainForSpan(daily.length));
}

/** At most `maxLabels` dates, evenly spaced, including the first and last. Chart points stay daily. */
export function axisDateTicks(dates: string[], maxLabels = 8): string[] {
  if (dates.length <= maxLabels) return dates;
  const last = dates.length - 1;
  const out: string[] = [];
  for (let i = 0; i < maxLabels; i++) {
    const idx = Math.round((i * last) / (maxLabels - 1));
    const date = dates[idx];
    if (out[out.length - 1] !== date) out.push(date);
  }
  return out;
}

export function chartSeriesForPreset(
  points: PingSeriesPoint[],
  preset: '7d' | '30d' | 'all',
  today = new Date(),
): PingSeriesPoint[] {
  if (preset === '7d') return fillDaily(points, 7, today);
  if (preset === '30d') return fillDaily(points, 30, today);
  return fillThroughToday(points, today);
}
