const API_URL = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080';

export interface Client {
  id: string;
  tenant_id: string;
  user_id?: string;
  company_name: string;
  contact_name: string;
  email: string;
  phone?: string;
  address?: string;
  year_end?: string;
  utr?: string;
  company_number?: string;
  company_type?: string;
  incorporation_date?: string;
  vat_number?: string;
  vat_quarter?: string;
  status: string;
  risk_score?: number;
  email_status: string;
  last_contact_at?: string;
  created_at: string;
  updated_at: string;
}

export interface Service {
  id: string;
  tenant_id: string;
  client_id?: string;
  staff_id?: string;
  type_id?: string;
  name: string;
  period?: string;
  status: string;
  priority: string;
  risk_level?: string;
  deadline?: string;
  kanban_position: number;
  docs_required: number;
  docs_received: number;
  hmrc_reference?: string;
  filed_at?: string;
  completed_at?: string;
  completion_notes?: string;
  version: number;
  created_at: string;
  updated_at: string;
  client_name?: string;
  staff_name?: string;
}

export interface Document {
  id: string;
  tenant_id: string;
  client_id?: string;
  service_id?: string;
  uploaded_by?: string;
  type_id?: string;
  name: string;
  original_name: string;
  file_path?: string;
  file_size?: number;
  mime_type?: string;
  status: string;
  access: string;
  version: number;
  expiry_date?: string;
  chase_count: number;
  ai_summary?: string;
  review_note?: string;
  reviewed_by?: string;
  reviewed_at?: string;
  created_at: string;
  updated_at: string;
  client_name?: string;
  type_name?: string;
  uploaded_by_name?: string;
  reviewed_by_name?: string;
}

export interface DashboardStats {
  total_clients: number;
  active_clients: number;
  inactive_clients: number;
  total_services: number;
  services_in_progress: number;
  services_overdue: number;
  services_due_soon: number;
  services_completed: number;
  total_documents: number;
  documents_requested: number;
  documents_pending: number;
  documents_approved: number;
  recent_activity: ActivityItem[];
}

export interface ActivityItem {
  id: string;
  action: string;
  entity_type: string;
  entity_id?: string;
  description: string;
  user_name?: string;
  created_at: string;
}

// Token refresh state to prevent multiple simultaneous refresh attempts
let isRefreshing = false;
let refreshPromise: Promise<boolean> | null = null;

/**
 * Attempt to refresh the access token using the httpOnly refresh_token cookie.
 * The backend sets new httpOnly cookies on success - no localStorage needed.
 */
async function tryRefreshToken(): Promise<boolean> {
  // Skip refresh if no prior session exists (incognito/fresh browser)
  // This prevents 400 errors when there's no refresh cookie to send
  if (typeof window !== 'undefined' && !localStorage.getItem('access_token')) {
    return false;
  }

  // If already refreshing, wait for the existing refresh to complete
  if (isRefreshing && refreshPromise) {
    return refreshPromise;
  }

  isRefreshing = true;
  refreshPromise = (async () => {
    try {
      const res = await fetch(`${API_URL}/api/v1/auth/refresh`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'include', // Send httpOnly cookies
      });

      if (!res.ok) {
        return false;
      }

      // Backend sets new httpOnly cookies automatically via Set-Cookie header
      // No need to store tokens client-side
      return true;
    } catch {
      return false;
    } finally {
      isRefreshing = false;
      refreshPromise = null;
    }
  })();

  return refreshPromise;
}

// Custom event for auth expiry - components can listen and handle navigation
// Fix #36: Use custom event instead of window.location.href for SPA-friendly redirect
export const AUTH_EXPIRED_EVENT = 'auth-expired';

/**
 * Clear auth state and trigger redirect to login.
 * Note: httpOnly cookies are cleared by the backend on logout via Set-Cookie.
 * We clear client-side tokens and user data here.
 */
function clearAuthAndRedirect(): void {
  if (typeof window !== 'undefined') {
    // Clear tokens and user data from localStorage
    // This prevents redirect loops when access_token is stale but still present
    localStorage.removeItem('access_token');
    localStorage.removeItem('user');

    // Dispatch custom event for React components to handle
    window.dispatchEvent(new CustomEvent(AUTH_EXPIRED_EVENT));
  }
}

/**
 * Helper to make authenticated requests with automatic token refresh.
 * Uses httpOnly cookies for authentication (credentials: 'include').
 * No localStorage token handling needed - cookies are sent automatically.
 */
async function authFetch(url: string, options: RequestInit = {}): Promise<Response> {
  const makeRequest = async () => {
    const headers: HeadersInit = {
      'Content-Type': 'application/json',
      ...(options.headers || {}),
    };

    return fetch(`${API_URL}${url}`, {
      ...options,
      headers,
      credentials: 'include', // Send httpOnly cookies automatically
    });
  };

  let res = await makeRequest();

  // If 401 Unauthorized, try to refresh the token
  if (res.status === 401) {
    const refreshed = await tryRefreshToken();

    if (refreshed) {
      // Retry the original request with the new token
      res = await makeRequest();
    } else {
      // Refresh failed, redirect to login
      clearAuthAndRedirect();
    }
  }

  return res;
}

// Dashboard API
export async function getDashboardStats(): Promise<DashboardStats> {
  const res = await authFetch('/api/v1/dashboard/stats');
  if (!res.ok) throw new Error('Failed to fetch dashboard stats');
  return res.json();
}

export interface DashboardDeadline {
  id: string;
  name: string;
  status: string;
  priority: string;
  deadline: string;
  client_id: string;
  client_name: string;
  urgency: 'overdue' | 'today' | 'urgent' | 'soon' | 'upcoming';
}

export async function getDashboardDeadlines(): Promise<{ deadlines: DashboardDeadline[] }> {
  const res = await authFetch('/api/v1/dashboard/deadlines');
  if (!res.ok) throw new Error('Failed to fetch deadlines');
  return res.json();
}

export async function getKanban(): Promise<Record<string, Service[]>> {
  const res = await authFetch('/api/v1/dashboard/kanban');
  if (!res.ok) throw new Error('Failed to fetch kanban');
  return res.json();
}

// Clients API
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

// Client Notes API
export interface ClientNote {
  id: string;
  tenant_id: string;
  client_id: string;
  staff_id: string;
  staff_name?: string;
  note: string;
  is_pinned?: boolean;
  created_at: string;
  updated_at: string;
}

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

// Staff Assignment API
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

// Companies House API
export interface CHCompanySearchResult {
  company_number: string;
  company_name: string;
  company_status: string;
  company_type: string;
  date_of_creation?: string;
  address_snippet?: string;
  registered_office_address?: {
    address_line_1?: string;
    address_line_2?: string;
    locality?: string;
    region?: string;
    postal_code?: string;
    country?: string;
  };
}

export interface CHCompanyProfile {
  company_number: string;
  company_name: string;
  company_status: string;
  company_type: string;
  date_of_creation?: string;
  registered_office_address?: {
    address_line_1?: string;
    address_line_2?: string;
    locality?: string;
    region?: string;
    postal_code?: string;
    country?: string;
  };
  sic_codes?: string[];
  accounts?: {
    accounting_reference_date?: { day?: string; month?: string };
    last_accounts?: { made_up_to?: string; type?: string };
    next_due?: string;
    next_made_up_to?: string;
  };
  confirmation_statement?: {
    last_made_up_to?: string;
    next_due?: string;
    next_made_up_to?: string;
  };
}

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

// Directors (synced from Companies House)
export interface Director {
  id: string;
  tenant_id: string;
  client_id: string;
  name: string;
  role: string;
  appointed_date?: string;
  resigned_date?: string;
  nationality?: string;
  dob_month?: number;
  dob_year?: number;
  is_active: boolean;
  created_at: string;
}

export async function getClientDirectors(clientId: string): Promise<{ directors: Director[] }> {
  const res = await authFetch(`/api/v1/clients/${clientId}/directors`);
  if (!res.ok) {
    const error = await res.json();
    throw new Error(error.message || 'Failed to fetch directors');
  }
  return res.json();
}

// PSC - Persons with Significant Control (synced from Companies House)
export interface PSC {
  id: string;
  tenant_id: string;
  client_id: string;
  name: string;
  ownership_percentage?: string;
  notified_date?: string;
  ceased_date?: string;
  nature_of_control?: string[];
  is_active: boolean;
  created_at: string;
}

export async function getClientPSC(clientId: string): Promise<{ psc: PSC[] }> {
  const res = await authFetch(`/api/v1/clients/${clientId}/psc`);
  if (!res.ok) {
    const error = await res.json();
    throw new Error(error.message || 'Failed to fetch PSC');
  }
  return res.json();
}

