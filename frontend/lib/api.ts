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
  is_active: boolean;
  two_factor_enabled: boolean;
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
