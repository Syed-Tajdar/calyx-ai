'use client';

import Link from 'next/link';
import { useEffect, useMemo, useState } from 'react';
import { useRouter } from 'next/navigation';
import { ArrowLeft } from 'lucide-react';
import { CoverageMapDynamic } from '@/components/coverage/CoverageMapDynamic';
import { StateDetailPanel } from '@/components/coverage/StateDetailPanel';
import { StateHeatmapChart } from '@/components/coverage/StateHeatmapChart';
import { ErrorBanner, PageLoader } from '@/components/ui/PageStatus';
import { PageHeader } from '@/components/ui/PageHeader';
import { Button } from '@/components/ui/button';
import { ApiError, fetchCoverageHeatmap, HeatmapStateRow } from '@/lib/api';
import { getToken } from '@/lib/auth';

export default function CoveragePage(): JSX.Element {
  const router = useRouter();
  const [rows, setRows] = useState<HeatmapStateRow[]>([]);
  const [totalPings, setTotalPings] = useState(0);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [selectedState, setSelectedState] = useState<string | null>(null);
  const [selectionCleared, setSelectionCleared] = useState(false);

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

  const sortedRows = useMemo(
    () => [...rows].sort((a, b) => b.ping_count - a.ping_count),
    [rows],
  );

  const selectedRow = useMemo(() => {
    if (selectionCleared && !selectedState) return null;
    if (selectedState) {
      return rows.find((r) => r.state.trim().toUpperCase() === selectedState) ?? null;
    }
    return sortedRows[0] ?? null;
  }, [rows, selectedState, selectionCleared, sortedRows]);

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

  return (
    <div className="space-y-8">
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
              No geographic data yet. Connect platforms and send pings with state codes.
            </div>
          ) : (
            <>
              <div className="grid items-start gap-4 lg:grid-cols-[1fr_220px]">
                <CoverageMapDynamic
                  data={rows}
                  selectedState={mapSelectedCode}
                  onSelectState={handleSelectState}
                  totalPings={totalPings}
                />
                <StateDetailPanel
                  selected={selectedRow}
                  rank={selectedRank}
                  totalPings={totalPings}
                  onClear={handleClearSelection}
                />
              </div>
              <StateHeatmapChart data={rows} />
            </>
          )}
        </>
      )}
    </div>
  );
}
