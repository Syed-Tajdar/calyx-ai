'use client';

import Link from 'next/link';
import { usePathname } from 'next/navigation';
import {
  NavAgentsIcon,
  NavCallsIcon,
  NavDashboardIcon,
  NavReportsIcon,
  NavSettingsIcon,
} from '@/components/ui/Icons';

const NAV = [
  { id: 'dashboard', label: 'Home', href: '/dashboard', Icon: NavDashboardIcon },
  { id: 'intel', label: 'Intel', href: '/intelligence', Icon: NavReportsIcon },
  { id: 'calls', label: 'Pings', href: '/pings', Icon: NavCallsIcon },
  { id: 'agents', label: 'Quality', href: '/scoring', Icon: NavAgentsIcon },
  { id: 'settings', label: 'Settings', href: '/settings', Icon: NavSettingsIcon },
] as const;

function resolveActiveTab(pathname: string): string {
  if (pathname.startsWith('/dashboard')) return 'dashboard';
  if (
    pathname.startsWith('/intelligence') ||
    pathname.startsWith('/ask') ||
    pathname.startsWith('/compliance') ||
    pathname.startsWith('/attribution')
  ) {
    return 'intel';
  }
  if (pathname.startsWith('/reports') || pathname.startsWith('/coverage')) return 'intel';
  if (pathname.startsWith('/pings') || pathname.startsWith('/calls')) return 'calls';
  if (pathname.startsWith('/scoring')) return 'agents';
  if (pathname.startsWith('/settings')) return 'settings';
  return 'dashboard';
}

export function BottomNav(): JSX.Element {
  const pathname = usePathname();
  const active = resolveActiveTab(pathname);

  return (
    <nav className="fixed bottom-0 left-0 right-0 z-50 border-t border-loom-border bg-loom-surface/95 backdrop-blur-lg">
      <div className="mx-auto flex max-w-5xl items-center justify-around px-2 py-2">
        {NAV.map((item) => {
          const isActive = item.id === active;
          return (
            <Link
              key={item.id}
              href={item.href}
              className={`flex flex-col items-center gap-0.5 rounded-lg px-3 py-1.5 transition ${
                isActive ? 'text-loom-accent' : 'text-loom-muted hover:text-white'
              }`}
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
