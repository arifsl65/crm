-- ============================================================================
-- Migration 000006: Enhance service_types and document_types tables
-- Adds additional columns for better service/document type management
-- ============================================================================

-- Add new columns to service_types
ALTER TABLE service_types
    ADD COLUMN IF NOT EXISTS category VARCHAR(100) DEFAULT 'general',
    ADD COLUMN IF NOT EXISTS default_priority VARCHAR(20) DEFAULT 'normal',
    ADD COLUMN IF NOT EXISTS default_deadline_days INTEGER,
    ADD COLUMN IF NOT EXISTS required_docs TEXT[],
    ADD COLUMN IF NOT EXISTS checklist_template TEXT[],
    ADD COLUMN IF NOT EXISTS is_recurring BOOLEAN DEFAULT false,
    ADD COLUMN IF NOT EXISTS recurrence_pattern VARCHAR(50),
    ADD COLUMN IF NOT EXISTS hmrc_relevant BOOLEAN DEFAULT false,
    ADD COLUMN IF NOT EXISTS sort_order INTEGER DEFAULT 0,
    ADD COLUMN IF NOT EXISTS updated_at TIMESTAMP DEFAULT NOW();

-- Rename deadline_pattern and deadline_offset to be consistent
-- (keeping old columns for backward compatibility, the new columns are preferred)

-- Add new columns to document_types
ALTER TABLE document_types
    ADD COLUMN IF NOT EXISTS category VARCHAR(100) DEFAULT 'general',
    ADD COLUMN IF NOT EXISTS allowed_mime_types TEXT[] DEFAULT ARRAY['application/pdf', 'image/jpeg', 'image/png'],
    ADD COLUMN IF NOT EXISTS max_file_size_mb INTEGER DEFAULT 10,
    ADD COLUMN IF NOT EXISTS retention_days INTEGER,
    ADD COLUMN IF NOT EXISTS requires_approval BOOLEAN DEFAULT true,
    ADD COLUMN IF NOT EXISTS expiry_required BOOLEAN DEFAULT false,
    ADD COLUMN IF NOT EXISTS sort_order INTEGER DEFAULT 0,
    ADD COLUMN IF NOT EXISTS updated_at TIMESTAMP DEFAULT NOW();

-- Migrate has_expiry to expiry_required
UPDATE document_types SET expiry_required = has_expiry WHERE has_expiry IS NOT NULL;

-- Create indexes for better query performance
CREATE INDEX IF NOT EXISTS idx_service_types_tenant_category ON service_types(tenant_id, category) WHERE is_active = true;
CREATE INDEX IF NOT EXISTS idx_service_types_sort ON service_types(tenant_id, sort_order);
CREATE INDEX IF NOT EXISTS idx_document_types_tenant_category ON document_types(tenant_id, category) WHERE is_active = true;
CREATE INDEX IF NOT EXISTS idx_document_types_sort ON document_types(tenant_id, sort_order);

-- Add updated_at triggers
CREATE TRIGGER trg_service_types_updated_at BEFORE UPDATE ON service_types FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
CREATE TRIGGER trg_document_types_updated_at BEFORE UPDATE ON document_types FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- Add RLS policies for update/delete (existing policies only cover select and insert)
CREATE POLICY service_types_update ON service_types FOR UPDATE
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid OR current_setting('app.role', true) = 'super_admin')
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::uuid OR current_setting('app.role', true) = 'super_admin');

CREATE POLICY service_types_delete ON service_types FOR DELETE
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid OR current_setting('app.role', true) = 'super_admin');

CREATE POLICY document_types_update ON document_types FOR UPDATE
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid OR current_setting('app.role', true) = 'super_admin')
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::uuid OR current_setting('app.role', true) = 'super_admin');

CREATE POLICY document_types_delete ON document_types FOR DELETE
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid OR current_setting('app.role', true) = 'super_admin');

-- Update schema version
INSERT INTO schema_version (version, description) VALUES (6, 'Enhanced service_types and document_types tables');
