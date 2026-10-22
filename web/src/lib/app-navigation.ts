import type { LucideIcon } from 'lucide-react';
import { isRouteVisible } from '@/lib/v1-features';
import {
  BarChart3,
  Bell,
  Brain,
  MessageSquareText,
  LayoutDashboard,
  Map,
  Radio,
  Settings,
  Star,
} from 'lucide-react';

export interface NavItem {
  href: string;
  label: string;
  icon: LucideIcon;
}

export interface NavGroup {
  id: string;
  label: string;
  items: NavItem[];
}

const ALL_NAV_GROUPS: NavGroup[] = [
  {
    id: 'operations',
    label: 'Operations',
    items: [
      { href: '/dashboard', label: 'Dashboard', icon: LayoutDashboard },
      { href: '/pings', label: 'Pings', icon: Radio },
      { href: '/scoring', label: 'Scoring', icon: Star },
    ],
  },
  {
    id: 'intelligence',
    label: 'Analytics',
    items: [
      { href: '/intelligence', label: 'Intelligence', icon: Brain },
      { href: '/reports', label: 'Reports', icon: BarChart3 },
      { href: '/coverage', label: 'Coverage', icon: Map },
    ],
  },
  {
    id: 'ai',
    label: 'Intelligence',
    items: [
      { href: '/ask', label: 'Ask AI', icon: MessageSquareText },
      { href: '/alerts', label: 'Alerts', icon: Bell },
    ],
  },
  {
    id: 'system',
    label: 'System',
    items: [{ href: '/settings', label: 'Settings', icon: Settings }],
  },
];

/** Out-of-scope routes are dropped, and any group left empty disappears with them. */
export const NAV_GROUPS: NavGroup[] = ALL_NAV_GROUPS.map((group) => ({
  ...group,
  items: group.items.filter((item) => isRouteVisible(item.href)),
})).filter((group) => group.items.length > 0);

export const SIDEBAR_NAV_ITEMS: NavItem[] = NAV_GROUPS.flatMap((group) => group.items);

const ALL_BOTTOM_NAV_ITEMS = [
  { id: 'dashboard', label: 'Dashboard', href: '/dashboard', icon: LayoutDashboard },
  { id: 'intel', label: 'Intelligence', href: '/intelligence', icon: Brain },
  { id: 'calls', label: 'Pings', href: '/pings', icon: Radio },
  { id: 'scoring', label: 'Scoring', href: '/scoring', icon: Star },
  { id: 'settings', label: 'Settings', href: '/settings', icon: Settings },
] as const;

export const BOTTOM_NAV_ITEMS = ALL_BOTTOM_NAV_ITEMS.filter((item) =>
  isRouteVisible(item.href),
);

export function isNavActive(pathname: string, href: string): boolean {
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

export function resolveBottomNavTab(pathname: string): string {
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
  if (pathname.startsWith('/scoring')) return 'scoring';
  if (pathname.startsWith('/settings')) return 'settings';
  return 'dashboard';
}
