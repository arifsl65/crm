/**
 * Reminders API - Reminder CRUD and management.
 */

import { authFetch } from './core';
import type { Reminder } from './types';

export async function getReminders(params?: {
  limit?: number;
  offset?: number;
  status?: string;
  upcoming?: boolean;
}): Promise<{ reminders: Reminder[]; count: number }> {
  const searchParams = new URLSearchParams();
  if (params?.limit) searchParams.set('limit', String(params.limit));
  if (params?.offset) searchParams.set('offset', String(params.offset));
  if (params?.status) searchParams.set('status', params.status);
  if (params?.upcoming) searchParams.set('upcoming', 'true');

  const res = await authFetch(`/api/v1/reminders?${searchParams}`);
  if (!res.ok) throw new Error('Failed to fetch reminders');
  return res.json();
}

export async function getReminder(id: string): Promise<{ reminder: Reminder }> {
  const res = await authFetch(`/api/v1/reminders/${id}`);
  if (!res.ok) throw new Error('Failed to fetch reminder');
  return res.json();
}

export async function getUpcomingReminders(limit?: number): Promise<{ reminders: Reminder[]; count: number }> {
  const searchParams = new URLSearchParams();
  if (limit) searchParams.set('limit', String(limit));

  const res = await authFetch(`/api/v1/reminders/upcoming?${searchParams}`);
  if (!res.ok) throw new Error('Failed to fetch upcoming reminders');
  return res.json();
}

export async function createReminder(data: {
  client_id?: string;
  email_id?: string;
  document_id?: string;
  service_id?: string;
  remind_at: string;
  reason?: string;
}): Promise<{ reminder: Reminder }> {
  const res = await authFetch('/api/v1/reminders', {
    method: 'POST',
    body: JSON.stringify(data),
  });
  if (!res.ok) {
    const error = await res.json();
    throw new Error(error.message || 'Failed to create reminder');
  }
  return res.json();
}

export async function completeReminder(id: string): Promise<void> {
  const res = await authFetch(`/api/v1/reminders/${id}/complete`, {
    method: 'POST',
  });
  if (!res.ok) throw new Error('Failed to complete reminder');
}

export async function dismissReminder(id: string): Promise<void> {
  const res = await authFetch(`/api/v1/reminders/${id}/dismiss`, {
    method: 'POST',
  });
  if (!res.ok) throw new Error('Failed to dismiss reminder');
}

export async function deleteReminder(id: string): Promise<void> {
  const res = await authFetch(`/api/v1/reminders/${id}`, {
    method: 'DELETE',
  });
  if (!res.ok) throw new Error('Failed to delete reminder');
}
