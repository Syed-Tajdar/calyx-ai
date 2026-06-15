'use client';

import {
  NavAgentsIcon,
  NavCallsIcon,
  NavDashboardIcon,
  NavReportsIcon,
  NavSettingsIcon,
} from '@/components/ui/Icons';

const NAV = [
  { id: 'dashboard', label: 'Dashboard', Icon: NavDashboardIcon },
  { id: 'reports', label: 'Reports', Icon: NavReportsIcon },
  { id: 'calls', label: 'Pings', Icon: NavCallsIcon },
  { id: 'agents', label: 'Agents', Icon: NavAgentsIcon },
  { id: 'settings', label: 'Settings', Icon: NavSettingsIcon },
] as const;

interface BottomNavProps {
  active?: string;
  onLogout?: () => void;
}

export function BottomNav({ active = 'dashboard', onLogout }: BottomNavProps): JSX.Element {
  return (
    <nav className="fixed bottom-0 left-0 right-0 z-50 border-t border-loom-border bg-loom-surface/95 backdrop-blur-lg">
      <div className="mx-auto flex max-w-5xl items-center justify-around px-2 py-2">
        {NAV.map((item) => {
          const isActive = item.id === active;
          return (
            <button
              key={item.id}
              type="button"
              onClick={item.id === 'settings' ? onLogout : undefined}
              className={`flex flex-col items-center gap-0.5 rounded-lg px-3 py-1.5 transition ${
                isActive ? 'text-loom-accent' : 'text-loom-muted hover:text-white'
              }`}
            >
              <item.Icon className="h-5 w-5" />
              <span className="text-[9px] font-medium">{item.label}</span>
            </button>
          );
        })}
      </div>
    </nav>
  );
}
