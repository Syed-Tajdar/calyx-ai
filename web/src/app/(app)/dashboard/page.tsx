'use client';

import { useEffect, useMemo, useState } from 'react';
import { useRouter } from 'next/navigation';
import { Brain, DollarSign, Phone, Shield, Star, TrendingUp, Users } from 'lucide-react';
import { MetricCard, MetricCardFrame } from '@/components/dashboard/MetricCard';
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
  fetchPings,
  fetchPlatformMix,
  fetchScores,
  MetricsResult,
  PingRecord,
  PlatformMixRow,
  ScoreRecord,
} from '@/lib/api';
import { getToken } from '@/lib/auth';
import { computeDashboardStats } from '@/lib/dashboard-stats';
import { DEMO_PUBLISHERS } from '@/lib/demo-publishers';
import { SHOW_MOCK_PANELS } from '@/lib/v1-features';
import { FilterBar } from '@/components/ui/FilterBar';
import { dateFilterToQuery, useDateFilter } from '@/lib/date-filter';
import { formatPlatformLabel } from '@/lib/platform-options';
import {
  buildFilledPingTimeline,
  chartFiltersForLastDays,
} from '@/lib/ping-timeline';
import { useHybridPingFeed } from '@/lib/use-hybrid-ping-feed';
import { usePlatformOptions } from '@/lib/use-platform-options';
import { LiveActivityStrip } from '@/components/dashboard/LiveActivityStrip';

