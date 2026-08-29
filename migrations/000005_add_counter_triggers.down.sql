-- Rollback: Remove Counter Triggers

-- Drop triggers
DROP TRIGGER IF EXISTS tr_documents_increment_service_count ON documents;
DROP TRIGGER IF EXISTS tr_documents_decrement_service_count ON documents;
DROP TRIGGER IF EXISTS tr_documents_update_service_count ON documents;
DROP TRIGGER IF EXISTS tr_emails_increment_thread_count ON emails;
DROP TRIGGER IF EXISTS tr_emails_decrement_thread_count ON emails;
DROP TRIGGER IF EXISTS tr_emails_update_chase_opened ON emails;

-- Drop functions
DROP FUNCTION IF EXISTS update_service_docs_count();
DROP FUNCTION IF EXISTS decrement_service_docs_count();
DROP FUNCTION IF EXISTS update_service_docs_count_on_update();
DROP FUNCTION IF EXISTS update_thread_message_count();
DROP FUNCTION IF EXISTS decrement_thread_message_count();
DROP FUNCTION IF EXISTS update_chase_opened_counter();
DROP FUNCTION IF EXISTS update_document_chase_count();

-- Drop indexes (only if they were specifically created for triggers)
DROP INDEX IF EXISTS idx_documents_service_id;
DROP INDEX IF EXISTS idx_email_threads_thread_key;
DROP INDEX IF EXISTS idx_chase_log_clients_email_id;
