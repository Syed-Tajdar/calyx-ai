'use client';

import { useEffect, useState } from 'react';
import { useRouter } from 'next/navigation';
import Link from 'next/link';
import { ExposureSummaryCard } from '@/components/compliance/ExposureSummaryCard';
import { RiskScoresTable } from '@/components/compliance/RiskScoresTable';
import { ViolationsTable } from '@/components/compliance/ViolationsTable';
import { ComplianceRiskRing } from '@/components/calyx/ComplianceRiskRing';
import { IntelligenceSubNav } from '@/components/intelligence/IntelligenceSubNav';
import { ErrorBanner, PageLoader } from '@/components/ui/PageStatus';
import { PageHeader } from '@/components/ui/PageHeader';
import {
  ApiError,
  ExposureSummary,
  fetchComplianceExposure,
  fetchComplianceRiskScores,
  fetchComplianceViolations,
  RiskScoreRecord,
  ViolationRecord,
} from '@/lib/api';
import { getToken } from '@/lib/auth';
import { useDateFilter } from '@/lib/date-filter';
import { FilterBar } from '@/components/ui/FilterBar';

export default function CompliancePage(): JSX.Element {
  const router = useRouter();
  const { showDemo } = useDateFilter();
  const [exposure, setExposure] = useState<ExposureSummary | null>(null);
  const [scores, setScores] = useState<RiskScoreRecord[]>([]);
  const [violations, setViolations] = useState<ViolationRecord[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  useEffect(() => {
    const token = getToken();
    if (!token) return;

    setLoading(true);
    Promise.all([
      fetchComplianceExposure(token, { show_demo: showDemo || undefined }),
      fetchComplianceRiskScores(token, 25, { show_demo: showDemo || undefined }),
      fetchComplianceViolations(token, 25, { show_demo: showDemo || undefined }),
    ])
      .then(([exp, scoreRes, violationRes]) => {
        setExposure(exp);
        setScores(scoreRes.data);
        setViolations(violationRes.data);
      })
      .catch((err: unknown) => {
        if (err instanceof ApiError && err.status === 401) {
          router.replace('/login');
          return;
        }
        setError('Failed to load compliance data');
      })
      .finally(() => setLoading(false));
  }, [router, showDemo]);

  const avgRisk = exposure?.avg_risk_score ?? 0;

  return (
    <div className="space-y-8">
      <PageHeader
        title="TCPA Compliance"
        description="Risk scoring, DNC matches, and liability exposure — your competitive moat."
      />
      <IntelligenceSubNav />
      <FilterBar showDemoToggle />

      {loading && <PageLoader />}
      {error && <ErrorBanner message={error} />}

      {!loading && !error && (
        <>
          <div className="grid gap-6 lg:grid-cols-[auto_1fr]">
            <div className="calyx-glass calyx-glass-lit flex flex-col items-center justify-center p-6">
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
          <ViolationsTable violations={violations} />
        </>
      )}
    </div>
  );
}
