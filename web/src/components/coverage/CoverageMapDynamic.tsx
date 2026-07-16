'use client';

import dynamic from 'next/dynamic';
import type { HeatmapStateRow } from '@/lib/api';
import type { CountryFilter } from '@/components/coverage/CoverageMap';

interface CoverageMapDynamicProps {
  data: HeatmapStateRow[];
  selectedState: string | null;
  onSelectState: (code: string | null) => void;
  totalPings: number;
  countryFilter?: CountryFilter;
  onCountryFilterChange?: (filter: CountryFilter) => void;
}

const CoverageMap = dynamic(
  () => import('@/components/coverage/CoverageMap').then((m) => m.CoverageMap),
  {
    ssr: false,
    loading: () => (
      <div className="flex min-h-[420px] items-center justify-center text-sm text-calyx-muted">
        Loading map…
      </div>
    ),
  },
);

export function CoverageMapDynamic(props: CoverageMapDynamicProps): JSX.Element {
  return <CoverageMap {...props} />;
}