// CH Filings (direct from Companies House API)
export interface CHFiling {
  transaction_id: string;
  type: string;
  description: string;
  date: string;
  category: string;
  subcategory?: string;
  action_date?: string;
  pages?: number;
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

// Services API
export async function getServices(params?: {
  limit?: number;
  offset?: number;
  status?: string;
  priority?: string;
  client_id?: string;
  search?: string;
}): Promise<{ services: Service[]; limit: number; offset: number }> {
  const searchParams = new URLSearchParams();
  if (params?.limit) searchParams.set('limit', String(params.limit));
  if (params?.offset) searchParams.set('offset', String(params.offset));
  if (params?.status) searchParams.set('status', params.status);
  if (params?.priority) searchParams.set('priority', params.priority);
  if (params?.client_id) searchParams.set('client_id', params.client_id);
  if (params?.search) searchParams.set('search', params.search);

  const res = await authFetch(`/api/v1/services?${searchParams}`);
  if (!res.ok) throw new Error('Failed to fetch services');
  return res.json();
}

export async function getService(id: string): Promise<Service> {
  const res = await authFetch(`/api/v1/services/${id}`);
  if (!res.ok) throw new Error('Failed to fetch service');
  return res.json();
}

export async function createService(data: {
  client_id: string;
  name: string;
  type_id?: string;
  period?: string;
  priority?: string;
  risk_level?: string;
  deadline?: string;
  docs_required?: number;
  staff_id?: string;
}): Promise<{ id: string }> {
  const res = await authFetch('/api/v1/services', {
    method: 'POST',
    body: JSON.stringify(data),
  });
  if (!res.ok) {
    const error = await res.json();
    throw new Error(error.message || 'Failed to create service');
  }
  return res.json();
}

export async function updateService(id: string, data: Partial<Service>): Promise<void> {
  const res = await authFetch(`/api/v1/services/${id}`, {
    method: 'PATCH',
    body: JSON.stringify(data),
  });
  if (!res.ok) {
    const error = await res.json();
    throw new Error(error.message || 'Failed to update service');
  }
}

export async function updateServiceStatus(id: string, status: string): Promise<void> {
  const res = await authFetch(`/api/v1/services/${id}/status`, {
    method: 'PATCH',
    body: JSON.stringify({ status }),
  });
  if (!res.ok) throw new Error('Failed to update service status');
}

export async function completeService(id: string, notes?: string): Promise<void> {
  const res = await authFetch(`/api/v1/services/${id}/complete`, {
    method: 'POST',
    body: JSON.stringify({ notes }),
  });
  if (!res.ok) throw new Error('Failed to complete service');
}

// Documents API
export async function getDocuments(params?: {
  limit?: number;
  offset?: number;
  status?: string;
  client_id?: string;
  type_id?: string;
  search?: string;
}): Promise<{ documents: Document[]; limit: number; offset: number }> {
  const searchParams = new URLSearchParams();
  if (params?.limit) searchParams.set('limit', String(params.limit));
  if (params?.offset) searchParams.set('offset', String(params.offset));
  if (params?.status) searchParams.set('status', params.status);
  if (params?.client_id) searchParams.set('client_id', params.client_id);
  if (params?.type_id) searchParams.set('type_id', params.type_id);
  if (params?.search) searchParams.set('search', params.search);

  const res = await authFetch(`/api/v1/documents?${searchParams}`);
  if (!res.ok) throw new Error('Failed to fetch documents');
  return res.json();
}

export async function getDocument(id: string): Promise<Document> {
  const res = await authFetch(`/api/v1/documents/${id}`);
  if (!res.ok) throw new Error('Failed to fetch document');
  return res.json();
}

export async function createDocumentRequest(data: {
  client_id?: string;
  service_id?: string;
  type_id?: string;
  name: string;
  expiry_date?: string;
  request_note?: string;
}): Promise<{ id: string }> {
  const res = await authFetch('/api/v1/documents', {
    method: 'POST',
    body: JSON.stringify(data),
  });
  if (!res.ok) {
    const error = await res.json();
    throw new Error(error.message || 'Failed to create document request');
  }
  return res.json();
}

export async function approveDocument(id: string, note?: string): Promise<void> {
  const res = await authFetch(`/api/v1/documents/${id}/approve`, {
    method: 'POST',
    body: JSON.stringify({ note }),
  });
  if (!res.ok) throw new Error('Failed to approve document');
}

export async function rejectDocument(id: string, note: string): Promise<void> {
  const res = await authFetch(`/api/v1/documents/${id}/reject`, {
    method: 'POST',
    body: JSON.stringify({ note }),
  });
  if (!res.ok) throw new Error('Failed to reject document');
}

export interface DocumentVersion {
  id: string;
  document_id: string;
  version: number;
  file_path?: string;
  file_size?: number;
  mime_type?: string;
  uploaded_by?: string;
  uploaded_by_name?: string;
  created_at: string;
}

export async function getDocumentVersions(id: string): Promise<{ versions: DocumentVersion[] }> {
  const res = await authFetch(`/api/v1/documents/${id}/versions`);
  if (!res.ok) throw new Error('Failed to fetch document versions');
  return res.json();
}

export async function restoreDocumentVersion(documentId: string, versionId: string): Promise<void> {
  const res = await authFetch(`/api/v1/documents/${documentId}/versions/${versionId}/restore`, {
    method: 'POST',
  });
  if (!res.ok) throw new Error('Failed to restore document version');
}

// Download document - returns signed URL for download
export async function downloadDocument(id: string): Promise<{ download_url: string; expires_at: string }> {
  const res = await authFetch(`/api/v1/documents/${id}/download`);
  if (!res.ok) throw new Error('Failed to get document download URL');
  return res.json();
}

// Update document metadata
export async function updateDocument(id: string, data: {
  name?: string;
  type_id?: string;
  expiry_date?: string;
  notes?: string;
}): Promise<Document> {
  const res = await authFetch(`/api/v1/documents/${id}`, {
    method: 'PATCH',
    body: JSON.stringify(data),
  });
  if (!res.ok) {
    const error = await res.json();
    throw new Error(error.message || 'Failed to update document');
  }
  return res.json();
}

// Request document renewal (for expired documents)
export async function requestDocumentRenewal(id: string, data?: {
  note?: string;
  new_expiry_date?: string;
}): Promise<{ message: string }> {
  const res = await authFetch(`/api/v1/documents/${id}/renewal`, {
    method: 'POST',
    body: JSON.stringify(data || {}),
  });
  if (!res.ok) {
    const error = await res.json();
    throw new Error(error.message || 'Failed to request document renewal');
  }
  return res.json();
}

// Bulk approve multiple documents
export async function bulkApproveDocuments(documentIds: string[]): Promise<{ approved: number; failed: number }> {
  const res = await authFetch('/api/v1/documents/bulk-approve', {
    method: 'POST',
    body: JSON.stringify({ document_ids: documentIds }),
  });
  if (!res.ok) {
    const error = await res.json();
    throw new Error(error.message || 'Failed to bulk approve documents');
  }
  return res.json();
}

// Firm Documents API - documents with client_id=NULL (internal firm documents)
export async function getFirmDocuments(params?: {
  limit?: number;
  offset?: number;
  search?: string;
}): Promise<{ documents: Document[]; limit: number; offset: number }> {
  const searchParams = new URLSearchParams();
  if (params?.limit) searchParams.set('limit', String(params.limit));
  if (params?.offset) searchParams.set('offset', String(params.offset));
  if (params?.search) searchParams.set('search', params.search);

  const res = await authFetch(`/api/v1/documents/firm?${searchParams}`);
  if (!res.ok) throw new Error('Failed to fetch firm documents');
  return res.json();
}

export async function uploadFirmDocument(data: {
  name: string;
  type_id?: string;
  access?: 'private' | 'staff' | 'all';
}): Promise<{ id: string }> {
  const res = await authFetch('/api/v1/documents/firm', {
    method: 'POST',
    body: JSON.stringify(data),
  });
  if (!res.ok) {
    const error = await res.json();
    throw new Error(error.message || 'Failed to upload firm document');
  }
  return res.json();
}

export async function getFirmDocumentAccess(id: string): Promise<{
  access: string;
  staff_ids?: string[];
}> {
  const res = await authFetch(`/api/v1/documents/firm/${id}/access`);
  if (!res.ok) throw new Error('Failed to fetch document access');
  return res.json();
}

export async function updateFirmDocumentAccess(id: string, data: {
  access: 'private' | 'staff' | 'all';
  staff_ids?: string[];
}): Promise<void> {
  const res = await authFetch(`/api/v1/documents/firm/${id}/access`, {
    method: 'PATCH',
    body: JSON.stringify(data),
  });
  if (!res.ok) throw new Error('Failed to update document access');
}

// Bulk Document Request API
export interface BulkDocumentRequest {
  client_ids: string[];
  document_requests: Array<{
    type_id?: string;
    name: string;
    expiry_date?: string;
  }>;
  request_note?: string;
}

export async function bulkRequestDocuments(data: BulkDocumentRequest): Promise<{
  created: number;
  document_ids: string[];
}> {
  const res = await authFetch('/api/v1/documents/bulk-request', {
    method: 'POST',
    body: JSON.stringify(data),
  });
  if (!res.ok) {
    const error = await res.json();
    throw new Error(error.message || 'Failed to create bulk document request');
  }
  return res.json();
}

// Document Upload URL API
export async function getDocumentUploadUrl(data: {
  filename: string;
  content_type: string;
  client_id?: string;
  service_id?: string;
}): Promise<{
  upload_url: string;
  document_id: string;
  expires_at: string;
}> {
  const res = await authFetch('/api/v1/documents/upload-url', {
    method: 'POST',
    body: JSON.stringify(data),
  });
  if (!res.ok) {
    const error = await res.json();
    throw new Error(error.message || 'Failed to get upload URL');
  }
  return res.json();
}

// Confirm document upload after uploading to signed URL
export async function confirmDocumentUpload(documentId: string, data: {
  name: string;
  original_name: string;
  file_size: number;
  mime_type: string;
  type_id?: string;
}): Promise<Document> {
  const res = await authFetch(`/api/v1/documents/${documentId}`, {
    method: 'PATCH',
    body: JSON.stringify({
      ...data,
      status: 'pending_review',
    }),
  });
  if (!res.ok) {
    const error = await res.json();
    throw new Error(error.message || 'Failed to confirm document upload');
  }
  return res.json();
}

export async function generateQRToken(data: {
  client_id: string;
  document_type_id?: string;
  note?: string;
  expires_in_minutes?: number;
}): Promise<{ token: string; expires_at: string; upload_url: string }> {
  const res = await authFetch('/api/v1/documents/qr', {
    method: 'POST',
    body: JSON.stringify(data),
  });
  if (!res.ok) {
    const error = await res.json();
    throw new Error(error.message || 'Failed to generate QR token');
  }
  return res.json();
}

export async function verifyQRToken(token: string): Promise<{
  client_name: string;
  expires_at: string;
  note?: string;
}> {
  const res = await fetch(`${process.env.NEXT_PUBLIC_API_URL || ''}/api/v1/documents/qr/${token}`);
  if (!res.ok) throw new Error('Invalid or expired token');
  return res.json();
}

export async function uploadViaQR(token: string, file: File): Promise<{ id: string; message: string }> {
  const formData = new FormData();
  formData.append('file', file);

  const res = await fetch(`${process.env.NEXT_PUBLIC_API_URL || ''}/api/v1/documents/qr/${token}/upload`, {
    method: 'POST',
    body: formData,
  });
  if (!res.ok) {
    const error = await res.json();
    throw new Error(error.message || 'Failed to upload document');
  }
  return res.json();
}

// Service Types
export interface ServiceType {
  id: string;
  tenant_id: string;
  name: string;
  category: string;
  description?: string;
  default_priority: string;
  default_deadline_days?: number;
  required_docs?: string[];
  checklist_template?: string[];
  is_recurring: boolean;
  recurrence_pattern?: string;
  hmrc_relevant: boolean;
  is_active: boolean;
  sort_order: number;
  created_at: string;
  updated_at: string;
  service_count?: number;
}

export async function getServiceTypes(params?: {
  limit?: number;
  offset?: number;
  category?: string;
  active?: boolean;
  search?: string;
}): Promise<{ service_types: ServiceType[]; count: number }> {
  const searchParams = new URLSearchParams();
  if (params?.limit) searchParams.set('limit', String(params.limit));
  if (params?.offset) searchParams.set('offset', String(params.offset));
  if (params?.category) searchParams.set('category', params.category);
  if (params?.active !== undefined) searchParams.set('active', String(params.active));
  if (params?.search) searchParams.set('search', params.search);

  const res = await authFetch(`/api/v1/service-types?${searchParams}`);
  if (!res.ok) throw new Error('Failed to fetch service types');
  return res.json();
}

export async function getServiceType(id: string): Promise<ServiceType> {
  const res = await authFetch(`/api/v1/service-types/${id}`);
  if (!res.ok) throw new Error('Failed to fetch service type');
  return res.json();
}

export async function createServiceType(data: {
  name: string;
  category: string;
  description?: string;
  default_priority?: string;
  default_deadline_days?: number;
  required_docs?: string[];
  checklist_template?: string[];
  is_recurring?: boolean;
  recurrence_pattern?: string;
  hmrc_relevant?: boolean;
}): Promise<ServiceType> {
  const res = await authFetch('/api/v1/service-types', {
    method: 'POST',
    body: JSON.stringify(data),
  });
  if (!res.ok) {
    const error = await res.json();
    throw new Error(error.message || 'Failed to create service type');
  }
  return res.json();
}

export async function updateServiceType(id: string, data: Partial<ServiceType>): Promise<ServiceType> {
  const res = await authFetch(`/api/v1/service-types/${id}`, {
    method: 'PATCH',
    body: JSON.stringify(data),
  });
  if (!res.ok) {
    const error = await res.json();
    throw new Error(error.message || 'Failed to update service type');
  }
  return res.json();
}

export async function deleteServiceType(id: string): Promise<void> {
  const res = await authFetch(`/api/v1/service-types/${id}`, {
    method: 'DELETE',
  });
  if (!res.ok) throw new Error('Failed to delete service type');
}

export async function getServiceTypeCategories(): Promise<{ categories: string[] }> {
  const res = await authFetch('/api/v1/service-types/categories');
  if (!res.ok) throw new Error('Failed to fetch service type categories');
  return res.json();
}

export async function cloneServiceType(id: string): Promise<ServiceType> {
  const res = await authFetch(`/api/v1/service-types/${id}/clone`, {
    method: 'POST',
  });
  if (!res.ok) throw new Error('Failed to clone service type');
  return res.json();
}

// Document Types
export interface DocumentType {
  id: string;
  tenant_id: string;
  name: string;
  category: string;
  description?: string;
  allowed_mime_types?: string[];
  max_file_size_mb?: number;
  retention_days?: number;
  requires_approval: boolean;
  expiry_required: boolean;
  is_active: boolean;
  sort_order: number;
  created_at: string;
  updated_at: string;
  document_count?: number;
}

export async function getDocumentTypes(params?: {
  limit?: number;
  offset?: number;
  category?: string;
  active?: boolean;
  search?: string;
}): Promise<{ document_types: DocumentType[]; count: number }> {
  const searchParams = new URLSearchParams();
  if (params?.limit) searchParams.set('limit', String(params.limit));
  if (params?.offset) searchParams.set('offset', String(params.offset));
  if (params?.category) searchParams.set('category', params.category);
  if (params?.active !== undefined) searchParams.set('active', String(params.active));
  if (params?.search) searchParams.set('search', params.search);

  const res = await authFetch(`/api/v1/document-types?${searchParams}`);
  if (!res.ok) throw new Error('Failed to fetch document types');
  return res.json();
}

export async function getDocumentType(id: string): Promise<DocumentType> {
  const res = await authFetch(`/api/v1/document-types/${id}`);
  if (!res.ok) throw new Error('Failed to fetch document type');
  return res.json();
}

export async function createDocumentType(data: {
  name: string;
  category: string;
  description?: string;
  allowed_mime_types?: string[];
  max_file_size_mb?: number;
  retention_days?: number;
  requires_approval?: boolean;
  expiry_required?: boolean;
}): Promise<DocumentType> {
  const res = await authFetch('/api/v1/document-types', {
    method: 'POST',
    body: JSON.stringify(data),
  });
  if (!res.ok) {
    const error = await res.json();
    throw new Error(error.message || 'Failed to create document type');
  }
  return res.json();
}

export async function updateDocumentType(id: string, data: Partial<DocumentType>): Promise<DocumentType> {
  const res = await authFetch(`/api/v1/document-types/${id}`, {
    method: 'PATCH',
    body: JSON.stringify(data),
  });
  if (!res.ok) {
    const error = await res.json();
    throw new Error(error.message || 'Failed to update document type');
  }
  return res.json();
}

export async function deleteDocumentType(id: string): Promise<void> {
  const res = await authFetch(`/api/v1/document-types/${id}`, {
    method: 'DELETE',
  });
  if (!res.ok) throw new Error('Failed to delete document type');
}

export async function getDocumentTypeCategories(): Promise<{ categories: string[] }> {
  const res = await authFetch('/api/v1/document-types/categories');
  if (!res.ok) throw new Error('Failed to fetch document type categories');
  return res.json();
}

// ============================================================================
// Email Types and API
// ============================================================================

export interface Email {
  id: string;
  tenant_id: string;
  client_id?: string;
  staff_id?: string;
  template_id?: string;
  thread_id?: string;
  reply_to_id?: string;
  direction: 'inbound' | 'outbound';
  to_email: string;
  to_name?: string;
  from_email: string;
  subject: string;
  body_html: string;
  body_text?: string;
  type: 'chase' | 'notification' | 'invite' | 'manual';
  status: 'queued' | 'sent' | 'delivered' | 'opened' | 'clicked' | 'bounced' | 'complained';
  resend_id?: string;
  is_read: boolean;
  ai_summary?: string;
  sentiment?: string;
  sent_at?: string;
  opened_at?: string;
  bounced_at?: string;
  bounce_reason?: string;
  created_at: string;
  client_name?: string;
  staff_name?: string;
}

export interface EmailAccount {
  id: string;
  tenant_id: string;
  user_id?: string;
  email: string;
  type: 'shared' | 'personal';
  auth_method: 'imap' | 'oauth';
  provider: 'imap' | 'google' | 'microsoft' | 'zoho';
  imap_host?: string;
  imap_port?: number;
  status: 'active' | 'error' | 'disconnected';
  last_sync_at?: string;
  error_message?: string;
  oauth_expires_at?: string;
  created_at: string;
  updated_at: string;
  user_name?: string;
}

export interface EmailTemplate {
  id: string;
  tenant_id: string;
  name: string;
  subject: string;
  body_html: string;
  body_text?: string;
  type: 'chase' | 'notification' | 'welcome' | 'custom';
  category?: string;
  placeholders?: string[];
  is_default: boolean;
  is_active: boolean;
  created_by?: string;
  created_at: string;
  updated_at: string;
}

export interface EmailStats {
  total: number;
  sent: number;
  received: number;
  delivered: number;
  opened: number;
  bounced: number;
  unread: number;
}

// Email API functions
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

// Email Account API functions
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

// Email Template API functions
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

// Notifications API
export interface Notification {
  id: string;
  tenant_id: string;
  user_id: string;
  type: 'document' | 'deadline' | 'email' | 'system' | 'reminder';
  title: string;
  message: string;
  entity_type?: string;
  entity_id?: string;
  link?: string;
  is_read: boolean;
  remind_at?: string;
  dismissed_at?: string;
  created_at: string;
}

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

// Settings API
export interface CompanySettings {
  id: string;
  tenant_id: string;
  firm_name: string;
  email: string;
  phone?: string;
  address?: string;
  logo_url?: string;
  stripe_account_id?: string;
  stripe_connected: boolean;
  reminder_rules?: {
    day3?: boolean;
    day7?: boolean;
    day14?: boolean;
  };
  updated_by?: string;
  created_at: string;
  updated_at: string;
}

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

// Users/Staff API
export interface User {
  id: string;
  tenant_id: string;
  email: string;
  name: string;
  role: 'super_admin' | 'tenant_admin' | 'staff' | 'client';
  phone?: string;
  avatar_url?: string;
  status?: 'pending' | 'active' | 'inactive';
  is_active: boolean;
  two_factor_enabled: boolean;
  specialism?: string;
  last_login_at?: string;
  created_at: string;
  updated_at: string;
}

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

// E-Sign API
export interface ESignRequest {
  id: string;
  tenant_id: string;
  client_id: string;
  document_id?: string;
  template_type: string;
  status: 'pending' | 'signed' | 'expired' | 'declined';
  signer_email: string;
  signer_name?: string;
  sent_at?: string;
  signed_at?: string;
  expires_at?: string;
  signature_data?: Record<string, unknown>;
  auto_create_service: boolean;
  service_type_id?: string;
  created_service_id?: string;
  created_at: string;
  client_name?: string;
}

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

// Public E-Sign endpoints (no auth required)
export interface SigningPageData {
  request_id: string;
  template_type: string;
  template_title: string;
  signer_email: string;
  signer_name?: string;
  client_name: string;
  tenant_name: string;
  expires_at?: string;
}

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

// Reminders API
export interface Reminder {
  id: string;
  tenant_id: string;
  user_id: string;
  client_id?: string;
  email_id?: string;
  document_id?: string;
  service_id?: string;
  remind_at: string;
  reason?: string;
  status: 'pending' | 'completed' | 'dismissed';
  created_at: string;
  updated_at: string;
  client_name?: string;
  document_name?: string;
  service_name?: string;
}

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

// Subscription API
export interface Subscription {
  id: string;
  tenant_id: string;
  stripe_customer_id: string;
  stripe_subscription_id?: string;
  plan: 'starter' | 'professional' | 'enterprise';
  status: 'trialing' | 'active' | 'past_due' | 'canceled' | 'unpaid';
  current_period_start?: string;
  current_period_end?: string;
  created_at: string;
  updated_at: string;
}

export interface Invoice {
  id: string;
  tenant_id: string;
  stripe_invoice_id: string;
  amount_due: number;
  amount_paid: number;
  currency: string;
  status: 'draft' | 'open' | 'paid' | 'void' | 'uncollectible';
  invoice_pdf?: string;
  period_start?: string;
  period_end?: string;
  created_at: string;
}

export interface UsageStats {
  clients: number;
  users: number;
  services: number;
  documents: number;
  emails_monthly: number;
}

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

// Push Tokens API
export interface PushToken {
  id: string;
  tenant_id: string;
  user_id: string;
  token: string;
  platform: 'ios' | 'android' | 'web';
  is_active: boolean;
  last_used_at?: string;
  created_at: string;
}

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

// ============================================================================
// Portal API (Client-facing portal)
// ============================================================================

export interface PortalUser {
  id: string;
  email: string;
  name: string;
  role: string;
  two_factor_enabled: boolean;
}

export interface PortalDashboard {
  client_name: string;
  pending_documents: number;
  upcoming_deadlines: number;
  active_services: number;
  recent_activity: Array<{
    id: string;
    type: string;
    description: string;
    created_at: string;
  }>;
}

export interface PortalDocument {
  id: string;
  name: string;
  type_name?: string;
  status: string;
  expiry_date?: string;
  created_at: string;
}

export interface PortalService {
  id: string;
  name: string;
  status: string;
  deadline?: string;
  docs_required: number;
  docs_received: number;
}

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

/**
 * Logout the user by calling the backend logout endpoint (which clears cookies)
 * and clearing any client-side state.
 */
export async function logout(): Promise<void> {
  try {
    // Call backend logout to clear httpOnly cookies
    await authFetch('/api/v1/auth/logout', { method: 'POST' });
  } catch {
    // Continue with client-side cleanup even if backend call fails
  }
  // Clear client-side state
  if (typeof window !== 'undefined') {
    sessionStorage.removeItem('user');
    window.dispatchEvent(new CustomEvent(AUTH_EXPIRED_EVENT));
  }
}

// ============================================================================
// AUTH API - Authentication & Authorization Endpoints
// ============================================================================
// These endpoints handle user authentication, 2FA, password management,
// session management, and token operations. Most are used by the login page
// or settings pages rather than the main app.
// ============================================================================

/**
 * Auth response containing JWT tokens and user info.
 * Tokens are also set as httpOnly cookies by the backend.
 */
export interface AuthResponse {
  access_token: string;
  refresh_token: string;
  expires_at: string;
  user: AuthUser;
}

export interface AuthUser {
  id: string;
  email: string;
  name: string;
  role: 'super_admin' | 'tenant_admin' | 'staff' | 'client';
  tenant_id: string;
}

/**
 * Active session information for the current user.
 * Used in security settings to manage logged-in devices.
 */
export interface Session {
  id: string;
  user_id: string;
  ip_address: string;
  user_agent: string;
  is_current: boolean;
  last_active_at: string;
  created_at: string;
  expires_at: string;
}

/**
 * 2FA setup response with TOTP secret and QR code.
 * QR code can be scanned with authenticator apps like Google Authenticator.
 */
export interface TwoFactorSetupResponse {
  secret: string;
  qr_code?: string;
  recovery_codes?: string[];
}

/**
 * Login with email and password.
 *
 * WHY: Primary authentication method. Returns JWT tokens and sets httpOnly cookies.
 * Rate limited: 5 attempts per IP+email, 15min lockout on failure.
 *
 * @param email - User's email address
 * @param password - User's password (min 8 chars)
 * @param tenantDomain - Optional: specify tenant if email exists in multiple tenants
 */
export async function login(
  email: string,
  password: string,
  tenantDomain?: string
): Promise<AuthResponse> {
  const res = await fetch(`${API_URL}/api/v1/auth/login`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    credentials: 'include', // Receive httpOnly cookies
    body: JSON.stringify({ email, password, tenant_domain: tenantDomain }),
  });
  if (!res.ok) {
    const error = await res.json();
    throw new Error(error.message || 'Login failed');
  }
  return res.json();
}

