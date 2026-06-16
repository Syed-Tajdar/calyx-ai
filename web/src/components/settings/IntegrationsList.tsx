import type { IntegrationRecord } from '@/lib/api';

interface IntegrationsListProps {
  integrations: IntegrationRecord[];
}

function maskToken(token: string): string {
  if (token.length <= 8) return '••••••••';
  return `${token.slice(0, 4)}…${token.slice(-4)}`;
}

export function IntegrationsList({ integrations }: IntegrationsListProps): JSX.Element {
  return (
    <div className="loom-card overflow-hidden">
      <div className="border-b border-loom-border px-4 py-3">
        <p className="text-[10px] font-bold uppercase tracking-wider text-loom-accent">
          Integrations
        </p>
        <h3 className="text-sm font-semibold text-white">Connected Platforms</h3>
      </div>
      <div className="overflow-x-auto">
        <table className="w-full min-w-[400px] text-left text-xs">
          <thead>
            <tr className="border-b border-loom-border text-[10px] uppercase text-loom-muted">
              <th className="px-4 py-3 font-medium">Platform</th>
              <th className="px-4 py-3 font-medium">Name</th>
              <th className="px-4 py-3 font-medium">Token</th>
              <th className="px-4 py-3 font-medium">Status</th>
            </tr>
          </thead>
          <tbody>
            {integrations.map((item) => (
              <tr key={item.id} className="border-b border-loom-border/40">
                <td className="px-4 py-3 capitalize text-white">{item.platform}</td>
                <td className="px-4 py-3 text-loom-muted">{item.name}</td>
                <td className="px-4 py-3 font-mono text-[10px] text-loom-muted">
                  {maskToken(item.webhook_token)}
                </td>
                <td className="px-4 py-3">
                  <span
                    className={`rounded-full px-2 py-0.5 text-[10px] font-medium ${
                      item.is_active
                        ? 'bg-emerald-500/15 text-emerald-400'
                        : 'bg-red-500/15 text-red-400'
                    }`}
                  >
                    {item.is_active ? 'Active' : 'Inactive'}
                  </span>
                </td>
              </tr>
            ))}
            {integrations.length === 0 && (
              <tr>
                <td colSpan={4} className="px-4 py-12 text-center text-loom-muted">
                  No integrations configured.
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </div>
    </div>
  );
}
