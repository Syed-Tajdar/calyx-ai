'use client';

import { useRouter } from 'next/navigation';
import { useEffect, useState } from 'react';
import { BottomNav } from '@/components/dashboard/BottomNav';
import { Sidebar } from '@/components/layout/Sidebar';
import { TopBar } from '@/components/layout/TopBar';
import { DesignBackground } from '@/components/ui/DesignBackground';
import { Skeleton } from '@/components/ui/skeleton';
import { DateFilterProvider } from '@/lib/date-filter';
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
      <div className="relative flex min-h-screen items-center justify-center bg-calyx-bg">
        <Skeleton className="h-10 w-10 rounded-full" />
      </div>
    );
  }

  return (
    <DateFilterProvider>
      <div className="relative min-h-screen bg-calyx-bg pb-20 lg:pb-0">
        <DesignBackground />
        <div className="relative z-10 flex min-h-screen">
          <Sidebar />
          <div className="flex min-w-0 flex-1 flex-col">
            <TopBar onLogout={logout} />
            <main className="mx-auto w-full max-w-content flex-1 overflow-x-hidden px-4 py-7 sm:px-6 lg:px-8">
              {children}
            </main>
          </div>
        </div>
        <BottomNav />
      </div>
    </DateFilterProvider>
  );
}
