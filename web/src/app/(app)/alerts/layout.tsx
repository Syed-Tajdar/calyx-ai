import { notFound } from 'next/navigation';
import { isRouteVisible } from '@/lib/v1-features';

/**
 * Alert centre (v2 Phase 2D), behind NEXT_PUBLIC_FEATURE_WORKFLOWS (lib/v1-features.ts). The HTTP 404
 * status comes from middleware.ts; this is the backstop so the page can never render when closed.
 */
export default function AlertsLayout({
  children,
}: {
  children: React.ReactNode;
}): React.ReactNode {
  if (!isRouteVisible('/alerts')) notFound();
  return children;
}
