/**
 * AI Integration API - AI chat, document processing, email analysis, risk analysis.
 */

import { authFetch } from './core';
import type {
  AIJob,
  AIChatMessage,
  AIChatConversation,
  AIRiskAnalysis,
  AIDocumentExtraction,
  AIEmailAnalysis,
  AIWorkloadRebalance,
} from './types';

// ============================================================================
// AI Chat
// ============================================================================

export async function aiChat(data: {
  message: string;
  conversation_id?: string;
  context?: Record<string, unknown>;
}): Promise<{
  response: string;
  conversation_id: string;
}> {
  const res = await authFetch('/api/v1/ai/chat', {
    method: 'POST',
    body: JSON.stringify(data),
  });
  if (!res.ok) {
    const error = await res.json();
    throw new Error(error.message || 'AI chat failed');
  }
  return res.json();
}

export async function aiChatStream(data: {
  message: string;
  conversation_id?: string;
  context?: Record<string, unknown>;
}): Promise<ReadableStream<Uint8Array>> {
  const res = await authFetch('/api/v1/ai/chat/stream', {
    method: 'POST',
    body: JSON.stringify(data),
  });
  if (!res.ok) {
    const error = await res.json();
    throw new Error(error.message || 'AI chat stream failed');
  }
  if (!res.body) throw new Error('No response body');
  return res.body;
}

export async function getAIChatHistory(params?: {
  limit?: number;
  offset?: number;
}): Promise<{ conversations: AIChatConversation[]; count: number }> {
  const searchParams = new URLSearchParams();
  if (params?.limit) searchParams.set('limit', String(params.limit));
  if (params?.offset) searchParams.set('offset', String(params.offset));

  const res = await authFetch(`/api/v1/ai/chat/history?${searchParams}`);
  if (!res.ok) throw new Error('Failed to fetch AI chat history');
  return res.json();
}

export async function saveAIChatHistory(data: {
  conversation_id: string;
  messages: AIChatMessage[];
}): Promise<{ message: string }> {
  const res = await authFetch('/api/v1/ai/chat/history', {
    method: 'POST',
    body: JSON.stringify(data),
  });
  if (!res.ok) throw new Error('Failed to save AI chat history');
  return res.json();
}

export async function deleteAIChat(id: string): Promise<void> {
  const res = await authFetch(`/api/v1/ai/chat/${id}`, {
    method: 'DELETE',
  });
  if (!res.ok) throw new Error('Failed to delete AI chat');
}

// ============================================================================
// AI Document Processing
// ============================================================================

export async function aiExtractDocument(data: {
  file_key: string;
}): Promise<{ job_id: string } | AIDocumentExtraction> {
  const res = await authFetch('/api/v1/ai/documents/extract', {
    method: 'POST',
    body: JSON.stringify(data),
  });
  if (!res.ok) {
    const error = await res.json();
    throw new Error(error.message || 'Document extraction failed');
  }
  return res.json();
}

export async function aiClassifyDocument(data: {
  file_key: string;
  text: string;
}): Promise<{
  document_type: string;
  confidence: number;
  subcategory?: string;
  key_entities?: string[];
  summary?: string;
}> {
  const res = await authFetch('/api/v1/ai/documents/classify', {
    method: 'POST',
    body: JSON.stringify(data),
  });
  if (!res.ok) {
    const error = await res.json();
    throw new Error(error.message || 'Document classification failed');
  }
  return res.json();
}

export async function aiSummarizeDocument(data: {
  text: string;
  file_key?: string;
}): Promise<{
  summary: string;
  key_points?: string[];
  financial_data?: unknown;
  action_items?: string[];
}> {
  const res = await authFetch('/api/v1/ai/documents/summarize', {
    method: 'POST',
    body: JSON.stringify(data),
  });
  if (!res.ok) {
    const error = await res.json();
    throw new Error(error.message || 'Document summarization failed');
  }
  return res.json();
}

export async function aiRenameDocument(data: {
  text: string;
  original_filename?: string;
  document_type?: string;
  client_name?: string;
  file_key?: string;
}): Promise<{ suggested_name: string; alternatives?: string[] }> {
  const res = await authFetch('/api/v1/ai/documents/rename', {
    method: 'POST',
    body: JSON.stringify(data),
  });
  if (!res.ok) {
    const error = await res.json();
    throw new Error(error.message || 'Document rename failed');
  }
  return res.json();
}

