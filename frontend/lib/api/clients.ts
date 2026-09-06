/**
 * Clients API - Client CRUD, notes, directors, PSC, and Companies House integration.
 */

import { authFetch } from './core';
import type {
  Client,
  Document,
  Service,
  Email,
  ClientNote,
  Director,
  PSC,
  CHCompanySearchResult,
  CHCompanyProfile,
  CHFiling,
  CHOfficer,
} from './types';

// ============================================================================
// Clients CRUD
// ============================================================================

export async function getClients(params?: {
  limit?: number;
  offset?: number;
  status?: string;
  search?: string;
}): Promise<{ clients: Client[]; limit: number; offset: number }> {
  const searchParams = new URLSearchParams();
  if (params?.limit) searchParams.set('limit', String(params.limit));
  if (params?.offset) searchParams.set('offset', String(params.offset));
  if (params?.status) searchParams.set('status', params.status);
  if (params?.search) searchParams.set('search', params.search);

  const res = await authFetch(`/api/v1/clients?${searchParams}`);
  if (!res.ok) throw new Error('Failed to fetch clients');
  return res.json();
}

export async function getClient(id: string): Promise<Client> {
  const res = await authFetch(`/api/v1/clients/${id}`);
  if (!res.ok) throw new Error('Failed to fetch client');
  return res.json();
}

export async function createClient(data: Partial<Client>): Promise<{ id: string }> {
  const res = await authFetch('/api/v1/clients', {
    method: 'POST',
    body: JSON.stringify(data),
  });
  if (!res.ok) {
    const error = await res.json();
    throw new Error(error.message || 'Failed to create client');
  }
  return res.json();
}

export async function updateClient(id: string, data: Partial<Client>): Promise<void> {
  const res = await authFetch(`/api/v1/clients/${id}`, {
    method: 'PATCH',
    body: JSON.stringify(data),
  });
  if (!res.ok) {
    const error = await res.json();
    throw new Error(error.message || 'Failed to update client');
  }
}

export async function deleteClient(id: string): Promise<void> {
  const res = await authFetch(`/api/v1/clients/${id}`, {
    method: 'DELETE',
  });
  if (!res.ok) throw new Error('Failed to delete client');
}

export async function restoreClient(id: string): Promise<void> {
  const res = await authFetch(`/api/v1/clients/${id}/restore`, {
    method: 'POST',
  });
  if (!res.ok) throw new Error('Failed to restore client');
}

// ============================================================================
// Client Related Data
// ============================================================================

export async function getClientDocuments(clientId: string): Promise<{ documents: Document[] }> {
  const res = await authFetch(`/api/v1/clients/${clientId}/documents`);
  if (!res.ok) throw new Error('Failed to fetch client documents');
  return res.json();
}

export async function getClientServices(clientId: string): Promise<{ services: Service[] }> {
  const res = await authFetch(`/api/v1/clients/${clientId}/services`);
  if (!res.ok) throw new Error('Failed to fetch client services');
  return res.json();
}

export async function getClientEmails(clientId: string, params?: {
  limit?: number;
  offset?: number;
  direction?: 'inbound' | 'outbound';
}): Promise<{ emails: Email[]; count: number }> {
  const searchParams = new URLSearchParams();
  if (params?.limit) searchParams.set('limit', String(params.limit));
  if (params?.offset) searchParams.set('offset', String(params.offset));
  if (params?.direction) searchParams.set('direction', params.direction);

  const res = await authFetch(`/api/v1/clients/${clientId}/emails?${searchParams}`);
  if (!res.ok) throw new Error('Failed to fetch client emails');
  return res.json();
}

// ============================================================================
// Client Notes
// ============================================================================

export async function getClientNotes(clientId: string): Promise<{ notes: ClientNote[] }> {
  const res = await authFetch(`/api/v1/clients/${clientId}/notes`);
  if (!res.ok) throw new Error('Failed to fetch client notes');
  return res.json();
}

export async function createClientNote(clientId: string, note: string): Promise<{ id: string }> {
  const res = await authFetch(`/api/v1/clients/${clientId}/notes`, {
    method: 'POST',
    body: JSON.stringify({ note }),
  });
  if (!res.ok) {
    const error = await res.json();
    throw new Error(error.message || 'Failed to create note');
  }
  return res.json();
}

export async function updateClientNote(clientId: string, noteId: string, note: string): Promise<void> {
  const res = await authFetch(`/api/v1/clients/${clientId}/notes/${noteId}`, {
    method: 'PATCH',
    body: JSON.stringify({ note }),
  });
  if (!res.ok) {
    const error = await res.json();
    throw new Error(error.message || 'Failed to update note');
  }
}

