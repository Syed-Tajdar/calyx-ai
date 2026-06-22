'use client';

import { useEffect, useState } from 'react';
import { useRouter } from 'next/navigation';
import { ExposureSummaryCard } from '@/components/compliance/ExposureSummaryCard';
import { RiskScoresTable } from '@/components/compliance/RiskScoresTable';
import { ComplianceRiskRing } from '@/components/calyx/ComplianceRiskRing';
import { IntelligenceSubNav } from '@/components/intelligence/IntelligenceSubNav';
import { ErrorBanner, PageLoader } from '@/components/ui/PageStatus';
import { PageHeader } from '@/components/ui/PageHeader';
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

  const avgRisk = exposure?.avg_risk_score ?? 0;

  return (
    <div className="space-y-8">
      <PageHeader
        title="TCPA Compliance"
        description="Risk scoring, DNC matches, and liability exposure — your competitive moat."
      />
      <IntelligenceSubNav />

      {loading && <PageLoader />}
      {error && <ErrorBanner message={error} />}

      {!loading && !error && (
        <>
          <div className="grid gap-6 lg:grid-cols-[auto_1fr]">
            <div className="calyx-card flex flex-col items-center justify-center p-6">
              <ComplianceRiskRing
                score={avgRisk}
                exposureUsd={exposure?.total_exposure_usd ?? 0}
                size="md"
              />
              <p className="mt-3 text-xs text-calyx-muted">Portfolio risk score</p>
            </div>
            <ExposureSummaryCard exposure={exposure} />
          </div>
          <RiskScoresTable scores={scores} />
        </>
      )}
    </div>
  );
}
