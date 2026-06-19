'use client';

import { useRouter } from 'next/navigation';
import { useEffect, useState, FormEvent } from 'react';
import { LogOut, Moon, Search, Sun } from 'lucide-react';
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
import { useTheme } from '@/lib/theme-provider';

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
  const { theme, toggleTheme } = useTheme();

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
    router.push(`/calls/${encodeURIComponent(trimmed)}`);
  }

  const tenantLabel = me?.tenant_name ?? payload?.tenant_id?.slice(0, 8) ?? '—';

  return (
    <header className="sticky top-0 z-40 flex h-14 items-center justify-between gap-4 border-b border-calyx-border bg-calyx-bg/95 px-4 backdrop-blur-md lg:px-6">
      <Logo variant="compact" className="lg:hidden" />

      <form onSubmit={handleSearch} className="hidden max-w-md flex-1 lg:flex">
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

      <div className="flex items-center gap-1">
        <Button
          type="button"
          variant="ghost"
          size="icon"
          onClick={toggleTheme}
          aria-label={theme === 'dark' ? 'Switch to light mode' : 'Switch to dark mode'}
        >
          {theme === 'dark' ? <Sun className="h-4 w-4" /> : <Moon className="h-4 w-4" />}
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
  );
}
