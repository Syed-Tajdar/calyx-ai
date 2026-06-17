'use client';

import { useEffect, useMemo, useState } from 'react';
import { useRouter } from 'next/navigation';
import { AppHeader } from '@/components/dashboard/AppHeader';
import { MetricCard } from '@/components/dashboard/MetricCard';
import { PerformanceTable } from '@/components/dashboard/PerformanceTable';
import { PingChart } from '@/components/dashboard/PingChart';
import { PingsTable } from '@/components/dashboard/PingsTable';
import { PlatformChart } from '@/components/dashboard/PlatformChart';
import { SectionHeader } from '@/components/dashboard/SectionHeader';
import { ScoresTable } from '@/components/scoring/ScoresTable';
import { IntelligenceHubCards } from '@/components/intelligence/IntelligenceHubCards';
import { ErrorBanner, PageLoader } from '@/components/ui/PageStatus';
import {
  ApiError,
  fetchAnalyticsMetrics,
  fetchPings,
  fetchScores,
  MetricsResult,
  PingRecord,
  ScoreRecord,
} from '@/lib/api';
import { getToken } from '@/lib/auth';
import { computeDashboardStats } from '@/lib/dashboard-stats';

export default function DashboardPage(): JSX.Element {
  const router = useRouter();
  const [pings, setPings] = useState<PingRecord[]>([]);
  const [total, setTotal] = useState(0);
  const [metrics, setMetrics] = useState<MetricsResult | null>(null);
  const [scores, setScores] = useState<ScoreRecord[]>([]);
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    const token = getToken();
    if (!token) return;

    Promise.all([
      fetchPings(token, 50, 0),
      fetchAnalyticsMetrics(token),
      fetchScores(token, { limit: 5, offset: 0 }),
    ])
      .then(([pingRes, metricsRes, scoresRes]) => {
        setPings(pingRes.data);
        setTotal(pingRes.total);
        setMetrics(metricsRes);
        if (scoresRes && 'data' in scoresRes) {
          setScores(scoresRes.data);
        }
      })
      .catch((err: unknown) => {
        if (err instanceof ApiError && err.status === 401) {
          router.replace('/login');
          return;
        }
        setError('Failed to load dashboard data');
      })
      .finally(() => setLoading(false));
  }, [router]);

  const stats = useMemo(() => computeDashboardStats(pings, total), [pings, total]);

  return (
    <>
      <AppHeader />

      {loading && <PageLoader />}
      {error && <ErrorBanner message={error} />}

      {!loading && !error && (
        <div className="space-y-5">
          <SectionHeader title="Inbound Operations" />

          <SectionHeader title="Intelligence" />
          <IntelligenceHubCards />

          <section className="grid grid-cols-2 gap-3">
            <MetricCard
              label="Total Inbound Pings"
              value={stats.totalPings.toLocaleString()}
              trend={`RPC $${(metrics?.rpc ?? 0).toFixed(2)}`}
              trendUp
              icon="calls"
            />
            <MetricCard
              label="Unique Pings"
              value={stats.uniquePings.toLocaleString()}
              trend={`CPA $${(metrics?.cpa ?? 0).toFixed(2)}`}
              trendUp
              icon="answered"
            />
            <MetricCard
              label="Duplicate Pings"
              value={stats.duplicatePings.toLocaleString()}
              trend={stats.duplicatePings > 0 ? 'Review dupes' : '0%'}
              trendUp={stats.duplicatePings === 0}
              icon="missed"
            />
            <MetricCard
              label="Unique Rate"
              value={`${stats.uniqueRate.toFixed(1)}%`}
              trend={`ROI ${(metrics?.roi ?? 0).toFixed(1)}%`}
              trendUp={(metrics?.roi ?? 0) >= 0}
              icon="rate"
            />
          </section>

          <PingChart data={stats.timeline} />

          <section className="grid gap-4 md:grid-cols-2">
            <PlatformChart data={stats.platforms} />
            <PerformanceTable publishers={stats.publishers} />
          </section>

          <ScoresTable scores={scores} compact />

          <PingsTable pings={pings} linkable />
        </div>
      )}
    </>
  );
}
