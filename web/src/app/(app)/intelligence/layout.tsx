import { notFound } from 'next/navigation';
import { isRouteVisible } from '@/lib/v1-features';

/**
 * Out of v1 scope (lib/v1-features.ts). The HTTP 404 status comes from middleware.ts; this is the
 * backstop so the page can never render even if the middleware matcher changes.
 */
export default function IntelligenceLayout({
  children,
}: {
  children: React.ReactNode;
}): React.ReactNode {
  if (!isRouteVisible('/intelligence')) notFound();
  return children;
}
