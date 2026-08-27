-- ============================================================================
-- Accountant CRM - Initial Schema Rollback (v3.2)
-- Drops all tables in reverse dependency order
-- ============================================================================

-- Drop triggers first
DROP TRIGGER IF EXISTS trg_client_notes_updated_at ON client_notes;
DROP TRIGGER IF EXISTS trg_reminders_updated_at ON reminders;
DROP TRIGGER IF EXISTS trg_tenant_subscriptions_updated_at ON tenant_subscriptions;
DROP TRIGGER IF EXISTS trg_email_threads_updated_at ON email_threads;
DROP TRIGGER IF EXISTS trg_email_accounts_updated_at ON email_accounts;
DROP TRIGGER IF EXISTS trg_email_templates_updated_at ON email_templates;
DROP TRIGGER IF EXISTS trg_company_settings_updated_at ON company_settings;
DROP TRIGGER IF EXISTS trg_tenants_updated_at ON tenants;
DROP TRIGGER IF EXISTS trg_services_updated_at ON services;
DROP TRIGGER IF EXISTS trg_documents_updated_at ON documents;
DROP TRIGGER IF EXISTS trg_clients_updated_at ON clients;
DROP TRIGGER IF EXISTS trg_users_updated_at ON users;
DROP TRIGGER IF EXISTS trg_update_thread_message_count ON emails;
DROP TRIGGER IF EXISTS trg_update_service_docs_count ON documents;
DROP TRIGGER IF EXISTS trg_staff_client_same_tenant ON staff_clients;

-- Drop functions
DROP FUNCTION IF EXISTS update_updated_at_column();
DROP FUNCTION IF EXISTS update_thread_message_count();
DROP FUNCTION IF EXISTS update_service_docs_count();
DROP FUNCTION IF EXISTS check_staff_client_same_tenant();

-- Drop tables in reverse dependency order
DROP TABLE IF EXISTS schema_version;
DROP TABLE IF EXISTS outbox;
DROP TABLE IF EXISTS deletion_audit;
DROP TABLE IF EXISTS ai_jobs;
DROP TABLE IF EXISTS totp_backup_codes;
DROP TABLE IF EXISTS refresh_tokens;
DROP TABLE IF EXISTS webhook_idempotency;
DROP TABLE IF EXISTS tenant_invoices;
DROP TABLE IF EXISTS tenant_subscriptions;
DROP TABLE IF EXISTS client_notes;
DROP TABLE IF EXISTS chase_log_clients;
DROP TABLE IF EXISTS reminders;
DROP TABLE IF EXISTS magic_link_tokens;
DROP TABLE IF EXISTS upload_tokens;
DROP TABLE IF EXISTS push_tokens;
DROP TABLE IF EXISTS e_sign_requests;
DROP TABLE IF EXISTS chase_logs;
DROP TABLE IF EXISTS psc;
DROP TABLE IF EXISTS directors;
DROP TABLE IF EXISTS notifications;
DROP TABLE IF EXISTS audit_logs;
DROP TABLE IF EXISTS email_accounts;
DROP TABLE IF EXISTS emails;
DROP TABLE IF EXISTS email_templates;
DROP TABLE IF EXISTS email_threads;
DROP TABLE IF EXISTS company_settings;
DROP TABLE IF EXISTS service_requirements;
DROP TABLE IF EXISTS sessions;
DROP TABLE IF EXISTS document_access;
DROP TABLE IF EXISTS documents;
DROP TABLE IF EXISTS services;
DROP TABLE IF EXISTS service_types;
DROP TABLE IF EXISTS document_types;
DROP TABLE IF EXISTS staff_clients;
DROP TABLE IF EXISTS clients;
DROP TABLE IF EXISTS users;
DROP TABLE IF EXISTS tenants;

-- Drop email partitions (2026-2027)
DROP TABLE IF EXISTS emails_2027_02;
DROP TABLE IF EXISTS emails_2027_01;
DROP TABLE IF EXISTS emails_2026_12;
DROP TABLE IF EXISTS emails_2026_11;
DROP TABLE IF EXISTS emails_2026_10;
DROP TABLE IF EXISTS emails_2026_09;
DROP TABLE IF EXISTS emails_2026_08;
DROP TABLE IF EXISTS emails_2026_07;
DROP TABLE IF EXISTS emails_2026_06;
DROP TABLE IF EXISTS emails_2026_05;
DROP TABLE IF EXISTS emails_2026_04;
DROP TABLE IF EXISTS emails_2026_03;
DROP TABLE IF EXISTS emails_2026_02;
DROP TABLE IF EXISTS emails_2026_01;

-- Drop audit_logs partitions (2026-2027)
DROP TABLE IF EXISTS audit_logs_2027_02;
DROP TABLE IF EXISTS audit_logs_2027_01;
DROP TABLE IF EXISTS audit_logs_2026_12;
DROP TABLE IF EXISTS audit_logs_2026_11;
DROP TABLE IF EXISTS audit_logs_2026_10;
DROP TABLE IF EXISTS audit_logs_2026_09;
DROP TABLE IF EXISTS audit_logs_2026_08;
DROP TABLE IF EXISTS audit_logs_2026_07;
DROP TABLE IF EXISTS audit_logs_2026_06;
DROP TABLE IF EXISTS audit_logs_2026_05;
DROP TABLE IF EXISTS audit_logs_2026_04;
DROP TABLE IF EXISTS audit_logs_2026_03;
DROP TABLE IF EXISTS audit_logs_2026_02;
DROP TABLE IF EXISTS audit_logs_2026_01;

-- Drop ENUMs
DROP TYPE IF EXISTS ai_job_status;
DROP TYPE IF EXISTS subscription_status;
DROP TYPE IF EXISTS audit_severity;
DROP TYPE IF EXISTS reminder_status;
DROP TYPE IF EXISTS push_platform;
DROP TYPE IF EXISTS e_sign_status;
DROP TYPE IF EXISTS psc_ownership;
DROP TYPE IF EXISTS director_role;
DROP TYPE IF EXISTS notification_type;
DROP TYPE IF EXISTS email_account_status;
DROP TYPE IF EXISTS email_account_provider;
DROP TYPE IF EXISTS email_account_type;
DROP TYPE IF EXISTS email_template_type;
DROP TYPE IF EXISTS email_status;
DROP TYPE IF EXISTS email_type;
DROP TYPE IF EXISTS email_direction;
DROP TYPE IF EXISTS deadline_pattern;
DROP TYPE IF EXISTS risk_level;
DROP TYPE IF EXISTS service_priority;
DROP TYPE IF EXISTS service_status;
DROP TYPE IF EXISTS document_access_level;
DROP TYPE IF EXISTS document_status;
DROP TYPE IF EXISTS client_email_status;
DROP TYPE IF EXISTS client_status;
DROP TYPE IF EXISTS user_status;
DROP TYPE IF EXISTS user_role;
