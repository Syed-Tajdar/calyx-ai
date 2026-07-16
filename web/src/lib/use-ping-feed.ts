'use client';

import { useCallback, useEffect, useRef, useState } from 'react';
import { useRouter } from 'next/navigation';
import { ApiError, fetchPings, PingFetchFilters, PingRecord } from '@/lib/api';
import { getToken } from '@/lib/auth';

export function pingRowKey(p: PingRecord): string {
  return `${p.external_call_id}-${p.received_at}`;
}

interface UsePingFeedOptions {
  limit: number;
  offset: number;
  filters: PingFetchFilters;
  pollMs?: number;
  enabled?: boolean;
}

interface UsePingFeedResult {
  pings: PingRecord[];
  total: number;
  distinctTotal: number;
  loading: boolean;
  error: string;
  newRowKeys: Set<string>;
  isLive: boolean;
  refresh: () => void;
}

export function usePingFeed({
  limit,
  offset,
  filters,
  pollMs = 8000,
  enabled = true,
}: UsePingFeedOptions): UsePingFeedResult {
  const router = useRouter();
  const [pings, setPings] = useState<PingRecord[]>([]);
  const [total, setTotal] = useState(0);
  const [distinctTotal, setDistinctTotal] = useState(0);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [newRowKeys, setNewRowKeys] = useState<Set<string>>(new Set());
  const knownKeysRef = useRef<Set<string>>(new Set());
  const filtersKey = JSON.stringify(filters);

  const load = useCallback(
    async (silent = false) => {
      const token = getToken();
      if (!token || !enabled) return;

      if (!silent) setLoading(true);
      setError('');

      try {
        const res = await fetchPings(token, limit, offset, filters);
        if (silent && knownKeysRef.current.size > 0) {
          const fresh = new Set<string>();
          for (const row of res.data) {
            const key = pingRowKey(row);
            if (!knownKeysRef.current.has(key)) {
              fresh.add(key);
            }
          }
          if (fresh.size > 0) {
            setNewRowKeys(fresh);
            window.setTimeout(() => setNewRowKeys(new Set()), 2500);
          }
        }
        knownKeysRef.current = new Set(res.data.map(pingRowKey));
        setPings(res.data);
        setTotal(res.total);
        setDistinctTotal(res.distinct_total ?? res.total);
      } catch (err: unknown) {
        if (err instanceof ApiError && err.status === 401) {
          router.replace('/login');
          return;
        }
        if (!silent) setError('Failed to load pings');
      } finally {
        if (!silent) setLoading(false);
      }
    },
    [enabled, filters, filtersKey, limit, offset, router],
  );

  useEffect(() => {
    knownKeysRef.current = new Set();
    void load(false);
  }, [load]);

  useEffect(() => {
    if (!pollMs || !enabled) return undefined;

    const interval = window.setInterval(() => {
      if (document.visibilityState === 'visible') {
        void load(true);
      }
    }, pollMs);

    return () => window.clearInterval(interval);
  }, [enabled, load, pollMs]);

  return {
    pings,
    total,
    distinctTotal,
    loading,
    error,
    newRowKeys,
    isLive: Boolean(pollMs && enabled),
    refresh: () => {
      void load(false);
    },
  };
}
