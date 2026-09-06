/**
 * Notifications API - Notification CRUD and management.
 */

import { authFetch } from './core';
import type { Notification } from './types';

// ============================================================================
// Notifications
// ============================================================================

export async function getNotifications(params?: {
  limit?: number;
  offset?: number;
  unread?: boolean;
  type?: string;
}): Promise<{ notifications: Notification[]; count: number; unread_count: number }> {
  const searchParams = new URLSearchParams();
  if (params?.limit) searchParams.set('limit', String(params.limit));
  if (params?.offset) searchParams.set('offset', String(params.offset));
  if (params?.unread) searchParams.set('unread', 'true');
  if (params?.type) searchParams.set('type', params.type);

  const res = await authFetch(`/api/v1/notifications?${searchParams}`);
  if (!res.ok) throw new Error('Failed to fetch notifications');
  return res.json();
}

export async function getNotification(id: string): Promise<{ notification: Notification }> {
  const res = await authFetch(`/api/v1/notifications/${id}`);
  if (!res.ok) throw new Error('Failed to fetch notification');
  return res.json();
}

export async function getUnreadNotificationCount(): Promise<{ unread_count: number }> {
  const res = await authFetch('/api/v1/notifications/unread-count');
  if (!res.ok) throw new Error('Failed to fetch unread count');
  return res.json();
}

export async function markNotificationRead(id: string): Promise<void> {
  const res = await authFetch(`/api/v1/notifications/${id}/read`, {
    method: 'PATCH',
  });
  if (!res.ok) throw new Error('Failed to mark notification as read');
}

export async function markAllNotificationsRead(): Promise<{ count: number }> {
  const res = await authFetch('/api/v1/notifications/read-all', {
    method: 'POST',
  });
  if (!res.ok) throw new Error('Failed to mark all notifications as read');
  return res.json();
}

export async function dismissNotification(id: string): Promise<void> {
  const res = await authFetch(`/api/v1/notifications/${id}`, {
    method: 'DELETE',
  });
  if (!res.ok) throw new Error('Failed to dismiss notification');
}

export async function dismissAllNotifications(): Promise<{ count: number }> {
  const res = await authFetch('/api/v1/notifications/dismiss-all', {
    method: 'POST',
  });
  if (!res.ok) throw new Error('Failed to dismiss all notifications');
  return res.json();
}

export async function createNotification(data: {
  user_id?: string;
  type: 'document' | 'deadline' | 'email' | 'system' | 'reminder';
  title: string;
  message: string;
  entity_type?: string;
  entity_id?: string;
  link?: string;
}): Promise<{ notification: Notification }> {
  const res = await authFetch('/api/v1/notifications', {
    method: 'POST',
    body: JSON.stringify(data),
  });
  if (!res.ok) {
    const error = await res.json();
    throw new Error(error.message || 'Failed to create notification');
  }
  return res.json();
}
