'use client';

import { useEffect, useMemo, useState } from 'react';
import { useRouter } from 'next/navigation';
import { AppHeader } from '@/components/dashboard/AppHeader';
import { BottomNav } from '@/components/dashboard/BottomNav';
import { MetricCard } from '@/components/dashboard/MetricCard';
import { PerformanceTable } from '@/components/dashboard/PerformanceTable';
import { PingChart } from '@/components/dashboard/PingChart';
import { PingsTable } from '@/components/dashboard/PingsTable';
import { PlatformChart } from '@/components/dashboard/PlatformChart';
import { SectionHeader } from '@/components/dashboard/SectionHeader';
import { DesignBackground } from '@/components/ui/DesignBackground';
import { Logo } from '@/components/ui/Logo';
import { fetchPings, PingRecord } from '@/lib/api';
import { computeDashboardStats } from '@/lib/dashboard-stats';

function getStoredToken(): string | null {
  if (typeof window === 'undefined') return null;
  return localStorage.getItem('callloom_token') ?? sessionStorage.getItem('callloom_token');
}

export default function DashboardPage(): JSX.Element {
  const router = useRouter();
  const [pings, setPings] = useState<PingRecord[]>([]);
  const [total, setTotal] = useState(0);
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    const token = getStoredToken();
    if (!token) {
      router.replace('/login');
      return;
    }

    fetchPings(token)
      .then((res) => {
        setPings(res.data);
        setTotal(res.total);
      })
      .catch(() => setError('Failed to load pings'))
      .finally(() => setLoading(false));
  }, [router]);

  const stats = useMemo(() => computeDashboardStats(pings, total), [pings, total]);

  function logout(): void {
    localStorage.removeItem('callloom_token');
    sessionStorage.removeItem('callloom_token');
    router.push('/login');
  }

  return (
    <div className="relative min-h-screen bg-loom-bg pb-20">
      <DesignBackground />

      {/* Top brand bar */}
      <div className="relative z-10 flex items-center justify-between px-4 py-4 sm:px-6">
        <Logo variant="compact" />
        <button
          type="button"
          onClick={logout}
          className="rounded-full border border-loom-border px-4 py-1.5 text-xs text-loom-muted transition hover:border-loom-accent hover:text-white"
        >
          Log out
        </button>
      </div>

      {/* Dashboard shell — matches phone mockup from design */}
      <div className="relative z-10 px-4 sm:px-6">
        <div className="loom-dashboard-shell">
          <AppHeader />

          {loading && (
            <div className="flex h-64 items-center justify-center">
              <div className="h-10 w-10 animate-spin rounded-full border-2 border-loom-border border-t-loom-accent" />
            </div>
          )}

          {error && (
            <div className="rounded-xl border border-red-500/30 bg-red-500/10 px-4 py-3 text-sm text-red-300">
              {error}
            </div>
          )}

          {!loading && !error && (
            <div className="space-y-5">
              <SectionHeader title="Inbound Operations" />

              {/* 2×2 metric cards — design mockup layout */}
              <section className="grid grid-cols-2 gap-3">
                <MetricCard
                  label="Total Inbound Pings"
                  value={stats.totalPings.toLocaleString()}
                  trend="12.5%"
                  trendUp
                  icon="calls"
                />
                <MetricCard
                  label="Unique Pings"
                  value={stats.uniquePings.toLocaleString()}
                  trend="8.2%"
                  trendUp
                  icon="answered"
                />
                <MetricCard
                  label="Duplicate Pings"
                  value={stats.duplicatePings.toLocaleString()}
                  trend={stats.duplicatePings > 0 ? '3.1%' : '0%'}
                  trendUp={stats.duplicatePings === 0}
                  icon="missed"
                />
                <MetricCard
                  label="Unique Rate"
                  value={`${stats.uniqueRate.toFixed(1)}%`}
                  trend={`$${stats.totalRevenue.toFixed(0)} rev`}
                  trendUp
                  icon="rate"
                />
              </section>

              <PingChart data={stats.timeline} />

              <section className="grid gap-4 md:grid-cols-2">
                <PlatformChart data={stats.platforms} />
                <PerformanceTable publishers={stats.publishers} />
              </section>

              <PingsTable pings={pings} />
            </div>
          )}
        </div>
      </div>

      <BottomNav active="dashboard" onLogout={logout} />
    </div>
  );
}
