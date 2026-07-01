'use client';

import Link from 'next/link';
import { usePathname } from 'next/navigation';
import { useEffect, useState } from 'react';
import {
  BarChart3,
  Brain,
  LayoutDashboard,
  Map,
  Phone,
  Settings,
  Star,
} from 'lucide-react';
import { Logo } from '@/components/ui/Logo';
import { ApiError, fetchMe, MeResult } from '@/lib/api';
import { getToken } from '@/lib/auth';
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
  const [me, setMe] = useState<MeResult | null>(null);

  useEffect(() => {
    const token = getToken();
    if (!token) return;
    fetchMe(token)
      .then(setMe)
      .catch((err: unknown) => {
        if (err instanceof ApiError) return;
      });
  }, []);

  const tenantLabel = me?.tenant_name ?? 'Tenant';

  return (
    <aside
      className="calyx-glass hidden lg:flex lg:flex-col lg:border-r lg:border-calyx-border/60"
      style={{ width: 'var(--sidebar-width)' }}
    >
      <div className="flex items-center gap-2.5 border-b border-calyx-border px-5 py-[18px]">
        <Logo variant="compact" />
      </div>
      <nav className="flex flex-1 flex-col gap-0.5 p-3">
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
                  ? 'bg-calyx-teal/15 text-calyx-teal'
                  : 'text-calyx-muted hover:bg-calyx-card hover:text-calyx-text',
              )}
            >
              <Icon className="h-4 w-4 shrink-0" />
              {item.label}
            </Link>
          );
        })}
      </nav>
      <div className="border-t border-calyx-border p-4 text-[11px] text-calyx-dim">
        <span className="font-medium text-calyx-muted">MOCK_MODE</span>
        <span className="mx-1.5">·</span>
        <span>{tenantLabel}</span>
      </div>
    </aside>
  );
}
