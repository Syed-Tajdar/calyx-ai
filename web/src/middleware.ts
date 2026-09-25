import { NextResponse } from 'next/server';
import type { NextRequest } from 'next/server';

/**
 * Security headers + CSP with per-request nonce for Next.js inline scripts.
 * Production forbids unsafe-eval; development allows it for React Refresh.
 */
export function middleware(request: NextRequest): NextResponse {
  const nonce = Buffer.from(crypto.randomUUID()).toString('base64');
  const isProd = process.env.APP_ENV === 'production';
  const api = process.env.NEXT_PUBLIC_API_URL ?? 'http://localhost:3000';
  const ws = process.env.NEXT_PUBLIC_WS_URL ?? api;
  const wsOrigin = ws.replace(/^http/, 'ws');

  const scriptSrc = isProd
    ? `'self' 'nonce-${nonce}' 'strict-dynamic'`
    : `'self' 'nonce-${nonce}' 'unsafe-eval' 'unsafe-inline'`;

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
