'use client';

import { useEffect, useState } from 'react';
import { useRouter } from 'next/navigation';
import { AppHeader } from '@/components/dashboard/AppHeader';
import { PingsTable } from '@/components/dashboard/PingsTable';
import { SectionHeader } from '@/components/dashboard/SectionHeader';
import { ErrorBanner, PageLoader } from '@/components/ui/PageStatus';
import { ApiError, fetchPings, PingRecord } from '@/lib/api';
import { getToken } from '@/lib/auth';

const PAGE_SIZE = 25;

export default function PingsPage(): JSX.Element {
  const router = useRouter();
  const [pings, setPings] = useState<PingRecord[]>([]);
  const [total, setTotal] = useState(0);
  const [offset, setOffset] = useState(0);
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    const token = getToken();
    if (!token) return;

    setLoading(true);
    fetchPings(token, PAGE_SIZE, offset)
      .then((res) => {
        setPings(res.data);
        setTotal(res.total);
      })
      .catch((err: unknown) => {
        if (err instanceof ApiError && err.status === 401) {
          router.replace('/login');
          return;
        }
        setError('Failed to load pings');
      })
      .finally(() => setLoading(false));
  }, [router, offset]);

  const canPrev = offset > 0;
  const canNext = offset + PAGE_SIZE < total;

  return (
    <>
      <AppHeader />
      <SectionHeader title="Ping Feed" />

      {loading && <PageLoader />}
      {error && <ErrorBanner message={error} />}

      {!loading && !error && (
        <div className="space-y-4">
          <PingsTable pings={pings} linkable />
          <div className="flex items-center justify-between text-xs text-loom-muted">
            <span>
              Showing {total === 0 ? 0 : offset + 1}–{Math.min(offset + PAGE_SIZE, total)} of{' '}
              {total}
            </span>
            <div className="flex gap-2">
              <button
                type="button"
                disabled={!canPrev}
                onClick={() => setOffset((o) => Math.max(0, o - PAGE_SIZE))}
                className="rounded-lg border border-loom-border px-3 py-1.5 transition hover:border-loom-accent disabled:opacity-40"
              >
                Previous
              </button>
              <button
                type="button"
                disabled={!canNext}
                onClick={() => setOffset((o) => o + PAGE_SIZE)}
                className="rounded-lg border border-loom-border px-3 py-1.5 transition hover:border-loom-accent disabled:opacity-40"
              >
                Next
              </button>
            </div>
          </div>
        </div>
      )}
    </>
  );
}
