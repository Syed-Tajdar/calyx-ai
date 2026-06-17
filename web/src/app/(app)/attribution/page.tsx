'use client';

import { useEffect, useState } from 'react';
import { useRouter } from 'next/navigation';
import Link from 'next/link';
import { AttributionEventsTable } from '@/components/attribution/AttributionEventsTable';
import { PlatformStatusGrid } from '@/components/attribution/PlatformStatusGrid';
import { AppHeader } from '@/components/dashboard/AppHeader';
import { SectionHeader } from '@/components/dashboard/SectionHeader';
import { ErrorBanner, PageLoader } from '@/components/ui/PageStatus';
import {
  ApiError,
  AttributionEventRecord,
  fetchAttributionEvents,
  fetchAttributionStatus,
  PlatformStatusSummary,
} from '@/lib/api';
import { getToken } from '@/lib/auth';

export default function AttributionPage(): JSX.Element {
  const router = useRouter();
  const [platforms, setPlatforms] = useState<PlatformStatusSummary[]>([]);
  const [events, setEvents] = useState<AttributionEventRecord[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  useEffect(() => {
    const token = getToken();
    if (!token) return;

    Promise.all([fetchAttributionStatus(token), fetchAttributionEvents(token, 25)])
      .then(([statusRes, eventsRes]) => {
        setPlatforms(statusRes.data);
        setEvents(eventsRes.data);
      })
      .catch((err: unknown) => {
        if (err instanceof ApiError && err.status === 401) {
          router.replace('/login');
          return;
        }
        setError('Failed to load attribution data');
      })
      .finally(() => setLoading(false));
  }, [router]);

  return (
    <>
      <AppHeader />
      <div className="mb-3 flex items-center justify-between">
        <SectionHeader title="Ad Attribution" />
        <Link href="/intelligence" className="text-xs text-loom-accent hover:underline">
          Intelligence hub
        </Link>
      </div>

      {loading && <PageLoader />}
      {error && <ErrorBanner message={error} />}

      {!loading && !error && (
        <div className="space-y-4">
          <PlatformStatusGrid platforms={platforms} />
          <AttributionEventsTable events={events} />
        </div>
      )}
    </>
  );
}
