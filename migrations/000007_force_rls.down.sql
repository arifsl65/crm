-- Rollback: Disable FORCE ROW LEVEL SECURITY
-- WARNING: This removes defense-in-depth security. Only use for debugging.

-- =============================================================================
-- CORE TABLES
-- =============================================================================

ALTER TABLE users NO FORCE ROW LEVEL SECURITY;
ALTER TABLE clients NO FORCE ROW LEVEL SECURITY;
ALTER TABLE staff_clients NO FORCE ROW LEVEL SECURITY;
ALTER TABLE documents NO FORCE ROW LEVEL SECURITY;
ALTER TABLE document_access NO FORCE ROW LEVEL SECURITY;
ALTER TABLE services NO FORCE ROW LEVEL SECURITY;
ALTER TABLE sessions NO FORCE ROW LEVEL SECURITY;

-- =============================================================================
-- CONFIG TABLES
-- =============================================================================

ALTER TABLE document_types NO FORCE ROW LEVEL SECURITY;
ALTER TABLE service_types NO FORCE ROW LEVEL SECURITY;
ALTER TABLE service_requirements NO FORCE ROW LEVEL SECURITY;
ALTER TABLE company_settings NO FORCE ROW LEVEL SECURITY;

-- =============================================================================
-- EMAIL TABLES
-- =============================================================================

ALTER TABLE emails NO FORCE ROW LEVEL SECURITY;
ALTER TABLE email_templates NO FORCE ROW LEVEL SECURITY;
ALTER TABLE email_accounts NO FORCE ROW LEVEL SECURITY;
ALTER TABLE email_threads NO FORCE ROW LEVEL SECURITY;

-- =============================================================================
-- AUDIT & NOTIFICATIONS
-- =============================================================================

ALTER TABLE audit_logs NO FORCE ROW LEVEL SECURITY;
ALTER TABLE notifications NO FORCE ROW LEVEL SECURITY;

-- =============================================================================
-- COMPANIES HOUSE TABLES
-- =============================================================================

ALTER TABLE directors NO FORCE ROW LEVEL SECURITY;
ALTER TABLE psc NO FORCE ROW LEVEL SECURITY;

-- =============================================================================
-- TRACKING TABLES
-- =============================================================================

ALTER TABLE chase_logs NO FORCE ROW LEVEL SECURITY;
ALTER TABLE chase_log_clients NO FORCE ROW LEVEL SECURITY;
ALTER TABLE client_notes NO FORCE ROW LEVEL SECURITY;

-- =============================================================================
-- WORKFLOW TABLES
-- =============================================================================

ALTER TABLE e_sign_requests NO FORCE ROW LEVEL SECURITY;
ALTER TABLE reminders NO FORCE ROW LEVEL SECURITY;

-- =============================================================================
-- AUTH TABLES
-- =============================================================================

ALTER TABLE magic_link_tokens NO FORCE ROW LEVEL SECURITY;
ALTER TABLE upload_tokens NO FORCE ROW LEVEL SECURITY;
ALTER TABLE push_tokens NO FORCE ROW LEVEL SECURITY;
ALTER TABLE refresh_tokens NO FORCE ROW LEVEL SECURITY;
ALTER TABLE totp_backup_codes NO FORCE ROW LEVEL SECURITY;

-- =============================================================================
-- MULTI-TENANT TABLES
-- =============================================================================

ALTER TABLE tenants NO FORCE ROW LEVEL SECURITY;
ALTER TABLE tenant_subscriptions NO FORCE ROW LEVEL SECURITY;
ALTER TABLE tenant_invoices NO FORCE ROW LEVEL SECURITY;

-- =============================================================================
-- AI & ASYNC TABLES
-- =============================================================================

ALTER TABLE ai_jobs NO FORCE ROW LEVEL SECURITY;
ALTER TABLE outbox NO FORCE ROW LEVEL SECURITY;

-- =============================================================================
-- REVOKE APPLICATION ROLE PERMISSIONS
-- =============================================================================

-- Revoke default privileges first
ALTER DEFAULT PRIVILEGES IN SCHEMA public REVOKE SELECT, INSERT, UPDATE, DELETE ON TABLES FROM app_user;
ALTER DEFAULT PRIVILEGES IN SCHEMA public REVOKE USAGE, SELECT ON SEQUENCES FROM app_user;

-- Revoke existing permissions
REVOKE SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public FROM app_user;
REVOKE USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public FROM app_user;
REVOKE USAGE ON SCHEMA public FROM app_user;
REVOKE CONNECT ON DATABASE neondb FROM app_user;

-- Drop the role (if no active connections)
DROP ROLE IF EXISTS app_user;

-- =============================================================================
-- Update schema migrations
-- =============================================================================

DELETE FROM schema_migrations WHERE version = 7;
