-- ============================================================================
-- Migration 000006: Rollback enhanced service_types and document_types
-- ============================================================================

-- Drop RLS policies
DROP POLICY IF EXISTS service_types_update ON service_types;
DROP POLICY IF EXISTS service_types_delete ON service_types;
DROP POLICY IF EXISTS document_types_update ON document_types;
DROP POLICY IF EXISTS document_types_delete ON document_types;

-- Drop triggers
DROP TRIGGER IF EXISTS trg_service_types_updated_at ON service_types;
DROP TRIGGER IF EXISTS trg_document_types_updated_at ON document_types;

-- Drop indexes
DROP INDEX IF EXISTS idx_service_types_tenant_category;
DROP INDEX IF EXISTS idx_service_types_sort;
DROP INDEX IF EXISTS idx_document_types_tenant_category;
DROP INDEX IF EXISTS idx_document_types_sort;

-- Remove added columns from service_types
ALTER TABLE service_types
    DROP COLUMN IF EXISTS category,
    DROP COLUMN IF EXISTS default_priority,
    DROP COLUMN IF EXISTS default_deadline_days,
    DROP COLUMN IF EXISTS required_docs,
    DROP COLUMN IF EXISTS checklist_template,
    DROP COLUMN IF EXISTS is_recurring,
    DROP COLUMN IF EXISTS recurrence_pattern,
    DROP COLUMN IF EXISTS hmrc_relevant,
    DROP COLUMN IF EXISTS sort_order,
    DROP COLUMN IF EXISTS updated_at;

-- Remove added columns from document_types
ALTER TABLE document_types
    DROP COLUMN IF EXISTS category,
    DROP COLUMN IF EXISTS allowed_mime_types,
    DROP COLUMN IF EXISTS max_file_size_mb,
    DROP COLUMN IF EXISTS retention_days,
    DROP COLUMN IF EXISTS requires_approval,
    DROP COLUMN IF EXISTS expiry_required,
    DROP COLUMN IF EXISTS sort_order,
    DROP COLUMN IF EXISTS updated_at;

-- Remove schema version
DELETE FROM schema_version WHERE version = 6;
