import type { Metadata } from 'next';
import { GeistSans } from 'geist/font/sans';
import type { ReactNode } from 'react';
import { Toaster } from 'sonner';
import { ThemeProvider } from '@/lib/theme-provider';
import './globals.css';

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
 * (Also avoids next/font/google network fetch during Docker builds.)
 */
export const dynamic = 'force-dynamic';

export default function RootLayout({
  children,
}: Readonly<{
  children: ReactNode;
}>): JSX.Element {
  return (
    <html lang="en" className={`dark ${GeistSans.variable}`} suppressHydrationWarning>
      <body className="font-sans antialiased">
        <ThemeProvider>
          {children}
          <Toaster theme="dark" position="top-right" richColors closeButton />
        </ThemeProvider>
      </body>
    </html>
  );
}
