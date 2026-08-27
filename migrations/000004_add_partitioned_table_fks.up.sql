-- Migration: 000004_add_partitioned_table_fks
-- Description: Add missing FK constraints to/from partitioned tables (emails, audit_logs)
-- Date: 2026-08-27
--
-- PostgreSQL 18+ supports FKs on partitioned tables.
-- For FKs TO partitioned tables, composite keys are required (must include partition key).

-- ============================================================================
-- PART 1: FKs FROM partitioned emails table (outbound)
-- ============================================================================

ALTER TABLE emails
ADD CONSTRAINT emails_tenant_id_fkey
FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE;

ALTER TABLE emails
ADD CONSTRAINT emails_client_id_fkey
FOREIGN KEY (client_id) REFERENCES clients(id) ON DELETE SET NULL;

ALTER TABLE emails
ADD CONSTRAINT emails_staff_id_fkey
FOREIGN KEY (staff_id) REFERENCES users(id) ON DELETE SET NULL;

ALTER TABLE emails
ADD CONSTRAINT emails_template_id_fkey
FOREIGN KEY (template_id) REFERENCES email_templates(id) ON DELETE SET NULL;

ALTER TABLE emails
ADD CONSTRAINT emails_claimed_by_fkey
FOREIGN KEY (claimed_by) REFERENCES users(id) ON DELETE SET NULL;

-- Self-referencing FK (requires partition key in composite)
ALTER TABLE emails
ADD CONSTRAINT emails_reply_to_id_fkey
FOREIGN KEY (reply_to_id, created_at) REFERENCES emails(id, created_at) ON DELETE SET NULL;

-- ============================================================================
-- PART 2: FKs FROM partitioned audit_logs table (outbound)
-- ============================================================================

ALTER TABLE audit_logs
ADD CONSTRAINT audit_logs_tenant_id_fkey
FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE;

ALTER TABLE audit_logs
ADD CONSTRAINT audit_logs_user_id_fkey
FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE SET NULL;

-- ============================================================================
-- PART 3: FKs TO partitioned emails table (inbound - requires composite FK)
--
-- PostgreSQL constraint: Cannot create UNIQUE index on just 'id' for partitioned
-- tables - partition key must be included. Therefore, referencing tables need
-- an additional column to store email's created_at for the composite FK.
-- ============================================================================

-- 3a. email_threads.first_email_id -> emails
ALTER TABLE email_threads
ADD COLUMN first_email_created_at TIMESTAMP;

ALTER TABLE email_threads
ADD CONSTRAINT email_threads_first_email_id_fkey
FOREIGN KEY (first_email_id, first_email_created_at)
REFERENCES emails(id, created_at) ON DELETE SET NULL;

COMMENT ON COLUMN email_threads.first_email_created_at IS
'Required for composite FK to partitioned emails table. Must be set when first_email_id is set.';

-- 3b. reminders.email_id -> emails
ALTER TABLE reminders
ADD COLUMN email_created_at TIMESTAMP;

ALTER TABLE reminders
ADD CONSTRAINT reminders_email_id_fkey
FOREIGN KEY (email_id, email_created_at)
REFERENCES emails(id, created_at) ON DELETE CASCADE;

COMMENT ON COLUMN reminders.email_created_at IS
'Required for composite FK to partitioned emails table. Must be set when email_id is set.';

-- 3c. chase_log_clients.email_id -> emails
ALTER TABLE chase_log_clients
ADD COLUMN email_created_at TIMESTAMP;

ALTER TABLE chase_log_clients
ADD CONSTRAINT chase_log_clients_email_id_fkey
FOREIGN KEY (email_id, email_created_at)
REFERENCES emails(id, created_at) ON DELETE CASCADE;

COMMENT ON COLUMN chase_log_clients.email_created_at IS
'Required for composite FK to partitioned emails table. Must be set when email_id is set.';

-- 3d. ai_jobs.email_id -> emails
ALTER TABLE ai_jobs
ADD COLUMN email_created_at TIMESTAMP;

ALTER TABLE ai_jobs
ADD CONSTRAINT ai_jobs_email_id_fkey
FOREIGN KEY (email_id, email_created_at)
REFERENCES emails(id, created_at) ON DELETE SET NULL;

COMMENT ON COLUMN ai_jobs.email_created_at IS
'Required for composite FK to partitioned emails table. Must be set when email_id is set.';
