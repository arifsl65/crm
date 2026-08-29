-- Migration: Add Counter Triggers
-- Description: Triggers to automatically update counter fields for:
--   1. services.docs_received when documents are added
--   2. email_threads.message_count when emails are added
--   3. chase_logs.opened when webhooks update status

-- ============================================================================
-- DROP EXISTING TRIGGERS FROM 000001 (to prevent duplicates)
-- ============================================================================
-- These triggers were created in 000001_initial_schema.up.sql
-- We drop them here and recreate with improved logic (separate triggers per operation)

DROP TRIGGER IF EXISTS trg_update_service_docs_count ON documents;
DROP TRIGGER IF EXISTS trg_update_thread_message_count ON emails;

-- ============================================================================
-- TRIGGER 1: Update service document count on document INSERT
-- ============================================================================

CREATE OR REPLACE FUNCTION update_service_docs_count()
RETURNS TRIGGER AS $$
BEGIN
    -- Only increment if document is linked to a service
    IF NEW.service_id IS NOT NULL THEN
        UPDATE services
        SET docs_received = docs_received + 1,
            updated_at = NOW()
        WHERE id = NEW.service_id;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Also handle decrement on DELETE
CREATE OR REPLACE FUNCTION decrement_service_docs_count()
RETURNS TRIGGER AS $$
BEGIN
    IF OLD.service_id IS NOT NULL THEN
        UPDATE services
        SET docs_received = GREATEST(0, docs_received - 1),
            updated_at = NOW()
        WHERE id = OLD.service_id;
    END IF;
    RETURN OLD;
END;
$$ LANGUAGE plpgsql;

-- Handle UPDATE when service_id changes
CREATE OR REPLACE FUNCTION update_service_docs_count_on_update()
RETURNS TRIGGER AS $$
BEGIN
    -- Decrement old service
    IF OLD.service_id IS NOT NULL AND (NEW.service_id IS NULL OR OLD.service_id != NEW.service_id) THEN
        UPDATE services
        SET docs_received = GREATEST(0, docs_received - 1),
            updated_at = NOW()
        WHERE id = OLD.service_id;
    END IF;

    -- Increment new service
    IF NEW.service_id IS NOT NULL AND (OLD.service_id IS NULL OR OLD.service_id != NEW.service_id) THEN
        UPDATE services
        SET docs_received = docs_received + 1,
            updated_at = NOW()
        WHERE id = NEW.service_id;
    END IF;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER tr_documents_increment_service_count
    AFTER INSERT ON documents
    FOR EACH ROW
    EXECUTE FUNCTION update_service_docs_count();

CREATE TRIGGER tr_documents_decrement_service_count
    AFTER DELETE ON documents
    FOR EACH ROW
    EXECUTE FUNCTION decrement_service_docs_count();

CREATE TRIGGER tr_documents_update_service_count
    AFTER UPDATE OF service_id ON documents
    FOR EACH ROW
    WHEN (OLD.service_id IS DISTINCT FROM NEW.service_id)
    EXECUTE FUNCTION update_service_docs_count_on_update();

-- ============================================================================
-- TRIGGER 2: Update email thread message count on email INSERT
-- ============================================================================

CREATE OR REPLACE FUNCTION update_thread_message_count()
RETURNS TRIGGER AS $$
BEGIN
    -- Only update if thread_id is set
    IF NEW.thread_id IS NOT NULL THEN
        UPDATE email_threads
        SET message_count = message_count + 1,
            last_message_at = NEW.created_at,
            updated_at = NOW()
        WHERE thread_key = NEW.thread_id;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION decrement_thread_message_count()
RETURNS TRIGGER AS $$
BEGIN
    IF OLD.thread_id IS NOT NULL THEN
        UPDATE email_threads
        SET message_count = GREATEST(1, message_count - 1),
            updated_at = NOW()
        WHERE thread_key = OLD.thread_id;
    END IF;
    RETURN OLD;
END;
$$ LANGUAGE plpgsql;

-- Note: emails table is partitioned, so we need to create triggers on the parent table
-- PostgreSQL 11+ supports triggers on partitioned tables
CREATE TRIGGER tr_emails_increment_thread_count
    AFTER INSERT ON emails
    FOR EACH ROW
    EXECUTE FUNCTION update_thread_message_count();

CREATE TRIGGER tr_emails_decrement_thread_count
    AFTER DELETE ON emails
    FOR EACH ROW
    EXECUTE FUNCTION decrement_thread_message_count();

-- ============================================================================
-- TRIGGER 3: Update chase log counters atomically
-- ============================================================================

-- This function updates chase_logs.opened counter when an email is marked as opened
-- It's called by webhook handlers or email status updates
CREATE OR REPLACE FUNCTION update_chase_opened_counter()
RETURNS TRIGGER AS $$
BEGIN
    -- When email status changes to 'opened', increment the chase log's opened counter
    IF NEW.status = 'opened' AND (OLD.status IS NULL OR OLD.status != 'opened') THEN
        -- Find the chase log through chase_log_clients
        UPDATE chase_logs
        SET opened = opened + 1
        WHERE id IN (
            SELECT chase_log_id
            FROM chase_log_clients
            WHERE email_id = NEW.id
        );
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER tr_emails_update_chase_opened
    AFTER UPDATE OF status ON emails
    FOR EACH ROW
    WHEN (NEW.status = 'opened' AND (OLD.status IS NULL OR OLD.status != 'opened'))
    EXECUTE FUNCTION update_chase_opened_counter();

-- ============================================================================
-- TRIGGER 4: Update document chase count on chase
-- ============================================================================

CREATE OR REPLACE FUNCTION update_document_chase_count()
RETURNS TRIGGER AS $$
BEGIN
    -- When a document request email is sent, increment the chase count
    IF NEW.service_id IS NOT NULL THEN
        UPDATE documents
        SET chase_count = chase_count + 1,
            last_chased_at = NOW(),
            updated_at = NOW()
        WHERE service_id = NEW.service_id
        AND status = 'requested';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Note: This trigger would be on chase_logs, but we need more context about
-- which documents are being chased. For now, this is a placeholder that can
-- be refined based on the actual chase workflow.

-- ============================================================================
-- INDEXES for performance on counter lookups
-- ============================================================================

-- Ensure efficient lookups for the triggers
CREATE INDEX IF NOT EXISTS idx_documents_service_id ON documents(service_id) WHERE service_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_email_threads_thread_key ON email_threads(thread_key);
CREATE INDEX IF NOT EXISTS idx_chase_log_clients_email_id ON chase_log_clients(email_id) WHERE email_id IS NOT NULL;

-- ============================================================================
-- Comments for documentation
-- ============================================================================

COMMENT ON FUNCTION update_service_docs_count() IS 'Increments services.docs_received when a document is linked to a service';
COMMENT ON FUNCTION update_thread_message_count() IS 'Increments email_threads.message_count when an email is added to a thread';
COMMENT ON FUNCTION update_chase_opened_counter() IS 'Increments chase_logs.opened when an email in the chase is marked as opened';
