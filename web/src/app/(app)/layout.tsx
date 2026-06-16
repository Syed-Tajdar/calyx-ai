import { AppShell } from '@/components/layout/AppShell';

export default function AppLayout({
  children,
}: {
  children: React.ReactNode;
}): JSX.Element {
  return <AppShell>{children}</AppShell>;
}
