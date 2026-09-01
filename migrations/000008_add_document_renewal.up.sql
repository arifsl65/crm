-- ============================================================================
-- Migration: Add document renewal workflow fields
-- Week 7: Complete Documents + Demo
-- ============================================================================

-- Add renewal workflow fields to documents table
ALTER TABLE documents
ADD COLUMN IF NOT EXISTS renewal_requested BOOLEAN DEFAULT false,
ADD COLUMN IF NOT EXISTS renewal_requested_at TIMESTAMP,
ADD COLUMN IF NOT EXISTS renewal_requested_by UUID REFERENCES users(id),
ADD COLUMN IF NOT EXISTS renewal_note TEXT;

-- Add index for expiring documents query
-- Note: We filter for documents that can expire (have an expiry_date and are approved)
CREATE INDEX IF NOT EXISTS idx_documents_expiry_date
ON documents(tenant_id, expiry_date)
WHERE expiry_date IS NOT NULL AND status = 'approved';

-- Add index for renewal requests
CREATE INDEX IF NOT EXISTS idx_documents_renewal_requested
ON documents(tenant_id, renewal_requested)
WHERE renewal_requested = true;

-- Comment on new columns
COMMENT ON COLUMN documents.renewal_requested IS 'Whether a renewal has been requested for this document';
COMMENT ON COLUMN documents.renewal_requested_at IS 'When the renewal was requested';
COMMENT ON COLUMN documents.renewal_requested_by IS 'User who requested the renewal';
COMMENT ON COLUMN documents.renewal_note IS 'Optional note attached to the renewal request';
