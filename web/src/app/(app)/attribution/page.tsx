'use client';

import { useEffect, useState } from 'react';
import { useRouter } from 'next/navigation';
import { AttributionEventsTable } from '@/components/attribution/AttributionEventsTable';
import { PlatformStatusGrid } from '@/components/attribution/PlatformStatusGrid';
import { IntelligenceSubNav } from '@/components/intelligence/IntelligenceSubNav';
import { ErrorBanner, PageLoader } from '@/components/ui/PageStatus';
import { PageHeader } from '@/components/ui/PageHeader';
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
    <div className="space-y-8">
      <PageHeader
        title="Ad Attribution"
        description="Meta, Google, and TikTok conversion fires — fired vs failed per platform."
      />
      <IntelligenceSubNav />

      {loading && <PageLoader />}
      {error && <ErrorBanner message={error} />}

      {!loading && !error && (
        <>
          <PlatformStatusGrid platforms={platforms} />
          <AttributionEventsTable events={events} />
        </>
      )}
    </div>
  );
}