// ============================================================================
// AI Email Analysis
// ============================================================================

export async function aiSummarizeEmail(data: {
  email_id?: string;
  subject: string;
  body: string;
}): Promise<{ summary: string; key_points?: string[] }> {
  const res = await authFetch('/api/v1/ai/emails/summarize', {
    method: 'POST',
    body: JSON.stringify(data),
  });
  if (!res.ok) {
    const error = await res.json();
    throw new Error(error.message || 'Email summarization failed');
  }
  return res.json();
}

export async function aiAnalyzeEmailSentiment(data: {
  email_id?: string;
  subject: string;
  body: string;
}): Promise<AIEmailAnalysis> {
  const res = await authFetch('/api/v1/ai/emails/sentiment', {
    method: 'POST',
    body: JSON.stringify(data),
  });
  if (!res.ok) {
    const error = await res.json();
    throw new Error(error.message || 'Sentiment analysis failed');
  }
  return res.json();
}

export async function aiExtractEmailPromises(data: {
  email_id?: string;
  subject: string;
  body: string;
}): Promise<{
  promises: Array<{
    promise: string;
    due_date?: string;
    assignee?: string;
  }>;
  action_items: string[];
}> {
  const res = await authFetch('/api/v1/ai/emails/promises', {
    method: 'POST',
    body: JSON.stringify(data),
  });
  if (!res.ok) {
    const error = await res.json();
    throw new Error(error.message || 'Promise extraction failed');
  }
  return res.json();
}

export async function aiDraftEmail(data: {
  email_id?: string;
  reply_to_subject?: string;
  reply_to_body?: string;
  context?: string;
  tone?: 'formal' | 'friendly' | 'urgent';
  purpose?: string;
}): Promise<{ subject: string; body: string; alternatives?: Array<{ subject: string; body: string }> }> {
  const res = await authFetch('/api/v1/ai/emails/draft', {
    method: 'POST',
    body: JSON.stringify(data),
  });
  if (!res.ok) {
    const error = await res.json();
    throw new Error(error.message || 'Email draft failed');
  }
  return res.json();
}

export async function aiMatchEmailToClient(data: {
  from_email: string;
  subject: string;
  body?: string;
}): Promise<{
  client_id?: string;
  client_name?: string;
  confidence: number;
  alternatives?: Array<{ client_id: string; client_name: string; confidence: number }>;
}> {
  const res = await authFetch('/api/v1/ai/emails/match-client', {
    method: 'POST',
    body: JSON.stringify(data),
  });
  if (!res.ok) {
    const error = await res.json();
    throw new Error(error.message || 'Client matching failed');
  }
  return res.json();
}

export async function aiSummarizeEmailThread(data: {
  thread_id: string;
}): Promise<{
  summary: string;
  key_topics: string[];
  participants: string[];
  action_items: string[];
}> {
  const res = await authFetch('/api/v1/ai/emails/thread-summary', {
    method: 'POST',
    body: JSON.stringify(data),
  });
  if (!res.ok) {
    const error = await res.json();
    throw new Error(error.message || 'Thread summary failed');
  }
  return res.json();
}

export async function aiFindAlternateEmail(data: {
  email: string;
  client_name?: string;
  company_name?: string;
}): Promise<{
  alternatives: Array<{
    email: string;
    source: string;
    confidence: number;
  }>;
}> {
  const res = await authFetch('/api/v1/ai/emails/find-alternate', {
    method: 'POST',
    body: JSON.stringify(data),
  });
  if (!res.ok) {
    const error = await res.json();
    throw new Error(error.message || 'Alternate email search failed');
  }
  return res.json();
}

// ============================================================================
// AI Form Auto-Fill
// ============================================================================

export async function aiExtractFormData(data: {
  file_key: string;
  form_type?: string;
}): Promise<{ extracted_data: Record<string, unknown>; confidence: number }> {
  const res = await authFetch('/api/v1/ai/forms/extract', {
    method: 'POST',
    body: JSON.stringify(data),
  });
  if (!res.ok) {
    const error = await res.json();
    throw new Error(error.message || 'Form extraction failed');
  }
  return res.json();
}

