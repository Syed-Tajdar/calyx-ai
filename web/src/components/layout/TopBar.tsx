'use client';

import { useRouter } from 'next/navigation';
import { FormEvent, useEffect, useState } from 'react';
import { LogOut, Search, X } from 'lucide-react';
import { Logo } from '@/components/ui/Logo';
import { Avatar, AvatarFallback } from '@/components/ui/avatar';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import { ApiError, fetchMe, MeResult } from '@/lib/api';
import { getToken } from '@/lib/auth';
import { decodeJwtPayload } from '@/lib/jwt';

interface TopBarProps {
  onLogout: () => void;
}

export function TopBar({ onLogout }: TopBarProps): JSX.Element {
  const router = useRouter();
  const token = getToken();
  const payload = token ? decodeJwtPayload(token) : null;
  const initials = payload?.email?.slice(0, 2).toUpperCase() ?? 'CA';
  const [me, setMe] = useState<MeResult | null>(null);
  const [search, setSearch] = useState('');
  const [mobileSearchOpen, setMobileSearchOpen] = useState(false);

  useEffect(() => {
    if (!token) return;
    fetchMe(token)
      .then(setMe)
      .catch((err: unknown) => {
        if (err instanceof ApiError && err.status === 401) {
          onLogout();
        }
      });
  }, [token, onLogout]);

  function handleSearch(e: FormEvent): void {
    e.preventDefault();
    const trimmed = search.trim();
    if (!trimmed) return;
    setMobileSearchOpen(false);
    router.push(`/calls/${encodeURIComponent(trimmed)}`);
  }

  const tenantLabel = me?.tenant_name ?? payload?.tenant_id?.slice(0, 8) ?? '—';

  const searchForm = (
    <form onSubmit={handleSearch} className="w-full">
      <div className="relative w-full">
        <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-calyx-muted" />
        <Input
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          placeholder="Search by call ID…"
          className="pl-9"
        />
      </div>
    </form>
  );

  return (
    <>
      <header
        className="calyx-glass sticky top-0 z-40 flex items-center justify-between gap-4 border-b border-calyx-border/60 px-4 lg:px-6"
        style={{ height: 'var(--topbar-height)' }}
      >
        <Logo variant="compact" className="lg:hidden" />

        <div className="hidden max-w-md flex-1 lg:block">{searchForm}</div>

        <div className="flex flex-1 items-center justify-end gap-1 lg:flex-none">
          <Button
            type="button"
            variant="ghost"
            size="icon"
            className="lg:hidden"
            onClick={() => setMobileSearchOpen(true)}
            aria-label="Search calls"
          >
            <Search className="h-4 w-4" />
          </Button>

          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button variant="ghost" className="h-9 gap-2 px-2">
                <Avatar className="h-8 w-8">
                  <AvatarFallback>{initials}</AvatarFallback>
                </Avatar>
                <span className="hidden max-w-[140px] truncate text-sm text-calyx-muted sm:inline">
                  {payload?.email ?? 'Account'}
                </span>
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end" className="w-56">
              <DropdownMenuLabel className="font-normal">
                <p className="text-sm font-medium text-calyx-text">{payload?.email}</p>
                <p className="text-xs text-calyx-muted">{tenantLabel}</p>
              </DropdownMenuLabel>
              <DropdownMenuSeparator />
              <DropdownMenuItem onClick={onLogout} className="text-calyx-danger focus:text-calyx-danger">
                <LogOut className="mr-2 h-4 w-4" />
                Log out
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        </div>
      </header>

      {mobileSearchOpen && (
        <div className="fixed inset-0 z-50 flex flex-col bg-calyx-bg/98 p-4 backdrop-blur-md lg:hidden">
          <div className="mb-4 flex items-center justify-between">
            <p className="text-sm font-semibold text-calyx-text">Search by call ID</p>
            <Button
              type="button"
              variant="ghost"
              size="icon"
              onClick={() => setMobileSearchOpen(false)}
              aria-label="Close search"
            >
              <X className="h-5 w-5" />
            </Button>
          </div>
          {searchForm}
        </div>
      )}
    </>
  );
}
