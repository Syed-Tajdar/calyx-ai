import { Inbox } from 'lucide-react';
import { cn } from '@/lib/utils';

interface EmptyStateProps {
  title: string;
  description?: string;
  className?: string;
}

export function EmptyState({ title, description, className }: EmptyStateProps): JSX.Element {
  return (
    <div className={cn('flex flex-col items-center justify-center py-12 text-center', className)}>
      <Inbox className="mb-3 h-10 w-10 text-calyx-muted" />
      <p className="text-sm font-medium text-calyx-text">{title}</p>
      {description && <p className="mt-1 max-w-sm text-xs text-calyx-muted">{description}</p>}
    </div>
  );
}