export async function deleteClientNote(clientId: string, noteId: string): Promise<void> {
  const res = await authFetch(`/api/v1/clients/${clientId}/notes/${noteId}`, {
    method: 'DELETE',
  });
  if (!res.ok) throw new Error('Failed to delete note');
}

// ============================================================================
// Staff Assignment
// ============================================================================

export async function assignStaffToClient(
  clientId: string,
  staffId: string,
  isPrimary?: boolean
): Promise<void> {
  const res = await authFetch(`/api/v1/clients/${clientId}/assign`, {
    method: 'POST',
    body: JSON.stringify({ staff_id: staffId, is_primary: isPrimary ?? false }),
  });
  if (!res.ok) {
    const error = await res.json();
    throw new Error(error.message || 'Failed to assign staff');
  }
}

export async function getSuppressedClients(params?: {
  limit?: number;
  offset?: number;
}): Promise<{ clients: Client[]; count: number }> {
  const searchParams = new URLSearchParams();
  if (params?.limit) searchParams.set('limit', String(params.limit));
  if (params?.offset) searchParams.set('offset', String(params.offset));

  const res = await authFetch(`/api/v1/clients/suppressed?${searchParams}`);
  if (!res.ok) throw new Error('Failed to fetch suppressed clients');
  return res.json();
}

export async function bulkReassignClients(data: {
  client_ids: string[];
  from_staff_id?: string;
  to_staff_id: string;
}): Promise<{ reassigned: number }> {
  const res = await authFetch('/api/v1/clients/bulk-reassign', {
    method: 'POST',
    body: JSON.stringify(data),
  });
  if (!res.ok) {
    const error = await res.json();
    throw new Error(error.message || 'Failed to bulk reassign clients');
  }
  return res.json();
}

// ============================================================================
// Directors & PSC (Companies House synced)
// ============================================================================

export async function getClientDirectors(clientId: string): Promise<{ directors: Director[] }> {
  const res = await authFetch(`/api/v1/clients/${clientId}/directors`);
  if (!res.ok) {
    const error = await res.json();
    throw new Error(error.message || 'Failed to fetch directors');
  }
  return res.json();
}

export async function getClientPSC(clientId: string): Promise<{ psc: PSC[] }> {
  const res = await authFetch(`/api/v1/clients/${clientId}/psc`);
  if (!res.ok) {
    const error = await res.json();
    throw new Error(error.message || 'Failed to fetch PSC');
  }
  return res.json();
}

// ============================================================================
// Companies House API
// ============================================================================

export async function searchCompaniesHouse(query: string): Promise<{ items: CHCompanySearchResult[]; total_results: number }> {
  const res = await authFetch(`/api/v1/ch/search?q=${encodeURIComponent(query)}`);
  if (!res.ok) {
    const error = await res.json();
    throw new Error(error.message || 'Companies House search failed');
  }
  return res.json();
}

export async function getCompanyFromCH(companyNumber: string): Promise<CHCompanyProfile> {
  const res = await authFetch(`/api/v1/ch/company/${companyNumber}`);
  if (!res.ok) {
    const error = await res.json();
    throw new Error(error.message || 'Failed to fetch company from Companies House');
  }
  return res.json();
}

export async function syncClientWithCH(clientId: string): Promise<{ directors_synced: number; psc_synced: number }> {
  const res = await authFetch(`/api/v1/ch/sync/${clientId}`, {
    method: 'POST',
  });
  if (!res.ok) {
    const error = await res.json();
    throw new Error(error.message || 'Failed to sync with Companies House');
  }
  return res.json();
}

export async function getCHStatus(): Promise<{ status: string; circuit_state: string }> {
  const res = await authFetch('/api/v1/ch/status');
  if (!res.ok) throw new Error('Failed to get Companies House status');
  return res.json();
}

export async function getCHFilings(companyNumber: string, params?: {
  start_index?: number;
  items_per_page?: number;
  category?: string;
}): Promise<{ items: CHFiling[]; total_count: number; start_index: number; items_per_page: number }> {
  const searchParams = new URLSearchParams();
  if (params?.start_index) searchParams.set('start_index', String(params.start_index));
  if (params?.items_per_page) searchParams.set('items_per_page', String(params.items_per_page));
  if (params?.category) searchParams.set('category', params.category);

  const res = await authFetch(`/api/v1/ch/company/${companyNumber}/filings?${searchParams}`);
  if (!res.ok) {
    const error = await res.json();
    throw new Error(error.message || 'Failed to fetch filings');
  }
  return res.json();
}

export async function getCHOfficers(companyNumber: string): Promise<{
  officers: CHOfficer[];
  total_results: number;
}> {
  const res = await authFetch(`/api/v1/ch/company/${companyNumber}/officers`);
  if (!res.ok) {
    const error = await res.json();
    throw new Error(error.message || 'Failed to fetch officers');
  }
  return res.json();
}
