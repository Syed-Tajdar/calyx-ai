'use client';

import { useEffect, useState } from 'react';
import { useRouter } from 'next/navigation';
import Link from 'next/link';
import { ExposureSummaryCard } from '@/components/compliance/ExposureSummaryCard';
import { RiskScoresTable } from '@/components/compliance/RiskScoresTable';
import { AppHeader } from '@/components/dashboard/AppHeader';
import { SectionHeader } from '@/components/dashboard/SectionHeader';
import { ErrorBanner, PageLoader } from '@/components/ui/PageStatus';
import {
  ApiError,
  ExposureSummary,
  fetchComplianceExposure,
  fetchComplianceRiskScores,
  RiskScoreRecord,
} from '@/lib/api';
import { getToken } from '@/lib/auth';

export default function CompliancePage(): JSX.Element {
  const router = useRouter();
  const [exposure, setExposure] = useState<ExposureSummary | null>(null);
  const [scores, setScores] = useState<RiskScoreRecord[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  useEffect(() => {
    const token = getToken();
    if (!token) return;

    Promise.all([fetchComplianceExposure(token), fetchComplianceRiskScores(token, 25)])
      .then(([exp, scoreRes]) => {
        setExposure(exp);
        setScores(scoreRes.data);
      })
      .catch((err: unknown) => {
        if (err instanceof ApiError && err.status === 401) {
          router.replace('/login');
          return;
        }
        setError('Failed to load compliance data');
      })
      .finally(() => setLoading(false));
  }, [router]);

  return (
    <>
      <AppHeader />
      <div className="mb-3 flex items-center justify-between">
        <SectionHeader title="TCPA Compliance" />
        <Link href="/intelligence" className="text-xs text-loom-accent hover:underline">
          Intelligence hub
        </Link>
      </div>

      {loading && <PageLoader />}
      {error && <ErrorBanner message={error} />}

      {!loading && !error && (
        <div className="space-y-4">
          <ExposureSummaryCard exposure={exposure} />
          <RiskScoresTable scores={scores} />
        </div>
      )}
    </>
  );
}
