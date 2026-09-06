/**
 * Admin API - Tenant management, audit logs, chase logs.
 */

import { authFetch } from './core';
import type { Tenant, AuditLog, AuditLogStats, ChaseLog, ChaseLogClient, ChaseStats } from './types';

// ============================================================================
// Tenant Management (Super Admin Only)
// ============================================================================

export async function getTenants(params?: {
  limit?: number;
  offset?: number;
  search?: string;
  plan?: string;
  is_active?: boolean;
}): Promise<{ tenants: Tenant[]; count: number }> {
  const searchParams = new URLSearchParams();
  if (params?.limit) searchParams.set('limit', String(params.limit));
  if (params?.offset) searchParams.set('offset', String(params.offset));
  if (params?.search) searchParams.set('search', params.search);
  if (params?.plan) searchParams.set('plan', params.plan);
  if (params?.is_active !== undefined) searchParams.set('is_active', String(params.is_active));

  const res = await authFetch(`/api/v1/admin/tenants?${searchParams}`);
  if (!res.ok) throw new Error('Failed to fetch tenants');
  return res.json();
}

export async function getTenant(id: string): Promise<{ tenant: Tenant }> {
  const res = await authFetch(`/api/v1/admin/tenants/${id}`);
  if (!res.ok) throw new Error('Failed to fetch tenant');
  return res.json();
}

export async function createTenant(data: {
  domain: string;
  name: string;
  plan?: string;
  custom_domain?: string;
}): Promise<{ tenant: Tenant }> {
  const res = await authFetch('/api/v1/admin/tenants', {
    method: 'POST',
    body: JSON.stringify(data),
  });
  if (!res.ok) {
    const error = await res.json();
    throw new Error(error.message || 'Failed to create tenant');
  }
  return res.json();
}

export async function updateTenant(id: string, data: Partial<Tenant>): Promise<void> {
  const res = await authFetch(`/api/v1/admin/tenants/${id}`, {
    method: 'PATCH',
    body: JSON.stringify(data),
  });
  if (!res.ok) {
    const error = await res.json();
    throw new Error(error.message || 'Failed to update tenant');
  }
}

export async function deleteTenant(id: string): Promise<void> {
  const res = await authFetch(`/api/v1/admin/tenants/${id}`, {
    method: 'DELETE',
  });
  if (!res.ok) throw new Error('Failed to delete tenant');
}

// ============================================================================
// Audit Logs
// ============================================================================

export async function getAuditLogs(params?: {
  limit?: number;
  offset?: number;
  user_id?: string;
  action?: string;
  entity_type?: string;
  severity?: string;
  from_date?: string;
  to_date?: string;
}): Promise<{ audit_logs: AuditLog[]; count: number }> {
  const searchParams = new URLSearchParams();
  if (params?.limit) searchParams.set('limit', String(params.limit));
  if (params?.offset) searchParams.set('offset', String(params.offset));
  if (params?.user_id) searchParams.set('user_id', params.user_id);
  if (params?.action) searchParams.set('action', params.action);
  if (params?.entity_type) searchParams.set('entity_type', params.entity_type);
  if (params?.severity) searchParams.set('severity', params.severity);
  if (params?.from_date) searchParams.set('from_date', params.from_date);
  if (params?.to_date) searchParams.set('to_date', params.to_date);

  const res = await authFetch(`/api/v1/audit-logs?${searchParams}`);
  if (!res.ok) throw new Error('Failed to fetch audit logs');
  return res.json();
}

export async function getAuditLogStats(params?: {
  from_date?: string;
  to_date?: string;
}): Promise<{ stats: AuditLogStats }> {
  const searchParams = new URLSearchParams();
  if (params?.from_date) searchParams.set('from_date', params.from_date);
  if (params?.to_date) searchParams.set('to_date', params.to_date);

  const res = await authFetch(`/api/v1/audit-logs/stats?${searchParams}`);
  if (!res.ok) throw new Error('Failed to fetch audit log stats');
  return res.json();
}

export async function getAuditLogActions(): Promise<{ actions: string[] }> {
  const res = await authFetch('/api/v1/audit-logs/actions');
  if (!res.ok) throw new Error('Failed to fetch audit log actions');
  return res.json();
}

export async function getAuditLogEntityTypes(): Promise<{ entity_types: string[] }> {
  const res = await authFetch('/api/v1/audit-logs/entity-types');
  if (!res.ok) throw new Error('Failed to fetch audit log entity types');
  return res.json();
}

export async function getAuditLog(id: string): Promise<{ audit_log: AuditLog }> {
  const res = await authFetch(`/api/v1/audit-logs/${id}`);
  if (!res.ok) throw new Error('Failed to fetch audit log');
  return res.json();
}

// ============================================================================
// Chase Logs
// ============================================================================

export async function getChaseLogs(params?: {
  limit?: number;
  offset?: number;
  from_date?: string;
  to_date?: string;
}): Promise<{ chase_logs: ChaseLog[]; count: number }> {
  const searchParams = new URLSearchParams();
  if (params?.limit) searchParams.set('limit', String(params.limit));
  if (params?.offset) searchParams.set('offset', String(params.offset));
  if (params?.from_date) searchParams.set('from_date', params.from_date);
  if (params?.to_date) searchParams.set('to_date', params.to_date);

  const res = await authFetch(`/api/v1/chase-logs?${searchParams}`);
  if (!res.ok) throw new Error('Failed to fetch chase logs');
  return res.json();
}

export async function getChaseLogStats(): Promise<{ stats: ChaseStats }> {
  const res = await authFetch('/api/v1/chase-logs/stats');
  if (!res.ok) throw new Error('Failed to fetch chase log stats');
  return res.json();
}

export async function createChaseLog(data: {
  client_ids: string[];
  template_id: string;
  subject?: string;
  body_html?: string;
}): Promise<{ chase_log: ChaseLog }> {
  const res = await authFetch('/api/v1/chase-logs', {
    method: 'POST',
    body: JSON.stringify(data),
  });
  if (!res.ok) {
    const error = await res.json();
    throw new Error(error.message || 'Failed to create chase log');
  }
  return res.json();
}

export async function getChaseLog(id: string): Promise<{
  chase_log: ChaseLog;
  clients: ChaseLogClient[];
}> {
  const res = await authFetch(`/api/v1/chase-logs/${id}`);
  if (!res.ok) throw new Error('Failed to fetch chase log');
  return res.json();
}