/**
 * Register a new user account.
 *
 * WHY: Self-registration for new users. Creates account and returns tokens.
 * New users are created with 'staff' role by default.
 *
 * @param email - User's email address
 * @param password - Password (min 8 chars)
 * @param name - User's display name
 * @param tenantId - UUID of the tenant to register under
 */
export async function register(
  email: string,
  password: string,
  name: string,
  tenantId: string
): Promise<AuthResponse> {
  const res = await fetch(`${API_URL}/api/v1/auth/register`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    credentials: 'include',
    body: JSON.stringify({ email, password, name, tenant_id: tenantId }),
  });
  if (!res.ok) {
    const error = await res.json();
    throw new Error(error.message || 'Registration failed');
  }
  return res.json();
}

/**
 * Request a magic link for passwordless login.
 *
 * WHY: Allows users to login without remembering their password.
 * Email contains a one-time link that expires in 15 minutes.
 * Rate limited: 3 requests per email per hour.
 *
 * @param email - User's email address
 */
export async function sendMagicLink(email: string): Promise<{ message: string }> {
  const res = await fetch(`${API_URL}/api/v1/auth/magic-link`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ email }),
  });
  if (!res.ok) {
    const error = await res.json();
    throw new Error(error.message || 'Failed to send magic link');
  }
  return res.json();
}

