'use client';

import Link from 'next/link';
import { usePathname } from 'next/navigation';
import { BOTTOM_NAV_ITEMS, resolveBottomNavTab } from '@/lib/app-navigation';
import { NavIcon } from '@/components/layout/NavIcon';
import { cn } from '@/lib/utils';

export function BottomNav(): JSX.Element {
  const pathname = usePathname();
  const active = resolveBottomNavTab(pathname);

  return (
    <nav className="fixed bottom-0 left-0 right-0 z-50 border-t border-calyx-border bg-calyx-surface/95 backdrop-blur-lg lg:hidden">
      <div className="mx-auto flex max-w-lg items-center justify-around px-2 py-2">
        {BOTTOM_NAV_ITEMS.map((item) => {
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
              <NavIcon icon={item.icon} className="size-5" />
              <span className="text-[9px] font-medium">{item.label}</span>
            </Link>
          );
        })}
      </div>
    </nav>
  );
}
