'use client';

import { useEffect, useState } from 'react';
import { useRouter } from 'next/navigation';
import { Pencil, Play, Trash2 } from 'lucide-react';
import { toast } from 'sonner';
import { Button } from '@/components/ui/button';
import { ErrorBanner, PageLoader } from '@/components/ui/PageStatus';
import { ProviderStatusBadge } from '@/components/ui/StatusPill';
import {
  ApiError,
  createProviderBinding,
  deleteProviderBinding,
  fetchProviderBindings,
  fetchProvidersCatalog,
  ProviderBinding,
  ProviderCatalogEntry,
  testProviderBinding,
  updateProviderBinding,
} from '@/lib/api';
import { getToken } from '@/lib/auth';
import {
  bindingIsActive,
  bindingSecretsLabel,
  findBindingForRow,
  PROVIDER_DOMAIN_ROWS,
} from '@/lib/provider-domains';

const FORM_DOMAINS = ['attribution', 'dnc', 'llm', 'enrichment'] as const;

export function ProviderBindingsSection(): JSX.Element {
  const router = useRouter();
  const [catalog, setCatalog] = useState<ProviderCatalogEntry[]>([]);
  const [bindings, setBindings] = useState<ProviderBinding[]>([]);
  const [domain, setDomain] = useState<string>(FORM_DOMAINS[0]);
  const [providerId, setProviderId] = useState('mock');
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState('');
  const [editingId, setEditingId] = useState<string | null>(null);
  const [editPriority, setEditPriority] = useState('0');
  const [testingId, setTestingId] = useState<string | null>(null);
  const [deletingId, setDeletingId] = useState<string | null>(null);

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
      toast.success('Binding created');
    } catch {
      setError('Failed to create binding');
    } finally {
      setSaving(false);
    }
  }

  async function handleTest(binding: ProviderBinding): Promise<void> {
    const token = getToken();
    if (!token) return;
    setTestingId(binding.id);
    try {
      const result = await testProviderBinding(token, binding.id, binding.domain);
      toast.success(`Test OK: ${JSON.stringify(result).slice(0, 80)}`);
    } catch {
      toast.error('Binding test failed');
    } finally {
      setTestingId(null);
    }
  }

  async function handleSaveEdit(binding: ProviderBinding): Promise<void> {
    const token = getToken();
    if (!token) return;
    setSaving(true);
    try {
      const updated = await updateProviderBinding(token, binding.id, {
        priority: Number(editPriority),
      });
      setBindings((prev) => prev.map((b) => (b.id === updated.id ? updated : b)));
      setEditingId(null);
      toast.success('Binding updated');
    } catch {
      toast.error('Failed to update binding');
    } finally {
      setSaving(false);
    }
  }

  async function handleDelete(binding: ProviderBinding): Promise<void> {
    if (!window.confirm(`Remove ${binding.domain} binding for ${binding.provider_id}?`)) return;
    const token = getToken();
    if (!token) return;
    setDeletingId(binding.id);
    try {
      await deleteProviderBinding(token, binding.id);
      setBindings((prev) => prev.filter((b) => b.id !== binding.id));
      toast.success('Binding removed');
    } catch (err: unknown) {
      if (err instanceof ApiError && err.status === 404) {
        setBindings((prev) => prev.filter((b) => b.id !== binding.id));
        toast.message('Binding was already removed');
      } else {
        toast.error('Failed to delete binding');
      }
    } finally {
      setDeletingId(null);
    }
  }

  function prefillDomain(rowDomain: string): void {
    const formDomain = rowDomain === 'transcription' ? 'enrichment' : rowDomain;
    if (FORM_DOMAINS.includes(formDomain as (typeof FORM_DOMAINS)[number])) {
      setDomain(formDomain);
      const first = catalog.find((p) => p.domain === formDomain);
      setProviderId(first?.id ?? 'mock');
    }
  }

  if (loading) return <PageLoader />;
  if (error) return <ErrorBanner message={error} />;

  return (
    <div className="space-y-6">
      <form onSubmit={handleCreate} className="calyx-glass calyx-glass-lit space-y-4 p-4">
        <p className="font-heading text-sm font-semibold text-calyx-text">Add provider binding</p>
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
              {FORM_DOMAINS.map((d) => (
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

      <div className="calyx-glass calyx-glass-lit overflow-hidden">
        <table className="w-full text-left text-sm">
          <thead className="border-b border-calyx-border text-xs uppercase text-calyx-muted">
            <tr>
              <th className="px-4 py-3">Domain</th>
              <th className="px-4 py-3">Provider</th>
              <th className="px-4 py-3">Secrets</th>
              <th className="px-4 py-3">Status</th>
              <th className="px-4 py-3 numeric">Priority</th>
              <th className="px-4 py-3" />
            </tr>
          </thead>
          <tbody>
            {PROVIDER_DOMAIN_ROWS.map((row) => {
              const binding = findBindingForRow(row, bindings);
              const active = bindingIsActive(binding);
              const secrets = bindingSecretsLabel(binding);

              return (
                <tr
                  key={row.domain}
                  className="border-b border-calyx-border/50 hover:bg-white/[0.03]"
                >
                  <td className="px-4 py-3 text-calyx-text">{row.label}</td>
                  <td className="px-4 py-3 text-calyx-text">{row.providerName}</td>
                  <td className="px-4 py-3 text-calyx-muted">{secrets}</td>
                  <td className="px-4 py-3">
                    <ProviderStatusBadge active={active} />
                  </td>
                  <td className="px-4 py-3 tabular-nums text-calyx-muted">
                    {binding ? (
                      editingId === binding.id ? (
                        <input
                          type="number"
                          className="calyx-input w-20 py-1 text-xs"
                          value={editPriority}
                          onChange={(e) => setEditPriority(e.target.value)}
                        />
                      ) : (
                        binding.priority
                      )
                    ) : (
                      '—'
                    )}
                  </td>
                  <td className="px-4 py-3">
                    <div className="flex justify-end gap-1">
                      {binding ? (
                        editingId === binding.id ? (
                          <>
                            <Button
                              type="button"
                              size="sm"
                              variant="outline"
                              onClick={() => void handleSaveEdit(binding)}
                            >
                              Save
                            </Button>
                            <Button
                              type="button"
                              size="sm"
                              variant="ghost"
                              onClick={() => setEditingId(null)}
                            >
                              Cancel
                            </Button>
                          </>
                        ) : (
                          <>
                            <Button
                              type="button"
                              size="icon"
                              variant="ghost"
                              className="h-8 w-8"
                              onClick={() => {
                                setEditingId(binding.id);
                                setEditPriority(String(binding.priority));
                              }}
                              aria-label="Edit binding"
                            >
                              <Pencil className="h-4 w-4" />
                            </Button>
                            <Button
                              type="button"
                              size="icon"
                              variant="ghost"
                              className="h-8 w-8"
                              disabled={testingId === binding.id}
                              onClick={() => void handleTest(binding)}
                              aria-label="Test binding"
                            >
                              <Play className="h-4 w-4" />
                            </Button>
                            <Button
                              type="button"
                              size="icon"
                              variant="ghost"
                              className="h-8 w-8 text-calyx-danger"
                              disabled={deletingId === binding.id}
                              onClick={() => void handleDelete(binding)}
                              aria-label="Delete binding"
                            >
                              <Trash2 className="h-4 w-4" />
                            </Button>
                          </>
                        )
                      ) : (
                        <Button
                          type="button"
                          size="sm"
                          variant="outline"
                          className="text-xs"
                          onClick={() => prefillDomain(row.domain)}
                        >
                          Configure
                        </Button>
                      )}
                    </div>
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
        <p className="border-t border-calyx-border/50 px-4 py-3 text-xs text-calyx-muted">
          Status shows Mock now; becomes Active automatically when MOCK_MODE=false and the real
          key is added.
        </p>
      </div>
    </div>
  );
}
