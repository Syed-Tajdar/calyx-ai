'use client';

import { AlertCenter } from '@/components/workflows/AlertCenter';
import { PageHeader } from '@/components/ui/PageHeader';

export default function AlertsPage(): JSX.Element {
  return (
    <div className="space-y-6">
      <PageHeader
        title="Alerts"
        description="Alerts from your workflow rules, the numbers each one used, and publisher pauses. Eastern time."
      />
      <AlertCenter />
    </div>
  );
}
