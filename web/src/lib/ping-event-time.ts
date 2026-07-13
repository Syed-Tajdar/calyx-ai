import type { PingRecord } from '@/lib/api';

/** Prefer platform source call time; fall back to ingestion time. */
export function pingEventAt(ping: Pick<PingRecord, 'received_at' | 'occurred_at'>): string {
  if (ping.occurred_at && ping.occurred_at.length > 0) {
    const t = new Date(ping.occurred_at).getTime();
    if (!Number.isNaN(t) && t > 0) {
      return ping.occurred_at;
    }
  }
  return ping.received_at;
}
