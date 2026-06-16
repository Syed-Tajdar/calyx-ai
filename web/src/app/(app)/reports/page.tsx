'use client';

import Link from 'next/link';
import { useEffect, useState } from 'react';
import { useRouter } from 'next/navigation';
import { AppHeader } from '@/components/dashboard/AppHeader';
import { SectionHeader } from '@/components/dashboard/SectionHeader';
import { AnalyticsMetricsGrid } from '@/components/reports/AnalyticsMetricsGrid';
import { PnLSummary } from '@/components/reports/PnLSummary';
import { ErrorBanner, PageLoader } from '@/components/ui/PageStatus';
import {
  ApiError,
  fetchAnalyticsMetrics,
  fetchPnL,
  MetricsResult,
  PnLResult,
} from '@/lib/api';
import { getToken } from '@/lib/auth';

export default function ReportsPage(): JSX.Element {
  const router = useRouter();
  const [metrics, setMetrics] = useState<MetricsResult | null>(null);
  const [pnl, setPnl] = useState<PnLResult | null>(null);
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    const token = getToken();
    if (!token) return;

    Promise.all([fetchAnalyticsMetrics(token), fetchPnL(token)])
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
  }, [router]);

  return (
    <>
      <AppHeader />
      <div className="mb-4 flex items-center justify-between">
        <SectionHeader title="Financial Analytics" />
        <Link
          href="/coverage"
          className="text-xs text-loom-accent transition hover:text-loom-accent-hover"
        >
          View coverage →
        </Link>
      </div>

      {loading && <PageLoader />}
      {error && <ErrorBanner message={error} />}

      {!loading && !error && (
        <div className="space-y-6">
          <AnalyticsMetricsGrid metrics={metrics} />
          <PnLSummary pnl={pnl} />
        </div>
      )}
    </>
  );
}
