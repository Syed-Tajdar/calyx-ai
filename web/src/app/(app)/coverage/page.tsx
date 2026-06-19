'use client';

import Link from 'next/link';
import { useEffect, useMemo, useState } from 'react';
import { useRouter } from 'next/navigation';
import { ArrowLeft } from 'lucide-react';
import { ErrorBanner, PageLoader } from '@/components/ui/PageStatus';
import { PageHeader } from '@/components/ui/PageHeader';
import { Button } from '@/components/ui/button';
import { ApiError, fetchCoverageHeatmap, HeatmapStateRow } from '@/lib/api';
import { getToken } from '@/lib/auth';

const US_STATES_MAX = 60;

function intensityClass(count: number, max: number): string {
  if (max === 0) return 'bg-calyx-surface';
  const ratio = count / max;
  if (ratio > 0.75) return 'bg-calyx-teal';
  if (ratio > 0.5) return 'bg-calyx-teal/70';
  if (ratio > 0.25) return 'bg-calyx-teal/40';
  if (ratio > 0) return 'bg-calyx-teal/20';
  return 'bg-calyx-surface';
}

export default function CoveragePage(): JSX.Element {
  const router = useRouter();
  const [rows, setRows] = useState<HeatmapStateRow[]>([]);
  const [totalPings, setTotalPings] = useState(0);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  useEffect(() => {
    const token = getToken();
    if (!token) return;

    fetchCoverageHeatmap(token)
      .then((res) => {
        setRows(res.data);
        setTotalPings(res.total_pings);
      })
      .catch((err: unknown) => {
        if (err instanceof ApiError && err.status === 401) {
          router.replace('/login');
          return;
        }
        setError('Failed to load coverage heatmap');
      })
      .finally(() => setLoading(false));
  }, [router]);

  const maxCount = useMemo(
    () => Math.max(...rows.map((r) => r.ping_count), 1),
    [rows],
  );

  return (
    <div className="space-y-6">
      <PageHeader
        title="Coverage Intelligence"
        description="Geographic ping demand heatmap by US state."
        action={
          <Button variant="outline" size="sm" asChild className="gap-2">
            <Link href="/reports">
              <ArrowLeft className="h-4 w-4" />
              Reports
            </Link>
          </Button>
        }
      />

      {loading && <PageLoader />}
      {error && <ErrorBanner message={error} />}

      {!loading && !error && (
        <>
          <p className="text-sm text-calyx-muted">
            {totalPings.toLocaleString()} pings across {rows.length} states
          </p>
          {rows.length === 0 ? (
            <div className="calyx-card p-8 text-center text-sm text-calyx-muted">
              No geographic data yet. Run the demo seed script or send pings with state codes.
            </div>
          ) : (
            <div className="calyx-card p-4">
              <div className="grid grid-cols-4 gap-2 xs:grid-cols-5 sm:grid-cols-8 md:grid-cols-10">
                {rows.slice(0, US_STATES_MAX).map((row) => (
                  <div
                    key={row.state}
                    className={`rounded-md p-2 text-center transition ${intensityClass(row.ping_count, maxCount)}`}
                    title={`${row.state}: ${row.ping_count} pings, $${row.revenue.toFixed(2)}`}
                  >
                    <p className="text-xs font-bold text-calyx-text">{row.state}</p>
                    <p className="text-[10px] text-calyx-muted">{row.ping_count}</p>
                  </div>
                ))}
              </div>
              <div className="mt-4 flex items-center gap-2 text-xs text-calyx-muted">
                <span>Low</span>
                <div className="h-2 flex-1 rounded bg-gradient-to-r from-calyx-surface via-calyx-teal/40 to-calyx-teal" />
                <span>High</span>
              </div>
            </div>
          )}
        </>
      )}
    </div>
  );
}
