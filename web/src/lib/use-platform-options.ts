'use client';

import { useEffect, useMemo, useState } from 'react';
import { useRouter } from 'next/navigation';
import { ApiError, fetchPlatformMix } from '@/lib/api';
import { getToken } from '@/lib/auth';
import { dateFilterToQuery, useDateFilter } from '@/lib/date-filter';
import { KNOWN_PLATFORMS, mergePlatformOptions } from '@/lib/platform-options';
import { platformsForV1 } from '@/lib/v1-features';

export function usePlatformOptions(): string[] {
  const router = useRouter();
  const { startDate } = useDateFilter();
  const dateFilters = useMemo(() => dateFilterToQuery(startDate, ''), [startDate]);
  const [platforms, setPlatforms] = useState<string[]>([...KNOWN_PLATFORMS]);

  useEffect(() => {
    const token = getToken();
    if (!token) return;

    fetchPlatformMix(token, dateFilters)
      .then((mix) => {
        setPlatforms(mergePlatformOptions(mix.map((row) => row.platform)));
      })
      .catch((err: unknown) => {
        if (err instanceof ApiError && err.status === 401) {
          router.replace('/login');
          return;
        }
        setPlatforms([...KNOWN_PLATFORMS]);
      });
  }, [router, dateFilters.from]);

  return platformsForV1(platforms);
}
