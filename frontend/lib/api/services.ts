/**
 * Services API - Service CRUD, types, requirements, alerts.
 */

import { authFetch } from './core';
import type { Service, ServiceType, ServiceTypeRequirement, ServiceAlert } from './types';

// ============================================================================
// Services CRUD
// ============================================================================

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

export async function deleteService(id: string): Promise<void> {
  const res = await authFetch(`/api/v1/services/${id}`, {
    method: 'DELETE',
  });
  if (!res.ok) throw new Error('Failed to delete service');
}

// ============================================================================
// Service Actions
// ============================================================================

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

export async function markServiceHMRC(id: string, data?: {
  reference?: string;
  notes?: string;
  filed_at?: string;
}): Promise<void> {
  const res = await authFetch(`/api/v1/services/${id}/hmrc-mark`, {
    method: 'POST',
    body: JSON.stringify(data || {}),
  });
  if (!res.ok) throw new Error('Failed to mark service as filed with HMRC');
}

// ============================================================================
// Bulk Operations
// ============================================================================

export async function bulkUpdateServices(data: {
  service_ids: string[];
  updates: {
    status?: string;
    priority?: string;
    staff_id?: string;
  };
}): Promise<{ updated: number }> {
  const res = await authFetch('/api/v1/services/bulk-update', {
    method: 'POST',
    body: JSON.stringify(data),
  });
  if (!res.ok) {
    const error = await res.json();
    throw new Error(error.message || 'Failed to bulk update services');
  }
  return res.json();
}

export async function reorderServices(data: {
  service_id: string;
  new_status: string;
  new_position: number;
}): Promise<void> {
  const res = await authFetch('/api/v1/services/reorder', {
    method: 'PATCH',
    body: JSON.stringify(data),
  });
  if (!res.ok) throw new Error('Failed to reorder services');
}

// ============================================================================
// Service Alerts
// ============================================================================

export async function getServiceAlerts(): Promise<{ alerts: ServiceAlert[] }> {
  const res = await authFetch('/api/v1/services/alerts');
  if (!res.ok) throw new Error('Failed to fetch service alerts');
  return res.json();
}

// ============================================================================
// Service Types
// ============================================================================

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

export async function reorderServiceTypes(data: {
  service_type_id: string;
  new_position: number;
}): Promise<void> {
  const res = await authFetch('/api/v1/service-types/reorder', {
    method: 'PATCH',
    body: JSON.stringify(data),
  });
  if (!res.ok) throw new Error('Failed to reorder service types');
}

// ============================================================================
// Service Type Requirements
// ============================================================================

export async function getServiceTypeRequirements(id: string): Promise<{
  requirements: ServiceTypeRequirement[];
}> {
  const res = await authFetch(`/api/v1/service-types/${id}/requirements`);
  if (!res.ok) throw new Error('Failed to fetch service type requirements');
  return res.json();
}

export async function addServiceTypeRequirement(id: string, data: {
  document_type_id: string;
  is_mandatory?: boolean;
}): Promise<void> {
  const res = await authFetch(`/api/v1/service-types/${id}/requirements`, {
    method: 'POST',
    body: JSON.stringify(data),
  });
  if (!res.ok) {
    const error = await res.json();
    throw new Error(error.message || 'Failed to add requirement');
  }
}

export async function removeServiceTypeRequirement(
  serviceTypeId: string,
  documentTypeId: string
): Promise<void> {
  const res = await authFetch(`/api/v1/service-types/${serviceTypeId}/requirements/${documentTypeId}`, {
    method: 'DELETE',
  });
  if (!res.ok) throw new Error('Failed to remove requirement');
}
