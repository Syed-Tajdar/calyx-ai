import { NextResponse } from 'next/server';
import type { NextRequest } from 'next/server';

/**
 * CSP with per-request nonce for Next.js App Router scripts.
 *
 * Next extracts the nonce from the *request* Content-Security-Policy header
 * and stamps it onto framework / flight inline scripts during SSR. Setting CSP
 * only on the response (as we did before) leaves those scripts un-nonced; when
 * script-src also lists a nonce, browsers ignore 'unsafe-inline' and block them.
 *
 * Pages must be dynamically rendered (see app/layout.tsx `force-dynamic`) so
 * SSR sees the request CSP. HTML is Cache-Control: no-store so a 304/cache HIT
 * cannot reuse a stale nonce.
 */
export function middleware(request: NextRequest): NextResponse {
  const nonce = Buffer.from(crypto.randomUUID()).toString('base64');
  const isProd = process.env.APP_ENV === 'production';
  const api = process.env.NEXT_PUBLIC_API_URL ?? 'http://localhost:3000';
  const ws = process.env.NEXT_PUBLIC_WS_URL ?? api;
  const wsOrigin = ws.replace(/^http/, 'ws');

  // No 'unsafe-inline' for scripts — nonce + strict-dynamic only.
  // Dev keeps 'unsafe-eval' for React Refresh / webpack HMR.
  const scriptSrc = isProd
    ? `'self' 'nonce-${nonce}' 'strict-dynamic'`
    : `'self' 'nonce-${nonce}' 'strict-dynamic' 'unsafe-eval'`;

  const csp = [
    `default-src 'self'`,
    `script-src ${scriptSrc}`,
    `style-src 'self' 'unsafe-inline'`,
    `img-src 'self' data: blob:`,
    `font-src 'self' data:`,
    `connect-src 'self' ${api} ${ws} ${wsOrigin}`,
    `frame-ancestors 'none'`,
    `base-uri 'self'`,
    `form-action 'self'`,
    `object-src 'none'`,
  ].join('; ');

  const requestHeaders = new Headers(request.headers);
  requestHeaders.set('x-nonce', nonce);
  // Required: Next reads the nonce from the request CSP during render.
  requestHeaders.set('Content-Security-Policy', csp);

  const res = NextResponse.next({
    request: { headers: requestHeaders },
  });
  res.headers.set('Content-Security-Policy', csp);
  res.headers.set('X-Content-Type-Options', 'nosniff');
  res.headers.set('Referrer-Policy', 'strict-origin-when-cross-origin');
  res.headers.set(
    'Permissions-Policy',
    'camera=(), microphone=(), geolocation=(), payment=(), usb=()',
  );
  res.headers.set('X-Frame-Options', 'DENY');
  // Never cache HTML that embeds a one-shot nonce (avoids 304 + stale nonce).
  res.headers.set('Cache-Control', 'no-store, no-cache, must-revalidate, private');
  res.headers.set('Pragma', 'no-cache');
  if (isProd) {
    res.headers.set(
      'Strict-Transport-Security',
      'max-age=31536000; includeSubDomains',
    );
  }
  return res;
}

export const config = {
  matcher: [
    '/((?!_next/static|_next/image|favicon.ico|.*\\.(?:svg|png|jpg|jpeg|gif|webp)$).*)',
  ],
};
