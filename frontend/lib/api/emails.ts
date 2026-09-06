/**
 * Emails API - Email CRUD, threads, accounts, templates.
 */

import { authFetch } from './core';
import type { Email, EmailAccount, EmailTemplate, EmailStats, EmailThread } from './types';

// ============================================================================
// Emails CRUD
// ============================================================================

export async function getEmails(params?: {
  limit?: number;
  offset?: number;
  client_id?: string;
  direction?: 'inbound' | 'outbound';
  status?: string;
  type?: string;
  search?: string;
}): Promise<{ emails: Email[]; count: number }> {
  const searchParams = new URLSearchParams();
  if (params?.limit) searchParams.set('limit', String(params.limit));
  if (params?.offset) searchParams.set('offset', String(params.offset));
  if (params?.client_id) searchParams.set('client_id', params.client_id);
  if (params?.direction) searchParams.set('direction', params.direction);
  if (params?.status) searchParams.set('status', params.status);
  if (params?.type) searchParams.set('type', params.type);
  if (params?.search) searchParams.set('search', params.search);

  const res = await authFetch(`/api/v1/emails?${searchParams}`);
  if (!res.ok) throw new Error('Failed to fetch emails');
  return res.json();
}

export async function getEmail(id: string): Promise<Email> {
  const res = await authFetch(`/api/v1/emails/${id}`);
  if (!res.ok) throw new Error('Failed to fetch email');
  return res.json();
}

export async function sendEmail(data: {
  to_email: string;
  to_name?: string;
  subject: string;
  body_html: string;
  body_text?: string;
  client_id?: string;
  template_id?: string;
  type?: 'chase' | 'notification' | 'invite' | 'manual';
}): Promise<Email> {
  const res = await authFetch('/api/v1/emails', {
    method: 'POST',
    body: JSON.stringify(data),
  });
  if (!res.ok) {
    const error = await res.json();
    throw new Error(error.message || 'Failed to send email');
  }
  return res.json();
}

export async function sendEmailFromTemplate(data: {
  template_id: string;
  to_email: string;
  to_name?: string;
  client_id?: string;
  placeholders?: Record<string, string>;
}): Promise<Email> {
  const res = await authFetch('/api/v1/emails/send-template', {
    method: 'POST',
    body: JSON.stringify(data),
  });
  if (!res.ok) {
    const error = await res.json();
    throw new Error(error.message || 'Failed to send email from template');
  }
  return res.json();
}

export async function markEmailRead(id: string): Promise<void> {
  const res = await authFetch(`/api/v1/emails/${id}/read`, {
    method: 'PATCH',
  });
  if (!res.ok) throw new Error('Failed to mark email as read');
}

export async function getEmailStats(): Promise<{ stats: EmailStats }> {
  const res = await authFetch('/api/v1/emails/stats');
  if (!res.ok) throw new Error('Failed to fetch email stats');
  return res.json();
}

export async function claimEmail(id: string): Promise<{ email: Email }> {
  const res = await authFetch(`/api/v1/emails/${id}/claim`, {
    method: 'PATCH',
  });
  if (!res.ok) throw new Error('Failed to claim email');
  return res.json();
}

export async function unclaimEmail(id: string): Promise<{ email: Email }> {
  const res = await authFetch(`/api/v1/emails/${id}/unclaim`, {
    method: 'PATCH',
  });
  if (!res.ok) throw new Error('Failed to unclaim email');
  return res.json();
}

// ============================================================================
// Email Threads
// ============================================================================

export async function getEmailThreads(params?: {
  limit?: number;
  offset?: number;
  client_id?: string;
  search?: string;
}): Promise<{ threads: EmailThread[]; count: number }> {
  const searchParams = new URLSearchParams();
  if (params?.limit) searchParams.set('limit', String(params.limit));
  if (params?.offset) searchParams.set('offset', String(params.offset));
  if (params?.client_id) searchParams.set('client_id', params.client_id);
  if (params?.search) searchParams.set('search', params.search);

  const res = await authFetch(`/api/v1/emails/threads?${searchParams}`);
  if (!res.ok) throw new Error('Failed to fetch email threads');
  return res.json();
}

export async function getEmailThread(id: string): Promise<{ thread: EmailThread }> {
  const res = await authFetch(`/api/v1/emails/threads/${id}`);
  if (!res.ok) throw new Error('Failed to fetch email thread');
  return res.json();
}

export async function getEmailThreadMessages(id: string, params?: {
  limit?: number;
  offset?: number;
}): Promise<{ emails: Email[]; count: number }> {
  const searchParams = new URLSearchParams();
  if (params?.limit) searchParams.set('limit', String(params.limit));
  if (params?.offset) searchParams.set('offset', String(params.offset));

  const res = await authFetch(`/api/v1/emails/threads/${id}/messages?${searchParams}`);
  if (!res.ok) throw new Error('Failed to fetch thread messages');
  return res.json();
}

// ============================================================================
// Email Accounts
// ============================================================================

export async function getEmailAccounts(params?: {
  limit?: number;
  offset?: number;
  provider?: string;
  status?: string;
  type?: 'shared' | 'personal';
}): Promise<{ email_accounts: EmailAccount[]; count: number }> {
  const searchParams = new URLSearchParams();
  if (params?.limit) searchParams.set('limit', String(params.limit));
  if (params?.offset) searchParams.set('offset', String(params.offset));
  if (params?.provider) searchParams.set('provider', params.provider);
  if (params?.status) searchParams.set('status', params.status);
  if (params?.type) searchParams.set('type', params.type);

  const res = await authFetch(`/api/v1/email-accounts?${searchParams}`);
  if (!res.ok) throw new Error('Failed to fetch email accounts');
  return res.json();
}

