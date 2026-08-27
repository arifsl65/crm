-- Migration: Remove RLS policies added in 000002
-- Rollback: Drop all policies added for the 23 tables

DROP POLICY IF EXISTS tenant_isolation_ai_jobs ON ai_jobs;
DROP POLICY IF EXISTS tenant_isolation_chase_log_clients ON chase_log_clients;
DROP POLICY IF EXISTS tenant_isolation_chase_logs ON chase_logs;
DROP POLICY IF EXISTS tenant_isolation_client_notes ON client_notes;
DROP POLICY IF EXISTS tenant_isolation_company_settings ON company_settings;
DROP POLICY IF EXISTS tenant_isolation_directors ON directors;
DROP POLICY IF EXISTS tenant_isolation_document_access ON document_access;
DROP POLICY IF EXISTS tenant_isolation_e_sign_requests ON e_sign_requests;
DROP POLICY IF EXISTS tenant_isolation_email_accounts ON email_accounts;
DROP POLICY IF EXISTS tenant_isolation_email_threads ON email_threads;
DROP POLICY IF EXISTS tenant_isolation_magic_link_tokens ON magic_link_tokens;
DROP POLICY IF EXISTS tenant_isolation_outbox ON outbox;
DROP POLICY IF EXISTS tenant_isolation_psc ON psc;
DROP POLICY IF EXISTS tenant_isolation_push_tokens ON push_tokens;
DROP POLICY IF EXISTS tenant_isolation_refresh_tokens ON refresh_tokens;
DROP POLICY IF EXISTS tenant_isolation_reminders ON reminders;
DROP POLICY IF EXISTS tenant_isolation_service_requirements ON service_requirements;
DROP POLICY IF EXISTS tenant_isolation_sessions ON sessions;
DROP POLICY IF EXISTS tenant_isolation_staff_clients ON staff_clients;
DROP POLICY IF EXISTS tenant_isolation_tenant_invoices ON tenant_invoices;
DROP POLICY IF EXISTS tenant_isolation_tenant_subscriptions ON tenant_subscriptions;
DROP POLICY IF EXISTS tenant_isolation_upload_tokens ON upload_tokens;
DROP POLICY IF EXISTS user_isolation_totp_backup_codes ON totp_backup_codes;

-- Revert schema version
UPDATE schema_version SET version = 1, applied_at = CURRENT_TIMESTAMP WHERE version = 2;
