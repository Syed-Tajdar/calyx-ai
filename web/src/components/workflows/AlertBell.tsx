'use client';

import Link from 'next/link';
import { useEffect, useState } from 'react';
import { Bell } from 'lucide-react';
import { ApiError, fetchWorkflowStatus } from '@/lib/api';
import { getToken } from '@/lib/auth';
import { isFeatureEnabled } from '@/lib/v1-features';
import { bellCountLabel } from '@/lib/workflows';

const POLL_MS = 60_000;

/**
 * Top-bar bell with the count of open alerts. Rendered only when the workflows feature is on in
 * this build; hidden for non-admins (the API answers 403) and when the API route is closed (404).
 */
export function AlertBell(): JSX.Element | null {
  const enabled = isFeatureEnabled('workflows');
  const [count, setCount] = useState<number | null>(null);
  const [hidden, setHidden] = useState(false);

  useEffect(() => {
    if (!enabled) return;
    let stop = false;
    async function poll(): Promise<void> {
      const token = getToken();
      if (!token || stop) return;
      try {
        const s = await fetchWorkflowStatus(token);
        if (!stop) setCount(s.open_alerts);
      } catch (err: unknown) {
        if (err instanceof ApiError && (err.status === 403 || err.status === 404)) {
          setHidden(true);
          stop = true;
        }
      }
    }
    void poll();
    const t = setInterval(() => void poll(), POLL_MS);
    return () => {
      stop = true;
      clearInterval(t);
    };
  }, [enabled]);

  if (!enabled || hidden) return null;
  const label = bellCountLabel(count ?? 0);
  return (
    <Link
      href="/alerts"
      className="relative inline-flex h-9 w-9 items-center justify-center rounded-lg text-calyx-muted hover:bg-calyx-surface hover:text-calyx-text"
      aria-label={label ? `Alerts: ${label} open` : 'Alerts'}
      data-testid="alert-bell"
    >
      <Bell className="h-4 w-4" />
      {label && (
        <span className="absolute -right-0.5 -top-0.5 min-w-[16px] rounded-full bg-calyx-danger px-1 text-center text-[10px] font-bold leading-4 text-white">
          {label}
        </span>
      )}
    </Link>
  );
}
