'use client';

import Link from 'next/link';
import { useEffect, useMemo, useState } from 'react';
import { useRouter } from 'next/navigation';
import { Map } from 'lucide-react';
import { PageHeader } from '@/components/ui/PageHeader';
import { FilterBar } from '@/components/ui/FilterBar';
import { AnalyticsMetricsGrid } from '@/components/reports/AnalyticsMetricsGrid';
import { PnLSummary } from '@/components/reports/PnLSummary';
import { ErrorBanner, PageLoader } from '@/components/ui/PageStatus';
import { Button } from '@/components/ui/button';
import {
  ApiError,
  fetchAnalyticsMetrics,
  fetchPnL,
  MetricsResult,
  PnLResult,
} from '@/lib/api';
import { getToken } from '@/lib/auth';
import { dateFilterToQuery, useDateFilter } from '@/lib/date-filter';

export default function ReportsPage(): JSX.Element {
  const router = useRouter();
  const { startDate, platform, preset, showDemo } = useDateFilter();
  const filters = useMemo(
    () => dateFilterToQuery(startDate, platform, showDemo),
    [startDate, platform, showDemo],
  );
  const [metrics, setMetrics] = useState<MetricsResult | null>(null);
  const [pnl, setPnl] = useState<PnLResult | null>(null);
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    const token = getToken();
    if (!token) return;

    setLoading(true);
    Promise.all([fetchAnalyticsMetrics(token, filters), fetchPnL(token, filters)])
      .then(([metricsRes, pnlRes]) => {
        setMetrics(metricsRes);
        setPnl(pnlRes);
      })
      .catch((err: unknown) => {
        if (err instanceof ApiError && err.status === 401) {
          router.replace('/login');
          return;
        }
        setError('Failed to load reports');
      })
      .finally(() => setLoading(false));
  }, [router, filters.from, filters.platform, filters.show_demo, preset]);

  return (
    <div className="space-y-8">
      <PageHeader
        title="Financial Analytics"
        description="CPA, RPC, ROI, and profit & loss summary."
        action={
          <Button variant="outline" size="sm" asChild className="gap-2">
            <Link href="/coverage">
              <Map className="h-4 w-4" />
              Coverage
            </Link>
          </Button>
        }
      />

      <FilterBar />

      {loading && <PageLoader />}
      {error && <ErrorBanner message={error} />}

      {!loading && !error && (
        <div className="grid gap-6 lg:grid-cols-2">
          <AnalyticsMetricsGrid metrics={metrics} />
          <PnLSummary pnl={pnl} />
        </div>
      )}
    </div>
  );
}