/**
 * Verify a magic link token and login.
 *
 * WHY: Completes passwordless login flow. Called when user clicks the magic link.
 * Returns tokens and sets cookies on success.
 *
 * @param token - The magic link token from the email URL
 */
export async function verifyMagicLink(token: string): Promise<AuthResponse> {
  const res = await fetch(`${API_URL}/api/v1/auth/magic-link?token=${encodeURIComponent(token)}`, {
    method: 'GET',
    credentials: 'include',
  });
  if (!res.ok) {
    const error = await res.json();
    throw new Error(error.message || 'Invalid or expired magic link');
  }
  return res.json();
}

/**
 * Accept a team invitation and set password.
 *
 * WHY: Allows invited users to activate their account.
 * Invitation emails contain a token that expires in 7 days.
 *
 * @param token - Invitation token from the email URL
 * @param password - New password to set (min 8 chars)
 * @param name - Optional: update display name
 */
export async function acceptInvite(
  token: string,
  password: string,
  name?: string
): Promise<AuthResponse> {
  const res = await fetch(`${API_URL}/api/v1/auth/invite-accept`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    credentials: 'include',
    body: JSON.stringify({ token, password, name }),
  });
  if (!res.ok) {
    const error = await res.json();
    throw new Error(error.message || 'Invalid or expired invitation');
  }
  return res.json();
}

/**
 * Request a password reset email.
 *
 * WHY: Allows users to recover their account when they forget their password.
 * Email contains a reset link that expires in 1 hour.
 * Rate limited: 3 requests per email per hour.
 *
 * @param email - User's email address
 */
export async function forgotPassword(email: string): Promise<{ message: string }> {
  const res = await fetch(`${API_URL}/api/v1/auth/reset-password`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ email }),
  });
  if (!res.ok) {
    const error = await res.json();
    throw new Error(error.message || 'Failed to send reset email');
  }
  return res.json();
}

/**
 * Reset password using a reset token.
 *
 * WHY: Completes password recovery flow. Called when user submits new password.
 * Token is from the password reset email link.
 *
 * @param token - Reset token from the email URL
 * @param newPassword - New password to set (min 8 chars)
 */
export async function resetPassword(
  token: string,
  newPassword: string
): Promise<{ message: string }> {
  const res = await fetch(`${API_URL}/api/v1/auth/reset-password/confirm`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ token, new_password: newPassword }),
  });
  if (!res.ok) {
    const error = await res.json();
    throw new Error(error.message || 'Failed to reset password');
  }
  return res.json();
}

/**
 * Change the current user's password.
 *
 * WHY: Allows logged-in users to change their password from settings.
 * Requires current password for security. Logs out all other sessions.
 *
 * @param currentPassword - Current password for verification
 * @param newPassword - New password to set (min 8 chars)
 */
