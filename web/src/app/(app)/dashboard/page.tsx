'use client';

import { useEffect, useMemo, useState } from 'react';
import { useRouter } from 'next/navigation';
import { Brain, DollarSign, Phone, Star, Users } from 'lucide-react';
import { MetricCard } from '@/components/dashboard/MetricCard';
import { PerformanceTable } from '@/components/dashboard/PerformanceTable';
import { PingChart } from '@/components/dashboard/PingChart';
import { PingsTable } from '@/components/dashboard/PingsTable';
import { PlatformChart } from '@/components/dashboard/PlatformChart';
import { SubsectionHeader } from '@/components/dashboard/SubsectionHeader';
import { ScoresTable } from '@/components/scoring/ScoresTable';
import { IntelligenceHubCards } from '@/components/intelligence/IntelligenceHubCards';
import { ComplianceRiskRing } from '@/components/calyx/ComplianceRiskRing';
import { ErrorBanner, PageLoader } from '@/components/ui/PageStatus';
import { PageHeader } from '@/components/ui/PageHeader';
import { Badge } from '@/components/ui/badge';
import {
  ApiError,
  ExposureSummary,
  fetchAnalyticsMetrics,
  fetchComplianceExposure,
  fetchScores,
  MetricsResult,
  ScoreRecord,
} from '@/lib/api';
import { getToken } from '@/lib/auth';
import { computeDashboardStats } from '@/lib/dashboard-stats';
import { FilterBar } from '@/components/ui/FilterBar';
import { dateFilterToQuery, useDateFilter } from '@/lib/date-filter';
import { useHybridPingFeed } from '@/lib/use-hybrid-ping-feed';
import { LiveActivityStrip } from '@/components/dashboard/LiveActivityStrip';

export default function DashboardPage(): JSX.Element {
  const router = useRouter();
  const { startDate, platform, preset } = useDateFilter();
  const filters = useMemo(() => dateFilterToQuery(startDate, platform), [startDate, platform]);
  const [metrics, setMetrics] = useState<MetricsResult | null>(null);
  const [exposure, setExposure] = useState<ExposureSummary | null>(null);
  const [scores, setScores] = useState<ScoreRecord[]>([]);
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(true);

  const { pings, newRowKeys, isLive, isPollingFallback, connected, liveEvents } = useHybridPingFeed({
    limit: 50,
    offset: 0,
    filters,
    pollMs: 8000,
  });

  useEffect(() => {
    const token = getToken();
    if (!token) return;

    setLoading(true);
    Promise.all([
      fetchAnalyticsMetrics(token, filters),
      fetchComplianceExposure(token),
      fetchScores(token, { limit: 5, offset: 0 }),
    ])
      .then(([metricsRes, exposureRes, scoresRes]) => {
        setMetrics(metricsRes);
        setExposure(exposureRes);
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
  }, [router, filters.from, filters.platform, preset]);

  const platformOptions = useMemo(
    () => Array.from(new Set(pings.map((p) => p.platform).filter(Boolean))),
    [pings],
  );

  const stats = useMemo(
    () => computeDashboardStats(pings, pings.length),
    [pings],
  );

  const avgScore =
    scores.length > 0
      ? (scores.reduce((sum, s) => sum + s.overall_score, 0) / scores.length).toFixed(1)
      : '—';

  return (
    <>
      <PageHeader
        title="Operations Overview"
        description="Real-time ping pipeline, financial metrics, and intelligence at a glance."
        action={
          isLive ? (
            <Badge variant="teal" className="gap-1.5">
              <span className="h-1.5 w-1.5 animate-pulse rounded-full bg-calyx-teal" />
              Live
            </Badge>
          ) : isPollingFallback ? (
            <Badge variant="secondary" className="gap-1.5 text-calyx-muted">
              Polling
            </Badge>
          ) : undefined
        }
      />

      <FilterBar showPlatform platforms={platformOptions} />

      {loading && <PageLoader />}
      {error && <ErrorBanner message={error} />}

      {!loading && !error && (
        <div className="space-y-8">
          <section className="grid grid-cols-2 gap-3 lg:grid-cols-4 lg:gap-4">
            <MetricCard
              label="Total Revenue"
              value={`$${(metrics?.total_revenue ?? 0).toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 })}`}
              trend={`RPC $${(metrics?.rpc ?? 0).toFixed(2)}`}
              trendUp
              icon={DollarSign}
              sparklineData={stats.timeline.map((t) => t.count)}
            />
            <MetricCard
              label="Total Pings"
              value={stats.totalPings.toLocaleString()}
              trend={`${stats.uniqueRate.toFixed(1)}% unique`}
              trendUp={stats.uniqueRate >= 50}
              icon={Phone}
              sparklineData={stats.timeline.map((t) => t.count)}
            />
            <MetricCard
              label="Avg Call Score"
              value={avgScore}
              trend={scores.length > 0 ? `${scores.length} scored` : 'No scores yet'}
              trendUp={Number(avgScore) >= 70}
              icon={Star}
            />
            <div className="calyx-metric-card flex flex-col items-center justify-center">
              <p className="mb-2 self-start text-[10px] font-semibold uppercase tracking-wide text-calyx-muted">
                Compliance Exposure
              </p>
              <ComplianceRiskRing
                score={Math.round(exposure?.avg_risk_score ?? 0)}
                exposureUsd={exposure?.total_exposure_usd ?? 0}
                size="sm"
              />
            </div>
          </section>

          <section className="grid gap-4 lg:grid-cols-2">
            <PingChart data={stats.timeline} />
            <PlatformChart data={stats.platforms} />
          </section>

          {connected && <LiveActivityStrip events={liveEvents} />}

          <section>
            <SubsectionHeader title="Calyx Intelligence" icon={Brain} />
            <IntelligenceHubCards />
          </section>

          <section className="grid gap-4 lg:grid-cols-2">
            <div>
              <SubsectionHeader title="Top Scores" icon={Star} />
              <ScoresTable scores={scores} compact />
            </div>
            <div>
              <SubsectionHeader title="Publishers" icon={Users} />
              <PerformanceTable publishers={stats.publishers} />
            </div>
          </section>

          <section>
            <SubsectionHeader title="Recent Pings" icon={Phone} />
            <PingsTable pings={pings} linkable highlightKeys={newRowKeys} />
          </section>
        </div>
      )}
    </>
  );
}
