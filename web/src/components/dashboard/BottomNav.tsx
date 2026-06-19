'use client';

import Link from 'next/link';
import { usePathname } from 'next/navigation';
import { Brain, Home, Phone, Settings, Star } from 'lucide-react';
import { cn } from '@/lib/utils';

const NAV = [
  { id: 'dashboard', label: 'Home', href: '/dashboard', Icon: Home },
  { id: 'intel', label: 'Intel', href: '/intelligence', Icon: Brain },
  { id: 'calls', label: 'Pings', href: '/pings', Icon: Phone },
  { id: 'agents', label: 'Scoring', href: '/scoring', Icon: Star },
  { id: 'settings', label: 'Settings', href: '/settings', Icon: Settings },
] as const;

function resolveActiveTab(pathname: string): string {
  if (pathname.startsWith('/dashboard')) return 'dashboard';
  if (
    pathname.startsWith('/intelligence') ||
    pathname.startsWith('/ask') ||
    pathname.startsWith('/compliance') ||
    pathname.startsWith('/attribution') ||
    pathname.startsWith('/reports') ||
    pathname.startsWith('/coverage')
  ) {
    return 'intel';
  }
  if (pathname.startsWith('/pings') || pathname.startsWith('/calls')) return 'calls';
  if (pathname.startsWith('/scoring')) return 'agents';
  if (pathname.startsWith('/settings')) return 'settings';
  return 'dashboard';
}

export function BottomNav(): JSX.Element {
  const pathname = usePathname();
  const active = resolveActiveTab(pathname);

  return (
    <nav className="fixed bottom-0 left-0 right-0 z-50 border-t border-calyx-border bg-calyx-surface/95 backdrop-blur-lg lg:hidden">
      <div className="mx-auto flex max-w-lg items-center justify-around px-2 py-2">
        {NAV.map((item) => {
          const isActive = item.id === active;
          return (
            <Link
              key={item.id}
              href={item.href}
              className={cn(
                'flex min-h-[44px] min-w-[44px] flex-col items-center justify-center gap-0.5 rounded-lg px-2 py-1.5 transition',
                isActive ? 'text-calyx-accent' : 'text-calyx-muted hover:text-calyx-text',
              )}
            >
              <item.Icon className="h-5 w-5" />
              <span className="text-[9px] font-medium">{item.label}</span>
            </Link>
          );
        })}
      </div>
    </nav>
  );
}
