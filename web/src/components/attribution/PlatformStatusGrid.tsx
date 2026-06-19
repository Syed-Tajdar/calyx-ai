import type { PlatformStatusSummary } from '@/lib/api';

interface PlatformStatusGridProps {
  platforms: PlatformStatusSummary[];
}

const PLATFORM_LABELS: Record<string, string> = {
  meta: 'Meta',
  google: 'Google',
  tiktok: 'TikTok',
  mock: 'Mock',
};

export function PlatformStatusGrid({ platforms }: PlatformStatusGridProps): JSX.Element {
  if (platforms.length === 0) {
    return (
      <div className="calyx-card p-4 text-sm text-calyx-muted">
        No attribution fires yet. Mock bridge emits conversions when pings have revenue.
      </div>
    );
  }

  return (
    <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
      {platforms.map((p) => {
        const total = p.fired_count + p.failed_count;
        const successRate = total > 0 ? Math.round((p.fired_count / total) * 100) : 0;
        return (
          <div key={p.platform} className="calyx-card p-4">
            <p className="text-sm font-semibold text-calyx-text">
              {PLATFORM_LABELS[p.platform] ?? p.platform}
            </p>
            <p className="mt-2 text-2xl font-bold text-emerald-400">{successRate}%</p>
            <p className="text-[10px] uppercase tracking-wide text-calyx-muted">Success rate</p>
            <div className="mt-3 flex justify-between text-xs text-calyx-muted">
              <span className="text-emerald-400">{p.fired_count} fired</span>
              <span className="text-red-400">{p.failed_count} failed</span>
            </div>
          </div>
        );
      })}
    </div>
  );
}
