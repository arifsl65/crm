-- ============================================================================
-- Rollback: Remove document renewal workflow fields
-- ============================================================================

DROP INDEX IF EXISTS idx_documents_renewal_requested;
DROP INDEX IF EXISTS idx_documents_expiry_date;

ALTER TABLE documents
DROP COLUMN IF EXISTS renewal_note,
DROP COLUMN IF EXISTS renewal_requested_by,
DROP COLUMN IF EXISTS renewal_requested_at,
DROP COLUMN IF EXISTS renewal_requested;
