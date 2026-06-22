import type { LucideIcon } from 'lucide-react';

interface SubsectionHeaderProps {
  title: string;
  icon?: LucideIcon;
}

export function SubsectionHeader({ title, icon: Icon }: SubsectionHeaderProps): JSX.Element {
  return (
    <div className="mb-3 flex items-center gap-2">
      {Icon && (
        <span className="flex h-7 w-7 items-center justify-center rounded-lg bg-calyx-accent/15 text-calyx-accent">
          <Icon className="h-3.5 w-3.5" />
        </span>
      )}
      <h2 className="text-xs font-bold uppercase tracking-[0.08em] text-calyx-text">{title}</h2>
    </div>
  );
}
