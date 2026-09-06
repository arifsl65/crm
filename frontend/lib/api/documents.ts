/**
 * Documents API - Document CRUD, versions, bulk operations, QR upload.
 */

import { authFetch, API_URL } from './core';
import type { Document, DocumentVersion, DocumentType, BulkDocumentRequest } from './types';

// ============================================================================
// Documents CRUD
// ============================================================================

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

export async function deleteDocument(id: string): Promise<void> {
  const res = await authFetch(`/api/v1/documents/${id}`, {
    method: 'DELETE',
  });
  if (!res.ok) throw new Error('Failed to delete document');
}

// ============================================================================
// Document Actions
// ============================================================================

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

export async function downloadDocument(id: string): Promise<{ download_url: string; expires_at: string }> {
  const res = await authFetch(`/api/v1/documents/${id}/download`);
  if (!res.ok) throw new Error('Failed to get document download URL');
  return res.json();
}

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

export async function cancelDocumentRenewal(id: string): Promise<void> {
  const res = await authFetch(`/api/v1/documents/${id}/renewal`, {
    method: 'DELETE',
  });
  if (!res.ok) throw new Error('Failed to cancel document renewal');
}

// ============================================================================
// Document Versions
// ============================================================================

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

// ============================================================================
// Bulk Operations
// ============================================================================

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

// ============================================================================
// Document Upload
// ============================================================================

export async function uploadDocument(id: string, file: File): Promise<Document> {
  const formData = new FormData();
  formData.append('file', file);

  const res = await fetch(`${API_URL}/api/v1/documents/${id}/upload`, {
    method: 'POST',
    credentials: 'include',
    body: formData,
  });
  if (!res.ok) {
    const error = await res.json();
    throw new Error(error.message || 'Failed to upload document');
  }
  return res.json();
}

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

// ============================================================================
// QR Upload
// ============================================================================

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
  const res = await fetch(`${API_URL}/api/v1/documents/qr/${token}`);
  if (!res.ok) throw new Error('Invalid or expired token');
  return res.json();
}

export async function uploadViaQR(token: string, file: File): Promise<{ id: string; message: string }> {
  const formData = new FormData();
  formData.append('file', file);

  const res = await fetch(`${API_URL}/api/v1/documents/qr/${token}/upload`, {
    method: 'POST',
    body: formData,
  });
  if (!res.ok) {
    const error = await res.json();
    throw new Error(error.message || 'Failed to upload document');
  }
  return res.json();
}

// ============================================================================
// Firm Documents
// ============================================================================

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

// ============================================================================
// Expiring Documents
// ============================================================================

export async function getExpiringDocuments(params?: {
  days?: number;
  limit?: number;
  offset?: number;
}): Promise<{ documents: Document[]; count: number }> {
  const searchParams = new URLSearchParams();
  if (params?.days) searchParams.set('days', String(params.days));
  if (params?.limit) searchParams.set('limit', String(params.limit));
  if (params?.offset) searchParams.set('offset', String(params.offset));

  const res = await authFetch(`/api/v1/documents/expiring?${searchParams}`);
  if (!res.ok) throw new Error('Failed to fetch expiring documents');
  return res.json();
}

// ============================================================================
// Document Types
// ============================================================================

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
