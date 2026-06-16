'use client';

import { useEffect, useState } from 'react';
import { useRouter } from 'next/navigation';
import { AppHeader } from '@/components/dashboard/AppHeader';
import { SectionHeader } from '@/components/dashboard/SectionHeader';
import { ScoresTable } from '@/components/scoring/ScoresTable';
import { ErrorBanner, PageLoader } from '@/components/ui/PageStatus';
import { ApiError, fetchScores, ScoreRecord } from '@/lib/api';
import { getToken } from '@/lib/auth';

const PAGE_SIZE = 25;

export default function ScoringPage(): JSX.Element {
  const router = useRouter();
  const [scores, setScores] = useState<ScoreRecord[]>([]);
  const [offset, setOffset] = useState(0);
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(true);
  const [hasMore, setHasMore] = useState(false);

  useEffect(() => {
    const token = getToken();
    if (!token) return;

    setLoading(true);
    fetchScores(token, { limit: PAGE_SIZE, offset })
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
  }, [router, offset]);

  return (
    <>
      <AppHeader />
      <SectionHeader title="Call Quality" />

      {loading && <PageLoader />}
      {error && <ErrorBanner message={error} />}

      {!loading && !error && (
        <div className="space-y-4">
          <ScoresTable scores={scores} />
          <div className="flex justify-end gap-2 text-xs">
            <button
              type="button"
              disabled={offset === 0}
              onClick={() => setOffset((o) => Math.max(0, o - PAGE_SIZE))}
              className="rounded-lg border border-loom-border px-3 py-1.5 transition hover:border-loom-accent disabled:opacity-40"
            >
              Previous
            </button>
            <button
              type="button"
              disabled={!hasMore}
              onClick={() => setOffset((o) => o + PAGE_SIZE)}
              className="rounded-lg border border-loom-border px-3 py-1.5 transition hover:border-loom-accent disabled:opacity-40"
            >
              Next
            </button>
          </div>
        </div>
      )}
    </>
  );
}
