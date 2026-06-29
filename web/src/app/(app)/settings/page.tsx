'use client';

import { useEffect, useState } from 'react';
import { useRouter } from 'next/navigation';
import { AlertRulesSection } from '@/components/settings/AlertRulesSection';
import { IntegrationsList } from '@/components/settings/IntegrationsList';
import { ProviderBindingsSection } from '@/components/settings/ProviderBindingsSection';
import { ErrorBanner, PageLoader } from '@/components/ui/PageStatus';
import { PageHeader } from '@/components/ui/PageHeader';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
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

  function handleRuleDeleted(id: string): void {
    setRules((prev) => prev.filter((r) => r.id !== id));
  }

  function handleIntegrationDeleted(id: string): void {
    setIntegrations((prev) => prev.filter((i) => i.id !== id));
  }

  return (
    <div className="space-y-8">
      <PageHeader
        title="Settings"
        description="Integrations, alert rules, and provider bindings for your tenant."
      />

      {loading && <PageLoader />}
      {error && <ErrorBanner message={error} />}

      {!loading && !error && (
        <Tabs defaultValue="integrations">
          <TabsList>
            <TabsTrigger value="integrations">Integrations</TabsTrigger>
            <TabsTrigger value="alerts">Alerts</TabsTrigger>
            <TabsTrigger value="providers">Providers</TabsTrigger>
          </TabsList>
          <TabsContent value="integrations">
            <IntegrationsList
              integrations={integrations}
              onDeleted={handleIntegrationDeleted}
            />
          </TabsContent>
          <TabsContent value="alerts">
            <AlertRulesSection
              rules={rules}
              onCreated={handleRuleCreated}
              onDeleted={handleRuleDeleted}
            />
          </TabsContent>
          <TabsContent value="providers">
            <ProviderBindingsSection />
          </TabsContent>
        </Tabs>
      )}
    </div>
  );
}
