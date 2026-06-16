'use client';

import { useEffect, useMemo, useState } from 'react';
import type { EnrichmentAttribute } from '@/lib/api';
import { fetchEnrichment } from '@/lib/api';
import { getToken } from '@/lib/auth';

interface EnrichmentPanelProps {
  aniHash: string;
}

export function EnrichmentPanel({ aniHash }: EnrichmentPanelProps): JSX.Element {
  const [attributes, setAttributes] = useState<EnrichmentAttribute[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [filter, setFilter] = useState('');

  useEffect(() => {
    const token = getToken();
    if (!token || !aniHash) {
      setLoading(false);
      return;
    }
    setLoading(true);
    setError('');
    fetchEnrichment(token, aniHash, 500)
      .then((res) => setAttributes(res.data))
      .catch(() => setError('Failed to load enrichment attributes'))
      .finally(() => setLoading(false));
  }, [aniHash]);

  const filtered = useMemo(() => {
    const q = filter.trim().toLowerCase();
    if (!q) return attributes.slice(0, 50);
    return attributes
      .filter(
        (a) =>
          a.attribute_key.toLowerCase().includes(q) ||
          a.attribute_value.toLowerCase().includes(q),
      )
      .slice(0, 50);
  }, [attributes, filter]);

  return (
    <div className="loom-card overflow-hidden">
      <div className="border-b border-loom-border px-4 py-3">
        <p className="text-[10px] font-bold uppercase tracking-wider text-loom-accent">
          Audience Enrichment
        </p>
        <p className="text-xs text-loom-muted">
          {loading ? 'Loading…' : `${attributes.length} attributes`}
        </p>
        <input
          type="text"
          placeholder="Filter attributes…"
          value={filter}
          onChange={(e) => setFilter(e.target.value)}
          className="loom-input mt-2 text-xs"
        />
      </div>
      {error && <p className="px-4 py-3 text-sm text-red-300">{error}</p>}
      {!error && !loading && attributes.length === 0 && (
        <p className="px-4 py-6 text-sm text-loom-muted">No enrichment data for this caller yet.</p>
      )}
      {!error && filtered.length > 0 && (
        <div className="max-h-72 overflow-y-auto">
          <table className="w-full text-left text-xs">
            <thead>
              <tr className="border-b border-loom-border text-[10px] uppercase text-loom-muted">
                <th className="px-4 py-2 font-medium">Attribute</th>
                <th className="px-4 py-2 font-medium">Value</th>
              </tr>
            </thead>
            <tbody>
              {filtered.map((attr) => (
                <tr key={attr.attribute_key} className="border-b border-loom-border/30">
                  <td className="px-4 py-2 font-mono text-[10px] text-loom-muted">
                    {attr.attribute_key}
                  </td>
                  <td className="px-4 py-2 text-white">{attr.attribute_value}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}