export async function changePassword(
  currentPassword: string,
  newPassword: string
): Promise<{ message: string }> {
  const res = await authFetch('/api/v1/auth/password', {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ current_password: currentPassword, new_password: newPassword }),
  });
  if (!res.ok) {
    const error = await res.json();
    throw new Error(error.message || 'Failed to change password');
  }
  return res.json();
}

/**
 * Get current authenticated user's profile.
 *
 * WHY: Fetch user details for displaying in header, settings, etc.
 * Returns full user object including settings and preferences.
 */
export async function getMe(): Promise<User> {
  const res = await authFetch('/api/v1/auth/me');
  if (!res.ok) throw new Error('Failed to fetch user profile');
  return res.json();
}

/**
 * Update current user's profile.
 *
 * WHY: Allows users to update their name, phone, avatar, and preferences.
 * Cannot change email or role through this endpoint.
 *
 * @param updates - Fields to update (name, phone, avatar_url, settings)
 */
export async function updateMe(updates: {
  name?: string;
  phone?: string;
  avatar_url?: string;
  settings?: Record<string, unknown>;
}): Promise<{ message: string }> {
  const res = await authFetch('/api/v1/auth/me', {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(updates),
  });
  if (!res.ok) {
    const error = await res.json();
    throw new Error(error.message || 'Failed to update profile');
  }
  return res.json();
}

/**
 * Get all active sessions for the current user.
 *
 * WHY: Security feature allowing users to see all logged-in devices.
 * Shows IP, user agent, and last active time for each session.
 * Users can identify and revoke suspicious sessions.
 */
export async function getSessions(): Promise<{ sessions: Session[] }> {
  const res = await authFetch('/api/v1/auth/sessions');
  if (!res.ok) throw new Error('Failed to fetch sessions');
  return res.json();
}

/**
 * Initialize 2FA setup by generating a TOTP secret.
 *
 * WHY: First step in enabling two-factor authentication.
 * Returns a secret that user adds to their authenticator app.
 * QR code is provided for easy scanning with Google Authenticator, Authy, etc.
 * User must verify with a code before 2FA is fully enabled.
 */
export async function setup2FA(): Promise<TwoFactorSetupResponse> {
  const res = await authFetch('/api/v1/auth/2fa/setup', { method: 'POST' });
  if (!res.ok) {
    const error = await res.json();
    throw new Error(error.message || 'Failed to setup 2FA');
  }
  return res.json();
}

/**
 * Verify TOTP code to complete 2FA setup.
 *
 * WHY: Confirms user has correctly configured their authenticator app.
 * After verification, 2FA is enabled and required for all future logins.
 *
 * @param code - 6-digit TOTP code from authenticator app
 */
export async function verify2FA(code: string): Promise<{ message: string; backup_codes?: string[] }> {
  const res = await authFetch('/api/v1/auth/2fa/verify', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ code }),
  });
  if (!res.ok) {
    const error = await res.json();
    throw new Error(error.message || 'Invalid verification code');
  }
  return res.json();
}

/**
 * Disable 2FA for the current user.
 *
 * WHY: Allows users to turn off 2FA if they lose their device or want simpler login.
 * Requires password confirmation for security.
 *
 * @param password - Current password for verification
 */
export async function disable2FA(password: string): Promise<{ message: string }> {
  const res = await authFetch('/api/v1/auth/2fa', {
    method: 'DELETE',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ password }),
  });
  if (!res.ok) {
    const error = await res.json();
    throw new Error(error.message || 'Failed to disable 2FA');
  }
  return res.json();
}

/**
 * Generate new backup codes for 2FA recovery.
 *
 * WHY: Provides one-time codes for account recovery if user loses their 2FA device.
 * Generates 10 new codes, invalidating any previously generated codes.
 * User should store these securely (printed or in password manager).
 */
export async function generateBackupCodes(): Promise<{ backup_codes: string[] }> {
  const res = await authFetch('/api/v1/auth/2fa/backup-codes', { method: 'POST' });
  if (!res.ok) {
    const error = await res.json();
    throw new Error(error.message || 'Failed to generate backup codes');
  }
  return res.json();
}

/**
 * Login using a backup code instead of TOTP.
 *
 * WHY: Emergency access when user has lost their 2FA device.
 * Each backup code can only be used once. After use, it's invalidated.
 * Public endpoint (no auth required) - used on login page.
 *
 * @param email - User's email address
 * @param password - User's password
 * @param backupCode - One-time backup code (format: XXXX-XXXX)
 */
export async function verifyBackupCode(
  email: string,
  password: string,
  backupCode: string
): Promise<AuthResponse> {
  const res = await fetch(`${API_URL}/api/v1/auth/2fa/backup-codes/verify`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    credentials: 'include',
    body: JSON.stringify({ email, password, backup_code: backupCode }),
  });
  if (!res.ok) {
    const error = await res.json();
    throw new Error(error.message || 'Invalid backup code');
  }
  return res.json();
}

/**
 * Revoke all tokens in a refresh token family.
 *
 * WHY: Security measure when token theft is detected.
 * When a refresh token is reused (indicating theft), the system automatically
 * revokes the entire token family. This endpoint allows manual revocation.
 *
 * @param familyId - UUID of the token family to revoke
 */
export async function revokeTokenFamily(familyId: string): Promise<{ message: string }> {
  const res = await authFetch('/api/v1/auth/refresh/revoke-family', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ family_id: familyId }),
  });
  if (!res.ok) {
    const error = await res.json();
    throw new Error(error.message || 'Failed to revoke token family');
  }
  return res.json();
}

// ============================================================================
// GRAPHQL API - Unified Query Interface for Dashboard & Aggregated Data
// ============================================================================
//
// GraphQL is the preferred interface for:
// - Dashboard data (aggregated stats, deadlines, activity, kanban board)
// - Complex queries with nested relationships (client with documents/services/emails)
// - Relay-style pagination for large datasets
// - AI analysis (troublemaker clients, anomalies)
// - Unified search across entities
// - Real-time notification state
//
// REST remains preferred for:
// - CRUD operations on individual entities
// - File uploads/downloads
// - Authentication flows
// - Webhook/integration endpoints
// ============================================================================

// -----------------------------------------------------------------------------
// GraphQL Enums
// -----------------------------------------------------------------------------

export type GQLClientStatus = 'ACTIVE' | 'INACTIVE' | 'ARCHIVED' | 'PROSPECT';

export type GQLDocumentStatus =
  | 'REQUESTED'
  | 'UPLOADED'
  | 'UNDER_REVIEW'
  | 'APPROVED'
  | 'REJECTED'
  | 'EXPIRED';

export type GQLServiceStatus =
  | 'NOT_STARTED'
  | 'IN_PROGRESS'
  | 'AWAITING_INFO'
  | 'UNDER_REVIEW'
  | 'COMPLETED'
  | 'ON_HOLD'
  | 'CANCELLED';

export type GQLServicePriority = 'LOW' | 'MEDIUM' | 'HIGH' | 'URGENT';

export type GQLEmailDirection = 'INBOUND' | 'OUTBOUND';

export type GQLEmailSentiment = 'POSITIVE' | 'NEUTRAL' | 'NEGATIVE' | 'URGENT';

export type GQLEmailStatus =
  | 'DRAFT'
  | 'QUEUED'
  | 'SENT'
  | 'DELIVERED'
  | 'BOUNCED'
  | 'FAILED';

// -----------------------------------------------------------------------------
// GraphQL Core Entity Types
// -----------------------------------------------------------------------------

export interface GQLUser {
  id: string;
  tenant_id: string;
  email: string;
  name: string;
  role: string;
  avatar_url?: string;
  is_active: boolean;
  last_login_at?: string;
  created_at: string;
  updated_at: string;
}

export interface GQLClient {
  id: string;
  tenant_id: string;
  user_id?: string;
  company_name: string;
  contact_name: string;
  email: string;
  phone?: string;
  address?: string;
  year_end?: string;
  utr?: string;
  company_number?: string;
  company_type?: string;
  incorporation_date?: string;
  vat_number?: string;
  vat_quarter?: string;
  status: GQLClientStatus;
  risk_score?: number;
  email_status: string;
  last_contact_at?: string;
  created_at: string;
  updated_at: string;
  // Nested resolvers
  documents?: GQLDocument[];
  services?: GQLService[];
  emails?: GQLEmail[];
  notes?: GQLClientNote[];
  assigned_staff?: GQLUser[];
}

export interface GQLClientNote {
  id: string;
  tenant_id: string;
  client_id: string;
  staff_id: string;
  note: string;
  created_at: string;
  updated_at: string;
  staff?: GQLUser;
}

export interface GQLDocumentType {
  id: string;
  tenant_id: string;
  name: string;
  description?: string;
  category?: string;
  is_active: boolean;
  created_at: string;
}

export interface GQLDocument {
  id: string;
  tenant_id: string;
  client_id: string;
  service_id?: string;
  name: string;
  original_name: string;
  file_key?: string;
  file_size?: number;
  mime_type?: string;
  status: GQLDocumentStatus;
  ai_summary?: string;
  ai_extracted?: Record<string, unknown>;
  requested_by?: string;
  uploaded_by?: string;
  reviewed_by?: string;
  reviewed_at?: string;
  expires_at?: string;
  created_at: string;
  updated_at: string;
  // Nested resolvers
  client?: GQLClient;
  service?: GQLService;
  document_type?: GQLDocumentType;
  requested_by_user?: GQLUser;
  uploaded_by_user?: GQLUser;
  reviewed_by_user?: GQLUser;
}

export interface GQLServiceType {
  id: string;
  tenant_id: string;
  name: string;
  description?: string;
  category?: string;
  default_deadline_days?: number;
  required_documents?: string[];
  is_active: boolean;
  created_at: string;
}