export default function DashboardPage(): JSX.Element {
  const router = useRouter();
  const { startDate, platform, preset, showDemo } = useDateFilter();
  const filters = useMemo(
    () => dateFilterToQuery(startDate, platform, showDemo),
    [startDate, platform, showDemo],
  );
  const [metrics, setMetrics] = useState<MetricsResult | null>(null);
  const [platformMix, setPlatformMix] = useState<PlatformMixRow[]>([]);
  const [exposure, setExposure] = useState<ExposureSummary | null>(null);
  const [scores, setScores] = useState<ScoreRecord[]>([]);
  const [chartPings, setChartPings] = useState<PingRecord[]>([]);
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(true);

  const chartFilters = useMemo(
    () => chartFiltersForLastDays(7, platform || undefined, showDemo),
    [platform, showDemo],
  );

  const { pings, newRowKeys, isPollingFallback, connected, liveEvents } = useHybridPingFeed({
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
      fetchPlatformMix(token, filters),
      fetchComplianceExposure(token, { show_demo: showDemo || undefined }),
      fetchScores(token, { limit: 5, offset: 0, show_demo: showDemo || undefined }),
      fetchPings(token, 100, 0, chartFilters),
    ])
      .then(([metricsRes, platformMixRes, exposureRes, scoresRes, chartPingsRes]) => {
        setMetrics(metricsRes);
        setPlatformMix(platformMixRes);
        setExposure(exposureRes);
        if (scoresRes && 'data' in scoresRes) {
          setScores(scoresRes.data);
        }
        setChartPings(chartPingsRes.data);
      })
      .catch((err: unknown) => {
        if (err instanceof ApiError && err.status === 401) {
          router.replace('/login');
          return;
        }
        setError('Failed to load dashboard data');
      })
      .finally(() => setLoading(false));
  }, [
    router,
    filters.from,
    filters.platform,
    filters.show_demo,
    preset,
    chartFilters.from,
    chartFilters.platform,
    chartFilters.show_demo,
    showDemo,
  ]);

  const platformOptions = usePlatformOptions();

  const stats = useMemo(
    () => computeDashboardStats(pings, pings.length),
    [pings],
  );

  const pingTimeline = useMemo(
    () => buildFilledPingTimeline(chartPings, 7),
    [chartPings],
  );

  const chartPlatformData = useMemo(
    () =>
      platformMix
        .filter((row) => row.platform !== 'mock')
        .map((row) => ({
          name: formatPlatformLabel(row.platform),
          slug: row.platform,
          value: row.count,
          percent: row.percent,
        })),
    [platformMix],
  );

  const avgScore =
    scores.length > 0
      ? (scores.reduce((sum, s) => sum + s.overall_score, 0) / scores.length).toFixed(1)
      : '—';

  const scoreSparkline = useMemo(
    () => scores.map((s) => s.overall_score),
    [scores],
  );

  return (
    <>
      <PageHeader
        title="Operations Overview"
        description="Real-time ping pipeline, financial metrics, and intelligence at a glance."
        action={
          connected ? (
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
          <section
            className={`grid grid-cols-2 gap-3 lg:gap-4 ${
              SHOW_MOCK_PANELS ? 'lg:grid-cols-4' : 'lg:grid-cols-2'
            }`}
          >
            <MetricCard
              label="Total Revenue"
              value={`$${(metrics?.total_revenue ?? 0).toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 })}`}
              trend={`RPC $${(metrics?.rpc ?? 0).toFixed(2)}`}
              icon={DollarSign}
              iconPulseDelayMs={0}
              sparklineData={stats.timeline.map((t) => t.count)}
            />
            <MetricCard
              label="Total Pings"
              value={(metrics?.total_pings ?? 0).toLocaleString()}
              trend={`${stats.uniqueRate.toFixed(1)}% unique`}
              icon={Phone}
              iconPulseDelayMs={400}
              sparklineData={pingTimeline.map((t) => t.count)}
            />
            {/* Scoring and compliance are mock-only pipelines — out of v1 scope. */}
            {SHOW_MOCK_PANELS && (
              <>
                <MetricCard
                  label="Avg Call Score"
                  value={avgScore}
                  trend={scores.length > 0 ? `${scores.length} scored` : 'No scores yet'}
                  icon={TrendingUp}
                  iconPulseDelayMs={800}
                  sparklineData={scoreSparkline.length > 1 ? scoreSparkline : undefined}
                />
                <MetricCardFrame>
                  <div className="flex items-start justify-between">
                    <p className="text-[10px] font-semibold uppercase leading-tight tracking-wide text-calyx-muted">
                      Compliance Exposure
                    </p>
                    <span className="calyx-metric-icon-pulse">
                      <Shield className="size-[18px] text-calyx-teal" strokeWidth={2} aria-hidden />
                    </span>
                  </div>
                  <div className="mt-2">
                    <ComplianceRiskRing
                      score={Math.round(exposure?.avg_risk_score ?? 0)}
                      exposureUsd={exposure?.total_exposure_usd ?? 0}
                      size="sm"
                      align="start"
                    />
                  </div>
                </MetricCardFrame>
              </>
            )}
          </section>

          <section className="grid gap-4 lg:grid-cols-2">
            <PingChart data={pingTimeline} />
            <PlatformChart data={chartPlatformData} />
          </section>

          {connected && <LiveActivityStrip events={liveEvents} />}

          {/*
            Intelligence entry points (Ask AI / TCPA / Attribution), Top Scores and the Publishers
            table all render mock data. The Publishers table is the worst offender: DEMO_PUBLISHERS
            is a hardcoded fixture that ignores the demo toggle, so it showed invented revenue even
            while the rest of the dashboard said "No pings yet".
          */}
          {SHOW_MOCK_PANELS && (
            <>
              <section>
                <SubsectionHeader title="Calyx Intelligence" icon={Brain} />
                <IntelligenceHubCards />
              </section>

              <section className="grid gap-4 lg:grid-cols-2 lg:items-stretch">
                <div className="flex min-h-[320px] flex-col">
                  <SubsectionHeader title="Top Scores" icon={Star} />
                  <ScoresTable scores={scores} compact />
                </div>
                <div className="flex min-h-[320px] flex-col">
                  <SubsectionHeader title="Publishers" icon={Users} />
                  <PerformanceTable publishers={DEMO_PUBLISHERS} />
                </div>
              </section>
            </>
          )}

          <section>
            <SubsectionHeader title="Recent Pings" icon={Phone} />
            <PingsTable pings={pings} linkable highlightKeys={newRowKeys} />
          </section>
        </div>
      )}
    </>
  );
}
