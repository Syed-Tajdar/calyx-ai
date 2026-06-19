'use client';

import { FormEvent, useEffect, useState } from 'react';
import { useRouter } from 'next/navigation';
import { Eye, EyeOff, Lock, Mail } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { DesignBackground } from '@/components/ui/DesignBackground';
import { Logo } from '@/components/ui/Logo';
import { getToken, setToken } from '@/lib/auth';
import { login } from '@/lib/api';

const IS_DEV = process.env.NODE_ENV === 'development';

export default function LoginPage(): JSX.Element {
  const router = useRouter();
  const [email, setEmail] = useState(IS_DEV ? 'admin@callloom.dev' : '');
  const [password, setPassword] = useState(IS_DEV ? 'callloom_dev' : '');
  const [showPassword, setShowPassword] = useState(false);
  const [remember, setRemember] = useState(true);
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    if (getToken()) {
      router.replace('/dashboard');
    }
  }, [router]);

  async function handleSubmit(e: FormEvent): Promise<void> {
    e.preventDefault();
    setLoading(true);
    setError('');
    try {
      const token = await login(email, password);
      setToken(token, remember);
      router.push('/dashboard');
    } catch {
      setError('Invalid email or password');
    } finally {
      setLoading(false);
    }
  }

  return (
    <div className="relative flex min-h-screen items-center justify-center bg-calyx-bg p-4">
      <DesignBackground />
      <Card className="relative z-10 w-full max-w-md border-calyx-border bg-calyx-card/95 backdrop-blur-sm">
        <CardHeader className="items-center text-center">
          <Logo className="mb-2" />
          <CardTitle className="text-xl">Sign in to Calyx AI</CardTitle>
          <CardDescription>
            Pay-per-call analytics, TCPA compliance intelligence, and Ask AI reporting.
          </CardDescription>
        </CardHeader>
        <CardContent>
          <form onSubmit={handleSubmit} className="space-y-4">
            <div>
              <label htmlFor="email" className="mb-1.5 block text-xs font-medium text-calyx-muted">
                Email
              </label>
              <div className="relative">
                <Mail className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-calyx-muted" />
                <Input
                  id="email"
                  type="email"
                  className="pl-10"
                  placeholder="you@company.com"
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                  required
                  autoComplete="email"
                />
              </div>
            </div>
            <div>
              <label htmlFor="password" className="mb-1.5 block text-xs font-medium text-calyx-muted">
                Password
              </label>
              <div className="relative">
                <Lock className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-calyx-muted" />
                <Input
                  id="password"
                  type={showPassword ? 'text' : 'password'}
                  className="pl-10 pr-10"
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  required
                  autoComplete="current-password"
                />
                <button
                  type="button"
                  onClick={() => setShowPassword((v) => !v)}
                  className="absolute right-3 top-1/2 -translate-y-1/2 text-calyx-muted hover:text-calyx-text"
                  aria-label={showPassword ? 'Hide password' : 'Show password'}
                >
                  {showPassword ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
                </button>
              </div>
            </div>
            <label className="flex cursor-pointer items-center gap-2 text-sm text-calyx-muted">
              <input
                type="checkbox"
                checked={remember}
                onChange={(e) => setRemember(e.target.checked)}
                className="h-4 w-4 rounded border-calyx-border accent-calyx-accent"
              />
              Remember me
            </label>
            {error && (
              <p className="rounded-lg border border-calyx-danger/30 bg-calyx-danger/10 px-3 py-2 text-sm text-calyx-danger">
                {error}
              </p>
            )}
            <Button type="submit" disabled={loading} className="w-full">
              {loading ? 'Signing in…' : 'Sign in'}
            </Button>
          </form>
        </CardContent>
      </Card>
    </div>
  );
}