export async function aiAutoFillVAT(data: {
  client_id: string;
  period_start: string;
  period_end: string;
}): Promise<{
  vat_data: Record<string, unknown>;
  boxes: Record<string, number>;
  confidence: number;
}> {
  const res = await authFetch('/api/v1/ai/forms/vat', {
    method: 'POST',
    body: JSON.stringify(data),
  });
  if (!res.ok) {
    const error = await res.json();
    throw new Error(error.message || 'VAT auto-fill failed');
  }
  return res.json();
}

export async function aiAutoFillCT600(data: {
  client_id: string;
  period_start: string;
  period_end: string;
}): Promise<{
  ct600_data: Record<string, unknown>;
  confidence: number;
}> {
  const res = await authFetch('/api/v1/ai/forms/ct600', {
    method: 'POST',
    body: JSON.stringify(data),
  });
  if (!res.ok) {
    const error = await res.json();
    throw new Error(error.message || 'CT600 auto-fill failed');
  }
  return res.json();
}

export async function aiAutoFillSA(data: {
  client_id: string;
  tax_year: string;
}): Promise<{
  sa_data: Record<string, unknown>;
  confidence: number;
}> {
  const res = await authFetch('/api/v1/ai/forms/sa', {
    method: 'POST',
    body: JSON.stringify(data),
  });
  if (!res.ok) {
    const error = await res.json();
    throw new Error(error.message || 'Self Assessment auto-fill failed');
  }
  return res.json();
}

// ============================================================================
// AI Risk Analysis
// ============================================================================

export async function aiAnalyzeClientRisk(data: {
  client_id: string;
}): Promise<AIRiskAnalysis> {
  const res = await authFetch('/api/v1/ai/risk/client', {
    method: 'POST',
    body: JSON.stringify(data),
  });
  if (!res.ok) {
    const error = await res.json();
    throw new Error(error.message || 'Client risk analysis failed');
  }
  return res.json();
}

export async function aiAnalyzeServiceRisk(data: {
  service_id: string;
}): Promise<AIRiskAnalysis> {
  const res = await authFetch('/api/v1/ai/risk/service', {
    method: 'POST',
    body: JSON.stringify(data),
  });
  if (!res.ok) {
    const error = await res.json();
    throw new Error(error.message || 'Service risk analysis failed');
  }
  return res.json();
}

// ============================================================================
// AI Template Generation
// ============================================================================

export async function aiGenerateTemplate(data: {
  purpose: 'chase' | 'notification' | 'welcome' | 'reminder' | 'custom';
  context?: string;
  tone?: 'formal' | 'friendly' | 'urgent';
  placeholders?: string[];
}): Promise<{
  name: string;
  subject: string;
  body_html: string;
  body_text: string;
  placeholders: string[];
}> {
  const res = await authFetch('/api/v1/ai/templates/generate', {
    method: 'POST',
    body: JSON.stringify(data),
  });
  if (!res.ok) {
    const error = await res.json();
    throw new Error(error.message || 'Template generation failed');
  }
  return res.json();
}

// ============================================================================
// AI Client Operations
// ============================================================================

export async function aiCheckDuplicateClients(data: {
  company_name: string;
  email?: string;
  company_number?: string;
}): Promise<{
  duplicates: Array<{
    client_id: string;
    company_name: string;
    similarity_score: number;
    matching_fields: string[];
  }>;
}> {
  const res = await authFetch('/api/v1/ai/clients/duplicate-check', {
    method: 'POST',
    body: JSON.stringify(data),
  });
  if (!res.ok) {
    const error = await res.json();
    throw new Error(error.message || 'Duplicate check failed');
  }
  return res.json();
}

// ============================================================================
// AI Service Operations
// ============================================================================

export async function aiAutoNameService(data: {
  client_id: string;
  service_type_id?: string;
  period?: string;
}): Promise<{ suggested_name: string; alternatives?: string[] }> {
  const res = await authFetch('/api/v1/ai/services/auto-name', {
    method: 'POST',
    body: JSON.stringify(data),
  });
  if (!res.ok) {
    const error = await res.json();
    throw new Error(error.message || 'Service auto-name failed');
  }
  return res.json();
}

