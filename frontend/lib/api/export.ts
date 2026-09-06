/**
 * Export API - Export data to CSV/Excel.
 */

import { authFetch } from './core';

export async function exportClients(params?: {
  status?: string;
  from_date?: string;
  to_date?: string;
}): Promise<Blob> {
  const searchParams = new URLSearchParams();
  if (params?.status) searchParams.set('status', params.status);
  if (params?.from_date) searchParams.set('from_date', params.from_date);
  if (params?.to_date) searchParams.set('to_date', params.to_date);

  const res = await authFetch(`/api/v1/export/clients?${searchParams}`);
  if (!res.ok) throw new Error('Failed to export clients');
  return res.blob();
}

export async function exportServices(params?: {
  status?: string;
  client_id?: string;
  from_date?: string;
  to_date?: string;
}): Promise<Blob> {
  const searchParams = new URLSearchParams();
  if (params?.status) searchParams.set('status', params.status);
  if (params?.client_id) searchParams.set('client_id', params.client_id);
  if (params?.from_date) searchParams.set('from_date', params.from_date);
  if (params?.to_date) searchParams.set('to_date', params.to_date);

  const res = await authFetch(`/api/v1/export/services?${searchParams}`);
  if (!res.ok) throw new Error('Failed to export services');
  return res.blob();
}

export async function exportChaseLogs(params?: {
  from_date?: string;
  to_date?: string;
}): Promise<Blob> {
  const searchParams = new URLSearchParams();
  if (params?.from_date) searchParams.set('from_date', params.from_date);
  if (params?.to_date) searchParams.set('to_date', params.to_date);

  const res = await authFetch(`/api/v1/export/chase?${searchParams}`);
  if (!res.ok) throw new Error('Failed to export chase logs');
  return res.blob();
}
