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
  created_at: string;
  updated_at: string;
  client_name?: string;
  type_name?: string;
  uploaded_by_name?: string;
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

// Helper to get token
function getAuthToken(): string | null {
  if (typeof window !== 'undefined') {
    return localStorage.getItem('access_token');
  }
  return null;
}

// Helper to make authenticated requests
async function authFetch(url: string, options: RequestInit = {}): Promise<Response> {
  const token = getAuthToken();
  const headers: HeadersInit = {
    'Content-Type': 'application/json',
    ...(options.headers || {}),
  };

  if (token) {
    (headers as Record<string, string>)['Authorization'] = `Bearer ${token}`;
  }

  const res = await fetch(`${API_URL}${url}`, {
    ...options,
    headers,
  });

  if (res.status === 401) {
    // Token expired, redirect to login
    if (typeof window !== 'undefined') {
      localStorage.removeItem('access_token');
      localStorage.removeItem('refresh_token');
      localStorage.removeItem('user');
      window.location.href = '/login';
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

export async function getDashboardDeadlines(): Promise<{ deadlines: Service[] }> {
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