export async function aiGenerateCompletionSummary(data: {
  service_id: string;
}): Promise<{
  summary: string;
  key_actions: string[];
  next_steps?: string[];
}> {
  const res = await authFetch('/api/v1/ai/services/completion-summary', {
    method: 'POST',
    body: JSON.stringify(data),
  });
  if (!res.ok) {
    const error = await res.json();
    throw new Error(error.message || 'Completion summary generation failed');
  }
  return res.json();
}

// ============================================================================
// AI Dashboard Analytics
// ============================================================================

export async function aiFindTroublemakers(params?: {
  limit?: number;
}): Promise<{
  clients: Array<{
    client_id: string;
    client_name: string;
    company_name: string;
    risk_score: number;
    risk_level: string;
    risk_factors: Array<{ factor: string; severity: string; description: string }>;
    recommended_actions: string[];
    overdue_services: number;
    pending_documents: number;
  }>;
}> {
  const searchParams = new URLSearchParams();
  if (params?.limit) searchParams.set('limit', String(params.limit));

  const res = await authFetch(`/api/v1/ai/dashboard/troublemakers?${searchParams}`, {
    method: 'POST',
  });
  if (!res.ok) throw new Error('Failed to find troublemaker clients');
  return res.json();
}

export async function aiDetectAnomalies(params?: {
  limit?: number;
}): Promise<{
  anomalies: Array<{
    type: string;
    entity_type: string;
    entity_id: string;
    title: string;
    description: string;
    severity: 'low' | 'medium' | 'high' | 'critical';
    detected_at: string;
    client_name?: string;
  }>;
}> {
  const searchParams = new URLSearchParams();
  if (params?.limit) searchParams.set('limit', String(params.limit));

  const res = await authFetch(`/api/v1/ai/dashboard/anomalies?${searchParams}`, {
    method: 'POST',
  });
  if (!res.ok) throw new Error('Failed to detect anomalies');
  return res.json();
}

export async function aiAnalyzeStaffActivity(data: {
  staff_id?: string;
  from_date?: string;
  to_date?: string;
}): Promise<{
  activity_score: number;
  metrics: Record<string, number>;
  insights: string[];
  recommendations: string[];
}> {
  const res = await authFetch('/api/v1/ai/staff/activity', {
    method: 'POST',
    body: JSON.stringify(data),
  });
  if (!res.ok) throw new Error('Failed to analyze staff activity');
  return res.json();
}

// ============================================================================
// AI Workload Rebalance
// ============================================================================

export async function aiGetStaffWorkloads(): Promise<{
  workloads: Array<{
    user_id: string;
    user_name: string;
    client_count: number;
    service_count: number;
    overdue_count: number;
    workload_score: number;
  }>;
}> {
  const res = await authFetch('/api/v1/ai/staff/workload');
  if (!res.ok) throw new Error('Failed to get staff workloads');
  return res.json();
}

export async function aiCalculateRebalance(): Promise<{
  recommendations: AIWorkloadRebalance[];
  before_variance: number;
  after_variance: number;
}> {
  const res = await authFetch('/api/v1/ai/staff/rebalance', {
    method: 'POST',
  });
  if (!res.ok) throw new Error('Failed to calculate rebalance');
  return res.json();
}

export async function aiApplyRebalance(data: {
  reassignments: Array<{
    client_id: string;
    from_staff_id: string;
    to_staff_id: string;
  }>;
}): Promise<{ applied: number; message: string }> {
  const res = await authFetch('/api/v1/ai/staff/rebalance/apply', {
    method: 'POST',
    body: JSON.stringify(data),
  });
  if (!res.ok) {
    const error = await res.json();
    throw new Error(error.message || 'Failed to apply rebalance');
  }
  return res.json();
}

// ============================================================================
// AI Job Status
// ============================================================================

export async function getAIJobStatus(id: string): Promise<{ job: AIJob }> {
  const res = await authFetch(`/api/v1/ai/jobs/${id}`);
  if (!res.ok) throw new Error('Failed to get AI job status');
  return res.json();
}
