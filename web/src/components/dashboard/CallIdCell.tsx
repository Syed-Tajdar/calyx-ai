'use client';

import Link from 'next/link';
import { useState } from 'react';
import { Check, Copy } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { cn } from '@/lib/utils';

/** Show enough of a UUID/id to scan; full value always in title + copy. */
const DISPLAY_MAX = 20;

export function formatCallIdDisplay(id: string, maxLen = DISPLAY_MAX): string {
  if (!id) return '—';
  if (id.length <= maxLen) return id;
  return `${id.slice(0, maxLen)}…`;
}

interface CallIdCellProps {
  callId: string;
  href?: string;
  className?: string;
}

export function CallIdCell({ callId, href, className }: CallIdCellProps): JSX.Element {
  const [copied, setCopied] = useState(false);
  const display = formatCallIdDisplay(callId);

  async function handleCopy(): Promise<void> {
    if (!callId) return;
    try {
      await navigator.clipboard.writeText(callId);
      setCopied(true);
      window.setTimeout(() => setCopied(false), 1500);
    } catch {
      // Clipboard may be denied; title still exposes full id for manual copy.
    }
  }

  const idNode = href ? (
    <Link
      href={href}
      title={callId}
      className="truncate text-calyx-accent hover:underline"
    >
      {display}
    </Link>
  ) : (
    <span title={callId} className="truncate text-calyx-muted">
      {display}
    </span>
  );

  return (
    <div className={cn('flex max-w-[14rem] items-center gap-1', className)}>
      <span className="min-w-0 flex-1 font-mono text-xs">{idNode}</span>
      {callId ? (
        <Button
          type="button"
          variant="ghost"
          size="icon"
          className="h-7 w-7 shrink-0 text-calyx-muted hover:text-calyx-text"
          aria-label={copied ? 'Copied call ID' : 'Copy call ID'}
          title={copied ? 'Copied' : 'Copy call ID'}
          onClick={(e) => {
            e.preventDefault();
            e.stopPropagation();
            void handleCopy();
          }}
        >
          {copied ? <Check className="h-3.5 w-3.5 text-calyx-success" /> : <Copy className="h-3.5 w-3.5" />}
        </Button>
      ) : null}
    </div>
  );
}
