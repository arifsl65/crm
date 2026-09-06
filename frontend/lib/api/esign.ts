/**
 * E-Sign API - Electronic signature requests and signing flow.
 */

import { authFetch, API_URL } from './core';
import type { ESignRequest, SigningPageData } from './types';

// ============================================================================
// E-Sign Requests (Authenticated)
// ============================================================================

export async function getESignRequests(params?: {
  limit?: number;
  offset?: number;
  status?: string;
  client_id?: string;
}): Promise<{ e_sign_requests: ESignRequest[]; count: number }> {
  const searchParams = new URLSearchParams();
  if (params?.limit) searchParams.set('limit', String(params.limit));
  if (params?.offset) searchParams.set('offset', String(params.offset));
  if (params?.status) searchParams.set('status', params.status);
  if (params?.client_id) searchParams.set('client_id', params.client_id);

  const res = await authFetch(`/api/v1/e-sign?${searchParams}`);
  if (!res.ok) throw new Error('Failed to fetch e-sign requests');
  return res.json();
}

export async function getESignRequest(id: string): Promise<{ e_sign_request: ESignRequest }> {
  const res = await authFetch(`/api/v1/e-sign/${id}`);
  if (!res.ok) throw new Error('Failed to fetch e-sign request');
  return res.json();
}

export async function createESignRequest(data: {
  client_id: string;
  document_id?: string;
  template_type: string;
  signer_email: string;
  signer_name?: string;
  expires_in_days?: number;
  auto_create_service?: boolean;
  service_type_id?: string;
}): Promise<{ e_sign_request: ESignRequest }> {
  const res = await authFetch('/api/v1/e-sign', {
    method: 'POST',
    body: JSON.stringify(data),
  });
  if (!res.ok) {
    const error = await res.json();
    throw new Error(error.message || 'Failed to create e-sign request');
  }
  return res.json();
}

export async function sendESignRequest(id: string): Promise<{ message: string }> {
  const res = await authFetch(`/api/v1/e-sign/${id}/send`, {
    method: 'POST',
  });
  if (!res.ok) throw new Error('Failed to send e-sign request');
  return res.json();
}

export async function deleteESignRequest(id: string): Promise<void> {
  const res = await authFetch(`/api/v1/e-sign/${id}`, {
    method: 'DELETE',
  });
  if (!res.ok) throw new Error('Failed to delete e-sign request');
}

// ============================================================================
// Public E-Sign Endpoints (No Auth Required)
// ============================================================================

export async function getSigningPageData(token: string): Promise<SigningPageData> {
  const res = await fetch(`${API_URL}/api/v1/e-sign/sign/${token}`);
  if (res.status === 410) {
    const data = await res.json();
    throw new Error(data.error || 'expired');
  }
  if (!res.ok) throw new Error('Invalid or expired signing link');
  return res.json();
}

export async function submitSignature(token: string, signatureData: {
  signature: string;
  full_name: string;
  agreed_to_terms: boolean;
}): Promise<{ message: string; signed_at: string }> {
  const res = await fetch(`${API_URL}/api/v1/e-sign/sign/${token}`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ signature_data: signatureData }),
  });
  if (!res.ok) {
    const error = await res.json();
    throw new Error(error.error || 'Failed to submit signature');
  }
  return res.json();
}
