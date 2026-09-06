/**
 * Users API - User/Staff CRUD and workload management.
 */

import { authFetch } from './core';
import type { User } from './types';

// ============================================================================
// Users CRUD
// ============================================================================

export async function getUsers(params?: {
  limit?: number;
  offset?: number;
  role?: string;
  search?: string;
}): Promise<{ users: User[]; count: number }> {
  const searchParams = new URLSearchParams();
  if (params?.limit) searchParams.set('limit', String(params.limit));
  if (params?.offset) searchParams.set('offset', String(params.offset));
  if (params?.role) searchParams.set('role', params.role);
  if (params?.search) searchParams.set('search', params.search);

  const res = await authFetch(`/api/v1/users?${searchParams}`);
  if (!res.ok) throw new Error('Failed to fetch users');
  return res.json();
}

export async function getUser(id: string): Promise<{ user: User }> {
  const res = await authFetch(`/api/v1/users/${id}`);
  if (!res.ok) throw new Error('Failed to fetch user');
  return res.json();
}

export async function createUser(data: {
  email: string;
  name: string;
  role: string;
  phone?: string;
}): Promise<{ user: User }> {
  const res = await authFetch('/api/v1/users', {
    method: 'POST',
    body: JSON.stringify(data),
  });
  if (!res.ok) {
    const error = await res.json();
    throw new Error(error.message || 'Failed to create user');
  }
  return res.json();
}

export async function updateUser(id: string, data: Partial<User>): Promise<void> {
  const res = await authFetch(`/api/v1/users/${id}`, {
    method: 'PATCH',
    body: JSON.stringify(data),
  });
  if (!res.ok) {
    const error = await res.json();
    throw new Error(error.message || 'Failed to update user');
  }
}

export async function deleteUser(id: string): Promise<void> {
  const res = await authFetch(`/api/v1/users/${id}`, {
    method: 'DELETE',
  });
  if (!res.ok) throw new Error('Failed to delete user');
}

export async function restoreUser(id: string): Promise<void> {
  const res = await authFetch(`/api/v1/users/${id}/restore`, {
    method: 'POST',
  });
  if (!res.ok) throw new Error('Failed to restore user');
}

// ============================================================================
// User Actions
// ============================================================================

export async function reset2FA(id: string): Promise<void> {
  const res = await authFetch(`/api/v1/users/${id}/2fa`, {
    method: 'DELETE',
  });
  if (!res.ok) {
    const error = await res.json();
    throw new Error(error.message || 'Failed to reset 2FA');
  }
}

export async function resendInvite(id: string): Promise<void> {
  const res = await authFetch(`/api/v1/users/${id}/resend-invite`, {
    method: 'POST',
  });
  if (!res.ok) {
    const error = await res.json();
    throw new Error(error.message || 'Failed to resend invite');
  }
}

// ============================================================================
// User Clients & Workload
// ============================================================================

export async function getUserClients(id: string): Promise<{
  clients: Array<{
    id: string;
    company_name: string;
    contact_name?: string;
    is_primary: boolean;
    assigned_at: string;
  }>;
  count: number;
}> {
  const res = await authFetch(`/api/v1/users/${id}/clients`);
  if (!res.ok) throw new Error('Failed to fetch user clients');
  return res.json();
}

export async function getStaffWorkload(): Promise<{
  workload: Array<{
    user_id: string;
    user_name: string;
    client_count: number;
    primary_count: number;
  }>;
  count: number;
}> {
  const res = await authFetch('/api/v1/users/workload');
  if (!res.ok) throw new Error('Failed to fetch staff workload');
  return res.json();
}
