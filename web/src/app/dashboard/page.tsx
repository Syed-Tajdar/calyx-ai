'use client';

import { useEffect, useState } from 'react';
import { useRouter } from 'next/navigation';
import { fetchPings, PingRecord } from '@/lib/api';

export default function DashboardPage(): JSX.Element {
  const router = useRouter();
  const [pings, setPings] = useState<PingRecord[]>([]);
  const [total, setTotal] = useState(0);
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    const token = localStorage.getItem('callloom_token');
    if (!token) {
      router.replace('/login');
      return;
    }

    fetchPings(token)
      .then((res) => {
        setPings(res.data);
        setTotal(res.total);
      })
      .catch(() => setError('Failed to load pings'))
      .finally(() => setLoading(false));
  }, [router]);

  function logout(): void {
    localStorage.removeItem('callloom_token');
    router.push('/login');
  }

  return (
    <main style={{ padding: 24 }}>
      <header style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
        <div>
          <h1>Ping Dashboard</h1>
          <p>{total} total pings</p>
        </div>
        <button onClick={logout} style={{ padding: '8px 16px' }}>
          Log out
        </button>
      </header>

      {loading && <p>Loading…</p>}
      {error && <p style={{ color: 'crimson' }}>{error}</p>}

      {!loading && !error && (
        <table style={{ width: '100%', marginTop: 24, borderCollapse: 'collapse' }}>
          <thead>
            <tr>
              {['Time', 'Platform', 'Call ID', 'State', 'Event', 'Revenue', 'Dup'].map((h) => (
                <th key={h} style={{ textAlign: 'left', borderBottom: '1px solid #ccc', padding: 8 }}>
                  {h}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {pings.map((p) => (
              <tr key={`${p.external_call_id}-${p.received_at}`}>
                <td style={{ padding: 8 }}>{p.received_at}</td>
                <td style={{ padding: 8 }}>{p.platform}</td>
                <td style={{ padding: 8, fontFamily: 'monospace', fontSize: 12 }}>
                  {p.external_call_id.slice(0, 8)}…
                </td>
                <td style={{ padding: 8 }}>{p.state || '—'}</td>
                <td style={{ padding: 8 }}>{p.event_type}</td>
                <td style={{ padding: 8 }}>${p.revenue.toFixed(2)}</td>
                <td style={{ padding: 8 }}>{p.is_duplicate ? 'yes' : 'no'}</td>
              </tr>
            ))}
            {pings.length === 0 && (
              <tr>
                <td colSpan={7} style={{ padding: 24, textAlign: 'center', color: '#666' }}>
                  No pings yet. Send a Ringba pixel to the ingestion webhook.
                </td>
              </tr>
            )}
          </tbody>
        </table>
      )}
    </main>
  );
}
