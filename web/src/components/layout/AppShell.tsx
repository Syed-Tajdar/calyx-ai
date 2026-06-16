'use client';

import { useRouter } from 'next/navigation';
import { useEffect, useState } from 'react';
import { BottomNav } from '@/components/dashboard/BottomNav';
import { DesignBackground } from '@/components/ui/DesignBackground';
import { Logo } from '@/components/ui/Logo';
import { clearToken, getToken } from '@/lib/auth';

interface AppShellProps {
  children: React.ReactNode;
}

export function AppShell({ children }: AppShellProps): JSX.Element | null {
  const router = useRouter();
  const [ready, setReady] = useState(false);

  useEffect(() => {
    if (!getToken()) {
      router.replace('/login');
      return;
    }
    setReady(true);
  }, [router]);

  function logout(): void {
    clearToken();
    router.push('/login');
  }

  if (!ready) {
    return (
      <div className="relative flex min-h-screen items-center justify-center bg-loom-bg">
        <div className="h-10 w-10 animate-spin rounded-full border-2 border-loom-border border-t-loom-accent" />
      </div>
    );
  }

  return (
    <div className="relative min-h-screen bg-loom-bg pb-20">
      <DesignBackground />
      <div className="relative z-10 flex items-center justify-between px-4 py-4 sm:px-6">
        <Logo variant="compact" />
        <button
          type="button"
          onClick={logout}
          className="rounded-full border border-loom-border px-4 py-1.5 text-xs text-loom-muted transition hover:border-loom-accent hover:text-white"
        >
          Log out
        </button>
      </div>
      <div className="relative z-10 px-4 sm:px-6">
        <div className="loom-dashboard-shell">{children}</div>
      </div>
      <BottomNav />
    </div>
  );
}
