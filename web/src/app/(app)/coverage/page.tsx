'use client';

import Link from 'next/link';
import { useEffect, useMemo, useState } from 'react';
import { useRouter } from 'next/navigation';
import { ArrowLeft } from 'lucide-react';
import { CoverageMapDynamic } from '@/components/coverage/CoverageMapDynamic';
import type { CountryFilter } from '@/components/coverage/CoverageMap';
import { StateDetailPanel } from '@/components/coverage/StateDetailPanel';
import { StateHeatmapChart } from '@/components/coverage/StateHeatmapChart';
import { ErrorBanner, PageLoader } from '@/components/ui/PageStatus';
import { PageHeader } from '@/components/ui/PageHeader';
import { Button } from '@/components/ui/button';
import { ApiError, fetchCoverageHeatmap, HeatmapStateRow } from '@/lib/api';
import { getToken } from '@/lib/auth';
import { countryForRegion } from '@/lib/na-regions';

export default function CoveragePage(): JSX.Element {
  const router = useRouter();
  const [rows, setRows] = useState<HeatmapStateRow[]>([]);
  const [totalPings, setTotalPings] = useState(0);
  const [usRegions, setUsRegions] = useState(0);
  const [caRegions, setCaRegions] = useState(0);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [selectedState, setSelectedState] = useState<string | null>(null);
  const [selectionCleared, setSelectionCleared] = useState(false);
  const [countryFilter, setCountryFilter] = useState<CountryFilter>('all');

  useEffect(() => {
    const token = getToken();
    if (!token) return;

    fetchCoverageHeatmap(token)
      .then((res) => {
        setRows(res.data);
        setTotalPings(res.total_pings);
        setUsRegions(res.us_regions ?? 0);
        setCaRegions(res.ca_regions ?? 0);
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

  const visibleRows = useMemo(() => {
    if (countryFilter === 'all') return rows;
    return rows.filter((r) => (r.country || countryForRegion(r.state)) === countryFilter);
  }, [rows, countryFilter]);

  const visibleTotal = useMemo(
    () => visibleRows.reduce((sum, r) => sum + r.ping_count, 0),
    [visibleRows],
  );

  const sortedRows = useMemo(
    () => [...visibleRows].sort((a, b) => b.ping_count - a.ping_count),
    [visibleRows],
  );

  const selectedRow = useMemo(() => {
    if (selectionCleared && !selectedState) return null;
    if (selectedState) {
      return visibleRows.find((r) => r.state.trim().toUpperCase() === selectedState) ?? null;
    }
    return sortedRows[0] ?? null;
  }, [visibleRows, selectedState, selectionCleared, sortedRows]);

  const mapSelectedCode = useMemo(() => {
    if (selectedState) return selectedState;
    if (selectionCleared) return null;
    return sortedRows[0]?.state.trim().toUpperCase() ?? null;
  }, [selectedState, selectionCleared, sortedRows]);

  const selectedRank = useMemo(() => {
    if (!selectedRow) return null;
    const index = sortedRows.findIndex((r) => r.state === selectedRow.state);
    return index >= 0 ? index + 1 : null;
  }, [selectedRow, sortedRows]);

  const handleClearSelection = (): void => {
    setSelectedState(null);
    setSelectionCleared(true);
  };

  const handleSelectState = (code: string | null): void => {
    setSelectedState(code);
    setSelectionCleared(false);
  };

  const handleCountryFilter = (filter: CountryFilter): void => {
    setCountryFilter(filter);
    setSelectedState(null);
    setSelectionCleared(true);
  };

  const regionCount = usRegions + caRegions;

  return (
    <div className="space-y-8">
      <PageHeader
        title="Coverage Intelligence"
        description="Geographic ping demand heatmap — United States and Canada."
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
          <div className="space-y-0.5">
            <p className="text-sm text-calyx-muted">
              {totalPings.toLocaleString()} pings across {regionCount} regions · US + Canada
            </p>
            <p className="text-xs text-calyx-dim">
              {usRegions} US · {caRegions} CA
              {countryFilter !== 'all' && (
                <span>
                  {' '}
                  · filter showing {visibleTotal.toLocaleString()} pings / {visibleRows.length}{' '}
                  regions
                </span>
              )}
            </p>
          </div>
          {rows.length === 0 ? (
            <div className="calyx-card p-8 text-center text-sm text-calyx-muted">
              No geographic data yet. Connect platforms and send pings with US or Canadian region
              codes.
            </div>
          ) : (
            <>
              <div className="grid items-start gap-4 lg:grid-cols-[1fr_220px]">
                <CoverageMapDynamic
                  data={rows}
                  selectedState={mapSelectedCode}
                  onSelectState={handleSelectState}
                  totalPings={totalPings}
                  countryFilter={countryFilter}
                  onCountryFilterChange={handleCountryFilter}
                />
                <StateDetailPanel
                  selected={selectedRow}
                  rank={selectedRank}
                  totalPings={visibleTotal}
                  onClear={handleClearSelection}
                />
              </div>
              <StateHeatmapChart data={visibleRows} />
            </>
          )}
        </>
      )}
    </div>
  );
}
