import Link from 'next/link';
import type { LucideIcon } from 'lucide-react';
import { ArrowRight } from 'lucide-react';
import { cn } from '@/lib/utils';

interface IntelligenceCommandCardProps {
  href: string;
  title: string;
  description: string;
  icon: LucideIcon;
  accent?: 'teal' | 'indigo' | 'amber';
}

const ACCENT_BAR = {
  teal: 'bg-calyx-teal',
  indigo: 'bg-calyx-accent',
  amber: 'bg-calyx-warning',
};

export function IntelligenceCommandCard({
  href,
  title,
  description,
  icon: Icon,
  accent = 'teal',
}: IntelligenceCommandCardProps): JSX.Element {
  return (
    <Link
      href={href}
      className={cn(
        'group relative flex flex-col overflow-hidden rounded-xl border border-calyx-border bg-calyx-card p-5 transition',
        'hover:border-calyx-accent/40 hover:shadow-[var(--shadow-glow)]',
      )}
    >
      <div className={cn('absolute left-0 top-0 h-full w-1', ACCENT_BAR[accent])} />
      <div className="mb-4 flex h-10 w-10 items-center justify-center rounded-lg bg-calyx-accent-muted">
        <Icon className="h-5 w-5 text-calyx-accent" />
      </div>
      <h3 className="text-sm font-semibold text-calyx-text">{title}</h3>
      <p className="mt-1.5 flex-1 text-xs leading-relaxed text-calyx-muted">{description}</p>
      <span className="mt-4 inline-flex items-center gap-1 text-[11px] font-semibold uppercase tracking-wider text-calyx-teal group-hover:text-calyx-accent">
        Open
        <ArrowRight className="h-3 w-3 transition-transform group-hover:translate-x-0.5" />
      </span>
    </Link>
  );
}
