import type { Metadata } from 'next';
import { Inter } from 'next/font/google';
import { GeistSans } from 'geist/font/sans';
import type { ReactNode } from 'react';
import { Toaster } from 'sonner';
import { ThemeProvider } from '@/lib/theme-provider';
import './globals.css';

const inter = Inter({
  subsets: ['latin'],
  weight: ['400', '500', '600'],
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

export default function RootLayout({
  children,
}: Readonly<{
  children: ReactNode;
}>): JSX.Element {
  return (
    <html lang="en" className={`dark ${GeistSans.variable} ${inter.variable}`} suppressHydrationWarning>
      <body className="font-sans antialiased">
        <ThemeProvider>
          {children}
          <Toaster theme="dark" position="top-right" richColors closeButton />
        </ThemeProvider>
      </body>
    </html>
  );
}
