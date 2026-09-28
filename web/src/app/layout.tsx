import type { Metadata } from 'next';
import localFont from 'next/font/local';
import type { ReactNode } from 'react';
import { Toaster } from 'sonner';
import { ThemeProvider } from '@/lib/theme-provider';
import './globals.css';

/**
 * Local OFL fonts — no network fetch at Docker build time.
 * Geist (Vercel, SIL OFL 1.1) for headings; Inter (Rasmus Andersson, SIL OFL 1.1) for body.
 * CSS variables must match globals.css (--font-geist, --font-inter).
 */
const geistHeading = localFont({
  src: [
    { path: '../fonts/geist/Geist-Regular.woff2', weight: '400', style: 'normal' },
    { path: '../fonts/geist/Geist-Medium.woff2', weight: '500', style: 'normal' },
    { path: '../fonts/geist/Geist-SemiBold.woff2', weight: '600', style: 'normal' },
    { path: '../fonts/geist/Geist-Bold.woff2', weight: '700', style: 'normal' },
  ],
  variable: '--font-geist',
  display: 'swap',
});

const interBody = localFont({
  src: [
    { path: '../fonts/inter/Inter-Regular.woff2', weight: '400', style: 'normal' },
    { path: '../fonts/inter/Inter-Medium.woff2', weight: '500', style: 'normal' },
    { path: '../fonts/inter/Inter-SemiBold.woff2', weight: '600', style: 'normal' },
    { path: '../fonts/inter/Inter-Bold.woff2', weight: '700', style: 'normal' },
  ],
  variable: '--font-inter',
  display: 'swap',
});

export const metadata: Metadata = {
  title: 'Calyx AI',
  description: 'Pay-per-call analytics and intelligence platform',
};

export const viewport = {
  width: 'device-width',
  initialScale: 1,
  maximumScale: 1,
};

/**
 * CSP nonces are applied during SSR from the request CSP header (middleware).
 * Static prerender has no per-request nonce — force dynamic for every page.
 */
export const dynamic = 'force-dynamic';

export default function RootLayout({
  children,
}: Readonly<{
  children: ReactNode;
}>): JSX.Element {
  return (
    <html
      lang="en"
      className={`dark ${geistHeading.variable} ${interBody.variable}`}
      suppressHydrationWarning
    >
      <body className="font-sans antialiased">
        <ThemeProvider>
          {children}
          <Toaster theme="dark" position="top-right" richColors closeButton />
        </ThemeProvider>
      </body>
    </html>
  );
}
