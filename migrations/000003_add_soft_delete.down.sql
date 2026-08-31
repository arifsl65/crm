-- ============================================================================
-- Migration: 000003_add_soft_delete (ROLLBACK)
-- Description: Remove deleted_at columns from clients and documents tables
-- ============================================================================

-- Drop indexes first
DROP INDEX IF EXISTS idx_documents_active;
DROP INDEX IF EXISTS idx_clients_active;
DROP INDEX IF EXISTS idx_documents_deleted;
DROP INDEX IF EXISTS idx_clients_deleted;

-- Remove columns
ALTER TABLE documents DROP COLUMN IF EXISTS deleted_at;
ALTER TABLE clients DROP COLUMN IF EXISTS deleted_at;

-- Revert schema migration
DELETE FROM schema_migrations WHERE version = 3;
