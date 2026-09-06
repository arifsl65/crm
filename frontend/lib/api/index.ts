/**
 * API Barrel File - Re-exports all API modules for unified imports.
 *
 * Usage: import { getClients, Client, authFetch } from '@/lib/api';
 *
 * This file re-exports everything from domain-specific modules,
 * maintaining backwards compatibility with the original api.ts.
 */

// Core utilities (authFetch, API_URL, etc.)
export { API_URL, AUTH_EXPIRED_EVENT, authFetch, tryRefreshToken, clearAuthAndRedirect } from './core';

// All types and interfaces
export * from './types';

// Dashboard
export {
  getDashboardStats,
  getDashboardDeadlines,
  getKanban,
  getDashboardPendingDocuments,
  getDashboardWorkload,
  getDashboardRecentClients,
} from './dashboard';

// Clients (including Companies House)
export {
  getClients,
  getClient,
  createClient,
  updateClient,
  deleteClient,
  restoreClient,
  getClientDocuments,
  getClientServices,
  getClientEmails,
  getClientNotes,
  createClientNote,
  updateClientNote,
  deleteClientNote,
  assignStaffToClient,
  getSuppressedClients,
  bulkReassignClients,
  getClientDirectors,
  getClientPSC,
  searchCompaniesHouse,
  getCompanyFromCH,
  syncClientWithCH,
  getCHStatus,
  getCHFilings,
  getCHOfficers,
} from './clients';

// Documents
export {
  getDocuments,
  getDocument,
  createDocumentRequest,
  updateDocument,
  deleteDocument,
  approveDocument,
  rejectDocument,
  downloadDocument,
  requestDocumentRenewal,
  cancelDocumentRenewal,
  getDocumentVersions,
  restoreDocumentVersion,
  bulkApproveDocuments,
  bulkRequestDocuments,
  uploadDocument,
  getDocumentUploadUrl,
  confirmDocumentUpload,
  generateQRToken,
  verifyQRToken,
  uploadViaQR,
  getFirmDocuments,
  uploadFirmDocument,
  getFirmDocumentAccess,
  updateFirmDocumentAccess,
  getExpiringDocuments,
  getDocumentTypes,
  getDocumentType,
  createDocumentType,
  updateDocumentType,
  deleteDocumentType,
  getDocumentTypeCategories,
} from './documents';

// Services
export {
  getServices,
  getService,
  createService,
  updateService,
  deleteService,
  updateServiceStatus,
  completeService,
  markServiceHMRC,
  bulkUpdateServices,
  reorderServices,
  getServiceAlerts,
  getServiceTypes,
  getServiceType,
  createServiceType,
  updateServiceType,
  deleteServiceType,
  getServiceTypeCategories,
  cloneServiceType,
  reorderServiceTypes,
  getServiceTypeRequirements,
  addServiceTypeRequirement,
  removeServiceTypeRequirement,
} from './services';

// Emails
export {
  getEmails,
  getEmail,
  sendEmail,
  sendEmailFromTemplate,
  markEmailRead,
  getEmailStats,
  claimEmail,
  unclaimEmail,
  getEmailThreads,
  getEmailThread,
  getEmailThreadMessages,
  getEmailAccounts,
  getEmailAccount,
  createIMAPAccount,
  updateEmailAccount,
  deleteEmailAccount,
  syncEmailAccount,
  testEmailAccountConnection,
  disconnectEmailAccount,
  reconnectEmailAccount,
  getEmailTemplates,
  getEmailTemplate,
  createEmailTemplate,
  updateEmailTemplate,
  deleteEmailTemplate,
} from './emails';

// Auth
export {
  login,
  register,
  logout,
  sendMagicLink,
  verifyMagicLink,
  acceptInvite,
  forgotPassword,
  resetPassword,
  changePassword,
  getMe,
  updateMe,
  getSessions,
  setup2FA,
  verify2FA,
  disable2FA,
  generateBackupCodes,
  verifyBackupCode,
  revokeTokenFamily,
} from './auth';

