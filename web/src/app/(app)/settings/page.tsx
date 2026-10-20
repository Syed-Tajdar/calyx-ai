'use client';

import { useEffect, useState } from 'react';
import { useRouter } from 'next/navigation';
import { AddIntegrationForm } from '@/components/settings/AddIntegrationForm';
import { IntegrationsList } from '@/components/settings/IntegrationsList';
import { ProviderBindingsSection } from '@/components/settings/ProviderBindingsSection';
import { WorkflowAlertsSection } from '@/components/settings/WorkflowAlertsSection';
import { ErrorBanner, PageLoader } from '@/components/ui/PageStatus';
import { PageHeader } from '@/components/ui/PageHeader';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { ApiError, fetchIntegrations, IntegrationRecord } from '@/lib/api';
import { getToken } from '@/lib/auth';
import { isFeatureEnabled, V1_ONLY } from '@/lib/v1-features';

/** Alerts tab = v2 Phase 2D Workflows (replaces the old mock alert rules). */
const SHOW_ALERTS = isFeatureEnabled('workflows');

export default function SettingsPage(): JSX.Element {
  const router = useRouter();
  const [integrations, setIntegrations] = useState<IntegrationRecord[]>([]);
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    const token = getToken();
    if (!token) return;

    fetchIntegrations(token)
      .then((integrationsRes) => setIntegrations(integrationsRes))
      .catch((err: unknown) => {
        if (err instanceof ApiError && err.status === 401) {
          router.replace('/login');
          return;
        }
        setError('Failed to load settings');
      })
      .finally(() => setLoading(false));
  }, [router]);

  function handleIntegrationDeleted(id: string): void {
    setIntegrations((prev) => prev.filter((i) => i.id !== id));
  }

  function handleIntegrationCreated(record: IntegrationRecord): void {
    setIntegrations((prev) => [record, ...prev]);
  }

  async function refreshIntegrations(): Promise<void> {
    const token = getToken();
    if (!token) return;
    try {
      const list = await fetchIntegrations(token);
      setIntegrations(list);
    } catch {
      /* ignore */
    }
  }

  return (
    <div className="space-y-8">
      <PageHeader
        title="Settings"
        description={
          V1_ONLY
            ? SHOW_ALERTS
              ? 'Integrations and alert rules for your tenant.'
              : 'Integrations for your tenant.'
            : 'Integrations, alert rules, and provider bindings for your tenant.'
        }
      />

      {loading && <PageLoader />}
      {error && <ErrorBanner message={error} />}

      {!loading && !error && (
        <Tabs defaultValue="integrations">
          <TabsList>
            <TabsTrigger value="integrations">Integrations</TabsTrigger>
            {SHOW_ALERTS && <TabsTrigger value="alerts">Alerts</TabsTrigger>}
            {!V1_ONLY && <TabsTrigger value="providers">Providers</TabsTrigger>}
          </TabsList>
          <TabsContent value="integrations" className="space-y-6">
            <AddIntegrationForm onCreated={handleIntegrationCreated} />
            <IntegrationsList
              integrations={integrations}
              onDeleted={handleIntegrationDeleted}
              onUpdated={() => void refreshIntegrations()}
            />
          </TabsContent>
          {SHOW_ALERTS && (
            <TabsContent value="alerts">
              <WorkflowAlertsSection />
            </TabsContent>
          )}
          {!V1_ONLY && (
            <TabsContent value="providers">
              <ProviderBindingsSection />
            </TabsContent>
          )}
        </Tabs>
      )}
    </div>
  );
}
