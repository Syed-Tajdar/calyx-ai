'use client';

import { useMemo } from 'react';
import type { PingFetchFilters, PingRecord } from '@/lib/api';
import { useLiveFeed, type LiveEventEnvelope } from '@/lib/use-live-feed';
import { usePingFeed, pingRowKey } from '@/lib/use-ping-feed';
import { pingEventAt } from '@/lib/ping-event-time';

interface UseHybridPingFeedOptions {
  limit: number;
  offset: number;
  filters: PingFetchFilters;
  pollMs?: number;
  enabled?: boolean;
}

interface UseHybridPingFeedResult {
  pings: PingRecord[];
  total: number;
  displayTotal: number;
  loaded: boolean;
  loading: boolean;
  error: string;
  newRowKeys: Set<string>;
  connected: boolean;
  isLive: boolean;
  isPollingFallback: boolean;
  liveEvents: LiveEventEnvelope[];
  refresh: () => void;
}

function wsPingToRecord(payload: Record<string, unknown>): PingRecord | null {
  const callId = typeof payload.call_id === 'string' ? payload.call_id : '';
  const tenantId = typeof payload.tenant_id === 'string' ? payload.tenant_id : '';
  if (!callId || !tenantId) return null;

  const receivedMs =
    typeof payload.received_at_unix_ms === 'number' ? payload.received_at_unix_ms : Date.now();
  const occurredMs =
    typeof payload.occurred_at_unix_ms === 'number' && payload.occurred_at_unix_ms > 0
      ? payload.occurred_at_unix_ms
      : receivedMs;

  const revenue = typeof payload.revenue === 'number' ? payload.revenue : 0;
  const payout = typeof payload.payout === 'number' ? payload.payout : 0;

  return {
    tenant_id: tenantId,
    platform: typeof payload.platform === 'string' ? payload.platform : 'ringba',
    external_call_id: callId,
    ani_hash: typeof payload.ani_hash === 'string' ? payload.ani_hash : '',
    campaign_id: typeof payload.campaign_id === 'string' ? payload.campaign_id : '',
    publisher_id: typeof payload.publisher_id === 'string' ? payload.publisher_id : '',
    target_id: typeof payload.target_id === 'string' ? payload.target_id : '',
    state: typeof payload.state === 'string' ? payload.state : '',
    event_type: typeof payload.event_type === 'string' ? payload.event_type : 'incoming',
    revenue,
    payout,
    is_duplicate: payload.is_duplicate === true ? 1 : 0,
    received_at: new Date(receivedMs).toISOString(),
    occurred_at: new Date(occurredMs).toISOString(),
  };
}

function pingMatchesFilters(row: PingRecord, filters: PingFetchFilters): boolean {
  if (
    !filters.show_demo &&
    (row.external_call_id.startsWith('apisync-') ||
      row.external_call_id.startsWith('demo-') ||
      row.external_call_id.startsWith('smoke-'))
  ) {
    return false;
  }
  if (filters.platform && row.platform !== filters.platform) return false;
  if (filters.from) {
    if (new Date(pingEventAt(row)).getTime() < new Date(filters.from).getTime()) return false;
  }
  if (filters.search) {
    const needle = filters.search.toLowerCase();
    const hay = row.external_call_id.toLowerCase();
    // Exact-then-prefix (Retreaver list shows truncated uuid).
    if (hay !== needle && !hay.startsWith(needle)) return false;
  }
  return true;
}

export function useHybridPingFeed({
  limit,
  offset,
  filters,
  pollMs = 8000,
  enabled = true,
}: UseHybridPingFeedOptions): UseHybridPingFeedResult {
  const poll = usePingFeed({
    limit,
    offset,
    filters,
    pollMs: enabled ? pollMs : 0,
    enabled,
  });

  const { connected, events, lastEvent } = useLiveFeed({ enabled });

  const wsPings = useMemo(() => {
    const rows: PingRecord[] = [];
    for (const event of events) {
      if (event.type !== 'ping_received') continue;
      const row = wsPingToRecord(event.payload);
      if (row && pingMatchesFilters(row, filters)) rows.push(row);
    }
    return rows;
  }, [events, filters]);

  // Cap live prepend so mock/WS ticks cannot drown ClickHouse history
  // (WS buffer holds up to 100 events; page size is typically 25).
  const LIVE_PREPEND_CAP = 5;

  const mergedPings = useMemo(() => {
    if (!connected || wsPings.length === 0) return poll.pings;
    const seen = new Set<string>();
    const merged: PingRecord[] = [];
    // Prefer poll rows when keys collide — they carry full CH fields.
    for (const row of poll.pings) {
      const key = pingRowKey(row);
      if (seen.has(key)) continue;
      seen.add(key);
      merged.push(row);
    }
    const liveOnly: PingRecord[] = [];
    for (const row of wsPings) {
      const key = pingRowKey(row);
      if (seen.has(key)) continue;
      seen.add(key);
      liveOnly.push(row);
      if (liveOnly.length >= LIVE_PREPEND_CAP) break;
    }
    return [...liveOnly, ...merged].slice(0, limit);
  }, [connected, wsPings, poll.pings, limit]);

  const newRowKeys = useMemo(() => {
    if (!lastEvent || lastEvent.type !== 'ping_received') return poll.newRowKeys;
    const row = wsPingToRecord(lastEvent.payload);
    if (!row || !pingMatchesFilters(row, filters)) return poll.newRowKeys;
    return new Set([pingRowKey(row)]);
  }, [lastEvent, poll.newRowKeys, filters]);

  const loaded = !poll.loading;

  const displayTotal = useMemo(() => {
    if (poll.loading) return 0;
    // Headline = distinct calls (Coverage / dashboard aligned).
    return poll.distinctTotal;
  }, [poll.loading, poll.distinctTotal]);

  return {
    pings: mergedPings,
    total: poll.total,
    displayTotal,
    loaded,
    loading: poll.loading,
    error: poll.error,
    newRowKeys,
    connected,
    isLive: connected,
    isPollingFallback: !connected && poll.isLive,
    liveEvents: events,
    refresh: poll.refresh,
  };
}
