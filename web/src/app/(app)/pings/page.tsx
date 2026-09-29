'use client';

import { Suspense, useEffect, useMemo, useState } from 'react';
import { useRouter, useSearchParams } from 'next/navigation';
import { Download } from 'lucide-react';
import { PingsTable } from '@/components/dashboard/PingsTable';
import { FilterBar } from '@/components/ui/FilterBar';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { ErrorBanner, PageLoader } from '@/components/ui/PageStatus';
import { PageHeader } from '@/components/ui/PageHeader';
import { toast } from 'sonner';
import { ApiError, downloadPingsExport } from '@/lib/api';
import { getToken } from '@/lib/auth';
import { dateFilterToQuery, useDateFilter } from '@/lib/date-filter';
import { useHybridPingFeed } from '@/lib/use-hybrid-ping-feed';
import { usePlatformOptions } from '@/lib/use-platform-options';

const PAGE_SIZE = 25;

export default function PingsPage(): JSX.Element {
  return (
    <Suspense fallback={<PageLoader />}>
      <PingsPageContent />
    </Suspense>
  );
}

function PingsPageContent(): JSX.Element {
  const router = useRouter();
  const searchParams = useSearchParams();
  const searchQuery = searchParams.get('search') ?? '';
  const { startDate, platform, showDemo, conversion } = useDateFilter();
  const [offset, setOffset] = useState(0);
  const [exporting, setExporting] = useState(false);

  const filters = useMemo(() => {
    const base = dateFilterToQuery(startDate, platform, showDemo, conversion);
    if (searchQuery) base.search = searchQuery;
    return base;
  }, [startDate, platform, searchQuery, showDemo, conversion]);

  // Reset pagination when a filter that changes the result set changes
  useEffect(() => {
    setOffset(0);
  }, [showDemo, conversion]);

  const { pings, total, displayTotal, loaded, loading, error, newRowKeys, isLive, isPollingFallback } =
    useHybridPingFeed({
      limit: PAGE_SIZE,
      offset,
      filters,
      pollMs: 8000,
    });

  const platforms = usePlatformOptions();

  const canPrev = offset > 0;
  const canNext = offset + PAGE_SIZE < total;

  async function handleExport(): Promise<void> {
    const token = getToken();
    if (!token) return;
    setExporting(true);
    try {
      await downloadPingsExport(token, filters);
      toast.success('Ping export downloaded');
    } catch (err: unknown) {
      if (err instanceof ApiError && err.status === 401) {
        router.replace('/login');
        return;
      }
      toast.error('Export failed. Try again.');
    } finally {
      setExporting(false);
    }
  }

  return (
    <div className="space-y-8">
      <PageHeader
        title="Ping Feed"
        description="Real-time RTB ping stream from connected platforms."
        action={
          <div className="flex items-center gap-2">
            {isLive ? (
              <Badge variant="teal" className="gap-1.5">
                <span className="h-1.5 w-1.5 animate-pulse rounded-full bg-calyx-teal" />
                Live
              </Badge>
            ) : isPollingFallback ? (
              <Badge variant="secondary">Polling</Badge>
            ) : null}
            <Badge variant="teal" className="text-sm">
              {loaded ? `${displayTotal.toLocaleString()} calls` : '…'}
            </Badge>
          </div>
        }
      />

      <div className="flex flex-wrap items-center justify-between gap-3">
        <FilterBar showPlatform platforms={platforms} showConversion />
        <Button
          type="button"
          variant="outline"
          size="sm"
          disabled={exporting}
          onClick={() => {
            void handleExport();
          }}
          className="gap-2"
        >
          <Download className="h-4 w-4" />
          {exporting ? 'Exporting…' : 'Export CSV'}
        </Button>
      </div>

      {searchQuery && (
        <p className="text-xs text-calyx-muted">
          Filtering by call ID: <span className="font-mono text-calyx-text">{searchQuery}</span>
        </p>
      )}

      {loading && <PageLoader />}
      {error && <ErrorBanner message={error} />}

      {!loading && !error && (
        <>
          <PingsTable pings={pings} linkable highlightKeys={newRowKeys} />
          <div className="flex items-center justify-between text-xs text-calyx-muted">
            <span>
              Showing {total === 0 ? 0 : offset + 1}–{Math.min(offset + PAGE_SIZE, total)} of{' '}
              {total}
            </span>
            <div className="flex gap-2">
              <Button
                type="button"
                variant="outline"
                size="sm"
                disabled={!canPrev}
                onClick={() => setOffset((o) => Math.max(0, o - PAGE_SIZE))}
              >
                Previous
              </Button>
              <Button
                type="button"
                variant="outline"
                size="sm"
                disabled={!canNext}
                onClick={() => setOffset((o) => o + PAGE_SIZE)}
              >
                Next
              </Button>
            </div>
          </div>
        </>
      )}
    </div>
  );
}
