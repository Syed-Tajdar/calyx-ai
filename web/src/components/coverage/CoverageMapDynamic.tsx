'use client';

import dynamic from 'next/dynamic';
import { CoverageMap } from '@/components/coverage/CoverageMap';

export const CoverageMapDynamic = dynamic(
  () => Promise.resolve({ default: CoverageMap }),
  {
    ssr: false,
    loading: () => (
      <div className="flex min-h-[400px] w-full items-center justify-center py-8">
        <p className="text-sm text-calyx-muted">Loading map…</p>
      </div>
    ),
  },
);
