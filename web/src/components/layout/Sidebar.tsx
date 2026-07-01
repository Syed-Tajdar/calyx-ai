'use client';

import Link from 'next/link';
import { usePathname } from 'next/navigation';
import { useEffect, useState } from 'react';
import { PanelLeft, PanelLeftClose } from 'lucide-react';
import { Logo } from '@/components/ui/Logo';
import { NavIcon } from '@/components/layout/NavIcon';
import { ApiError, fetchMe, MeResult } from '@/lib/api';
import { NAV_GROUPS, isNavActive } from '@/lib/app-navigation';
import { getToken } from '@/lib/auth';
import { isMockModeEnabled } from '@/lib/provider-domains';
import { useSidebarCollapse } from '@/lib/use-sidebar-collapse';
import { cn } from '@/lib/utils';

export function Sidebar(): JSX.Element {
  const pathname = usePathname();
  const [me, setMe] = useState<MeResult | null>(null);
  const { collapsed, toggle, ready } = useSidebarCollapse();

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
  const mockMode = isMockModeEnabled();
  const sidebarWidth = collapsed
    ? 'var(--sidebar-width-collapsed)'
    : 'var(--sidebar-width)';

  return (
    <aside
      className={cn(
        'calyx-glass hidden lg:flex lg:flex-col lg:border-r lg:border-calyx-border/60',
        !ready && 'opacity-0',
        ready && 'opacity-100 transition-opacity duration-150',
      )}
      style={{ width: sidebarWidth }}
    >
      <div
        className={cn(
          'flex items-center border-b border-calyx-border',
          collapsed ? 'flex-col gap-2 px-2 py-3' : 'justify-between gap-2 px-4 py-[18px] pl-5',
        )}
      >
        <div
          className={cn(
            'min-w-0',
            collapsed && 'flex justify-center [&_.leading-none]:hidden',
          )}
        >
          <Logo variant="compact" className={collapsed ? 'gap-0' : undefined} />
        </div>
        <button
          type="button"
          onClick={toggle}
          className="flex size-7 shrink-0 items-center justify-center rounded-lg border border-calyx-border bg-calyx-card text-calyx-muted transition hover:bg-calyx-surface hover:text-calyx-text"
          aria-label={collapsed ? 'Expand sidebar' : 'Collapse sidebar'}
        >
          {collapsed ? (
            <PanelLeft className="size-4" strokeWidth={2} aria-hidden />
          ) : (
            <PanelLeftClose className="size-4" strokeWidth={2} aria-hidden />
          )}
        </button>
      </div>

      <nav className="flex flex-1 flex-col gap-0.5 overflow-y-auto overflow-x-hidden p-3">
        {NAV_GROUPS.map((group) => (
          <div key={group.id} className="flex flex-col gap-0.5">
            {!collapsed && (
              <p className="px-3 pb-1 pt-2 text-[10px] font-bold uppercase tracking-wider text-calyx-dim first:pt-1">
                {group.label}
              </p>
            )}
            {group.items.map((item) => {
              const active = isNavActive(pathname, item.href);
              return (
                <Link
                  key={item.href}
                  href={item.href}
                  title={collapsed ? item.label : undefined}
                  className={cn(
                    'relative flex items-center rounded-lg py-2.5 text-sm font-medium transition-colors',
                    collapsed ? 'justify-center px-2' : 'gap-3 px-3',
                    active
                      ? 'bg-calyx-teal/10 text-calyx-teal before:absolute before:inset-y-1.5 before:left-0 before:w-0.5 before:rounded-r before:bg-calyx-teal'
                      : 'text-calyx-muted hover:bg-calyx-card hover:text-calyx-text',
                  )}
                >
                  <NavIcon icon={item.icon} />
                  {!collapsed && <span className="truncate">{item.label}</span>}
                </Link>
              );
            })}
          </div>
        ))}
      </nav>

      <div className="border-t border-calyx-border p-4">
        {collapsed ? (
          mockMode ? (
            <span
              className="mx-auto block size-2 rounded-full bg-calyx-muted"
              title={`Mock · ${tenantLabel}`}
            />
          ) : (
            <span
              className="mx-auto block size-2 rounded-full bg-calyx-teal"
              title={tenantLabel}
            />
          )
        ) : (
          <div className="flex flex-col gap-1.5">
            {mockMode && (
              <span className="inline-flex w-fit items-center rounded-full border border-calyx-border bg-calyx-card px-2 py-0.5 text-[11px] font-medium text-calyx-muted">
                Mock
              </span>
            )}
            <span className="truncate text-xs font-semibold text-calyx-text">{tenantLabel}</span>
          </div>
        )}
      </div>
    </aside>
  );
}
