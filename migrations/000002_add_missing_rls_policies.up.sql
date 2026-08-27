-- Migration: Add missing RLS policies for 23 tables
-- Issue: RLS enabled but no policies = blocks ALL access
-- Solution: Add tenant_isolation policies matching existing pattern

-- =============================================================================
-- TENANT-SCOPED TABLES (22 tables with tenant_id column)
-- =============================================================================

-- ai_jobs
CREATE POLICY tenant_isolation_ai_jobs ON ai_jobs FOR ALL
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid OR current_setting('app.role', true) = 'super_admin')
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::uuid OR current_setting('app.role', true) = 'super_admin');

-- chase_log_clients
CREATE POLICY tenant_isolation_chase_log_clients ON chase_log_clients FOR ALL
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid OR current_setting('app.role', true) = 'super_admin')
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::uuid OR current_setting('app.role', true) = 'super_admin');

-- chase_logs
CREATE POLICY tenant_isolation_chase_logs ON chase_logs FOR ALL
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid OR current_setting('app.role', true) = 'super_admin')
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::uuid OR current_setting('app.role', true) = 'super_admin');

-- client_notes
CREATE POLICY tenant_isolation_client_notes ON client_notes FOR ALL
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid OR current_setting('app.role', true) = 'super_admin')
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::uuid OR current_setting('app.role', true) = 'super_admin');

-- company_settings
CREATE POLICY tenant_isolation_company_settings ON company_settings FOR ALL
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid OR current_setting('app.role', true) = 'super_admin')
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::uuid OR current_setting('app.role', true) = 'super_admin');

-- directors
CREATE POLICY tenant_isolation_directors ON directors FOR ALL
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid OR current_setting('app.role', true) = 'super_admin')
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::uuid OR current_setting('app.role', true) = 'super_admin');

-- document_access
CREATE POLICY tenant_isolation_document_access ON document_access FOR ALL
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid OR current_setting('app.role', true) = 'super_admin')
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::uuid OR current_setting('app.role', true) = 'super_admin');

-- e_sign_requests
CREATE POLICY tenant_isolation_e_sign_requests ON e_sign_requests FOR ALL
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid OR current_setting('app.role', true) = 'super_admin')
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::uuid OR current_setting('app.role', true) = 'super_admin');

-- email_accounts
CREATE POLICY tenant_isolation_email_accounts ON email_accounts FOR ALL
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid OR current_setting('app.role', true) = 'super_admin')
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::uuid OR current_setting('app.role', true) = 'super_admin');

-- email_threads
CREATE POLICY tenant_isolation_email_threads ON email_threads FOR ALL
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid OR current_setting('app.role', true) = 'super_admin')
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::uuid OR current_setting('app.role', true) = 'super_admin');

-- magic_link_tokens
CREATE POLICY tenant_isolation_magic_link_tokens ON magic_link_tokens FOR ALL
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid OR current_setting('app.role', true) = 'super_admin')
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::uuid OR current_setting('app.role', true) = 'super_admin');

-- outbox
CREATE POLICY tenant_isolation_outbox ON outbox FOR ALL
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid OR current_setting('app.role', true) = 'super_admin')
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::uuid OR current_setting('app.role', true) = 'super_admin');

-- psc (Persons with Significant Control)
CREATE POLICY tenant_isolation_psc ON psc FOR ALL
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid OR current_setting('app.role', true) = 'super_admin')
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::uuid OR current_setting('app.role', true) = 'super_admin');

-- push_tokens
CREATE POLICY tenant_isolation_push_tokens ON push_tokens FOR ALL
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid OR current_setting('app.role', true) = 'super_admin')
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::uuid OR current_setting('app.role', true) = 'super_admin');

-- refresh_tokens
CREATE POLICY tenant_isolation_refresh_tokens ON refresh_tokens FOR ALL
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid OR current_setting('app.role', true) = 'super_admin')
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::uuid OR current_setting('app.role', true) = 'super_admin');

-- reminders
CREATE POLICY tenant_isolation_reminders ON reminders FOR ALL
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid OR current_setting('app.role', true) = 'super_admin')
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::uuid OR current_setting('app.role', true) = 'super_admin');

-- service_requirements
CREATE POLICY tenant_isolation_service_requirements ON service_requirements FOR ALL
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid OR current_setting('app.role', true) = 'super_admin')
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::uuid OR current_setting('app.role', true) = 'super_admin');

-- sessions
CREATE POLICY tenant_isolation_sessions ON sessions FOR ALL
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid OR current_setting('app.role', true) = 'super_admin')
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::uuid OR current_setting('app.role', true) = 'super_admin');

-- staff_clients
CREATE POLICY tenant_isolation_staff_clients ON staff_clients FOR ALL
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid OR current_setting('app.role', true) = 'super_admin')
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::uuid OR current_setting('app.role', true) = 'super_admin');

-- tenant_invoices
CREATE POLICY tenant_isolation_tenant_invoices ON tenant_invoices FOR ALL
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid OR current_setting('app.role', true) = 'super_admin')
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::uuid OR current_setting('app.role', true) = 'super_admin');

-- tenant_subscriptions
CREATE POLICY tenant_isolation_tenant_subscriptions ON tenant_subscriptions FOR ALL
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid OR current_setting('app.role', true) = 'super_admin')
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::uuid OR current_setting('app.role', true) = 'super_admin');

-- upload_tokens
CREATE POLICY tenant_isolation_upload_tokens ON upload_tokens FOR ALL
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid OR current_setting('app.role', true) = 'super_admin')
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::uuid OR current_setting('app.role', true) = 'super_admin');

-- =============================================================================
-- USER-SCOPED TABLE (1 table with user_id instead of tenant_id)
-- =============================================================================

-- totp_backup_codes (user-specific, isolated by user_id)
CREATE POLICY user_isolation_totp_backup_codes ON totp_backup_codes FOR ALL
    USING (user_id = current_setting('app.user_id', true)::uuid OR current_setting('app.role', true) = 'super_admin')
    WITH CHECK (user_id = current_setting('app.user_id', true)::uuid OR current_setting('app.role', true) = 'super_admin');

-- =============================================================================
-- Update schema version
-- =============================================================================
UPDATE schema_version SET version = 2, applied_at = CURRENT_TIMESTAMP WHERE version = 1;