export interface GQLService {
  id: string;
  tenant_id: string;
  client_id: string;
  service_type_id?: string;
  name: string;
  description?: string;
  status: GQLServiceStatus;
  priority: GQLServicePriority;
  deadline?: string;
  completed_at?: string;
  docs_required: number;
  docs_received: number;
  assigned_to?: string;
  hmrc_data?: Record<string, unknown>;
  created_at: string;
  updated_at: string;
  // Nested resolvers
  client?: GQLClient;
  service_type?: GQLServiceType;
  assigned_user?: GQLUser;
  documents?: GQLDocument[];
}

export interface GQLEmail {
  id: string;
  tenant_id: string;
  client_id?: string;
  staff_id?: string;
  thread_id?: string;
  direction: GQLEmailDirection;
  from_email: string;
  to_email: string;
  subject: string;
  body_text?: string;
  body_html?: string;
  status: GQLEmailStatus;
  is_read: boolean;
  sentiment?: GQLEmailSentiment;
  ai_summary?: string;
  ai_action_items?: string[];
  sent_at?: string;
  received_at?: string;
  created_at: string;
  // Nested resolvers
  client?: GQLClient;
  staff?: GQLUser;
  thread?: GQLEmailThread;
}

export interface GQLEmailThread {
  id: string;
  tenant_id: string;
  client_id?: string;
  subject: string;
  last_message_at: string;
  message_count: number;
  created_at: string;
  // Nested resolvers
  emails?: GQLEmail[];
  client?: GQLClient;
}

export interface GQLNotification {
  id: string;
  tenant_id: string;
  user_id: string;
  type: string;
  title: string;
  message: string;
  entity_type?: string;
  entity_id?: string;
  is_read: boolean;
  read_at?: string;
  created_at: string;
}

// -----------------------------------------------------------------------------
// GraphQL Dashboard Types
// -----------------------------------------------------------------------------

export interface GQLDashboardStats {
  total_clients: number;
  active_clients: number;
  total_services: number;
  active_services: number;
  services_at_risk: number;
  total_documents: number;
  pending_documents: number;
  unread_emails: number;
  unread_notifications: number;
}

export interface GQLUrgentItem {
  type: string;
  entity_id: string;
  title: string;
  description: string;
  due_at?: string;
  priority: string;
  client_name?: string;
}

export interface GQLDeadlineItem {
  service_id: string;
  service_name: string;
  client_id: string;
  client_name: string;
  deadline: string;
  days_remaining: number;
  status: GQLServiceStatus;
  priority: GQLServicePriority;
  docs_progress: string;
}

export interface GQLActivityItem {
  id: string;
  action: string;
  entity_type: string;
  entity_id?: string;
  description: string;
  user_name?: string;
  created_at: string;
}

export interface GQLKanbanService {
  id: string;
  name: string;
  client_id: string;
  client_name: string;
  priority: GQLServicePriority;
  deadline?: string;
  docs_progress: string;
}

export interface GQLKanbanBoard {
  not_started: GQLKanbanService[];
  in_progress: GQLKanbanService[];
  awaiting_info: GQLKanbanService[];
  under_review: GQLKanbanService[];
  completed: GQLKanbanService[];
}

export interface GQLDashboard {
  stats: GQLDashboardStats;
  urgent_items: GQLUrgentItem[];
  deadlines: GQLDeadlineItem[];
  recent_activity: GQLActivityItem[];
  kanban: GQLKanbanBoard;
}

// -----------------------------------------------------------------------------
// GraphQL AI Types
// -----------------------------------------------------------------------------

export interface GQLAIMessage {
  role: string;
  content: string;
  timestamp: string;
  metadata?: Record<string, unknown>;
}

export interface GQLAIConversation {
  id: string;
  conversation_id: string;
  user_id: string;
  tenant_id?: string;
  messages: GQLAIMessage[];
  created_at: string;
  updated_at: string;
}

export interface GQLRiskFactor {
  factor: string;
  severity: string;
  description: string;
}

export interface GQLTroublemakerClient {
  client_id: string;
  client_name: string;
  company_name: string;
  risk_score: number;
  risk_level: string;
  risk_factors: GQLRiskFactor[];
  recommended_actions: string[];
  last_contact_at?: string;
  overdue_services: number;
  pending_documents: number;
}

export interface GQLAnomalyItem {
  type: string;
  entity_type: string;
  entity_id: string;
  title: string;
  description: string;
  severity: string;
  detected_at: string;
  client_name?: string;
}

// -----------------------------------------------------------------------------
// GraphQL Search Types
// -----------------------------------------------------------------------------

export interface GQLSearchResult {
  id: string;
  type: string;
  title: string;
  subtitle?: string;
  description?: string;
  highlight?: string;
  url: string;
}

export interface GQLSearchResults {
  clients: GQLSearchResult[];
  documents: GQLSearchResult[];
  services: GQLSearchResult[];
  emails: GQLSearchResult[];
  total_count: number;
}

// -----------------------------------------------------------------------------
// GraphQL Pagination Types (Relay-style)
// -----------------------------------------------------------------------------

export interface GQLPageInfo {
  has_next_page: boolean;
  has_previous_page: boolean;
  start_cursor?: string;
  end_cursor?: string;
}

export interface GQLClientEdge {
  cursor: string;
  node: GQLClient;
}

export interface GQLClientConnection {
  edges: GQLClientEdge[];
  page_info: GQLPageInfo;
  total_count: number;
}

export interface GQLDocumentEdge {
  cursor: string;
  node: GQLDocument;
}

export interface GQLDocumentConnection {
  edges: GQLDocumentEdge[];
  page_info: GQLPageInfo;
  total_count: number;
}

export interface GQLServiceEdge {
  cursor: string;
  node: GQLService;
}

export interface GQLServiceConnection {
  edges: GQLServiceEdge[];
  page_info: GQLPageInfo;
  total_count: number;
}

// -----------------------------------------------------------------------------
// GraphQL Filter Input Types
// -----------------------------------------------------------------------------

export interface GQLClientFilter {
  status?: GQLClientStatus;
  search?: string;
  assigned_to?: string;
  has_overdue_services?: boolean;
  risk_score_min?: number;
  risk_score_max?: number;
}

export interface GQLDocumentFilter {
  client_id?: string;
  service_id?: string;
  status?: GQLDocumentStatus;
  document_type_id?: string;
  uploaded_after?: string;
  uploaded_before?: string;
}

export interface GQLServiceFilter {
  client_id?: string;
  status?: GQLServiceStatus;
  priority?: GQLServicePriority;
  service_type_id?: string;
  assigned_to?: string;
  deadline_before?: string;
  deadline_after?: string;
}

// -----------------------------------------------------------------------------
// GraphQL Response Types
// -----------------------------------------------------------------------------

export interface GraphQLResponse<T> {
  data?: T;
  errors?: Array<{
    message: string;
    path?: string[];
    extensions?: Record<string, unknown>;
  }>;
}

// -----------------------------------------------------------------------------
// Core GraphQL Query Function
// -----------------------------------------------------------------------------

/**
 * Execute a GraphQL query or mutation against the API.
 *
 * WHY: GraphQL provides a unified interface for complex queries with nested
 * relationships, pagination, and aggregated data. It's more efficient than
 * multiple REST calls when you need related data from different entities.
 *
 * @param query - The GraphQL query or mutation string
 * @param variables - Optional variables to pass to the query
 * @returns The typed response data
 * @throws Error if the request fails or returns GraphQL errors
 */
export async function graphqlQuery<T>(
  query: string,
  variables?: Record<string, unknown>
): Promise<T> {
  const res = await authFetch('/api/v1/graphql', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ query, variables }),
  });

  if (!res.ok) {
    const error = await res.json().catch(() => ({}));
    throw new Error(error.message || `GraphQL request failed: ${res.status}`);
  }

  const response: GraphQLResponse<T> = await res.json();

  if (response.errors && response.errors.length > 0) {
    throw new Error(response.errors[0].message);
  }

  if (!response.data) {
    throw new Error('No data returned from GraphQL query');
  }

  return response.data;
}

// -----------------------------------------------------------------------------
// Dashboard Queries
// -----------------------------------------------------------------------------

/**
 * Fetch the complete dashboard data in a single request.
 *
 * WHY: The dashboard needs aggregated stats, urgent items, deadlines, activity,
 * and kanban board data. GraphQL fetches all of this in one round-trip instead
 * of 5+ REST calls, significantly improving dashboard load time.
 */
export async function gqlGetDashboard(deadlineDays = 14, activityLimit = 10): Promise<GQLDashboard> {
  const query = `
    query Dashboard($deadlineDays: Int, $activityLimit: Int) {
      dashboard {
        stats {
          total_clients
          active_clients
          total_services
          active_services
          services_at_risk
          total_documents
          pending_documents
          unread_emails
          unread_notifications
        }
        urgent_items {
          type
          entity_id
          title
          description
          due_at
          priority
          client_name
        }
        deadlines(days: $deadlineDays) {
          service_id
          service_name
          client_id
          client_name
          deadline
          days_remaining
          status
          priority
          docs_progress
        }
        recent_activity(limit: $activityLimit) {
          id
          action
          entity_type
          entity_id
          description
          user_name
          created_at
        }
        kanban {
          not_started { id name client_id client_name priority deadline docs_progress }
          in_progress { id name client_id client_name priority deadline docs_progress }
          awaiting_info { id name client_id client_name priority deadline docs_progress }
          under_review { id name client_id client_name priority deadline docs_progress }
          completed { id name client_id client_name priority deadline docs_progress }
        }
      }
    }
  `;
  const data = await graphqlQuery<{ dashboard: GQLDashboard }>(query, {
    deadlineDays,
    activityLimit,
  });
  return data.dashboard;
}

