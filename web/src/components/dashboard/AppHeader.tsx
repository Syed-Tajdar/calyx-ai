import { BellIcon, MenuIcon } from '@/components/ui/Icons';

interface AppHeaderProps {
  onMenuClick?: () => void;
}

export function AppHeader({ onMenuClick }: AppHeaderProps): JSX.Element {
  return (
    <header className="flex items-center justify-between py-4">
      <button
        type="button"
        onClick={onMenuClick}
        className="flex h-9 w-9 items-center justify-center rounded-lg text-loom-muted transition hover:bg-loom-surface hover:text-white"
        aria-label="Menu"
      >
        <MenuIcon />
      </button>
      <h1 className="text-center text-xs font-bold uppercase tracking-[0.2em] text-white sm:text-sm">
        Call Tracking Reports
      </h1>
      <button
        type="button"
        className="relative flex h-9 w-9 items-center justify-center rounded-lg text-loom-muted transition hover:bg-loom-surface hover:text-white"
        aria-label="Notifications"
      >
        <BellIcon />
        <span className="absolute right-2 top-2 h-1.5 w-1.5 rounded-full bg-loom-accent" />
      </button>
    </header>
  );
}
