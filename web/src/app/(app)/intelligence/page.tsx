'use client';

import Link from 'next/link';
import { IntelligenceHubCards } from '@/components/intelligence/IntelligenceHubCards';
import { IntelligenceSubNav } from '@/components/intelligence/IntelligenceSubNav';
import { PageHeader } from '@/components/ui/PageHeader';

export default function IntelligencePage(): JSX.Element {
  return (
    <div className="space-y-6">
      <PageHeader
        title="Calyx Intelligence"
        description="TCPA compliance moat, natural-language Ask AI, and ad attribution — tenant-scoped and mock-ready."
      />
      <IntelligenceSubNav />
      <IntelligenceHubCards />
      <p className="text-center">
        <Link href="/reports" className="text-xs text-calyx-muted transition hover:text-calyx-accent">
          Financial reports →
        </Link>
      </p>
    </div>
  );
}
