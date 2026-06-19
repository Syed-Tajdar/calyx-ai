'use client';

import Link from 'next/link';
import { usePathname } from 'next/navigation';
import {
  BarChart3,
  Brain,
  LayoutDashboard,
  Map,
  Phone,
  Settings,
  Star,
} from 'lucide-react';
import { cn } from '@/lib/utils';

const NAV_ITEMS = [
  { href: '/dashboard', label: 'Dashboard', icon: LayoutDashboard },
  { href: '/pings', label: 'Pings', icon: Phone },
  { href: '/scoring', label: 'Scoring', icon: Star },
  { href: '/intelligence', label: 'Intelligence', icon: Brain },
  { href: '/reports', label: 'Reports', icon: BarChart3 },
  { href: '/coverage', label: 'Coverage', icon: Map },
  { href: '/settings', label: 'Settings', icon: Settings },
] as const;

function isActive(pathname: string, href: string): boolean {
  if (href === '/dashboard') return pathname === '/dashboard';
  if (href === '/intelligence') {
    return (
      pathname.startsWith('/intelligence') ||
      pathname.startsWith('/ask') ||
      pathname.startsWith('/compliance') ||
      pathname.startsWith('/attribution')
    );
  }
  if (href === '/reports') return pathname.startsWith('/reports');
  if (href === '/coverage') return pathname.startsWith('/coverage');
  return pathname.startsWith(href);
}

export function Sidebar(): JSX.Element {
  const pathname = usePathname();

  return (
    <aside className="hidden lg:flex lg:w-60 lg:flex-col lg:border-r lg:border-calyx-border lg:bg-calyx-surface">
      <div className="border-b border-calyx-border px-5 py-5">
        <p className="text-xs font-semibold uppercase tracking-widest text-calyx-teal">Calyx AI</p>
        <p className="mt-0.5 text-[10px] text-calyx-muted">Call Intelligence</p>
      </div>
      <nav className="flex flex-1 flex-col gap-1 p-3">
        {NAV_ITEMS.map((item) => {
          const active = isActive(pathname, item.href);
          const Icon = item.icon;
          return (
            <Link
              key={item.href}
              href={item.href}
              className={cn(
                'flex items-center gap-3 rounded-lg px-3 py-2.5 text-sm font-medium transition-colors',
                active
                  ? 'bg-calyx-accent-muted text-calyx-accent'
                  : 'text-calyx-muted hover:bg-calyx-card hover:text-calyx-text',
              )}
            >
              <Icon className="h-4 w-4 shrink-0" />
              {item.label}
            </Link>
          );
        })}
      </nav>
    </aside>
  );
}
