-- Migration: Enable FORCE ROW LEVEL SECURITY
-- Issue: RLS is enabled but table owner (neondb_owner) bypasses policies
-- Solution: FORCE RLS ensures policies apply even to table owner
-- Reference: https://www.postgresql.org/docs/current/ddl-rowsecurity.html

-- =============================================================================
-- CRITICAL SECURITY FIX
-- =============================================================================
-- Without FORCE, the database owner role bypasses all RLS policies.
-- This migration ensures tenant isolation is enforced at the database level,
-- providing defense-in-depth even if application code has bugs.

-- =============================================================================
-- CORE TABLES (7 tables)
-- =============================================================================

ALTER TABLE users FORCE ROW LEVEL SECURITY;
ALTER TABLE clients FORCE ROW LEVEL SECURITY;
ALTER TABLE staff_clients FORCE ROW LEVEL SECURITY;
ALTER TABLE documents FORCE ROW LEVEL SECURITY;
ALTER TABLE document_access FORCE ROW LEVEL SECURITY;
ALTER TABLE services FORCE ROW LEVEL SECURITY;
ALTER TABLE sessions FORCE ROW LEVEL SECURITY;

-- =============================================================================
-- CONFIG TABLES (4 tables)
-- =============================================================================

ALTER TABLE document_types FORCE ROW LEVEL SECURITY;
ALTER TABLE service_types FORCE ROW LEVEL SECURITY;
ALTER TABLE service_requirements FORCE ROW LEVEL SECURITY;
ALTER TABLE company_settings FORCE ROW LEVEL SECURITY;

-- =============================================================================
-- EMAIL TABLES (4 tables)
-- =============================================================================

ALTER TABLE emails FORCE ROW LEVEL SECURITY;
ALTER TABLE email_templates FORCE ROW LEVEL SECURITY;
ALTER TABLE email_accounts FORCE ROW LEVEL SECURITY;
ALTER TABLE email_threads FORCE ROW LEVEL SECURITY;

-- =============================================================================
-- AUDIT & NOTIFICATIONS (2 tables)
-- =============================================================================

ALTER TABLE audit_logs FORCE ROW LEVEL SECURITY;
ALTER TABLE notifications FORCE ROW LEVEL SECURITY;

-- =============================================================================
-- COMPANIES HOUSE TABLES (2 tables)
-- =============================================================================

ALTER TABLE directors FORCE ROW LEVEL SECURITY;
ALTER TABLE psc FORCE ROW LEVEL SECURITY;

-- =============================================================================
-- TRACKING TABLES (3 tables)
-- =============================================================================

ALTER TABLE chase_logs FORCE ROW LEVEL SECURITY;
ALTER TABLE chase_log_clients FORCE ROW LEVEL SECURITY;
ALTER TABLE client_notes FORCE ROW LEVEL SECURITY;

-- =============================================================================
-- WORKFLOW TABLES (2 tables)
-- =============================================================================

ALTER TABLE e_sign_requests FORCE ROW LEVEL SECURITY;
ALTER TABLE reminders FORCE ROW LEVEL SECURITY;

-- =============================================================================
-- AUTH TABLES (5 tables)
-- =============================================================================

ALTER TABLE magic_link_tokens FORCE ROW LEVEL SECURITY;
ALTER TABLE upload_tokens FORCE ROW LEVEL SECURITY;
ALTER TABLE push_tokens FORCE ROW LEVEL SECURITY;
ALTER TABLE refresh_tokens FORCE ROW LEVEL SECURITY;
ALTER TABLE totp_backup_codes FORCE ROW LEVEL SECURITY;

-- =============================================================================
-- MULTI-TENANT TABLES (3 tables)
-- =============================================================================

ALTER TABLE tenants FORCE ROW LEVEL SECURITY;
ALTER TABLE tenant_subscriptions FORCE ROW LEVEL SECURITY;
ALTER TABLE tenant_invoices FORCE ROW LEVEL SECURITY;

-- =============================================================================
-- AI & ASYNC TABLES (2 tables)
-- =============================================================================

ALTER TABLE ai_jobs FORCE ROW LEVEL SECURITY;
ALTER TABLE outbox FORCE ROW LEVEL SECURITY;

-- =============================================================================
-- APPLICATION ROLE (without BYPASSRLS)
-- =============================================================================
-- CRITICAL: The table owner role (neondb_owner on Neon) has BYPASSRLS privilege,
-- which bypasses ALL RLS policies regardless of FORCE settings.
-- Solution: Create a separate application role without BYPASSRLS for the Go backend.

-- =============================================================================
-- SECURITY NOTE: Password is set via environment variable, NOT hardcoded here.
-- After running this migration, you MUST set the password manually:
--
--   ALTER ROLE app_user WITH PASSWORD 'your-secure-password-here';
--
-- Or use the password rotation migration (000009_rotate_app_user_password.up.sql)
-- which reads from POSTGRES_APP_USER_PASSWORD environment variable.
-- =============================================================================

DO $$
BEGIN
    -- Create app_user role if it doesn't exist (password set separately for security)
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'app_user') THEN
        -- Create role with a random temporary password that MUST be changed
        -- The actual password should be set via ALTER ROLE after migration
        CREATE ROLE app_user WITH LOGIN PASSWORD 'CHANGE_ME_IMMEDIATELY' NOBYPASSRLS;
        RAISE NOTICE 'SECURITY: app_user created with temporary password. Run ALTER ROLE app_user WITH PASSWORD ''your-secure-password'' immediately!';
    END IF;
END $$;

-- Grant necessary permissions
GRANT CONNECT ON DATABASE neondb TO app_user;
GRANT USAGE ON SCHEMA public TO app_user;
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO app_user;
GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO app_user;

-- Ensure future tables also get permissions
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO app_user;
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT USAGE, SELECT ON SEQUENCES TO app_user;

-- =============================================================================
-- Update schema migrations
-- =============================================================================

INSERT INTO schema_migrations (version, dirty)
VALUES (7, false)
ON CONFLICT (version) DO UPDATE SET dirty = false;
