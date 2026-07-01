import type { PublisherQuality } from '@/components/ui/StatusPill';

export interface DemoPublisherRow {
  name: string;
  uniqueRate: number;
  duplicateRate: number;
  avgRevenue: number;
  quality: PublisherQuality;
}

export const DEMO_PUBLISHERS: DemoPublisherRow[] = [
  { name: 'MediaCo', uniqueRate: 94, duplicateRate: 6, avgRevenue: 18.4, quality: 'Good' },
  { name: 'LeadStream', uniqueRate: 88, duplicateRate: 12, avgRevenue: 14.2, quality: 'Good' },
  { name: 'DialPro', uniqueRate: 71, duplicateRate: 29, avgRevenue: 9.75, quality: 'Watch' },
  { name: 'CallStream', uniqueRate: 52, duplicateRate: 48, avgRevenue: 6.1, quality: 'Risk' },
];
