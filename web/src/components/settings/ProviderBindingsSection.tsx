'use client';

import { useEffect, useState } from 'react';
import { useRouter } from 'next/navigation';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { ErrorBanner, PageLoader } from '@/components/ui/PageStatus';
import {
  ApiError,
  createProviderBinding,
  fetchProviderBindings,
  fetchProvidersCatalog,
  ProviderBinding,
  ProviderCatalogEntry,
} from '@/lib/api';
import { getToken } from '@/lib/auth';

const DOMAINS = ['attribution', 'dnc', 'llm', 'enrichment'] as const;

export function ProviderBindingsSection(): JSX.Element {
  const router = useRouter();
  const [catalog, setCatalog] = useState<ProviderCatalogEntry[]>([]);
  const [bindings, setBindings] = useState<ProviderBinding[]>([]);
  const [domain, setDomain] = useState<string>(DOMAINS[0]);
  const [providerId, setProviderId] = useState('mock');
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState('');

  useEffect(() => {
    const token = getToken();
    if (!token) return;

    Promise.all([fetchProvidersCatalog(token), fetchProviderBindings(token)])
      .then(([catalogRes, bindingsRes]) => {
        setCatalog(catalogRes.providers);
        setBindings(bindingsRes.data);
      })
      .catch((err: unknown) => {
        if (err instanceof ApiError && err.status === 401) {
          router.replace('/login');
          return;
        }
        setError('Failed to load provider bindings');
      })
      .finally(() => setLoading(false));
  }, [router]);

  const domainProviders = catalog.filter((p) => p.domain === domain);

  async function handleCreate(e: React.FormEvent): Promise<void> {
    e.preventDefault();
    const token = getToken();
    if (!token) return;

    setSaving(true);
    setError('');
    try {
      const created = await createProviderBinding(token, {
        domain,
        provider_id: providerId,
        config_json: {},
      });
      setBindings((prev) => [created, ...prev]);
    } catch {
      setError('Failed to create binding');
    } finally {
      setSaving(false);
    }
  }

  if (loading) return <PageLoader />;
  if (error) return <ErrorBanner message={error} />;

  return (
    <div className="space-y-6">
      <form onSubmit={handleCreate} className="calyx-card space-y-4 p-4">
        <p className="text-sm font-semibold text-calyx-text">Add provider binding</p>
        <div className="grid gap-3 sm:grid-cols-3">
          <div>
            <label className="mb-1 block text-xs text-calyx-muted">Domain</label>
            <select
              value={domain}
              onChange={(e) => {
                setDomain(e.target.value);
                const first = catalog.find((p) => p.domain === e.target.value);
                setProviderId(first?.id ?? 'mock');
              }}
              className="w-full rounded-md border border-calyx-border bg-calyx-surface px-3 py-2 text-sm text-calyx-text"
            >
              {DOMAINS.map((d) => (
                <option key={d} value={d}>
                  {d}
                </option>
              ))}
            </select>
          </div>
          <div>
            <label className="mb-1 block text-xs text-calyx-muted">Provider</label>
            <select
              value={providerId}
              onChange={(e) => setProviderId(e.target.value)}
              className="w-full rounded-md border border-calyx-border bg-calyx-surface px-3 py-2 text-sm text-calyx-text"
            >
              {domainProviders.map((p) => (
                <option key={`${p.domain}-${p.id}`} value={p.id}>
                  {p.display_name}
                </option>
              ))}
            </select>
          </div>
          <div className="flex items-end">
            <Button type="submit" disabled={saving} className="w-full">
              {saving ? 'Saving…' : 'Add binding'}
            </Button>
          </div>
        </div>
      </form>

      <div className="calyx-card overflow-hidden">
        {bindings.length === 0 ? (
          <p className="p-4 text-sm text-calyx-muted">No provider bindings configured yet.</p>
        ) : (
          <table className="w-full text-left text-sm">
            <thead className="border-b border-calyx-border text-xs uppercase text-calyx-muted">
              <tr>
                <th className="px-4 py-3">Domain</th>
                <th className="px-4 py-3">Provider</th>
                <th className="px-4 py-3">Status</th>
                <th className="px-4 py-3">Priority</th>
              </tr>
            </thead>
            <tbody>
              {bindings.map((b) => (
                <tr key={b.id} className="border-b border-calyx-border/50">
                  <td className="px-4 py-3 capitalize">{b.domain}</td>
                  <td className="px-4 py-3 font-mono text-xs">{b.provider_id}</td>
                  <td className="px-4 py-3">
                    <Badge variant={b.is_active ? 'success' : 'secondary'}>
                      {b.is_active ? 'Active' : 'Inactive'}
                    </Badge>
                  </td>
                  <td className="px-4 py-3">{b.priority}</td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>
    </div>
  );
}
