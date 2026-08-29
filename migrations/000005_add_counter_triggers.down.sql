-- Rollback: Remove Counter Triggers

-- Drop triggers created by this migration
DROP TRIGGER IF EXISTS tr_documents_increment_service_count ON documents;
DROP TRIGGER IF EXISTS tr_documents_decrement_service_count ON documents;
DROP TRIGGER IF EXISTS tr_documents_update_service_count ON documents;
DROP TRIGGER IF EXISTS tr_emails_increment_thread_count ON emails;
DROP TRIGGER IF EXISTS tr_emails_decrement_thread_count ON emails;
DROP TRIGGER IF EXISTS tr_emails_update_chase_opened ON emails;

-- Drop functions specific to this migration
DROP FUNCTION IF EXISTS decrement_service_docs_count();
DROP FUNCTION IF EXISTS update_service_docs_count_on_update();
DROP FUNCTION IF EXISTS decrement_thread_message_count();
DROP FUNCTION IF EXISTS update_chase_opened_counter();
DROP FUNCTION IF EXISTS update_document_chase_count();

-- Drop indexes (only if they were specifically created for triggers)
DROP INDEX IF EXISTS idx_documents_service_id;
DROP INDEX IF EXISTS idx_email_threads_thread_key;
DROP INDEX IF EXISTS idx_chase_log_clients_email_id;

-- Restore original triggers from 000001_initial_schema.up.sql
-- These were dropped in the up migration, so we need to recreate them

CREATE OR REPLACE FUNCTION update_service_docs_count()
RETURNS TRIGGER AS $$
BEGIN
    IF TG_OP = 'INSERT' AND NEW.service_id IS NOT NULL THEN
        UPDATE services SET docs_received = docs_received + 1
        WHERE id = NEW.service_id;
    ELSIF TG_OP = 'DELETE' AND OLD.service_id IS NOT NULL THEN
        UPDATE services SET docs_received = GREATEST(0, docs_received - 1)
        WHERE id = OLD.service_id;
    ELSIF TG_OP = 'UPDATE' THEN
        IF OLD.service_id IS DISTINCT FROM NEW.service_id THEN
            IF OLD.service_id IS NOT NULL THEN
                UPDATE services SET docs_received = GREATEST(0, docs_received - 1)
                WHERE id = OLD.service_id;
            END IF;
            IF NEW.service_id IS NOT NULL THEN
                UPDATE services SET docs_received = docs_received + 1
                WHERE id = NEW.service_id;
            END IF;
        END IF;
    END IF;
    RETURN COALESCE(NEW, OLD);
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_update_service_docs_count
    AFTER INSERT OR UPDATE OR DELETE ON documents
    FOR EACH ROW EXECUTE FUNCTION update_service_docs_count();

CREATE OR REPLACE FUNCTION update_thread_message_count()
RETURNS TRIGGER AS $$
BEGIN
    IF TG_OP = 'INSERT' AND NEW.thread_id IS NOT NULL THEN
        UPDATE email_threads
        SET message_count = message_count + 1,
            last_message_at = NEW.created_at
        WHERE thread_key = NEW.thread_id;
    ELSIF TG_OP = 'DELETE' AND OLD.thread_id IS NOT NULL THEN
        UPDATE email_threads
        SET message_count = GREATEST(1, message_count - 1)
        WHERE thread_key = OLD.thread_id;
    ELSIF TG_OP = 'UPDATE' AND OLD.thread_id IS DISTINCT FROM NEW.thread_id THEN
        IF OLD.thread_id IS NOT NULL THEN
            UPDATE email_threads
            SET message_count = GREATEST(1, message_count - 1)
            WHERE thread_key = OLD.thread_id;
        END IF;
        IF NEW.thread_id IS NOT NULL THEN
            UPDATE email_threads
            SET message_count = message_count + 1,
                last_message_at = NEW.created_at
            WHERE thread_key = NEW.thread_id;
        END IF;
    END IF;
    RETURN COALESCE(NEW, OLD);
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_update_thread_message_count
    AFTER INSERT OR UPDATE OR DELETE ON emails
    FOR EACH ROW EXECUTE FUNCTION update_thread_message_count();
