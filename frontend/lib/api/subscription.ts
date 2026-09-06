/**
 * Subscription API - Billing, invoices, and usage.
 */

import { authFetch } from './core';
import type { Subscription, Invoice, UsageStats, PushToken } from './types';

// ============================================================================
// Subscription
// ============================================================================

export async function getSubscription(): Promise<{ subscription: Subscription }> {
  const res = await authFetch('/api/v1/subscription');
  if (!res.ok) throw new Error('Failed to fetch subscription');
  return res.json();
}

export async function getInvoices(): Promise<{ invoices: Invoice[]; count: number }> {
  const res = await authFetch('/api/v1/subscription/invoices');
  if (!res.ok) throw new Error('Failed to fetch invoices');
  return res.json();
}

export async function getUsageStats(): Promise<{ usage: UsageStats }> {
  const res = await authFetch('/api/v1/subscription/usage');
  if (!res.ok) throw new Error('Failed to fetch usage stats');
  return res.json();
}

export async function createBillingPortalSession(): Promise<{ url?: string; message: string }> {
  const res = await authFetch('/api/v1/subscription/portal', {
    method: 'POST',
  });
  if (!res.ok) throw new Error('Failed to create billing portal session');
  return res.json();
}

export async function createCheckoutSession(plan: string): Promise<{ url?: string; message: string }> {
  const res = await authFetch('/api/v1/subscription/checkout', {
    method: 'POST',
    body: JSON.stringify({ plan }),
  });
  if (!res.ok) throw new Error('Failed to create checkout session');
  return res.json();
}

// ============================================================================
// Push Tokens
// ============================================================================

export async function getPushTokens(): Promise<{ push_tokens: PushToken[]; count: number }> {
  const res = await authFetch('/api/v1/push-tokens');
  if (!res.ok) throw new Error('Failed to fetch push tokens');
  return res.json();
}

export async function registerPushToken(data: {
  token: string;
  platform: 'ios' | 'android' | 'web';
}): Promise<{ push_token: PushToken; message: string }> {
  const res = await authFetch('/api/v1/push-tokens', {
    method: 'POST',
    body: JSON.stringify(data),
  });
  if (!res.ok) {
    const error = await res.json();
    throw new Error(error.message || 'Failed to register push token');
  }
  return res.json();
}

export async function unregisterPushToken(id: string): Promise<void> {
  const res = await authFetch(`/api/v1/push-tokens/${id}`, {
    method: 'DELETE',
  });
  if (!res.ok) throw new Error('Failed to unregister push token');
}

export async function unregisterPushTokenByValue(token: string): Promise<void> {
  const res = await authFetch('/api/v1/push-tokens/unregister', {
    method: 'POST',
    body: JSON.stringify({ token }),
  });
  if (!res.ok) throw new Error('Failed to unregister push token');
}
