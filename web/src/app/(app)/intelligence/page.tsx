'use client';

import Link from 'next/link';
import { useRouter } from 'next/navigation';
import { AppHeader } from '@/components/dashboard/AppHeader';
import { SectionHeader } from '@/components/dashboard/SectionHeader';
import { IntelligenceHubCards } from '@/components/intelligence/IntelligenceHubCards';

export default function IntelligencePage(): JSX.Element {
  const router = useRouter();

  return (
    <>
      <AppHeader />
      <SectionHeader title="Phase 3 Intelligence" />
      <p className="mb-4 text-sm text-loom-muted">
        Ask AI, TCPA compliance, and ad attribution — all tenant-scoped and mock-ready.
      </p>
      <IntelligenceHubCards />
      <p className="mt-4 text-center">
        <Link href="/reports" className="text-xs text-loom-muted hover:text-loom-accent">
          Financial reports & coverage →
        </Link>
      </p>
      <button
        type="button"
        onClick={() => router.push('/dashboard')}
        className="mt-6 text-xs text-loom-accent hover:underline"
      >
        ← Back to dashboard
      </button>
    </>
  );
}
