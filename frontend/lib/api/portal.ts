/**
 * Portal API - Client portal endpoints.
 */

import { authFetch } from './core';
import type { PortalUser, PortalDashboard, PortalDocument, PortalService, PortalDeadline } from './types';

export async function getPortalMe(): Promise<{ user: PortalUser }> {
  const res = await authFetch('/api/v1/portal/me');
  if (!res.ok) throw new Error('Failed to fetch portal user');
  return res.json();
}

export async function getPortalDashboard(): Promise<PortalDashboard> {
  const res = await authFetch('/api/v1/portal/dashboard');
  if (!res.ok) throw new Error('Failed to fetch portal dashboard');
  return res.json();
}

export async function getPortalDocuments(): Promise<{ documents: PortalDocument[] }> {
  const res = await authFetch('/api/v1/portal/documents');
  if (!res.ok) throw new Error('Failed to fetch portal documents');
  return res.json();
}

export async function getPortalServices(): Promise<{ services: PortalService[] }> {
  const res = await authFetch('/api/v1/portal/services');
  if (!res.ok) throw new Error('Failed to fetch portal services');
  return res.json();
}

export async function getPortalDeadlines(): Promise<{ deadlines: PortalDeadline[] }> {
  const res = await authFetch('/api/v1/portal/deadlines');
  if (!res.ok) throw new Error('Failed to fetch portal deadlines');
  return res.json();
}

export async function changePortalPassword(data: {
  current_password: string;
  new_password: string;
}): Promise<{ message: string }> {
  const res = await authFetch('/api/v1/portal/password', {
    method: 'POST',
    body: JSON.stringify(data),
  });
  if (!res.ok) {
    const error = await res.json();
    throw new Error(error.message || 'Failed to change password');
  }
  return res.json();
}

export async function updatePortalProfile(data: {
  name?: string;
  phone?: string;
}): Promise<{ message: string }> {
  const res = await authFetch('/api/v1/portal/me', {
    method: 'PATCH',
    body: JSON.stringify(data),
  });
  if (!res.ok) {
    const error = await res.json();
    throw new Error(error.message || 'Failed to update profile');
  }
  return res.json();
}
