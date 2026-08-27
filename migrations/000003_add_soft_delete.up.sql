-- ============================================================================
-- Migration: 000003_add_soft_delete
-- Description: Add missing deleted_at columns to clients and documents tables
-- Aligns with db_final.md specification
-- ============================================================================

-- Add deleted_at column to clients table
ALTER TABLE clients ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMP;

-- Add deleted_at column to documents table
ALTER TABLE documents ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMP;

-- Add indexes for efficient soft-delete queries
CREATE INDEX IF NOT EXISTS idx_clients_deleted ON clients(deleted_at) WHERE deleted_at IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_documents_deleted ON documents(deleted_at) WHERE deleted_at IS NOT NULL;

-- Add index for active records queries (most common query pattern)
CREATE INDEX IF NOT EXISTS idx_clients_active ON clients(tenant_id, status) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_documents_active ON documents(tenant_id, status) WHERE deleted_at IS NULL;

-- Update schema version
UPDATE schema_version SET version = 3, description = 'Added soft delete columns to clients and documents', applied_at = CURRENT_TIMESTAMP WHERE version = 2;
