import type { ReactNode } from 'react';
import { EmptyState } from '@/components/ui/EmptyState';
import { Table, TableBody, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { cn } from '@/lib/utils';

interface DataTableProps {
  columns: string[];
  columnClassNames?: (string | undefined)[];
  emptyTitle: string;
  emptyDescription?: string;
  isEmpty: boolean;
  children: ReactNode;
  className?: string;
}

export function DataTable({
  columns,
  columnClassNames,
  emptyTitle,
  emptyDescription,
  isEmpty,
  children,
  className,
}: DataTableProps): JSX.Element {
  if (isEmpty) {
    return (
      <div className={cn('calyx-card', className)}>
        <EmptyState title={emptyTitle} description={emptyDescription} />
      </div>
    );
  }

  return (
    <div className={cn('calyx-card overflow-hidden', className)}>
      <Table>
        <TableHeader>
          <TableRow className="hover:bg-transparent">
            {columns.map((col, index) => (
              <TableHead key={col} className={columnClassNames?.[index]}>
                {col}
              </TableHead>
            ))}
          </TableRow>
        </TableHeader>
        <TableBody>{children}</TableBody>
      </Table>
    </div>
  );
}