// -----------------------------------------------------------------------------
// Client Queries
// -----------------------------------------------------------------------------

/**
 * Fetch a single client with all related data (documents, services, emails, notes).
 *
 * WHY: Client detail pages need the client plus related entities. GraphQL fetches
 * everything in one request with DataLoader batching for N+1 prevention.
 */
export async function gqlGetClient(
  id: string,
  options?: { documentsLimit?: number; servicesLimit?: number; emailsLimit?: number; notesLimit?: number }
): Promise<GQLClient | null> {
  const query = `
    query Client($id: UUID!, $documentsLimit: Int, $servicesLimit: Int, $emailsLimit: Int, $notesLimit: Int) {
      client(id: $id) {
        id tenant_id user_id company_name contact_name email phone address
        year_end utr company_number company_type incorporation_date
        vat_number vat_quarter status risk_score email_status
        last_contact_at created_at updated_at
        documents(limit: $documentsLimit) {
          id name original_name status ai_summary created_at
          document_type { id name category }
        }
        services(limit: $servicesLimit) {
          id name status priority deadline docs_required docs_received
          service_type { id name category }
          assigned_user { id name email }
        }
        emails(limit: $emailsLimit) {
          id subject direction status is_read sentiment created_at
        }
        notes(limit: $notesLimit) {
          id note created_at
          staff { id name }
        }
        assigned_staff { id name email role }
      }
    }
  `;
  const data = await graphqlQuery<{ client: GQLClient | null }>(query, {
    id,
    documentsLimit: options?.documentsLimit ?? 10,
    servicesLimit: options?.servicesLimit ?? 10,
    emailsLimit: options?.emailsLimit ?? 10,
    notesLimit: options?.notesLimit ?? 20,
  });
  return data.client;
}

/**
 * Fetch paginated clients with optional filters.
 *
 * WHY: Client lists need Relay-style pagination for infinite scroll and filtering.
 * GraphQL provides cursor-based pagination with total count for UI.
 */
export async function gqlGetClients(
  filter?: GQLClientFilter,
  first = 20,
  after?: string
): Promise<GQLClientConnection> {
  const query = `
    query Clients($filter: ClientFilter, $first: Int, $after: String) {
      clients(filter: $filter, first: $first, after: $after) {
        edges {
          cursor
          node {
            id company_name contact_name email phone status
            risk_score email_status last_contact_at created_at
          }
        }
        page_info { has_next_page has_previous_page start_cursor end_cursor }
        total_count
      }
    }
  `;
  const data = await graphqlQuery<{ clients: GQLClientConnection }>(query, {
    filter,
    first,
    after,
  });
  return data.clients;
}

// -----------------------------------------------------------------------------
// Document Queries
// -----------------------------------------------------------------------------

/**
 * Fetch a single document with related entities.
 *
 * WHY: Document detail pages need the document plus client, service, and user
 * relationships resolved in a single request.
 */
export async function gqlGetDocument(id: string): Promise<GQLDocument | null> {
  const query = `
    query Document($id: UUID!) {
      document(id: $id) {
        id tenant_id client_id service_id name original_name
        file_key file_size mime_type status ai_summary ai_extracted
        requested_by uploaded_by reviewed_by reviewed_at expires_at
        created_at updated_at
        client { id company_name contact_name email }
        service { id name status }
        document_type { id name description category }
        requested_by_user { id name email }
        uploaded_by_user { id name email }
        reviewed_by_user { id name email }
      }
    }
  `;
  const data = await graphqlQuery<{ document: GQLDocument | null }>(query, { id });
  return data.document;
}

/**
 * Fetch paginated documents with optional filters.
 *
 * WHY: Document lists support filtering by client, service, status, type, and
 * date range. GraphQL provides flexible filtering with cursor pagination.
 */
export async function gqlGetDocuments(
  filter?: GQLDocumentFilter,
  first = 20,
  after?: string
): Promise<GQLDocumentConnection> {
  const query = `
    query Documents($filter: DocumentFilter, $first: Int, $after: String) {
      documents(filter: $filter, first: $first, after: $after) {
        edges {
          cursor
          node {
            id name original_name status ai_summary created_at
            client { id company_name }
            document_type { id name category }
          }
        }
        page_info { has_next_page has_previous_page start_cursor end_cursor }
        total_count
      }
    }
  `;
  const data = await graphqlQuery<{ documents: GQLDocumentConnection }>(query, {
    filter,
    first,
    after,
  });
  return data.documents;
}

// -----------------------------------------------------------------------------
// Service Queries
// -----------------------------------------------------------------------------

/**
 * Fetch a single service with related entities.
 *
 * WHY: Service detail pages need the service plus client, type, assigned user,
 * and documents resolved efficiently.
 */
export async function gqlGetService(id: string, documentsLimit = 10): Promise<GQLService | null> {
  const query = `
    query Service($id: UUID!, $documentsLimit: Int) {
      service(id: $id) {
        id tenant_id client_id service_type_id name description
        status priority deadline completed_at docs_required docs_received
        assigned_to hmrc_data created_at updated_at
        client { id company_name contact_name email }
        service_type { id name description category default_deadline_days required_documents }
        assigned_user { id name email }
        documents(limit: $documentsLimit) {
          id name status created_at
          document_type { id name }
        }
      }
    }
  `;
  const data = await graphqlQuery<{ service: GQLService | null }>(query, { id, documentsLimit });
  return data.service;
}

/**
 * Fetch paginated services with optional filters.
 *
 * WHY: Service lists need filtering by client, status, priority, type, assignee,
 * and deadline range with cursor pagination for large datasets.
 */
export async function gqlGetServices(
  filter?: GQLServiceFilter,
  first = 20,
  after?: string
): Promise<GQLServiceConnection> {
  const query = `
    query Services($filter: ServiceFilter, $first: Int, $after: String) {
      services(filter: $filter, first: $first, after: $after) {
        edges {
          cursor
          node {
            id name status priority deadline docs_required docs_received created_at
            client { id company_name }
            service_type { id name }
            assigned_user { id name }
          }
        }
        page_info { has_next_page has_previous_page start_cursor end_cursor }
        total_count
      }
    }
  `;
  const data = await graphqlQuery<{ services: GQLServiceConnection }>(query, {
    filter,
    first,
    after,
  });
  return data.services;
}

// -----------------------------------------------------------------------------
// Email Queries
// -----------------------------------------------------------------------------

/**
 * Fetch a single email with related entities.
 *
 * WHY: Email detail view needs the email plus client, staff, and thread context.
 */
export async function gqlGetEmail(id: string): Promise<GQLEmail | null> {
  const query = `
    query Email($id: UUID!) {
      email(id: $id) {
        id tenant_id client_id staff_id thread_id direction
        from_email to_email subject body_text body_html status is_read
        sentiment ai_summary ai_action_items sent_at received_at created_at
        client { id company_name contact_name email }
        staff { id name email }
        thread { id subject message_count last_message_at }
      }
    }
  `;
  const data = await graphqlQuery<{ email: GQLEmail | null }>(query, { id });
  return data.email;
}

/**
 * Fetch an email thread with all messages.
 *
 * WHY: Thread view needs the full conversation history with client context.
 */
export async function gqlGetEmailThread(id: string, emailsLimit = 20): Promise<GQLEmailThread | null> {
  const query = `
    query EmailThread($id: UUID!, $emailsLimit: Int) {
      email_thread(id: $id) {
        id tenant_id client_id subject last_message_at message_count created_at
        emails(limit: $emailsLimit) {
          id direction from_email to_email subject body_text status is_read
          sentiment ai_summary sent_at received_at created_at
          staff { id name }
        }
        client { id company_name contact_name email }
      }
    }
  `;
  const data = await graphqlQuery<{ email_thread: GQLEmailThread | null }>(query, { id, emailsLimit });
  return data.email_thread;
}

// -----------------------------------------------------------------------------
// Notification Queries
// -----------------------------------------------------------------------------

/**
 * Fetch notifications for the current user.
 *
 * WHY: Notification lists need filtering by read status with efficient pagination.
 */
export async function gqlGetNotifications(
  unreadOnly = false,
  limit = 20
): Promise<GQLNotification[]> {
  const query = `
    query Notifications($unreadOnly: Boolean, $limit: Int) {
      notifications(unread_only: $unreadOnly, limit: $limit) {
        id tenant_id user_id type title message
        entity_type entity_id is_read read_at created_at
      }
    }
  `;
  const data = await graphqlQuery<{ notifications: GQLNotification[] }>(query, {
    unreadOnly,
    limit,
  });
  return data.notifications;
}

/**
 * Get the count of unread notifications.
 *
 * WHY: Badge counts need just the number without fetching all notifications.
 */
export async function gqlGetUnreadNotificationCount(): Promise<number> {
  const query = `
    query UnreadNotificationCount {
      unread_notification_count
    }
  `;
  const data = await graphqlQuery<{ unread_notification_count: number }>(query);
  return data.unread_notification_count;
}

// -----------------------------------------------------------------------------
// AI Queries
// -----------------------------------------------------------------------------

/**
 * Fetch AI chat conversations for the current user.
 *
 * WHY: AI chat history is stored in MongoDB and accessed via GraphQL proxy.
 */
export async function gqlGetAIConversations(limit = 20): Promise<GQLAIConversation[]> {
  const query = `
    query AIConversations($limit: Int) {
      ai_conversations(limit: $limit) {
        id conversation_id user_id tenant_id
        messages { role content timestamp metadata }
        created_at updated_at
      }
    }
  `;
  const data = await graphqlQuery<{ ai_conversations: GQLAIConversation[] }>(query, { limit });
  return data.ai_conversations;
}

