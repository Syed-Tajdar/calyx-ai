'use client';

import { useMemo } from 'react';
import type { PingFetchFilters, PingRecord } from '@/lib/api';
import { useLiveFeed, type LiveEventEnvelope } from '@/lib/use-live-feed';
import { usePingFeed, pingRowKey } from '@/lib/use-ping-feed';

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

  return {
    tenant_id: tenantId,
    platform: typeof payload.platform === 'string' ? payload.platform : 'ringba',
    external_call_id: callId,
    ani_hash: typeof payload.ani_hash === 'string' ? payload.ani_hash : '',
    campaign_id: typeof payload.campaign_id === 'string' ? payload.campaign_id : '',
    publisher_id: typeof payload.publisher_id === 'string' ? payload.publisher_id : '',
    target_id: '',
    state: '',
    event_type: typeof payload.event_type === 'string' ? payload.event_type : 'incoming',
    revenue: 0,
    payout: 0,
    is_duplicate: payload.is_duplicate === true ? 1 : 0,
    received_at: new Date(receivedMs).toISOString(),
  };
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
      if (row) rows.push(row);
    }
    return rows;
  }, [events]);

  const mergedPings = useMemo(() => {
    if (!connected || wsPings.length === 0) return poll.pings;
    const seen = new Set<string>();
    const merged: PingRecord[] = [];
    for (const row of [...wsPings, ...poll.pings]) {
      const key = pingRowKey(row);
      if (seen.has(key)) continue;
      seen.add(key);
      merged.push(row);
    }
    return merged.slice(0, limit);
  }, [connected, wsPings, poll.pings, limit]);

  const newRowKeys = useMemo(() => {
    if (!lastEvent || lastEvent.type !== 'ping_received') return poll.newRowKeys;
    const row = wsPingToRecord(lastEvent.payload);
    if (!row) return poll.newRowKeys;
    return new Set([pingRowKey(row)]);
  }, [lastEvent, poll.newRowKeys]);

  return {
    pings: mergedPings,
    total: connected ? Math.max(poll.total, mergedPings.length) : poll.total,
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
