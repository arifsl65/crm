'use client';

import { AuthGuard } from '@/components/auth-guard';
import { DashboardShell } from './components';

export default function DashboardLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <AuthGuard>
      <DashboardShell>{children}</DashboardShell>
    </AuthGuard>
  );
}
