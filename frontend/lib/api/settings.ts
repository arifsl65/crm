/**
 * Settings API - Company settings and branding.
 */

import { authFetch } from './core';
import type { CompanySettings } from './types';

export async function getSettings(): Promise<{ settings: CompanySettings }> {
  const res = await authFetch('/api/v1/settings');
  if (!res.ok) throw new Error('Failed to fetch settings');
  return res.json();
}

export async function updateSettings(data: Partial<CompanySettings>): Promise<void> {
  const res = await authFetch('/api/v1/settings', {
    method: 'PATCH',
    body: JSON.stringify(data),
  });
  if (!res.ok) {
    const error = await res.json();
    throw new Error(error.message || 'Failed to update settings');
  }
}

export async function getBranding(): Promise<{ branding: { firm_name: string; logo_url?: string; email: string; phone?: string } }> {
  const res = await authFetch('/api/v1/settings/branding');
  if (!res.ok) throw new Error('Failed to fetch branding');
  return res.json();
}

export async function updateBranding(data: { firm_name?: string; logo_url?: string }): Promise<void> {
  const res = await authFetch('/api/v1/settings/branding', {
    method: 'PATCH',
    body: JSON.stringify(data),
  });
  if (!res.ok) {
    const error = await res.json();
    throw new Error(error.message || 'Failed to update branding');
  }
}
