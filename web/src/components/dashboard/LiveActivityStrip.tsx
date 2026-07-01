'use client';

import type { LiveEventEnvelope } from '@/lib/use-live-feed';
import { LiveEventBadge } from '@/components/ui/StatusPill';
import { cn } from '@/lib/utils';

interface LiveActivityStripProps {
  events: LiveEventEnvelope[];
  className?: string;
}

export function LiveActivityStrip({
  events,
  className,
}: LiveActivityStripProps): JSX.Element | null {
  const recent = events.slice(0, 6);
  if (recent.length === 0) return null;

  return (
    <div className={cn('calyx-glass calyx-glass-lit overflow-hidden', className)}>
      <div className="border-b border-calyx-border/60 px-4 py-2">
        <p className="text-[10px] font-semibold uppercase tracking-wider text-calyx-teal">
          Live Activity
        </p>
      </div>
      <ul className="divide-y divide-calyx-border/40">
        {recent.map((event, index) => {
          const callId =
            typeof event.payload.call_id === 'string' ? event.payload.call_id : '—';
          return (
            <li
              key={`${event.type}-${event.emitted_at}-${index}`}
              className="flex items-center justify-between gap-3 px-4 py-2 text-xs motion-safe:animate-[calyx-flash_2s_ease-out]"
            >
              <LiveEventBadge eventType={event.type} />
              <span className="min-w-0 flex-1 truncate font-mono text-calyx-muted">{callId}</span>
              <span className="shrink-0 tabular-nums text-calyx-dim">
                {new Date(event.emitted_at).toLocaleTimeString()}
              </span>
            </li>
          );
        })}
      </ul>
    </div>
  );
}
