/**
 * v1 sync health (Part 1, item 2). The badge in Settings → Integrations must not depend on
 * `sync_status` alone: from 2026-10-18 to 2026-10-21 every sync failed while the row still said
 * `ok`. The time of the last successful sync is the ground truth, so a sync that has not
 * succeeded for 30 minutes reads red whatever the status says.
 */

export const SYNC_STALE_AFTER_MS = 30 * 60 * 1000;

/** True when the last success is missing or older than 30 minutes. */
export function isSyncStale(lastSuccessAt: string | null | undefined, now: number = Date.now()): boolean {
  if (!lastSuccessAt) return true;
  const t = Date.parse(lastSuccessAt);
  if (Number.isNaN(t)) return true;
  return now - t > SYNC_STALE_AFTER_MS;
}

const easternFormat = new Intl.DateTimeFormat('en-US', {
  timeZone: 'America/New_York',
  month: 'short',
  day: 'numeric',
  hour: 'numeric',
  minute: '2-digit',
  timeZoneName: 'short',
});

/** "Oct 21, 10:22 AM EDT", or "never" when there has been no successful sync yet. */
export function formatLastSuccess(lastSuccessAt: string | null | undefined): string {
  if (!lastSuccessAt) return 'never';
  const t = Date.parse(lastSuccessAt);
  if (Number.isNaN(t)) return 'unknown';
  return easternFormat.format(new Date(t));
}