// Users
export {
  getUsers,
  getUser,
  createUser,
  updateUser,
  deleteUser,
  restoreUser,
  reset2FA,
  resendInvite,
  getUserClients,
  getStaffWorkload,
} from './users';

// Notifications
export {
  getNotifications,
  getNotification,
  getUnreadNotificationCount,
  markNotificationRead,
  markAllNotificationsRead,
  dismissNotification,
  dismissAllNotifications,
  createNotification,
} from './notifications';

// Settings
export {
  getSettings,
  updateSettings,
  getBranding,
  updateBranding,
} from './settings';

// E-Sign
export {
  getESignRequests,
  getESignRequest,
  createESignRequest,
  sendESignRequest,
  deleteESignRequest,
  getSigningPageData,
  submitSignature,
} from './esign';

// Reminders
export {
  getReminders,
  getReminder,
  getUpcomingReminders,
  createReminder,
  completeReminder,
  dismissReminder,
  deleteReminder,
} from './reminders';

// Subscription
export {
  getSubscription,
  getInvoices,
  getUsageStats,
  createBillingPortalSession,
  createCheckoutSession,
  getPushTokens,
  registerPushToken,
  unregisterPushToken,
  unregisterPushTokenByValue,
} from './subscription';

// Portal
export {
  getPortalMe,
  getPortalDashboard,
  getPortalDocuments,
  getPortalServices,
  getPortalDeadlines,
  changePortalPassword,
  updatePortalProfile,
} from './portal';

// Admin
export {
  getTenants,
  getTenant,
  createTenant,
  updateTenant,
  deleteTenant,
  getAuditLogs,
  getAuditLogStats,
  getAuditLogActions,
  getAuditLogEntityTypes,
  getAuditLog,
  getChaseLogs,
  getChaseLogStats,
  createChaseLog,
  getChaseLog,
} from './admin';

// Search
export { globalSearch } from './search';

// Export
export { exportClients, exportServices, exportChaseLogs } from './export';

// GraphQL
export {
  graphqlQuery,
  gqlGetDashboard,
  gqlGetClient,
  gqlGetClients,
  gqlGetDocument,
  gqlGetDocuments,
  gqlGetService,
  gqlGetServices,
  gqlGetEmail,
  gqlGetEmailThread,
  gqlGetNotifications,
  gqlGetUnreadNotificationCount,
  gqlGetAIConversations,
  gqlGetAIConversation,
  gqlGetTroublemakerClients,
  gqlGetAnomalies,
  gqlSearch,
  gqlGetRecentSearches,
  gqlGetDocumentTypes,
  gqlGetServiceTypes,
  gqlGetMe,
  gqlMarkNotificationRead,
  gqlMarkAllNotificationsRead,
  gqlDismissNotification,
  gqlApproveDocument,
  gqlRejectDocument,
  gqlUpdateServiceStatus,
  gqlClaimEmail,
  gqlUnclaimEmail,
  gqlMarkEmailRead,
  gqlDeleteAIConversation,
} from './graphql';

// AI
export {
  aiChat,
  aiChatStream,
  getAIChatHistory,
  saveAIChatHistory,
  deleteAIChat,
  aiExtractDocument,
  aiClassifyDocument,
  aiSummarizeDocument,
  aiRenameDocument,
  aiSummarizeEmail,
  aiAnalyzeEmailSentiment,
  aiExtractEmailPromises,
  aiDraftEmail,
  aiMatchEmailToClient,
  aiSummarizeEmailThread,
  aiFindAlternateEmail,
  aiExtractFormData,
  aiAutoFillVAT,
  aiAutoFillCT600,
  aiAutoFillSA,
  aiAnalyzeClientRisk,
  aiAnalyzeServiceRisk,
  aiGenerateTemplate,
  aiCheckDuplicateClients,
  aiAutoNameService,
  aiGenerateCompletionSummary,
  aiFindTroublemakers,
  aiDetectAnomalies,
  aiAnalyzeStaffActivity,
  aiGetStaffWorkloads,
  aiCalculateRebalance,
  aiApplyRebalance,
  getAIJobStatus,
} from './ai';
