'use client';

import { useEffect, useState } from 'react';
import { useRouter } from 'next/navigation';
import { AppHeader } from '@/components/dashboard/AppHeader';
import { SectionHeader } from '@/components/dashboard/SectionHeader';
import { AlertRulesSection } from '@/components/settings/AlertRulesSection';
import { IntegrationsList } from '@/components/settings/IntegrationsList';
import { ErrorBanner, PageLoader } from '@/components/ui/PageStatus';
import {
  AlertRule,
  ApiError,
  fetchAlertRules,
  fetchIntegrations,
  IntegrationRecord,
} from '@/lib/api';
import { getToken } from '@/lib/auth';

export default function SettingsPage(): JSX.Element {
  const router = useRouter();
  const [integrations, setIntegrations] = useState<IntegrationRecord[]>([]);
  const [rules, setRules] = useState<AlertRule[]>([]);
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    const token = getToken();
    if (!token) return;

    Promise.all([fetchIntegrations(token), fetchAlertRules(token)])
      .then(([integrationsRes, rulesRes]) => {
        setIntegrations(integrationsRes);
        setRules(rulesRes.data);
      })
      .catch((err: unknown) => {
        if (err instanceof ApiError && err.status === 401) {
          router.replace('/login');
          return;
        }
        setError('Failed to load settings');
      })
      .finally(() => setLoading(false));
  }, [router]);

  function handleRuleCreated(rule: AlertRule): void {
    setRules((prev) => [rule, ...prev]);
  }

  return (
    <>
      <AppHeader />
      <SectionHeader title="Settings" />

      {loading && <PageLoader />}
      {error && <ErrorBanner message={error} />}

      {!loading && !error && (
        <div className="space-y-6">
          <IntegrationsList integrations={integrations} />
          <AlertRulesSection rules={rules} onCreated={handleRuleCreated} />
        </div>
      )}
    </>
  );
}
