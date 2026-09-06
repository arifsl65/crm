/**
 * GraphQL API - Unified query interface for dashboard and aggregated data.
 *
 * GraphQL is the preferred interface for:
 * - Dashboard data (aggregated stats, deadlines, activity, kanban board)
 * - Complex queries with nested relationships (client with documents/services/emails)
 * - Relay-style pagination for large datasets
 * - AI analysis (troublemaker clients, anomalies)
 * - Unified search across entities
 * - Real-time notification state
 */

import { authFetch } from './core';
import type {
  GraphQLResponse,
  GQLDashboard,
  GQLClient,
  GQLClientConnection,
  GQLClientFilter,
  GQLDocument,
  GQLDocumentConnection,
  GQLDocumentFilter,
  GQLService,
  GQLServiceConnection,
  GQLServiceFilter,
  GQLServiceStatus,
  GQLEmail,
  GQLEmailThread,
  GQLNotification,
  GQLAIConversation,
  GQLTroublemakerClient,
  GQLAnomalyItem,
  GQLSearchResults,
  GQLSearchResult,
  GQLDocumentType,
  GQLServiceType,
  GQLUser,
} from './types';

// ============================================================================
// Core GraphQL Query Function
// ============================================================================

/**
 * Execute a GraphQL query or mutation against the API.
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

// ============================================================================
// Dashboard Queries
// ============================================================================

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

// ============================================================================
// Client Queries
// ============================================================================

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

// ============================================================================
// Document Queries
// ============================================================================

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

// ============================================================================
// Service Queries
// ============================================================================

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

// ============================================================================
// Email Queries
// ============================================================================

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

// ============================================================================
// Notification Queries
// ============================================================================

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

export async function gqlGetUnreadNotificationCount(): Promise<number> {
  const query = `
    query UnreadNotificationCount {
      unread_notification_count
    }
  `;
  const data = await graphqlQuery<{ unread_notification_count: number }>(query);
  return data.unread_notification_count;
}

// ============================================================================
// AI Queries
// ============================================================================

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

// ============================================================================
// Search Queries
// ============================================================================

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

// ============================================================================
// Lookup Type Queries
// ============================================================================

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

// ============================================================================
// Mutations
// ============================================================================

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

export async function gqlMarkAllNotificationsRead(): Promise<number> {
  const mutation = `
    mutation MarkAllNotificationsRead {
      mark_all_notifications_read
    }
  `;
  const data = await graphqlQuery<{ mark_all_notifications_read: number }>(mutation);
  return data.mark_all_notifications_read;
}

export async function gqlDismissNotification(id: string): Promise<boolean> {
  const mutation = `
    mutation DismissNotification($id: UUID!) {
      dismiss_notification(id: $id)
    }
  `;
  const data = await graphqlQuery<{ dismiss_notification: boolean }>(mutation, { id });
  return data.dismiss_notification;
}

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

export async function gqlDeleteAIConversation(id: string): Promise<boolean> {
  const mutation = `
    mutation DeleteAIConversation($id: String!) {
      delete_ai_conversation(id: $id)
    }
  `;
  const data = await graphqlQuery<{ delete_ai_conversation: boolean }>(mutation, { id });
  return data.delete_ai_conversation;
}
