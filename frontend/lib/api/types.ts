/**
 * Shared types and interfaces for the API.
 * All interfaces are exported for use across the application.
 */

// ============================================================================
// Core Entity Types
// ============================================================================

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

// ============================================================================
// Dashboard Types
// ============================================================================

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

export interface PendingDocument {
  id: string;
  name: string;
  client_id: string;
  client_name: string;
  status: string;
  requested_at: string;
  requested_by_name?: string;
}

export interface WorkloadItem {
  staff_id: string;
  staff_name: string;
  client_count: number;
  service_count: number;
  overdue_count: number;
  primary_client_count: number;
}

export interface RecentClient {
  id: string;
  company_name: string;
  contact_name: string;
  last_activity: string;
  activity_type: string;
  activity_description?: string;
}

// ============================================================================
// Client Related Types
// ============================================================================

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

// ============================================================================
// Companies House Types
// ============================================================================

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

export interface CHOfficer {
  name: string;
  role: string;
  appointed_date?: string;
  resigned_date?: string;
  nationality?: string;
  occupation?: string;
  country_of_residence?: string;
  date_of_birth?: { month?: number; year?: number };
}

// ============================================================================
// Document Types
// ============================================================================

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

export interface BulkDocumentRequest {
  client_ids: string[];
  document_requests: Array<{
    type_id?: string;
    name: string;
    expiry_date?: string;
  }>;
  request_note?: string;
}

// ============================================================================
// Service Types
// ============================================================================

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

export interface ServiceTypeRequirement {
  service_type_id: string;
  document_type_id: string;
  document_type_name: string;
  is_mandatory: boolean;
}

export interface ServiceAlert {
  service_id: string;
  service_name: string;
  client_id: string;
  client_name: string;
  alert_type: 'overdue' | 'at_risk' | 'missing_docs' | 'deadline_approaching';
  message: string;
  deadline?: string;
  priority: string;
}

// ============================================================================
// Email Types
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

export interface EmailThread {
  id: string;
  tenant_id: string;
  thread_key: string;
  client_id?: string;
  client_name?: string;
  subject: string;
  message_count: number;
  last_message_at: string;
  created_at: string;
}

// ============================================================================
// Notification Types
// ============================================================================

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

// ============================================================================
// Settings Types
// ============================================================================

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

// ============================================================================
// User Types
// ============================================================================

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

// ============================================================================
// E-Sign Types
// ============================================================================

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

// ============================================================================
// Reminder Types
// ============================================================================

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

// ============================================================================
// Subscription Types
// ============================================================================

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

// ============================================================================
// Push Token Types
// ============================================================================

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

// ============================================================================
// Portal Types
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

export interface PortalDeadline {
  service_id: string;
  service_name: string;
  deadline: string;
  status: string;
  days_remaining: number;
}

// ============================================================================
// Auth Types
// ============================================================================

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

export interface TwoFactorSetupResponse {
  secret: string;
  qr_code?: string;
  recovery_codes?: string[];
}

// ============================================================================
// Admin / Tenant Types
// ============================================================================

export interface Tenant {
  id: string;
  domain: string;
  custom_domain?: string;
  name: string;
  plan: 'starter' | 'professional' | 'enterprise';
  is_active: boolean;
  user_count?: number;
  client_count?: number;
  created_at: string;
  updated_at: string;
}

// ============================================================================
// Chase Log Types
// ============================================================================

export interface ChaseLog {
  id: string;
  tenant_id: string;
  initiated_by: string;
  initiated_by_name?: string;
  template_id?: string;
  template_name?: string;
  total_sent: number;
  delivered: number;
  opened: number;
  clicked: number;
  bounced: number;
  created_at: string;
}

export interface ChaseLogClient {
  chase_log_id: string;
  client_id: string;
  client_name: string;
  email_id?: string;
  status: string;
}

export interface ChaseStats {
  total_campaigns: number;
  total_emails_sent: number;
  delivery_rate: number;
  open_rate: number;
  bounce_rate: number;
  campaigns_this_month: number;
}

// ============================================================================
// Audit Log Types
// ============================================================================

export interface AuditLog {
  id: string;
  tenant_id: string;
  user_id?: string;
  user_name?: string;
  user_email?: string;
  action: string;
  entity_type: string;
  entity_id?: string;
  entity_name?: string;
  old_value?: Record<string, unknown>;
  new_value?: Record<string, unknown>;
  ip_address?: string;
  user_agent?: string;
  severity: 'info' | 'warning' | 'critical';
  created_at: string;
}

export interface AuditLogStats {
  total_entries: number;
  by_action: Record<string, number>;
  by_entity_type: Record<string, number>;
  by_severity: Record<string, number>;
  by_user: Array<{ user_id: string; user_name: string; count: number }>;
}

// ============================================================================
// Search Types
// ============================================================================

export interface SearchResultItem {
  id: string;
  type: 'client' | 'document' | 'service' | 'email';
  title: string;
  subtitle?: string;
  description?: string;
  match?: string;
  url: string;
}

export interface GlobalSearchResults {
  clients: SearchResultItem[];
  documents: SearchResultItem[];
  services: SearchResultItem[];
  emails: SearchResultItem[];
  total_count: number;
}

// ============================================================================
// AI Types
// ============================================================================

export interface AIJob {
  id: string;
  tenant_id: string;
  type: string;
  status: 'pending' | 'processing' | 'completed' | 'failed';
  payload?: Record<string, unknown>;
  result?: Record<string, unknown>;
  error_message?: string;
  created_at: string;
  completed_at?: string;
}

export interface AIChatMessage {
  role: 'user' | 'assistant' | 'system';
  content: string;
  timestamp?: string;
  metadata?: Record<string, unknown>;
}

export interface AIChatConversation {
  id: string;
  conversation_id: string;
  user_id: string;
  tenant_id?: string;
  messages: AIChatMessage[];
  created_at: string;
  updated_at: string;
}

export interface AIRiskAnalysis {
  risk_score: number;
  risk_level: 'low' | 'medium' | 'high' | 'critical';
  risk_factors: Array<{
    factor: string;
    severity: string;
    description: string;
  }>;
  recommended_actions: string[];
}

export interface AIDocumentExtraction {
  document_type?: string;
  extracted_data: Record<string, unknown>;
  confidence: number;
  summary?: string;
}

export interface AIEmailAnalysis {
  sentiment: 'positive' | 'neutral' | 'negative' | 'urgent';
  sentiment_score: number;
  summary?: string;
  action_items?: string[];
  key_points?: string[];
}

export interface AIWorkloadRebalance {
  from_user_id: string;
  from_user_name: string;
  to_user_id: string;
  to_user_name: string;
  client_ids: string[];
  reason: string;
}

// ============================================================================
// GraphQL Types
// ============================================================================

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

export interface GraphQLResponse<T> {
  data?: T;
  errors?: Array<{
    message: string;
    path?: string[];
    extensions?: Record<string, unknown>;
  }>;
}