/**
 * Fetch a single AI conversation by ID.
 *
 * WHY: Loading a specific chat thread from MongoDB.
 */
export async function gqlGetAIConversation(id: string): Promise<GQLAIConversation | null> {
  const query = `
    query AIConversation($id: String!) {
      ai_conversation(id: $id) {
        id conversation_id user_id tenant_id
        messages { role content timestamp metadata }
        created_at updated_at
      }
    }
  `;
  const data = await graphqlQuery<{ ai_conversation: GQLAIConversation | null }>(query, { id });
  return data.ai_conversation;
}

/**
 * Fetch clients flagged as high-risk "troublemakers".
 *
 * WHY: AI analysis identifies problematic clients based on risk factors like
 * overdue services, pending documents, and communication patterns.
 */
export async function gqlGetTroublemakerClients(limit = 10): Promise<GQLTroublemakerClient[]> {
  const query = `
    query TroublemakerClients($limit: Int) {
      troublemaker_clients(limit: $limit) {
        client_id client_name company_name risk_score risk_level
        risk_factors { factor severity description }
        recommended_actions last_contact_at overdue_services pending_documents
      }
    }
  `;
  const data = await graphqlQuery<{ troublemaker_clients: GQLTroublemakerClient[] }>(query, { limit });
  return data.troublemaker_clients;
}

/**
 * Fetch detected anomalies across the system.
 *
 * WHY: AI analysis detects unusual patterns like sudden activity changes,
 * document anomalies, or communication irregularities.
 */
export async function gqlGetAnomalies(limit = 10): Promise<GQLAnomalyItem[]> {
  const query = `
    query Anomalies($limit: Int) {
      anomalies(limit: $limit) {
        type entity_type entity_id title description
        severity detected_at client_name
      }
    }
  `;
  const data = await graphqlQuery<{ anomalies: GQLAnomalyItem[] }>(query, { limit });
  return data.anomalies;
}

// -----------------------------------------------------------------------------
// Search Queries
// -----------------------------------------------------------------------------

/**
 * Unified search across clients, documents, services, and emails.
 *
 * WHY: Global search needs to query multiple entity types and return unified
 * results with highlights and deep links. GraphQL aggregates this efficiently.
 */
export async function gqlSearch(query: string, limit = 20): Promise<GQLSearchResults> {
  const gqlQuery = `
    query Search($query: String!, $limit: Int) {
      search(query: $query, limit: $limit) {
        clients { id type title subtitle description highlight url }
        documents { id type title subtitle description highlight url }
        services { id type title subtitle description highlight url }
        emails { id type title subtitle description highlight url }
        total_count
      }
    }
  `;
  const data = await graphqlQuery<{ search: GQLSearchResults }>(gqlQuery, { query, limit });
  return data.search;
}

/**
 * Get recent searches for the current user.
 *
 * WHY: Search history improves UX by showing recently accessed items.
 */
export async function gqlGetRecentSearches(limit = 5): Promise<GQLSearchResult[]> {
  const query = `
    query RecentSearches($limit: Int) {
      recent_searches(limit: $limit) {
        id type title subtitle description url
      }
    }
  `;
  const data = await graphqlQuery<{ recent_searches: GQLSearchResult[] }>(query, { limit });
  return data.recent_searches;
}

// -----------------------------------------------------------------------------
// Lookup Type Queries
// -----------------------------------------------------------------------------

/**
 * Fetch all document types for the tenant.
 *
 * WHY: Document type dropdowns and filters need the full list of available types.
 */
export async function gqlGetDocumentTypes(): Promise<GQLDocumentType[]> {
  const query = `
    query DocumentTypes {
      document_types {
        id tenant_id name description category is_active created_at
      }
    }
  `;
  const data = await graphqlQuery<{ document_types: GQLDocumentType[] }>(query);
  return data.document_types;
}

/**
 * Fetch all service types for the tenant.
 *
 * WHY: Service type dropdowns and filters need the full list of available types.
 */
export async function gqlGetServiceTypes(): Promise<GQLServiceType[]> {
  const query = `
    query ServiceTypes {
      service_types {
        id tenant_id name description category default_deadline_days
        required_documents is_active created_at
      }
    }
  `;
  const data = await graphqlQuery<{ service_types: GQLServiceType[] }>(query);
  return data.service_types;
}

/**
 * Fetch the current authenticated user.
 *
 * WHY: User context is needed throughout the app for permissions and display.
 */
export async function gqlGetMe(): Promise<GQLUser> {
  const query = `
    query Me {
      me {
        id tenant_id email name role avatar_url
        is_active last_login_at created_at updated_at
      }
    }
  `;
  const data = await graphqlQuery<{ me: GQLUser }>(query);
  return data.me;
}

// -----------------------------------------------------------------------------
// Mutations
// -----------------------------------------------------------------------------

/**
 * Mark a notification as read.
 *
 * WHY: Clicking a notification should mark it read without a full page reload.
 */
export async function gqlMarkNotificationRead(id: string): Promise<GQLNotification> {
  const mutation = `
    mutation MarkNotificationRead($id: UUID!) {
      mark_notification_read(id: $id) {
        id is_read read_at
      }
    }
  `;
  const data = await graphqlQuery<{ mark_notification_read: GQLNotification }>(mutation, { id });
  return data.mark_notification_read;
}

/**
 * Mark all notifications as read.
 *
 * WHY: "Mark all read" button clears the notification backlog in one action.
 */
export async function gqlMarkAllNotificationsRead(): Promise<number> {
  const mutation = `
    mutation MarkAllNotificationsRead {
      mark_all_notifications_read
    }
  `;
  const data = await graphqlQuery<{ mark_all_notifications_read: number }>(mutation);
  return data.mark_all_notifications_read;
}

/**
 * Dismiss (delete) a notification.
 *
 * WHY: Users can remove notifications they don't want to see anymore.
 */
export async function gqlDismissNotification(id: string): Promise<boolean> {
  const mutation = `
    mutation DismissNotification($id: UUID!) {
      dismiss_notification(id: $id)
    }
  `;
  const data = await graphqlQuery<{ dismiss_notification: boolean }>(mutation, { id });
  return data.dismiss_notification;
}

/**
 * Approve a document with an optional note.
 *
 * WHY: Quick document approval action from lists or detail views without
 * navigating to a separate approval form.
 */
export async function gqlApproveDocument(id: string, note?: string): Promise<GQLDocument> {
  const mutation = `
    mutation ApproveDocument($id: UUID!, $note: String) {
      approve_document(id: $id, note: $note) {
        id status reviewed_by reviewed_at updated_at
      }
    }
  `;
  const data = await graphqlQuery<{ approve_document: GQLDocument }>(mutation, { id, note });
  return data.approve_document;
}

/**
 * Reject a document with a required note explaining why.
 *
 * WHY: Document rejection requires a reason for audit trail and client communication.
 */
export async function gqlRejectDocument(id: string, note: string): Promise<GQLDocument> {
  const mutation = `
    mutation RejectDocument($id: UUID!, $note: String!) {
      reject_document(id: $id, note: $note) {
        id status reviewed_by reviewed_at updated_at
      }
    }
  `;
  const data = await graphqlQuery<{ reject_document: GQLDocument }>(mutation, { id, note });
  return data.reject_document;
}

/**
 * Update a service's status.
 *
 * WHY: Quick status updates from kanban board drag-drop or list actions.
 * Status changes also trigger completed_at timestamp when set to COMPLETED.
 */
export async function gqlUpdateServiceStatus(
  id: string,
  status: GQLServiceStatus
): Promise<GQLService> {
  const mutation = `
    mutation UpdateServiceStatus($id: UUID!, $status: ServiceStatus!) {
      update_service_status(id: $id, status: $status) {
        id status completed_at updated_at
      }
    }
  `;
  const data = await graphqlQuery<{ update_service_status: GQLService }>(mutation, { id, status });
  return data.update_service_status;
}

/**
 * Claim an email (assign to current user).
 *
 * WHY: Staff can claim unassigned inbound emails to handle them.
 */
export async function gqlClaimEmail(id: string): Promise<GQLEmail> {
  const mutation = `
    mutation ClaimEmail($id: UUID!) {
      claim_email(id: $id) {
        id staff_id
        staff { id name email }
      }
    }
  `;
  const data = await graphqlQuery<{ claim_email: GQLEmail }>(mutation, { id });
  return data.claim_email;
}

/**
 * Unclaim an email (remove assignment from current user).
 *
 * WHY: Staff can release an email back to the pool if they can't handle it.
 */
export async function gqlUnclaimEmail(id: string): Promise<GQLEmail> {
  const mutation = `
    mutation UnclaimEmail($id: UUID!) {
      unclaim_email(id: $id) {
        id staff_id
      }
    }
  `;
  const data = await graphqlQuery<{ unclaim_email: GQLEmail }>(mutation, { id });
  return data.unclaim_email;
}

/**
 * Mark an email as read.
 *
 * WHY: Opening an email should mark it as read for unread counts.
 */
export async function gqlMarkEmailRead(id: string): Promise<GQLEmail> {
  const mutation = `
    mutation MarkEmailRead($id: UUID!) {
      mark_email_read(id: $id) {
        id is_read
      }
    }
  `;
  const data = await graphqlQuery<{ mark_email_read: GQLEmail }>(mutation, { id });
  return data.mark_email_read;
}

/**
 * Delete an AI conversation.
 *
 * WHY: Users can clear chat history they no longer need.
 */
export async function gqlDeleteAIConversation(id: string): Promise<boolean> {
  const mutation = `
    mutation DeleteAIConversation($id: String!) {
      delete_ai_conversation(id: $id)
    }
  `;
  const data = await graphqlQuery<{ delete_ai_conversation: boolean }>(mutation, { id });
  return data.delete_ai_conversation;
}
