import type { ReactNode } from 'react';

interface PageHeaderProps {
  title: string;
  description?: ReactNode;
  action?: ReactNode;
}

export function PageHeader({ title, description, action }: PageHeaderProps): JSX.Element {
  return (
    <div className="mb-6 flex flex-col gap-2 sm:flex-row sm:items-end sm:justify-between">
      <div>
        <h1 className="font-heading text-2xl font-bold tracking-tight text-calyx-text">{title}</h1>
        {description && <div className="mt-1.5 text-sm text-calyx-muted">{description}</div>}
      </div>
      {action && <div className="shrink-0">{action}</div>}
    </div>
  );
}
