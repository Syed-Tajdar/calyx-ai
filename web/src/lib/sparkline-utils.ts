export interface SparklinePeriodDelta {
  pct: number;
  up: boolean;
}

export function sparklinePeriodDelta(data: number[]): SparklinePeriodDelta | null {
  if (data.length < 2) return null;
  const first = data[0];
  const last = data[data.length - 1];
  if (first === 0) return null;
  const raw = ((last - first) / Math.abs(first)) * 100;
  return {
    pct: Math.abs(raw),
    up: raw >= 0,
  };
}
