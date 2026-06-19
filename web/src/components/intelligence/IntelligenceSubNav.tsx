'use client';

import Link from 'next/link';
import { usePathname } from 'next/navigation';
import { cn } from '@/lib/utils';

const INTEL_TABS = [
  { href: '/intelligence', label: 'Hub' },
  { href: '/ask', label: 'Ask AI' },
  { href: '/compliance', label: 'Compliance' },
  { href: '/attribution', label: 'Attribution' },
] as const;

function isTabActive(pathname: string, href: string): boolean {
  if (href === '/intelligence') return pathname === '/intelligence';
  return pathname.startsWith(href);
}

export function IntelligenceSubNav(): JSX.Element {
  const pathname = usePathname();

  return (
    <nav className="mb-6 inline-flex gap-1 rounded-lg bg-calyx-surface p-1">
      {INTEL_TABS.map((tab) => (
        <Link
          key={tab.href}
          href={tab.href}
          className={cn(
            'rounded-md px-3 py-1.5 text-sm font-medium transition-colors',
            isTabActive(pathname, tab.href)
              ? 'bg-calyx-card text-calyx-text shadow-sm'
              : 'text-calyx-muted hover:text-calyx-text',
          )}
        >
          {tab.label}
        </Link>
      ))}
    </nav>
  );
}
