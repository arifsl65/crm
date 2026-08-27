-- Migration: 000004_add_partitioned_table_fks (ROLLBACK)
-- Description: Remove FK constraints added to/from partitioned tables

-- ============================================================================
-- PART 3: Remove FKs TO partitioned emails table + drop columns
-- ============================================================================

-- 3d. ai_jobs
ALTER TABLE ai_jobs DROP CONSTRAINT IF EXISTS ai_jobs_email_id_fkey;
ALTER TABLE ai_jobs DROP COLUMN IF EXISTS email_created_at;

-- 3c. chase_log_clients
ALTER TABLE chase_log_clients DROP CONSTRAINT IF EXISTS chase_log_clients_email_id_fkey;
ALTER TABLE chase_log_clients DROP COLUMN IF EXISTS email_created_at;

-- 3b. reminders
ALTER TABLE reminders DROP CONSTRAINT IF EXISTS reminders_email_id_fkey;
ALTER TABLE reminders DROP COLUMN IF EXISTS email_created_at;

-- 3a. email_threads
ALTER TABLE email_threads DROP CONSTRAINT IF EXISTS email_threads_first_email_id_fkey;
ALTER TABLE email_threads DROP COLUMN IF EXISTS first_email_created_at;

-- ============================================================================
-- PART 2: Remove FKs FROM partitioned audit_logs table
-- ============================================================================

ALTER TABLE audit_logs DROP CONSTRAINT IF EXISTS audit_logs_user_id_fkey;
ALTER TABLE audit_logs DROP CONSTRAINT IF EXISTS audit_logs_tenant_id_fkey;

-- ============================================================================
-- PART 1: Remove FKs FROM partitioned emails table
-- ============================================================================

ALTER TABLE emails DROP CONSTRAINT IF EXISTS emails_reply_to_id_fkey;
ALTER TABLE emails DROP CONSTRAINT IF EXISTS emails_claimed_by_fkey;
ALTER TABLE emails DROP CONSTRAINT IF EXISTS emails_template_id_fkey;
ALTER TABLE emails DROP CONSTRAINT IF EXISTS emails_staff_id_fkey;
ALTER TABLE emails DROP CONSTRAINT IF EXISTS emails_client_id_fkey;
ALTER TABLE emails DROP CONSTRAINT IF EXISTS emails_tenant_id_fkey;
