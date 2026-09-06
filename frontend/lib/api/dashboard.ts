/**
 * Dashboard API - Statistics, deadlines, kanban, and workload data.
 */

import { authFetch } from './core';
import type {
  DashboardStats,
  DashboardDeadline,
  Service,
  PendingDocument,
  WorkloadItem,
  RecentClient,
} from './types';

// Dashboard API
export async function getDashboardStats(): Promise<DashboardStats> {
  const res = await authFetch('/api/v1/dashboard/stats');
  if (!res.ok) throw new Error('Failed to fetch dashboard stats');
  return res.json();
}

export async function getDashboardDeadlines(): Promise<{ deadlines: DashboardDeadline[] }> {
  const res = await authFetch('/api/v1/dashboard/deadlines');
  if (!res.ok) throw new Error('Failed to fetch deadlines');
  return res.json();
}

export async function getKanban(): Promise<Record<string, Service[]>> {
  const res = await authFetch('/api/v1/dashboard/kanban');
  if (!res.ok) throw new Error('Failed to fetch kanban');
  return res.json();
}

export async function getDashboardPendingDocuments(params?: {
  limit?: number;
}): Promise<{ documents: PendingDocument[] }> {
  const searchParams = new URLSearchParams();
  if (params?.limit) searchParams.set('limit', String(params.limit));

  const res = await authFetch(`/api/v1/dashboard/pending-documents?${searchParams}`);
  if (!res.ok) throw new Error('Failed to fetch pending documents');
  return res.json();
}

export async function getDashboardWorkload(): Promise<{ workload: WorkloadItem[] }> {
  const res = await authFetch('/api/v1/dashboard/workload');
  if (!res.ok) throw new Error('Failed to fetch dashboard workload');
  return res.json();
}

export async function getDashboardRecentClients(params?: {
  limit?: number;
}): Promise<{ clients: RecentClient[] }> {
  const searchParams = new URLSearchParams();
  if (params?.limit) searchParams.set('limit', String(params.limit));

  const res = await authFetch(`/api/v1/dashboard/recent-clients?${searchParams}`);
  if (!res.ok) throw new Error('Failed to fetch recent clients');
  return res.json();
}
