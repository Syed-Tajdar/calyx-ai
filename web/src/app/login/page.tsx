'use client';

import Image from 'next/image';
import Link from 'next/link';
import { FormEvent, useState } from 'react';
import { useRouter } from 'next/navigation';
import { DesignBackground } from '@/components/ui/DesignBackground';
import { EyeIcon, GoogleIcon, LockIcon } from '@/components/ui/Icons';
import { Logo } from '@/components/ui/Logo';
import { login } from '@/lib/api';

export default function LoginPage(): JSX.Element {
  const router = useRouter();
  const [email, setEmail] = useState('admin@callloom.dev');
  const [password, setPassword] = useState('callloom_dev');
  const [showPassword, setShowPassword] = useState(false);
  const [remember, setRemember] = useState(true);
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);

  async function handleSubmit(e: FormEvent): Promise<void> {
    e.preventDefault();
    setLoading(true);
    setError('');
    try {
      const token = await login(email, password);
      if (remember) {
        localStorage.setItem('callloom_token', token);
      } else {
        sessionStorage.setItem('callloom_token', token);
      }
      router.push('/dashboard');
    } catch {
      setError('Invalid email or password');
    } finally {
      setLoading(false);
    }
  }

  return (
    <div className="relative min-h-screen overflow-hidden bg-loom-bg">
      <DesignBackground />

      <div className="relative z-10 flex min-h-screen">
        {/* Left — design illustration (cropped from sign-in PNG) */}
        <div className="relative hidden w-[52%] overflow-hidden lg:block">
          <Image
            src="/auth-character.png"
            alt=""
            fill
            className="object-cover object-left"
            priority
            sizes="52vw"
          />
        </div>

        {/* Right — login form matching design */}
        <div className="flex w-full flex-col lg:w-[48%]">
          <div className="p-6 lg:hidden">
            <Logo />
          </div>

          <div className="flex flex-1 flex-col justify-center px-6 py-10 sm:px-12 lg:px-16 xl:px-20">
            <div className="mx-auto w-full max-w-md">
              <h1 className="text-3xl font-bold tracking-tight text-white sm:text-4xl">
                Welcome back!
              </h1>
              <p className="mt-2 text-sm text-loom-muted">Good to see you again.</p>

              <form onSubmit={handleSubmit} className="mt-8 space-y-5">
                <div>
                  <label htmlFor="email" className="mb-2 block text-sm text-loom-muted">
                    Email Address
                  </label>
                  <input
                    id="email"
                    type="email"
                    className="loom-input"
                    placeholder="e.g.john@gmail.com"
                    value={email}
                    onChange={(e) => setEmail(e.target.value)}
                    required
                    autoComplete="email"
                  />
                </div>

                <div>
                  <label htmlFor="password" className="mb-2 block text-sm text-loom-muted">
                    Password
                  </label>
                  <div className="relative">
                    <span className="pointer-events-none absolute left-4 top-1/2 -translate-y-1/2 text-loom-muted">
                      <LockIcon />
                    </span>
                    <input
                      id="password"
                      type={showPassword ? 'text' : 'password'}
                      className="loom-input-icon-left pr-12"
                      value={password}
                      onChange={(e) => setPassword(e.target.value)}
                      required
                      autoComplete="current-password"
                    />
                    <button
                      type="button"
                      onClick={() => setShowPassword((v) => !v)}
                      className="absolute right-4 top-1/2 -translate-y-1/2 text-loom-muted transition hover:text-white"
                      aria-label={showPassword ? 'Hide password' : 'Show password'}
                    >
                      <EyeIcon />
                    </button>
                  </div>
                  <p className="mt-2 text-[11px] leading-relaxed text-loom-muted/80">
                    Must be at least 8 characters, at least one uppercase letter, one number, one
                    special character
                  </p>
                </div>

                <div className="flex items-center justify-between text-sm">
                  <label className="flex cursor-pointer items-center gap-2 text-loom-muted">
                    <input
                      type="checkbox"
                      checked={remember}
                      onChange={(e) => setRemember(e.target.checked)}
                      className="h-4 w-4 rounded border-loom-border bg-loom-surface accent-loom-accent"
                    />
                    Remember me
                  </label>
                  <Link href="#" className="text-loom-muted transition hover:text-loom-accent">
                    Forget Password?
                  </Link>
                </div>

                {error && (
                  <p className="rounded-xl border border-red-500/30 bg-red-500/10 px-4 py-3 text-sm text-red-300">
                    {error}
                  </p>
                )}

                <button type="submit" disabled={loading} className="loom-btn-primary">
                  {loading ? 'Signing in…' : 'Sign in'}
                </button>
              </form>

              <p className="mt-5 text-center text-sm text-loom-muted">
                <Link href="#" className="transition hover:text-loom-accent">
                  Terms &amp; Privacy
                </Link>
              </p>

              <p className="mt-3 text-center text-sm text-loom-muted">
                Don&apos;t have an account?{' '}
                <Link
                  href="#"
                  className="font-medium text-loom-accent transition hover:text-loom-accent-hover"
                >
                  Sign up
                </Link>
              </p>

              <div className="mt-8 flex items-center gap-4">
                <div className="h-px flex-1 bg-loom-border/60" />
                <span className="text-xs text-loom-muted">or</span>
                <div className="h-px flex-1 bg-loom-border/60" />
              </div>

              <button type="button" disabled className="loom-btn-google mt-6 opacity-70">
                <GoogleIcon />
                Continue with Google
              </button>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
