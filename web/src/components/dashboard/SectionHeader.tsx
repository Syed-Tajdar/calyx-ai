import { PhoneIcon } from '@/components/ui/Icons';

interface SectionHeaderProps {
  title: string;
}

export function SectionHeader({ title }: SectionHeaderProps): JSX.Element {
  return (
    <div className="mb-3 flex items-center gap-2">
      <span className="flex h-7 w-7 items-center justify-center rounded-lg bg-loom-accent/15 text-loom-accent">
        <PhoneIcon className="h-3.5 w-3.5" />
      </span>
      <h2 className="text-xs font-bold uppercase tracking-wider text-white">{title}</h2>
    </div>
  );
}
