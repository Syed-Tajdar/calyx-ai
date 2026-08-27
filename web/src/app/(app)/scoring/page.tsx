'use client';

import { useEffect, useState } from 'react';
import Link from 'next/link';
import { useRouter } from 'next/navigation';
import { ScoresTable } from '@/components/scoring/ScoresTable';
import { ErrorBanner, PageLoader } from '@/components/ui/PageStatus';
import { PageHeader } from '@/components/ui/PageHeader';
import { FilterBar } from '@/components/ui/FilterBar';
import { Button } from '@/components/ui/button';
import { ApiError, fetchScores, ScoreRecord } from '@/lib/api';
import { getToken } from '@/lib/auth';
import { useDateFilter } from '@/lib/date-filter';

const PAGE_SIZE = 25;

export default function ScoringPage(): JSX.Element {
  const router = useRouter();
  const { showDemo } = useDateFilter();
  const [scores, setScores] = useState<ScoreRecord[]>([]);
  const [offset, setOffset] = useState(0);
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(true);
  const [hasMore, setHasMore] = useState(false);

  useEffect(() => {
    setOffset(0);
  }, [showDemo]);

  useEffect(() => {
    const token = getToken();
    if (!token) return;

    setLoading(true);
    fetchScores(token, {
      limit: PAGE_SIZE,
      offset,
      show_demo: showDemo || undefined,
    })
      .then((res) => {
        if (res && 'data' in res) {
          setScores(res.data);
          setHasMore(res.data.length === PAGE_SIZE);
        }
      })
      .catch((err: unknown) => {
        if (err instanceof ApiError && err.status === 401) {
          router.replace('/login');
          return;
        }
        setError('Failed to load scores');
      })
      .finally(() => setLoading(false));
  }, [router, offset, showDemo]);

  return (
    <div className="space-y-8">
      <PageHeader
        title="Call Quality"
        description="AI-scored calls with sentiment and rule pass/fail breakdown."
        action={
          <Button type="button" variant="outline" size="sm" asChild>
            <Link href="/scoring/scorecards">Manage Scorecards</Link>
          </Button>
        }
      />

      <FilterBar showDemoToggle />

      {loading && <PageLoader />}
      {error && <ErrorBanner message={error} />}

      {!loading && !error && (
        <>
          <ScoresTable scores={scores} />
          <div className="flex justify-end gap-2">
            <Button
              type="button"
              variant="outline"
              size="sm"
              disabled={offset === 0}
              onClick={() => setOffset((o) => Math.max(0, o - PAGE_SIZE))}
            >
              Previous
            </Button>
            <Button
              type="button"
              variant="outline"
              size="sm"
              disabled={!hasMore}
              onClick={() => setOffset((o) => o + PAGE_SIZE)}
            >
              Next
            </Button>
          </div>
        </>
      )}
    </div>
  );
}