export async function getEmailAccount(id: string): Promise<EmailAccount> {
  const res = await authFetch(`/api/v1/email-accounts/${id}`);
  if (!res.ok) throw new Error('Failed to fetch email account');
  return res.json();
}

export async function createIMAPAccount(data: {
  email: string;
  type?: 'shared' | 'personal';
  imap_host: string;
  imap_port?: number;
  imap_password: string;
  user_id?: string;
}): Promise<EmailAccount> {
  const res = await authFetch('/api/v1/email-accounts/imap', {
    method: 'POST',
    body: JSON.stringify(data),
  });
  if (!res.ok) {
    const error = await res.json();
    throw new Error(error.message || 'Failed to create email account');
  }
  return res.json();
}

export async function updateEmailAccount(id: string, data: {
  imap_host?: string;
  imap_port?: number;
  imap_password?: string;
  type?: 'shared' | 'personal';
}): Promise<EmailAccount> {
  const res = await authFetch(`/api/v1/email-accounts/${id}`, {
    method: 'PATCH',
    body: JSON.stringify(data),
  });
  if (!res.ok) {
    const error = await res.json();
    throw new Error(error.message || 'Failed to update email account');
  }
  return res.json();
}

export async function deleteEmailAccount(id: string): Promise<void> {
  const res = await authFetch(`/api/v1/email-accounts/${id}`, {
    method: 'DELETE',
  });
  if (!res.ok) throw new Error('Failed to delete email account');
}

export async function syncEmailAccount(id: string): Promise<{ message: string; last_sync_at: string }> {
  const res = await authFetch(`/api/v1/email-accounts/${id}/sync`, {
    method: 'POST',
  });
  if (!res.ok) throw new Error('Failed to sync email account');
  return res.json();
}

export async function testEmailAccountConnection(id: string): Promise<{ success: boolean; message: string }> {
  const res = await authFetch(`/api/v1/email-accounts/${id}/test`, {
    method: 'POST',
  });
  if (!res.ok) throw new Error('Failed to test email account connection');
  return res.json();
}

export async function disconnectEmailAccount(id: string): Promise<void> {
  const res = await authFetch(`/api/v1/email-accounts/${id}/disconnect`, {
    method: 'POST',
  });
  if (!res.ok) throw new Error('Failed to disconnect email account');
}

export async function reconnectEmailAccount(id: string): Promise<void> {
  const res = await authFetch(`/api/v1/email-accounts/${id}/reconnect`, {
    method: 'POST',
  });
  if (!res.ok) throw new Error('Failed to reconnect email account');
}

// ============================================================================
// Email Templates
// ============================================================================

export async function getEmailTemplates(params?: {
  limit?: number;
  offset?: number;
  type?: string;
  category?: string;
  active?: boolean;
  search?: string;
}): Promise<{ email_templates: EmailTemplate[]; count: number }> {
  const searchParams = new URLSearchParams();
  if (params?.limit) searchParams.set('limit', String(params.limit));
  if (params?.offset) searchParams.set('offset', String(params.offset));
  if (params?.type) searchParams.set('type', params.type);
  if (params?.category) searchParams.set('category', params.category);
  if (params?.active !== undefined) searchParams.set('active', String(params.active));
  if (params?.search) searchParams.set('search', params.search);

  const res = await authFetch(`/api/v1/email-templates?${searchParams}`);
  if (!res.ok) throw new Error('Failed to fetch email templates');
  return res.json();
}

export async function getEmailTemplate(id: string): Promise<EmailTemplate> {
  const res = await authFetch(`/api/v1/email-templates/${id}`);
  if (!res.ok) throw new Error('Failed to fetch email template');
  return res.json();
}

export async function createEmailTemplate(data: {
  name: string;
  subject: string;
  body_html: string;
  body_text?: string;
  type: 'chase' | 'notification' | 'welcome' | 'custom';
  category?: string;
  placeholders?: string[];
  is_default?: boolean;
}): Promise<EmailTemplate> {
  const res = await authFetch('/api/v1/email-templates', {
    method: 'POST',
    body: JSON.stringify(data),
  });
  if (!res.ok) {
    const error = await res.json();
    throw new Error(error.message || 'Failed to create email template');
  }
  return res.json();
}

export async function updateEmailTemplate(id: string, data: {
  name?: string;
  subject?: string;
  body_html?: string;
  body_text?: string;
  type?: string;
  category?: string;
  placeholders?: string[];
  is_default?: boolean;
  is_active?: boolean;
}): Promise<EmailTemplate> {
  const res = await authFetch(`/api/v1/email-templates/${id}`, {
    method: 'PATCH',
    body: JSON.stringify(data),
  });
  if (!res.ok) {
    const error = await res.json();
    throw new Error(error.message || 'Failed to update email template');
  }
  return res.json();
}

export async function deleteEmailTemplate(id: string): Promise<void> {
  const res = await authFetch(`/api/v1/email-templates/${id}`, {
    method: 'DELETE',
  });
  if (!res.ok) throw new Error('Failed to delete email template');
}
