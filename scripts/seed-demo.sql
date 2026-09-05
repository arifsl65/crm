-- =============================================================================
-- Demo Seed Data - Accountant CRM (Enhanced for E2E Testing)
-- =============================================================================
-- PURPOSE: Complete test data for Today Dashboard E2E flows
-- TABLES: 16 tables (Full Dashboard + Email + Chase + Notifications)
-- USAGE: psql $DATABASE_URL -f scripts/seed-demo.sql
-- VERSION: 2.0 - Enhanced with AI summaries, client users, chase tracking
-- =============================================================================

-- Bypass RLS for seeding (set super_admin role context)
SET app.role = 'super_admin';
SET app.tenant_id = 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11';

-- Strict environment check
DO $$
DECLARE
    current_env TEXT;
BEGIN
    current_env := current_setting('app.env', true);
    IF current_env IS NULL OR current_env = '' THEN
        RAISE EXCEPTION 'app.env not set. Run: SET app.env = ''development'';';
    END IF;
    IF current_env NOT IN ('development', 'test') THEN
        RAISE EXCEPTION 'Refusing to seed: app.env = ''%'' (must be development or test)', current_env;
    END IF;
    RAISE NOTICE 'Environment check passed: %', current_env;
END $$;

-- =============================================================================
-- CLEANUP: Remove existing demo data (idempotent)
-- =============================================================================
DO $$
BEGIN
    -- Delete in FK-safe order (new tables first)
    -- Clean BOTH old tenant (a0000000...) and new tenant (a0eebc99...)
    DELETE FROM ai_jobs WHERE tenant_id IN ('a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'a0000000-0000-0000-0000-000000000001');
    DELETE FROM tenant_invoices WHERE tenant_id IN ('a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'a0000000-0000-0000-0000-000000000001');
    DELETE FROM tenant_subscriptions WHERE tenant_id IN ('a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'a0000000-0000-0000-0000-000000000001');
    DELETE FROM e_sign_requests WHERE tenant_id IN ('a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'a0000000-0000-0000-0000-000000000001');
    DELETE FROM reminders WHERE tenant_id IN ('a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'a0000000-0000-0000-0000-000000000001');
    DELETE FROM notifications WHERE tenant_id IN ('a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'a0000000-0000-0000-0000-000000000001');
    DELETE FROM chase_log_clients WHERE tenant_id IN ('a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'a0000000-0000-0000-0000-000000000001');
    DELETE FROM chase_logs WHERE tenant_id IN ('a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'a0000000-0000-0000-0000-000000000001');
    DELETE FROM email_threads WHERE tenant_id IN ('a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'a0000000-0000-0000-0000-000000000001');
    DELETE FROM emails WHERE tenant_id IN ('a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'a0000000-0000-0000-0000-000000000001');
    DELETE FROM email_accounts WHERE tenant_id IN ('a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'a0000000-0000-0000-0000-000000000001');
    DELETE FROM email_templates WHERE tenant_id IN ('a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'a0000000-0000-0000-0000-000000000001');
    DELETE FROM audit_logs WHERE tenant_id IN ('a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'a0000000-0000-0000-0000-000000000001');
    DELETE FROM staff_clients WHERE tenant_id IN ('a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'a0000000-0000-0000-0000-000000000001');
    DELETE FROM document_access WHERE tenant_id IN ('a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'a0000000-0000-0000-0000-000000000001');
    DELETE FROM documents WHERE tenant_id IN ('a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'a0000000-0000-0000-0000-000000000001');
    DELETE FROM services WHERE tenant_id IN ('a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'a0000000-0000-0000-0000-000000000001');
    DELETE FROM directors WHERE tenant_id IN ('a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'a0000000-0000-0000-0000-000000000001');
    DELETE FROM psc WHERE tenant_id IN ('a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'a0000000-0000-0000-0000-000000000001');
    DELETE FROM client_notes WHERE tenant_id IN ('a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'a0000000-0000-0000-0000-000000000001');
    DELETE FROM clients WHERE tenant_id IN ('a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'a0000000-0000-0000-0000-000000000001');
    DELETE FROM users WHERE tenant_id IN ('a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'a0000000-0000-0000-0000-000000000001');
    DELETE FROM service_requirements WHERE tenant_id IN ('a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'a0000000-0000-0000-0000-000000000001');
    DELETE FROM service_types WHERE tenant_id IN ('a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'a0000000-0000-0000-0000-000000000001');
    DELETE FROM document_types WHERE tenant_id IN ('a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'a0000000-0000-0000-0000-000000000001');
    DELETE FROM company_settings WHERE tenant_id IN ('a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'a0000000-0000-0000-0000-000000000001');
    DELETE FROM tenants WHERE id IN ('a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'a0000000-0000-0000-0000-000000000001');
    RAISE NOTICE 'Cleanup complete';
END $$;

-- =============================================================================
-- 1. TENANT
-- =============================================================================
INSERT INTO tenants (id, name, domain, plan, timezone, is_active) VALUES
('a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'Demo Accounting Firm', 'demo', 'enterprise', 'Europe/London', true);

-- =============================================================================
-- 2. USERS (1 Admin + 7 Staff + 4 Client Portal Users = 12 total)
-- =============================================================================
-- Password: Test123! (Argon2id hashed)
INSERT INTO users (id, tenant_id, name, email, password, role, status, specialism) VALUES
-- Admin (Test Admin from spec)
('b0000000-0000-0000-0000-000000000001', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'Test Admin', 'admin@test.com',
 '$argon2id$v=19$m=65536,t=3,p=4$ydTMvT8NgPucduh0wvcs6Q$rrEU5E5jQXcWQVYknzUiUB7Kz4kQND/4Kja5r9Pu+H4',
 'tenant_admin', 'active', NULL),
-- Staff
('b0000000-0000-0000-0000-000000000002', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'Sarah Johnson', 'sarah@demo.local',
 '$argon2id$v=19$m=65536,t=3,p=4$ydTMvT8NgPucduh0wvcs6Q$rrEU5E5jQXcWQVYknzUiUB7Kz4kQND/4Kja5r9Pu+H4',
 'staff', 'active', 'Tax'),
('b0000000-0000-0000-0000-000000000003', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'Mike Chen', 'mike@demo.local',
 '$argon2id$v=19$m=65536,t=3,p=4$ydTMvT8NgPucduh0wvcs6Q$rrEU5E5jQXcWQVYknzUiUB7Kz4kQND/4Kja5r9Pu+H4',
 'staff', 'active', 'VAT'),
('b0000000-0000-0000-0000-000000000004', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'Emma Wilson', 'emma@demo.local',
 '$argon2id$v=19$m=65536,t=3,p=4$ydTMvT8NgPucduh0wvcs6Q$rrEU5E5jQXcWQVYknzUiUB7Kz4kQND/4Kja5r9Pu+H4',
 'staff', 'active', 'Bookkeeping'),
('b0000000-0000-0000-0000-000000000005', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'James Brown', 'james@demo.local',
 '$argon2id$v=19$m=65536,t=3,p=4$ydTMvT8NgPucduh0wvcs6Q$rrEU5E5jQXcWQVYknzUiUB7Kz4kQND/4Kja5r9Pu+H4',
 'staff', 'active', 'Payroll'),
('b0000000-0000-0000-0000-000000000006', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'Lisa Taylor', 'lisa@demo.local',
 '$argon2id$v=19$m=65536,t=3,p=4$ydTMvT8NgPucduh0wvcs6Q$rrEU5E5jQXcWQVYknzUiUB7Kz4kQND/4Kja5r9Pu+H4',
 'staff', 'active', 'Accounts'),
('b0000000-0000-0000-0000-000000000007', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'David Lee', 'david@demo.local',
 '$argon2id$v=19$m=65536,t=3,p=4$ydTMvT8NgPucduh0wvcs6Q$rrEU5E5jQXcWQVYknzUiUB7Kz4kQND/4Kja5r9Pu+H4',
 'staff', 'active', 'Self Assessment'),
('b0000000-0000-0000-0000-000000000008', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'Rachel Green', 'rachel@demo.local',
 '$argon2id$v=19$m=65536,t=3,p=4$ydTMvT8NgPucduh0wvcs6Q$rrEU5E5jQXcWQVYknzUiUB7Kz4kQND/4Kja5r9Pu+H4',
 'staff', 'active', 'Corporation Tax'),

-- CLIENT PORTAL USERS (for "Uploaded by client" in document review)
('b0000000-0000-0000-0000-000000000101', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'John Smith', 'john@acme.co.uk',
 '$argon2id$v=19$m=65536,t=3,p=4$ydTMvT8NgPucduh0wvcs6Q$rrEU5E5jQXcWQVYknzUiUB7Kz4kQND/4Kja5r9Pu+H4',
 'client', 'active', NULL),
('b0000000-0000-0000-0000-000000000102', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'Peter Smith', 'peter@smithco.co.uk',
 '$argon2id$v=19$m=65536,t=3,p=4$ydTMvT8NgPucduh0wvcs6Q$rrEU5E5jQXcWQVYknzUiUB7Kz4kQND/4Kja5r9Pu+H4',
 'client', 'active', NULL),
('b0000000-0000-0000-0000-000000000103', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'Tom Jones', 'tom@jonestrading.co.uk',
 '$argon2id$v=19$m=65536,t=3,p=4$ydTMvT8NgPucduh0wvcs6Q$rrEU5E5jQXcWQVYknzUiUB7Kz4kQND/4Kja5r9Pu+H4',
 'client', 'active', NULL),
('b0000000-0000-0000-0000-000000000104', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'Alice Brown', 'alice@techstart.io',
 '$argon2id$v=19$m=65536,t=3,p=4$ydTMvT8NgPucduh0wvcs6Q$rrEU5E5jQXcWQVYknzUiUB7Kz4kQND/4Kja5r9Pu+H4',
 'client', 'active', NULL);

-- =============================================================================
-- 3. SERVICE TYPES (6 types)
-- =============================================================================
INSERT INTO service_types (id, tenant_id, name, description, deadline_pattern, deadline_offset, is_active) VALUES
('c0000000-0000-0000-0000-000000000001', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'VAT Return', 'Quarterly VAT filing', 'quarterly', 30, true),
('c0000000-0000-0000-0000-000000000002', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'CT600 Filing', 'Corporation Tax return', 'annual', 270, true),
('c0000000-0000-0000-0000-000000000003', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'Self Assessment', 'Personal tax return', 'annual', 300, true),
('c0000000-0000-0000-0000-000000000004', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'Payroll', 'Monthly payroll processing', 'monthly', 5, true),
('c0000000-0000-0000-0000-000000000005', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'Annual Accounts', 'Year-end accounts preparation', 'annual', 270, true),
('c0000000-0000-0000-0000-000000000006', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'ID Verification', 'Client identity verification', 'custom', NULL, true);

-- =============================================================================
-- 4. DOCUMENT TYPES (10 types - expanded)
-- =============================================================================
INSERT INTO document_types (id, tenant_id, name, description, has_expiry, is_active) VALUES
('d0000000-0000-0000-0000-000000000001', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'Bank Statement', 'Monthly bank statements', false, true),
('d0000000-0000-0000-0000-000000000002', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'Invoice', 'Sales or purchase invoices', false, true),
('d0000000-0000-0000-0000-000000000003', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'Receipt', 'Expense receipts', false, true),
('d0000000-0000-0000-0000-000000000004', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'ID Document', 'Government-issued ID', true, true),
('d0000000-0000-0000-0000-000000000005', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'Passport', 'Passport copy', true, true),
('d0000000-0000-0000-0000-000000000006', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'P60', 'End of year certificate', false, true),
('d0000000-0000-0000-0000-000000000007', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'UTR Letter', 'Unique Taxpayer Reference', false, true),
('d0000000-0000-0000-0000-000000000008', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'VAT Certificate', 'VAT registration certificate', true, true),
('d0000000-0000-0000-0000-000000000009', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'Utility Bill', 'Proof of address', false, true),
('d0000000-0000-0000-0000-000000000010', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'Dividend Voucher', 'Dividend payment records', false, true);

-- =============================================================================
-- 5. CLIENTS (17 clients - added Smith & Co Ltd and Jones Trading Ltd for spec)
-- =============================================================================
INSERT INTO clients (id, tenant_id, user_id, company_name, contact_name, email, phone, status, last_contact_at, company_number, vat_number) VALUES
-- MAIN CLIENT: Acme Corporation Ltd (used in Today Dashboard spec)
('e0000000-0000-0000-0000-000000000001', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'b0000000-0000-0000-0000-000000000002',
 'Acme Corporation Ltd', 'John Smith', 'john@acme.co.uk', '020 1234 5678',
 'active', NOW() - INTERVAL '2 days', '12345678', 'GB123456789'),

-- NEW: Smith & Co Ltd (for document review panel in spec)
('e0000000-0000-0000-0000-000000000016', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'b0000000-0000-0000-0000-000000000002',
 'Smith & Co Ltd', 'Peter Smith', 'peter@smithco.co.uk', '020 9876 5432',
 'active', NOW() - INTERVAL '1 day', '98765432', 'GB987654321'),

-- NEW: Jones Trading Ltd (for document review panel in spec)
('e0000000-0000-0000-0000-000000000017', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'b0000000-0000-0000-0000-000000000002',
 'Jones Trading Ltd', 'Tom Jones', 'tom@jonestrading.co.uk', '020 5555 1234',
 'active', NOW() - INTERVAL '3 days', '55551234', NULL),

-- Other clients
('e0000000-0000-0000-0000-000000000002', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'b0000000-0000-0000-0000-000000000002',
 'TechStart Solutions', 'Alice Brown', 'alice@techstart.io', '020 2345 6789',
 'active', NOW() - INTERVAL '1 day', '23456789', 'GB234567890'),
('e0000000-0000-0000-0000-000000000003', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'b0000000-0000-0000-0000-000000000002',
 'Green Energy Co', 'Bob Wilson', 'bob@greenenergy.com', '020 3456 7890',
 'active', NOW() - INTERVAL '3 days', '34567890', 'GB345678901'),

-- QUIET CLIENTS: Active but no contact for 14+ days
('e0000000-0000-0000-0000-000000000004', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'b0000000-0000-0000-0000-000000000002',
 'Silent Trading Ltd', 'Carol Davis', 'carol@silenttrading.co.uk', '020 4567 8901',
 'active', NOW() - INTERVAL '20 days', '45678901', NULL),
('e0000000-0000-0000-0000-000000000005', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'b0000000-0000-0000-0000-000000000002',
 'Dormant Holdings', 'Dan Evans', 'dan@dormant.co.uk', '020 5678 9012',
 'active', NOW() - INTERVAL '25 days', '56789012', NULL),
('e0000000-0000-0000-0000-000000000006', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'b0000000-0000-0000-0000-000000000002',
 'Ghost Services Ltd', 'Eve Foster', 'eve@ghost.co.uk', '020 6789 0123',
 'active', NOW() - INTERVAL '18 days', '67890123', 'GB456789012'),

-- AT RISK: Overdue services
('e0000000-0000-0000-0000-000000000007', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'b0000000-0000-0000-0000-000000000003',
 'Late Payers Inc', 'Frank Green', 'frank@latepayers.com', '020 7890 1234',
 'active', NOW() - INTERVAL '5 days', '78901234', 'GB567890123'),
('e0000000-0000-0000-0000-000000000008', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'b0000000-0000-0000-0000-000000000003',
 'Overdue Enterprises', 'Grace Hill', 'grace@overdue.co.uk', '020 8901 2345',
 'active', NOW() - INTERVAL '4 days', '89012345', 'GB678901234'),

-- AT RISK: Missing documents
('e0000000-0000-0000-0000-000000000009', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'b0000000-0000-0000-0000-000000000004',
 'Missing Docs Ltd', 'Henry Irving', 'henry@missingdocs.com', '020 9012 3456',
 'active', NOW() - INTERVAL '6 days', '90123456', NULL),
('e0000000-0000-0000-0000-000000000010', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'b0000000-0000-0000-0000-000000000004',
 'Incomplete Records Co', 'Ivy Jones', 'ivy@incomplete.co.uk', '020 0123 4567',
 'active', NOW() - INTERVAL '3 days', '01234567', 'GB789012345'),

-- Pending clients
('e0000000-0000-0000-0000-000000000011', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'b0000000-0000-0000-0000-000000000005',
 'Pending Paperwork Ltd', 'Jack King', 'jack@pending.co.uk', '020 1234 5670',
 'active', NOW() - INTERVAL '2 days', '12345670', NULL),
('e0000000-0000-0000-0000-000000000012', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'b0000000-0000-0000-0000-000000000005',
 'Document Chase Ltd', 'Kate Lewis', 'kate@docchase.com', '020 2345 6780',
 'active', NOW() - INTERVAL '4 days', '23456780', 'GB890123456'),

-- Inactive client
('e0000000-0000-0000-0000-000000000013', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'b0000000-0000-0000-0000-000000000006',
 'Closed Business Co', 'Leo Martin', 'leo@closed.co.uk', '020 3456 7891',
 'inactive', NOW() - INTERVAL '60 days', '34567891', NULL),

-- Normal active clients
('e0000000-0000-0000-0000-000000000014', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'b0000000-0000-0000-0000-000000000007',
 'Healthy Accounts Ltd', 'Mary Newton', 'mary@healthy.co.uk', '020 4567 8902',
 'active', NOW() - INTERVAL '1 day', '45678902', 'GB901234567'),
('e0000000-0000-0000-0000-000000000015', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'b0000000-0000-0000-0000-000000000008',
 'On Track Services', 'Nick Owen', 'nick@ontrack.co.uk', '020 5678 9013',
 'active', NOW() - INTERVAL '2 days', '56789013', 'GB012345678');

-- =============================================================================
-- 6. SERVICES (28 services - enhanced for Today Dashboard E2E spec)
-- =============================================================================
INSERT INTO services (id, tenant_id, client_id, staff_id, type_id, name, period, status, priority, deadline, docs_required, docs_received) VALUES

-- ============================================================================
-- DO FIRST: OVERDUE SERVICES (matches spec exactly)
-- ============================================================================
-- VAT Return Q2 - Acme Corporation Ltd - 3 days overdue (spec: "3 days overdue")
('f0000000-0000-0000-0000-000000000001', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'e0000000-0000-0000-0000-000000000001', 'b0000000-0000-0000-0000-000000000001',
 'c0000000-0000-0000-0000-000000000001', 'VAT Return Q2', 'Q2 2024',
 'in_progress', 'urgent', CURRENT_DATE - INTERVAL '3 days', 5, 0),

-- CT600 Filing - Acme Corporation Ltd - 2 days overdue (spec: "2 days overdue")
('f0000000-0000-0000-0000-000000000002', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'e0000000-0000-0000-0000-000000000001', 'b0000000-0000-0000-0000-000000000001',
 'c0000000-0000-0000-0000-000000000002', 'CT600 Filing', '2023-24',
 'in_progress', 'high', CURRENT_DATE - INTERVAL '2 days', 2, 0),

-- Payroll Submission - Acme Corporation Ltd - 1 day overdue (spec: "1 days overdue")
('f0000000-0000-0000-0000-000000000003', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'e0000000-0000-0000-0000-000000000001', 'b0000000-0000-0000-0000-000000000001',
 'c0000000-0000-0000-0000-000000000004', 'Payroll Submission', 'Sep 2024',
 'in_progress', 'high', CURRENT_DATE - INTERVAL '1 day', 2, 0),

-- ============================================================================
-- LATER TODAY: Due today, not overdue yet (spec: "Self Assessment Review")
-- ============================================================================
('f0000000-0000-0000-0000-000000000004', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'e0000000-0000-0000-0000-000000000001', 'b0000000-0000-0000-0000-000000000001',
 'c0000000-0000-0000-0000-000000000003', 'Self Assessment Review', '2023-24',
 'in_progress', 'normal', CURRENT_DATE, 6, 0),

-- ============================================================================
-- Additional services for other dashboard panels
-- ============================================================================
-- DUE TOMORROW (2)
('f0000000-0000-0000-0000-000000000006', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'e0000000-0000-0000-0000-000000000003', 'b0000000-0000-0000-0000-000000000007',
 'c0000000-0000-0000-0000-000000000001', 'VAT Return Q3', 'Q3 2024',
 'in_progress', 'normal', CURRENT_DATE + INTERVAL '1 day', 3, 0),
('f0000000-0000-0000-0000-000000000007', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'e0000000-0000-0000-0000-000000000014', 'b0000000-0000-0000-0000-000000000008',
 'c0000000-0000-0000-0000-000000000004', 'Payroll Processing', 'Sep 2024',
 'not_started', 'normal', CURRENT_DATE + INTERVAL '1 day', 2, 0),

-- THIS WEEK (4)
('f0000000-0000-0000-0000-000000000008', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'e0000000-0000-0000-0000-000000000010', 'b0000000-0000-0000-0000-000000000002',
 'c0000000-0000-0000-0000-000000000005', 'Annual Accounts', '2023-24',
 'in_progress', 'normal', CURRENT_DATE + INTERVAL '3 days', 5, 0),
('f0000000-0000-0000-0000-000000000009', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'e0000000-0000-0000-0000-000000000011', 'b0000000-0000-0000-0000-000000000003',
 'c0000000-0000-0000-0000-000000000001', 'VAT Return Q3', 'Q3 2024',
 'not_started', 'normal', CURRENT_DATE + INTERVAL '4 days', 3, 0),
('f0000000-0000-0000-0000-000000000010', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'e0000000-0000-0000-0000-000000000012', 'b0000000-0000-0000-0000-000000000004',
 'c0000000-0000-0000-0000-000000000003', 'Self Assessment', '2023-24',
 'in_progress', 'low', CURRENT_DATE + INTERVAL '5 days', 4, 0),
('f0000000-0000-0000-0000-000000000011', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'e0000000-0000-0000-0000-000000000015', 'b0000000-0000-0000-0000-000000000005',
 'c0000000-0000-0000-0000-000000000006', 'ID Verification', NULL,
 'not_started', 'normal', CURRENT_DATE + INTERVAL '6 days', 2, 0),

-- NEXT 30 DAYS (6)
('f0000000-0000-0000-0000-000000000012', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'e0000000-0000-0000-0000-000000000001', 'b0000000-0000-0000-0000-000000000006',
 'c0000000-0000-0000-0000-000000000001', 'VAT Return Q4', 'Q4 2024',
 'not_started', 'normal', CURRENT_DATE + INTERVAL '12 days', 3, 0),
('f0000000-0000-0000-0000-000000000013', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'e0000000-0000-0000-0000-000000000002', 'b0000000-0000-0000-0000-000000000007',
 'c0000000-0000-0000-0000-000000000002', 'CT600 Preparation', '2024-25',
 'not_started', 'low', CURRENT_DATE + INTERVAL '18 days', 5, 0),
('f0000000-0000-0000-0000-000000000014', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'e0000000-0000-0000-0000-000000000003', 'b0000000-0000-0000-0000-000000000008',
 'c0000000-0000-0000-0000-000000000005', 'Annual Accounts', '2024-25',
 'not_started', 'normal', CURRENT_DATE + INTERVAL '22 days', 6, 0),
('f0000000-0000-0000-0000-000000000015', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'e0000000-0000-0000-0000-000000000004', 'b0000000-0000-0000-0000-000000000002',
 'c0000000-0000-0000-0000-000000000004', 'Payroll Setup', 'Oct 2024',
 'not_started', 'low', CURRENT_DATE + INTERVAL '25 days', 2, 0),
('f0000000-0000-0000-0000-000000000016', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'e0000000-0000-0000-0000-000000000005', 'b0000000-0000-0000-0000-000000000003',
 'c0000000-0000-0000-0000-000000000003', 'Self Assessment', '2024-25',
 'not_started', 'normal', CURRENT_DATE + INTERVAL '28 days', 4, 0),
('f0000000-0000-0000-0000-000000000017', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'e0000000-0000-0000-0000-000000000006', 'b0000000-0000-0000-0000-000000000004',
 'c0000000-0000-0000-0000-000000000001', 'VAT Registration', NULL,
 'not_started', 'low', CURRENT_DATE + INTERVAL '30 days', 3, 0),

-- BEYOND 30 DAYS (4)
('f0000000-0000-0000-0000-000000000018', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'e0000000-0000-0000-0000-000000000014', 'b0000000-0000-0000-0000-000000000005',
 'c0000000-0000-0000-0000-000000000002', 'CT600 Filing', '2024-25',
 'not_started', 'low', CURRENT_DATE + INTERVAL '45 days', 5, 0),
('f0000000-0000-0000-0000-000000000019', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'e0000000-0000-0000-0000-000000000015', 'b0000000-0000-0000-0000-000000000006',
 'c0000000-0000-0000-0000-000000000005', 'Annual Accounts', '2024-25',
 'not_started', 'low', CURRENT_DATE + INTERVAL '50 days', 6, 0),
('f0000000-0000-0000-0000-000000000020', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'e0000000-0000-0000-0000-000000000001', 'b0000000-0000-0000-0000-000000000007',
 'c0000000-0000-0000-0000-000000000003', 'Self Assessment', '2024-25',
 'not_started', 'low', CURRENT_DATE + INTERVAL '55 days', 4, 0),
('f0000000-0000-0000-0000-000000000021', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'e0000000-0000-0000-0000-000000000002', 'b0000000-0000-0000-0000-000000000008',
 'c0000000-0000-0000-0000-000000000001', 'VAT Return Q1 2025', 'Q1 2025',
 'not_started', 'low', CURRENT_DATE + INTERVAL '60 days', 3, 0),

-- COMPLETED (4)
('f0000000-0000-0000-0000-000000000022', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'e0000000-0000-0000-0000-000000000001', 'b0000000-0000-0000-0000-000000000002',
 'c0000000-0000-0000-0000-000000000001', 'VAT Return Q1', 'Q1 2024',
 'completed', 'normal', CURRENT_DATE - INTERVAL '90 days', 3, 0),
('f0000000-0000-0000-0000-000000000023', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'e0000000-0000-0000-0000-000000000003', 'b0000000-0000-0000-0000-000000000003',
 'c0000000-0000-0000-0000-000000000002', 'CT600 Filing', '2022-23',
 'completed', 'normal', CURRENT_DATE - INTERVAL '120 days', 4, 0),
('f0000000-0000-0000-0000-000000000024', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'e0000000-0000-0000-0000-000000000014', 'b0000000-0000-0000-0000-000000000004',
 'c0000000-0000-0000-0000-000000000005', 'Annual Accounts', '2022-23',
 'completed', 'normal', CURRENT_DATE - INTERVAL '60 days', 5, 0),
('f0000000-0000-0000-0000-000000000025', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'e0000000-0000-0000-0000-000000000015', 'b0000000-0000-0000-0000-000000000005',
 'c0000000-0000-0000-0000-000000000004', 'Payroll', 'Aug 2024',
 'completed', 'normal', CURRENT_DATE - INTERVAL '30 days', 2, 0);

-- =============================================================================
-- 7. DOCUMENTS (26 documents - with AI summaries matching spec exactly)
-- =============================================================================
INSERT INTO documents (id, tenant_id, client_id, service_id, uploaded_by, type_id, name, original_name, status, file_size, mime_type, ai_summary, created_at) VALUES

-- ============================================================================
-- PENDING REVIEW (4): Matches E2E spec document review panel exactly
-- ============================================================================
-- Doc 1: Bank_Statement_Q2.pdf - Acme - 10 min ago
('aa000000-0000-0000-0000-000000000001', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'e0000000-0000-0000-0000-000000000001', 'f0000000-0000-0000-0000-000000000004',
 'b0000000-0000-0000-0000-000000000101', 'd0000000-0000-0000-0000-000000000001',
 'Bank_Statement_Q2.pdf', 'Bank_Statement_Q2.pdf', 'pending_review', 125000, 'application/pdf',
 'Q2 2024 bank statement (Apr-Jun). 45 transactions, net inflow £6,270. Largest: £5,000 payment to HMRC (VAT). Recurring: Monthly rent £1,200, utilities £150. ✓ Matches expected document type: Bank Statement',
 NOW() - INTERVAL '10 minutes'),

-- Doc 2: VAT_Receipts_June.pdf - Acme - 2 hours ago
('aa000000-0000-0000-0000-000000000002', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'e0000000-0000-0000-0000-000000000001', 'f0000000-0000-0000-0000-000000000001',
 'b0000000-0000-0000-0000-000000000101', 'd0000000-0000-0000-0000-000000000003',
 'VAT_Receipts_June.pdf', 'VAT_Receipts_June.pdf', 'pending_review', 340000, 'application/pdf',
 '12 receipts, total £4,250. Includes office supplies (£850), travel expenses (£1,200), client entertainment (£400), software subscriptions (£1,800). All receipts legible with dates.',
 NOW() - INTERVAL '2 hours'),

-- Doc 3: ID_Proof_Passport.jpg - Smith & Co Ltd - yesterday
('aa000000-0000-0000-0000-000000000003', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'e0000000-0000-0000-0000-000000000016', NULL,
 'b0000000-0000-0000-0000-000000000102', 'd0000000-0000-0000-0000-000000000005',
 'ID_Proof_Passport.jpg', 'ID_Proof_Passport.jpg', 'pending_review', 250000, 'image/jpeg',
 'UK passport, expires 2028. Name matches client record. Photo clearly visible. Document number: 123456789. Issue date: 2018.',
 NOW() - INTERVAL '1 day'),

-- Doc 4: Utility_Bill.pdf - Jones Trading Ltd - 2 days ago
('aa000000-0000-0000-0000-000000000004', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'e0000000-0000-0000-0000-000000000017', NULL,
 'b0000000-0000-0000-0000-000000000103', 'd0000000-0000-0000-0000-000000000009',
 'Utility_Bill.pdf', 'Utility_Bill.pdf', 'pending_review', 89000, 'application/pdf',
 'Electric bill, address verified. British Gas statement dated within last 3 months. Address: 45 High Street, London EC1A 1BB. Amount: £156.40.',
 NOW() - INTERVAL '2 days'),

-- ============================================================================
-- APPROVED (6): For Service Detail panel
-- ============================================================================
('aa000000-0000-0000-0000-000000000005', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'e0000000-0000-0000-0000-000000000001', 'f0000000-0000-0000-0000-000000000004',
 'b0000000-0000-0000-0000-000000000101', 'd0000000-0000-0000-0000-000000000006',
 'P60_2023-24.pdf', 'P60_2023-24.pdf', 'approved', 45000, 'application/pdf',
 'P60 for tax year 2023-24. Employer: Acme Corporation Ltd. Total pay: £45,000. Tax paid: £7,500. NI contributions: £4,200.',
 NOW() - INTERVAL '5 days'),

('aa000000-0000-0000-0000-000000000006', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'e0000000-0000-0000-0000-000000000001', 'f0000000-0000-0000-0000-000000000004',
 'b0000000-0000-0000-0000-000000000101', 'd0000000-0000-0000-0000-000000000001',
 'Bank_Interest_Statement.pdf', 'Bank_Interest_Statement.pdf', 'approved', 32000, 'application/pdf',
 'Annual interest statement. Total interest earned: £245.67. Account: Savings account ending 4521.',
 NOW() - INTERVAL '6 days'),

('aa000000-0000-0000-0000-000000000007', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'e0000000-0000-0000-0000-000000000003', 'f0000000-0000-0000-0000-000000000023',
 'b0000000-0000-0000-0000-000000000104', 'd0000000-0000-0000-0000-000000000002',
 'Invoice_Pack_2023.pdf', 'Invoice_Pack_2023.pdf', 'approved', 420000, 'application/pdf',
 '156 invoices for 2023. Total revenue: £284,500. All invoices numbered sequentially.',
 NOW() - INTERVAL '10 days'),

('aa000000-0000-0000-0000-000000000008', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'e0000000-0000-0000-0000-000000000015', 'f0000000-0000-0000-0000-000000000025',
 'b0000000-0000-0000-0000-000000000001', 'd0000000-0000-0000-0000-000000000001',
 'Bank_Statement_Jul_2024.pdf', 'Bank_Statement_Jul_2024.pdf', 'approved', 98000, 'application/pdf',
 'July 2024 statement. 28 transactions. Opening: £15,200. Closing: £18,450.',
 NOW() - INTERVAL '15 days'),

('aa000000-0000-0000-0000-000000000009', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'e0000000-0000-0000-0000-000000000002', NULL,
 'b0000000-0000-0000-0000-000000000104', 'd0000000-0000-0000-0000-000000000005',
 'Passport_Copy.jpg', 'Passport_Copy.jpg', 'approved', 280000, 'image/jpeg',
 'UK passport verified. Expires 2029. Photo matches records.',
 NOW() - INTERVAL '20 days'),

('aa000000-0000-0000-0000-000000000010', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'e0000000-0000-0000-0000-000000000003', NULL,
 'b0000000-0000-0000-0000-000000000001', 'd0000000-0000-0000-0000-000000000004',
 'Driving_License.jpg', 'Driving_License.jpg', 'approved', 180000, 'image/jpeg',
 'UK driving license. Valid until 2030. Address verified.',
 NOW() - INTERVAL '25 days'),

-- ============================================================================
-- REQUESTED (8): Missing documents for chase flow - linked to overdue services
-- ============================================================================
-- For VAT Return Q2 (service f001) - 2 missing docs
('aa000000-0000-0000-0000-000000000011', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'e0000000-0000-0000-0000-000000000001', 'f0000000-0000-0000-0000-000000000001',
 'b0000000-0000-0000-0000-000000000001', 'd0000000-0000-0000-0000-000000000001',
 'Bank Statement (Apr-Jun 2024)', 'bank_apr_jun.pdf', 'requested', NULL, NULL, NULL,
 NOW() - INTERVAL '8 days'),

('aa000000-0000-0000-0000-000000000012', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'e0000000-0000-0000-0000-000000000001', 'f0000000-0000-0000-0000-000000000001',
 'b0000000-0000-0000-0000-000000000001', 'd0000000-0000-0000-0000-000000000003',
 'VAT Receipts Q2', 'vat_receipts_q2.pdf', 'requested', NULL, NULL, NULL,
 NOW() - INTERVAL '8 days'),

-- For CT600 Filing (service f002) - 1 missing doc
('aa000000-0000-0000-0000-000000000013', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'e0000000-0000-0000-0000-000000000001', 'f0000000-0000-0000-0000-000000000002',
 'b0000000-0000-0000-0000-000000000001', 'd0000000-0000-0000-0000-000000000002',
 'Annual Accounts 2024', 'annual_accounts_2024.pdf', 'requested', NULL, NULL, NULL,
 NOW() - INTERVAL '7 days'),

-- For Payroll Submission (service f003) - 1 missing doc
('aa000000-0000-0000-0000-000000000014', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'e0000000-0000-0000-0000-000000000001', 'f0000000-0000-0000-0000-000000000003',
 'b0000000-0000-0000-0000-000000000001', 'd0000000-0000-0000-0000-000000000002',
 'Payroll Summary August', 'payroll_aug.pdf', 'requested', NULL, NULL, NULL,
 NOW() - INTERVAL '5 days'),

-- For Self Assessment Review (service f004) - 2 missing docs (spec shows these)
('aa000000-0000-0000-0000-000000000015', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'e0000000-0000-0000-0000-000000000001', 'f0000000-0000-0000-0000-000000000004',
 'b0000000-0000-0000-0000-000000000001', 'd0000000-0000-0000-0000-000000000010',
 'Dividend Vouchers', 'dividend_vouchers.pdf', 'requested', NULL, NULL, NULL,
 NOW() - INTERVAL '4 days'),

('aa000000-0000-0000-0000-000000000016', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'e0000000-0000-0000-0000-000000000001', 'f0000000-0000-0000-0000-000000000004',
 'b0000000-0000-0000-0000-000000000001', 'd0000000-0000-0000-0000-000000000001',
 'Rental Income Statement', 'rental_income.pdf', 'requested', NULL, NULL, NULL,
 NOW() - INTERVAL '4 days'),

-- Additional requested docs for other services
('aa000000-0000-0000-0000-000000000017', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'e0000000-0000-0000-0000-000000000009', 'f0000000-0000-0000-0000-000000000010',
 'b0000000-0000-0000-0000-000000000001', 'd0000000-0000-0000-0000-000000000006',
 'P60 (Missing)', 'p60.pdf', 'requested', NULL, NULL, NULL,
 NOW() - INTERVAL '6 days'),

('aa000000-0000-0000-0000-000000000018', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'e0000000-0000-0000-0000-000000000010', 'f0000000-0000-0000-0000-000000000008',
 'b0000000-0000-0000-0000-000000000001', 'd0000000-0000-0000-0000-000000000005',
 'Passport Copy (Missing)', 'passport.pdf', 'requested', NULL, NULL, NULL,
 NOW() - INTERVAL '5 days'),

-- ============================================================================
-- UPLOADED (4): Normal uploads awaiting review
-- ============================================================================
('aa000000-0000-0000-0000-000000000019', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'e0000000-0000-0000-0000-000000000008', 'f0000000-0000-0000-0000-000000000009',
 'b0000000-0000-0000-0000-000000000001', 'd0000000-0000-0000-0000-000000000001',
 'Bank_Statement_Jul.pdf', 'Bank_Statement_Jul.pdf', 'uploaded', 105000, 'application/pdf',
 'July 2024 bank statement. 32 transactions. Net movement: +£2,450.',
 NOW() - INTERVAL '3 days'),

('aa000000-0000-0000-0000-000000000020', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'e0000000-0000-0000-0000-000000000009', 'f0000000-0000-0000-0000-000000000010',
 'b0000000-0000-0000-0000-000000000001', 'd0000000-0000-0000-0000-000000000002',
 'Invoice_Pack.pdf', 'Invoice_Pack.pdf', 'uploaded', 280000, 'application/pdf',
 '45 invoices. Total: £67,500. Date range: Jul-Aug 2024.',
 NOW() - INTERVAL '4 days'),

('aa000000-0000-0000-0000-000000000021', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'e0000000-0000-0000-0000-000000000010', 'f0000000-0000-0000-0000-000000000008',
 'b0000000-0000-0000-0000-000000000001', 'd0000000-0000-0000-0000-000000000003',
 'Receipts_Bundle.pdf', 'Receipts_Bundle.pdf', 'uploaded', 156000, 'application/pdf',
 '28 expense receipts. Total: £3,200. Categories: Travel, Office, Entertainment.',
 NOW() - INTERVAL '2 days'),

('aa000000-0000-0000-0000-000000000022', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'e0000000-0000-0000-0000-000000000011', 'f0000000-0000-0000-0000-000000000009',
 'b0000000-0000-0000-0000-000000000001', 'd0000000-0000-0000-0000-000000000007',
 'UTR_Letter.pdf', 'UTR_Letter.pdf', 'uploaded', 32000, 'application/pdf',
 'HMRC UTR letter. UTR: 1234567890. Issued: January 2024.',
 NOW() - INTERVAL '1 day');

-- Update file_path for uploaded documents (OSS paths - files created in scripts/sample-docs/)
-- Note: Files need to be uploaded to OSS bucket: fzco-uploads with path: {tenant_id}/documents/{filename}
UPDATE documents SET file_path = 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11/documents/' || original_name
WHERE tenant_id = 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11'
  AND file_size IS NOT NULL
  AND status IN ('pending_review', 'approved', 'uploaded');

-- =============================================================================
-- 8. STAFF-CLIENT ASSIGNMENTS
-- =============================================================================
INSERT INTO staff_clients (tenant_id, staff_id, client_id, is_primary) VALUES
-- Sarah (overloaded with 6 clients)
('a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'b0000000-0000-0000-0000-000000000002', 'e0000000-0000-0000-0000-000000000001', true),
('a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'b0000000-0000-0000-0000-000000000002', 'e0000000-0000-0000-0000-000000000002', true),
('a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'b0000000-0000-0000-0000-000000000002', 'e0000000-0000-0000-0000-000000000003', true),
('a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'b0000000-0000-0000-0000-000000000002', 'e0000000-0000-0000-0000-000000000004', true),
('a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'b0000000-0000-0000-0000-000000000002', 'e0000000-0000-0000-0000-000000000005', true),
('a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'b0000000-0000-0000-0000-000000000002', 'e0000000-0000-0000-0000-000000000006', true),
('a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'b0000000-0000-0000-0000-000000000002', 'e0000000-0000-0000-0000-000000000016', true),
('a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'b0000000-0000-0000-0000-000000000002', 'e0000000-0000-0000-0000-000000000017', true),
-- Mike (2 clients)
('a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'b0000000-0000-0000-0000-000000000003', 'e0000000-0000-0000-0000-000000000007', true),
('a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'b0000000-0000-0000-0000-000000000003', 'e0000000-0000-0000-0000-000000000008', true),
-- Emma (2 clients)
('a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'b0000000-0000-0000-0000-000000000004', 'e0000000-0000-0000-0000-000000000009', true),
('a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'b0000000-0000-0000-0000-000000000004', 'e0000000-0000-0000-0000-000000000010', true),
-- James (2 clients)
('a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'b0000000-0000-0000-0000-000000000005', 'e0000000-0000-0000-0000-000000000011', true),
('a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'b0000000-0000-0000-0000-000000000005', 'e0000000-0000-0000-0000-000000000012', true),
-- Lisa (1 client)
('a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'b0000000-0000-0000-0000-000000000006', 'e0000000-0000-0000-0000-000000000013', true),
-- David (1 client)
('a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'b0000000-0000-0000-0000-000000000007', 'e0000000-0000-0000-0000-000000000014', true),
-- Rachel (1 client)
('a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'b0000000-0000-0000-0000-000000000008', 'e0000000-0000-0000-0000-000000000015', true);

-- =============================================================================
-- 9. EMAIL TEMPLATES (for chase emails)
-- =============================================================================
INSERT INTO email_templates (id, tenant_id, name, subject, body_html, body_text, type, is_default, is_active, created_by) VALUES
('e1000000-0000-0000-0000-000000000001', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'Document Chase - Single Service', 'Outstanding Documents - {{service_name}}',
 '<p>Dear {{client_contact}},</p><p>We''re still awaiting the following documents for your {{service_name}}:</p><ul>{{missing_docs_list}}</ul><p>The deadline was {{days_overdue}} days ago. To avoid penalties, please upload these as soon as possible.</p><p><a href="{{upload_link}}">📤 Upload Documents Now</a></p><p>Best regards,<br>{{sender_name}}<br>{{firm_name}}</p>',
 'Dear {{client_contact}},\n\nWe''re still awaiting documents for your {{service_name}}.\n\nPlease upload at: {{upload_link}}\n\nBest regards,\n{{sender_name}}',
 'chase', true, true, 'b0000000-0000-0000-0000-000000000001'),

('e1000000-0000-0000-0000-000000000002', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'Document Chase - Multiple Services', 'Urgent: Outstanding Documents - {{company_name}}',
 '<p>Dear {{client_contact}},</p><p>We have {{service_count}} services with overdue documents for {{company_name}}:</p>{{services_list}}<p>Please upload these documents as soon as possible to avoid penalties.</p><p><a href="{{upload_link}}">📤 Upload Documents Now</a></p><p>Best regards,<br>{{sender_name}}<br>{{firm_name}}</p>',
 'Dear {{client_contact}},\n\nWe have {{service_count}} services with overdue documents.\n\nPlease upload at: {{upload_link}}\n\nBest regards,\n{{sender_name}}',
 'chase', false, true, 'b0000000-0000-0000-0000-000000000001'),

('e1000000-0000-0000-0000-000000000003', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'Welcome Email', 'Welcome to {{firm_name}}',
 '<p>Dear {{client_contact}},</p><p>Welcome to {{firm_name}}! We''re delighted to have you as a client.</p><p>You can access your client portal at: <a href="{{portal_link}}">{{portal_link}}</a></p><p>Best regards,<br>{{firm_name}}</p>',
 'Dear {{client_contact}},\n\nWelcome to {{firm_name}}!\n\nAccess your portal at: {{portal_link}}\n\nBest regards,\n{{firm_name}}',
 'welcome', true, true, 'b0000000-0000-0000-0000-000000000001'),

('e1000000-0000-0000-0000-000000000004', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'Document Approved', 'Document Approved - {{document_name}}',
 '<p>Dear {{client_contact}},</p><p>Your document <strong>{{document_name}}</strong> has been reviewed and approved.</p><p>Thank you for your prompt submission.</p><p>Best regards,<br>{{firm_name}}</p>',
 'Dear {{client_contact}},\n\nYour document {{document_name}} has been approved.\n\nBest regards,\n{{firm_name}}',
 'notification', false, true, 'b0000000-0000-0000-0000-000000000001'),

('e1000000-0000-0000-0000-000000000005', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'Document Rejected', 'Action Required - {{document_name}}',
 '<p>Dear {{client_contact}},</p><p>Your document <strong>{{document_name}}</strong> requires attention.</p><p><strong>Reason:</strong> {{rejection_reason}}</p><p>{{rejection_notes}}</p><p>Please upload a corrected version at your earliest convenience.</p><p><a href="{{upload_link}}">📤 Upload Corrected Document</a></p><p>Best regards,<br>{{firm_name}}</p>',
 'Dear {{client_contact}},\n\nYour document {{document_name}} requires attention.\n\nReason: {{rejection_reason}}\n\nPlease upload at: {{upload_link}}\n\nBest regards,\n{{firm_name}}',
 'notification', false, true, 'b0000000-0000-0000-0000-000000000001');

-- =============================================================================
-- 10. EMAILS (10 chase/notification emails for tracking)
-- =============================================================================
INSERT INTO emails (id, tenant_id, client_id, staff_id, template_id, direction, to_email, to_name, from_email, subject, body_html, body_text, type, status, sent_at, created_at) VALUES
-- Recent chase emails (for chase history)
('e2000000-0000-0000-0000-000000000001', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'e0000000-0000-0000-0000-000000000007', 'b0000000-0000-0000-0000-000000000001',
 'e1000000-0000-0000-0000-000000000001', 'outbound',
 'frank@latepayers.com', 'Frank Green', 'info@demo-accounting.co.uk',
 'Outstanding Documents - VAT Return Q2',
 '<p>Dear Frank,</p><p>We''re still awaiting documents...</p>',
 'Dear Frank, We''re still awaiting documents...',
 'chase', 'delivered', NOW() - INTERVAL '2 days', NOW() - INTERVAL '2 days'),

('e2000000-0000-0000-0000-000000000002', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'e0000000-0000-0000-0000-000000000008', 'b0000000-0000-0000-0000-000000000001',
 'e1000000-0000-0000-0000-000000000001', 'outbound',
 'grace@overdue.co.uk', 'Grace Hill', 'info@demo-accounting.co.uk',
 'Outstanding Documents - CT600 Filing',
 '<p>Dear Grace,</p><p>We''re still awaiting documents...</p>',
 'Dear Grace, We''re still awaiting documents...',
 'chase', 'opened', NOW() - INTERVAL '3 days', NOW() - INTERVAL '3 days'),

('e2000000-0000-0000-0000-000000000003', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'e0000000-0000-0000-0000-000000000009', 'b0000000-0000-0000-0000-000000000002',
 'e1000000-0000-0000-0000-000000000001', 'outbound',
 'henry@missingdocs.com', 'Henry Irving', 'info@demo-accounting.co.uk',
 'Outstanding Documents - Self Assessment',
 '<p>Dear Henry,</p><p>We''re still awaiting documents...</p>',
 'Dear Henry, We''re still awaiting documents...',
 'chase', 'delivered', NOW() - INTERVAL '4 days', NOW() - INTERVAL '4 days'),

-- Document notification emails
('e2000000-0000-0000-0000-000000000004', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'e0000000-0000-0000-0000-000000000001', 'b0000000-0000-0000-0000-000000000001',
 'e1000000-0000-0000-0000-000000000004', 'outbound',
 'john@acme.co.uk', 'John Smith', 'info@demo-accounting.co.uk',
 'Document Approved - Bank Statement Q1',
 '<p>Dear John,</p><p>Your document has been approved...</p>',
 'Dear John, Your document has been approved...',
 'notification', 'delivered', NOW() - INTERVAL '5 days', NOW() - INTERVAL '5 days'),

('e2000000-0000-0000-0000-000000000005', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'e0000000-0000-0000-0000-000000000002', 'b0000000-0000-0000-0000-000000000001',
 'e1000000-0000-0000-0000-000000000003', 'outbound',
 'alice@techstart.io', 'Alice Brown', 'info@demo-accounting.co.uk',
 'Welcome to Demo Accounting Firm',
 '<p>Dear Alice,</p><p>Welcome to our firm...</p>',
 'Dear Alice, Welcome to our firm...',
 'notification', 'opened', NOW() - INTERVAL '10 days', NOW() - INTERVAL '10 days'),

-- More chase emails for stats
('e2000000-0000-0000-0000-000000000006', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'e0000000-0000-0000-0000-000000000010', 'b0000000-0000-0000-0000-000000000003',
 'e1000000-0000-0000-0000-000000000001', 'outbound',
 'ivy@incomplete.co.uk', 'Ivy Jones', 'info@demo-accounting.co.uk',
 'Outstanding Documents - Annual Accounts',
 '<p>Dear Ivy,</p><p>We''re still awaiting documents...</p>',
 'Dear Ivy, We''re still awaiting documents...',
 'chase', 'bounced', NOW() - INTERVAL '5 days', NOW() - INTERVAL '5 days'),

('e2000000-0000-0000-0000-000000000007', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'e0000000-0000-0000-0000-000000000011', 'b0000000-0000-0000-0000-000000000004',
 'e1000000-0000-0000-0000-000000000001', 'outbound',
 'jack@pending.co.uk', 'Jack King', 'info@demo-accounting.co.uk',
 'Outstanding Documents - VAT Return Q3',
 '<p>Dear Jack,</p><p>We''re still awaiting documents...</p>',
 'Dear Jack, We''re still awaiting documents...',
 'chase', 'delivered', NOW() - INTERVAL '6 days', NOW() - INTERVAL '6 days'),

('e2000000-0000-0000-0000-000000000008', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'e0000000-0000-0000-0000-000000000012', 'b0000000-0000-0000-0000-000000000005',
 'e1000000-0000-0000-0000-000000000001', 'outbound',
 'kate@docchase.com', 'Kate Lewis', 'info@demo-accounting.co.uk',
 'Outstanding Documents - Self Assessment',
 '<p>Dear Kate,</p><p>We''re still awaiting documents...</p>',
 'Dear Kate, We''re still awaiting documents...',
 'chase', 'clicked', NOW() - INTERVAL '7 days', NOW() - INTERVAL '7 days');

-- =============================================================================
-- 11. CHASE LOGS (3 historical bulk chases)
-- =============================================================================
INSERT INTO chase_logs (id, tenant_id, initiated_by, total_sent, delivered, opened, bounced, created_at) VALUES
('c1000000-0000-0000-0000-000000000001', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'b0000000-0000-0000-0000-000000000001', 5, 4, 2, 1, NOW() - INTERVAL '5 days'),
('c1000000-0000-0000-0000-000000000002', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'b0000000-0000-0000-0000-000000000002', 3, 3, 1, 0, NOW() - INTERVAL '10 days'),
('c1000000-0000-0000-0000-000000000003', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'b0000000-0000-0000-0000-000000000001', 8, 7, 4, 1, NOW() - INTERVAL '15 days');

-- =============================================================================
-- 12. CHASE LOG CLIENTS (link chases to clients)
-- =============================================================================
INSERT INTO chase_log_clients (id, tenant_id, chase_log_id, client_id, created_at) VALUES
-- Chase 1 (5 clients)
('cc000000-0000-0000-0000-000000000001', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'c1000000-0000-0000-0000-000000000001', 'e0000000-0000-0000-0000-000000000007', NOW() - INTERVAL '5 days'),
('cc000000-0000-0000-0000-000000000002', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'c1000000-0000-0000-0000-000000000001', 'e0000000-0000-0000-0000-000000000008', NOW() - INTERVAL '5 days'),
('cc000000-0000-0000-0000-000000000003', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'c1000000-0000-0000-0000-000000000001', 'e0000000-0000-0000-0000-000000000009', NOW() - INTERVAL '5 days'),
('cc000000-0000-0000-0000-000000000004', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'c1000000-0000-0000-0000-000000000001', 'e0000000-0000-0000-0000-000000000010', NOW() - INTERVAL '5 days'),
('cc000000-0000-0000-0000-000000000005', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'c1000000-0000-0000-0000-000000000001', 'e0000000-0000-0000-0000-000000000011', NOW() - INTERVAL '5 days'),
-- Chase 2 (3 clients)
('cc000000-0000-0000-0000-000000000006', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'c1000000-0000-0000-0000-000000000002', 'e0000000-0000-0000-0000-000000000004', NOW() - INTERVAL '10 days'),
('cc000000-0000-0000-0000-000000000007', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'c1000000-0000-0000-0000-000000000002', 'e0000000-0000-0000-0000-000000000005', NOW() - INTERVAL '10 days'),
('cc000000-0000-0000-0000-000000000008', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'c1000000-0000-0000-0000-000000000002', 'e0000000-0000-0000-0000-000000000006', NOW() - INTERVAL '10 days');

-- =============================================================================
-- 13. NOTIFICATIONS (5 for notification bell - spec shows "🔔 2")
-- =============================================================================
INSERT INTO notifications (id, tenant_id, user_id, type, title, message, entity_type, entity_id, link, is_read, created_at) VALUES
-- Unread notifications (for bell icon count)
('01000000-0000-0000-0000-000000000001', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'b0000000-0000-0000-0000-000000000001', 'document',
 'New document uploaded', 'John Smith uploaded Bank_Statement_Q2.pdf for Acme Corporation Ltd',
 'document', 'aa000000-0000-0000-0000-000000000001', '/documents/aa000000-0000-0000-0000-000000000001',
 false, NOW() - INTERVAL '10 minutes'),

('01000000-0000-0000-0000-000000000002', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'b0000000-0000-0000-0000-000000000001', 'deadline',
 'Service overdue', 'VAT Return Q2 for Acme Corporation Ltd is now 3 days overdue',
 'service', 'f0000000-0000-0000-0000-000000000001', '/services/f0000000-0000-0000-0000-000000000001',
 false, NOW() - INTERVAL '3 hours'),

-- Read notifications (older)
('01000000-0000-0000-0000-000000000003', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'b0000000-0000-0000-0000-000000000001', 'email',
 'Chase email opened', 'Grace Hill opened your chase email for CT600 Filing',
 'email', 'e2000000-0000-0000-0000-000000000002', '/emails/em000000-0000-0000-0000-000000000002',
 true, NOW() - INTERVAL '1 day'),

('01000000-0000-0000-0000-000000000004', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'b0000000-0000-0000-0000-000000000001', 'system',
 'Bulk chase completed', '5 chase emails sent successfully',
 'chase_log', 'c1000000-0000-0000-0000-000000000001', '/email/chase',
 true, NOW() - INTERVAL '5 days'),

('01000000-0000-0000-0000-000000000005', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'b0000000-0000-0000-0000-000000000001', 'document',
 'Document approved', 'You approved Bank Statement Q1 for Acme Corporation Ltd',
 'document', 'aa000000-0000-0000-0000-000000000005', '/documents/aa000000-0000-0000-0000-000000000005',
 true, NOW() - INTERVAL '5 days');

-- =============================================================================
-- 14. AUDIT LOGS (35+ entries for Activity panel)
-- =============================================================================
INSERT INTO audit_logs (id, tenant_id, user_id, action, entity_type, entity_id, metadata, severity, created_at) VALUES
-- TODAY: Login activity
('ab000000-0000-0000-0000-000000000001', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'b0000000-0000-0000-0000-000000000001', 'user.login', 'user', 'b0000000-0000-0000-0000-000000000001',
 '{"ip": "192.168.1.1"}', 'info', NOW() - INTERVAL '30 minutes'),
('ab000000-0000-0000-0000-000000000002', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'b0000000-0000-0000-0000-000000000002', 'user.login', 'user', 'b0000000-0000-0000-0000-000000000002',
 '{"ip": "192.168.1.2"}', 'info', NOW() - INTERVAL '45 minutes'),
('ab000000-0000-0000-0000-000000000003', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'b0000000-0000-0000-0000-000000000003', 'user.login', 'user', 'b0000000-0000-0000-0000-000000000003',
 '{"ip": "192.168.1.3"}', 'info', NOW() - INTERVAL '1 hour'),

-- TODAY: Document activity (matches spec)
('ab000000-0000-0000-0000-000000000004', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'b0000000-0000-0000-0000-000000000101', 'document.uploaded', 'document', 'aa000000-0000-0000-0000-000000000001',
 '{"filename": "Bank_Statement_Q2.pdf", "client": "Acme Corporation Ltd"}', 'info', NOW() - INTERVAL '10 minutes'),
('ab000000-0000-0000-0000-000000000005', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'b0000000-0000-0000-0000-000000000101', 'document.uploaded', 'document', 'aa000000-0000-0000-0000-000000000002',
 '{"filename": "VAT_Receipts_June.pdf", "client": "Acme Corporation Ltd"}', 'info', NOW() - INTERVAL '2 hours'),
('ab000000-0000-0000-0000-000000000006', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'b0000000-0000-0000-0000-000000000001', 'document.approved', 'document', 'aa000000-0000-0000-0000-000000000005',
 '{"filename": "P60_2023-24.pdf"}', 'info', NOW() - INTERVAL '5 hours'),
('ab000000-0000-0000-0000-000000000007', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'b0000000-0000-0000-0000-000000000001', 'document.approved', 'document', 'aa000000-0000-0000-0000-000000000006',
 '{"filename": "Bank_Interest_Statement.pdf"}', 'info', NOW() - INTERVAL '6 hours'),

-- TODAY: Service activity
('ab000000-0000-0000-0000-000000000008', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'b0000000-0000-0000-0000-000000000001', 'service.updated', 'service', 'f0000000-0000-0000-0000-000000000004',
 '{"status": "in_progress", "name": "Self Assessment Review"}', 'info', NOW() - INTERVAL '4 hours'),

-- TODAY: Chase activity
('ab000000-0000-0000-0000-000000000009', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'b0000000-0000-0000-0000-000000000001', 'email.sent', 'email', 'e2000000-0000-0000-0000-000000000001',
 '{"to": "frank@latepayers.com", "subject": "Outstanding Documents - VAT Return Q2", "type": "chase"}', 'info', NOW() - INTERVAL '2 days'),

-- YESTERDAY: Various activity
('ab000000-0000-0000-0000-000000000010', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'b0000000-0000-0000-0000-000000000001', 'user.login', 'user', 'b0000000-0000-0000-0000-000000000001',
 '{"ip": "192.168.1.1"}', 'info', NOW() - INTERVAL '1 day 2 hours'),
('ab000000-0000-0000-0000-000000000011', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'b0000000-0000-0000-0000-000000000102', 'document.uploaded', 'document', 'aa000000-0000-0000-0000-000000000003',
 '{"filename": "ID_Proof_Passport.jpg", "client": "Smith & Co Ltd"}', 'info', NOW() - INTERVAL '1 day'),
('ab000000-0000-0000-0000-000000000012', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'b0000000-0000-0000-0000-000000000001', 'client.created', 'client', 'e0000000-0000-0000-0000-000000000016',
 '{"company": "Smith & Co Ltd"}', 'info', NOW() - INTERVAL '1 day 4 hours'),
('ab000000-0000-0000-0000-000000000013', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'b0000000-0000-0000-0000-000000000001', 'client.created', 'client', 'e0000000-0000-0000-0000-000000000017',
 '{"company": "Jones Trading Ltd"}', 'info', NOW() - INTERVAL '1 day 5 hours'),
('ab000000-0000-0000-0000-000000000014', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'b0000000-0000-0000-0000-000000000003', 'service.created', 'service', 'f0000000-0000-0000-0000-000000000006',
 '{"name": "VAT Return Q3"}', 'info', NOW() - INTERVAL '1 day 6 hours'),

-- 2 DAYS AGO
('ab000000-0000-0000-0000-000000000015', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'b0000000-0000-0000-0000-000000000103', 'document.uploaded', 'document', 'aa000000-0000-0000-0000-000000000004',
 '{"filename": "Utility_Bill.pdf", "client": "Jones Trading Ltd"}', 'info', NOW() - INTERVAL '2 days'),
('ab000000-0000-0000-0000-000000000016', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'b0000000-0000-0000-0000-000000000001', 'user.login', 'user', 'b0000000-0000-0000-0000-000000000001',
 '{}', 'info', NOW() - INTERVAL '2 days 3 hours'),
('ab000000-0000-0000-0000-000000000017', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'b0000000-0000-0000-0000-000000000001', 'email.sent', 'email', 'e2000000-0000-0000-0000-000000000002',
 '{"to": "grace@overdue.co.uk", "subject": "Outstanding Documents - CT600 Filing"}', 'info', NOW() - INTERVAL '3 days'),
('ab000000-0000-0000-0000-000000000018', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'b0000000-0000-0000-0000-000000000002', 'document.approved', 'document', 'aa000000-0000-0000-0000-000000000007',
 '{}', 'info', NOW() - INTERVAL '2 days 4 hours'),
('ab000000-0000-0000-0000-000000000019', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'b0000000-0000-0000-0000-000000000003', 'service.completed', 'service', 'f0000000-0000-0000-0000-000000000025',
 '{}', 'info', NOW() - INTERVAL '2 days 5 hours'),

-- 3 DAYS AGO
('ab000000-0000-0000-0000-000000000020', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'b0000000-0000-0000-0000-000000000001', 'settings.updated', 'settings', NULL,
 '{"field": "reminder_rules"}', 'info', NOW() - INTERVAL '3 days 2 hours'),
('ab000000-0000-0000-0000-000000000021', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'b0000000-0000-0000-0000-000000000005', 'client.updated', 'client', 'e0000000-0000-0000-0000-000000000001',
 '{"field": "phone"}', 'info', NOW() - INTERVAL '3 days 4 hours'),
('ab000000-0000-0000-0000-000000000022', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'b0000000-0000-0000-0000-000000000006', 'document.uploaded', 'document', 'aa000000-0000-0000-0000-000000000008',
 '{}', 'info', NOW() - INTERVAL '3 days 6 hours'),

-- 5 DAYS AGO - Bulk chase
('ab000000-0000-0000-0000-000000000023', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'b0000000-0000-0000-0000-000000000001', 'chase.bulk_sent', 'chase_log', 'c1000000-0000-0000-0000-000000000001',
 '{"total_sent": 5, "delivered": 4, "bounced": 1}', 'info', NOW() - INTERVAL '5 days'),
('ab000000-0000-0000-0000-000000000024', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'b0000000-0000-0000-0000-000000000007', 'user.login', 'user', 'b0000000-0000-0000-0000-000000000007',
 '{}', 'info', NOW() - INTERVAL '5 days 1 hour'),
('ab000000-0000-0000-0000-000000000025', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'b0000000-0000-0000-0000-000000000008', 'service.created', 'service', 'f0000000-0000-0000-0000-000000000008',
 '{}', 'info', NOW() - INTERVAL '5 days 3 hours'),
('ab000000-0000-0000-0000-000000000026', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'b0000000-0000-0000-0000-000000000001', 'document.requested', 'document', 'aa000000-0000-0000-0000-000000000011',
 '{"document": "Bank Statement (Apr-Jun 2024)", "client": "Acme Corporation Ltd"}', 'info', NOW() - INTERVAL '8 days'),

-- More historical
('ab000000-0000-0000-0000-000000000027', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'b0000000-0000-0000-0000-000000000002', 'email.sent', 'email', 'e2000000-0000-0000-0000-000000000005',
 '{"subject": "Welcome to Demo Accounting Firm"}', 'info', NOW() - INTERVAL '10 days'),
('ab000000-0000-0000-0000-000000000028', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'b0000000-0000-0000-0000-000000000001', 'chase.bulk_sent', 'chase_log', 'c1000000-0000-0000-0000-000000000002',
 '{"total_sent": 3, "delivered": 3, "bounced": 0}', 'info', NOW() - INTERVAL '10 days'),
('ab000000-0000-0000-0000-000000000029', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'b0000000-0000-0000-0000-000000000001', 'chase.bulk_sent', 'chase_log', 'c1000000-0000-0000-0000-000000000003',
 '{"total_sent": 8, "delivered": 7, "bounced": 1}', 'info', NOW() - INTERVAL '15 days'),
('ab000000-0000-0000-0000-000000000030', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'b0000000-0000-0000-0000-000000000004', 'service.completed', 'service', 'f0000000-0000-0000-0000-000000000024',
 '{"name": "Annual Accounts 2022-23"}', 'info', NOW() - INTERVAL '60 days');

-- =============================================================================
-- 15. COMPANY SETTINGS
-- =============================================================================
INSERT INTO company_settings (id, tenant_id, firm_name, email, phone, address, reminder_rules) VALUES
('ac000000-0000-0000-0000-000000000001', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'Demo Accounting Firm', 'info@demo-accounting.co.uk', '020 1234 5678',
 '123 Accounting Lane, London, EC1A 1BB',
 '{"day3": true, "day7": true, "day14": true}');

-- =============================================================================
-- 16. DIRECTORS (company officers)
-- =============================================================================
INSERT INTO directors (id, tenant_id, client_id, name, role, appointed_date, nationality, dob_month, dob_year, is_active) VALUES
-- Acme Corporation Ltd
('d1000000-0000-0000-0000-000000000001', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'e0000000-0000-0000-0000-000000000001', 'John Smith', 'director', '2018-03-15', 'British', 5, 1975, true),
('d1000000-0000-0000-0000-000000000002', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'e0000000-0000-0000-0000-000000000001', 'Sarah Smith', 'director', '2018-03-15', 'British', 8, 1978, true),
('d1000000-0000-0000-0000-000000000003', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'e0000000-0000-0000-0000-000000000001', 'Michael Roberts', 'secretary', '2019-06-01', 'British', 11, 1982, true),
-- TechStart Solutions
('d1000000-0000-0000-0000-000000000004', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'e0000000-0000-0000-0000-000000000002', 'Alice Brown', 'director', '2020-01-10', 'British', 3, 1985, true),
('d1000000-0000-0000-0000-000000000005', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'e0000000-0000-0000-0000-000000000002', 'David Chen', 'director', '2020-01-10', 'British', 7, 1988, true),
-- Global Imports Ltd
('d1000000-0000-0000-0000-000000000006', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'e0000000-0000-0000-0000-000000000003', 'Robert Wilson', 'director', '2015-09-20', 'British', 2, 1970, true),
('d1000000-0000-0000-0000-000000000007', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'e0000000-0000-0000-0000-000000000003', 'Emily Watson', 'director', '2017-04-12', 'Irish', 12, 1980, true),
-- Smith & Co Ltd
('d1000000-0000-0000-0000-000000000008', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'e0000000-0000-0000-0000-000000000016', 'Peter Smith', 'director', '2019-02-28', 'British', 9, 1972, true),
-- Jones Trading Ltd
('d1000000-0000-0000-0000-000000000009', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'e0000000-0000-0000-0000-000000000017', 'Tom Jones', 'director', '2021-06-15', 'British', 4, 1980, true),
('d1000000-0000-0000-0000-000000000010', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'e0000000-0000-0000-0000-000000000017', 'Mary Jones', 'director', '2021-06-15', 'British', 10, 1982, true);

-- =============================================================================
-- 17. PSC (Persons of Significant Control)
-- =============================================================================
INSERT INTO psc (id, tenant_id, client_id, name, ownership_percentage, voting_rights, notified_date, nature_of_control, is_active) VALUES
-- Acme Corporation Ltd
('f1000000-0000-0000-0000-000000000001', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'e0000000-0000-0000-0000-000000000001', 'John Smith', '50-75%', '50-75%', '2018-03-15',
 '{"ownership": true, "voting": true, "appointment": false}', true),
('f1000000-0000-0000-0000-000000000002', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'e0000000-0000-0000-0000-000000000001', 'Sarah Smith', '25-50%', '25-50%', '2018-03-15',
 '{"ownership": true, "voting": true, "appointment": false}', true),
-- TechStart Solutions
('f1000000-0000-0000-0000-000000000003', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'e0000000-0000-0000-0000-000000000002', 'Alice Brown', '75%+', '75%+', '2020-01-10',
 '{"ownership": true, "voting": true, "appointment": true}', true),
-- Global Imports Ltd
('f1000000-0000-0000-0000-000000000004', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'e0000000-0000-0000-0000-000000000003', 'Robert Wilson', '50-75%', '50-75%', '2015-09-20',
 '{"ownership": true, "voting": true, "appointment": true}', true),
('f1000000-0000-0000-0000-000000000005', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'e0000000-0000-0000-0000-000000000003', 'Emily Watson', '25-50%', '25-50%', '2017-04-12',
 '{"ownership": true, "voting": false, "appointment": false}', true),
-- Smith & Co Ltd
('f1000000-0000-0000-0000-000000000006', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'e0000000-0000-0000-0000-000000000016', 'Peter Smith', '75%+', '75%+', '2019-02-28',
 '{"ownership": true, "voting": true, "appointment": true}', true),
-- Jones Trading Ltd
('f1000000-0000-0000-0000-000000000007', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'e0000000-0000-0000-0000-000000000017', 'Tom Jones', '50-75%', '50-75%', '2021-06-15',
 '{"ownership": true, "voting": true, "appointment": true}', true),
('f1000000-0000-0000-0000-000000000008', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'e0000000-0000-0000-0000-000000000017', 'Mary Jones', '25-50%', '25-50%', '2021-06-15',
 '{"ownership": true, "voting": true, "appointment": false}', true);

-- =============================================================================
-- 18. CLIENT_NOTES (staff notes on clients)
-- =============================================================================
INSERT INTO client_notes (id, tenant_id, client_id, staff_id, note, created_at) VALUES
-- Acme Corporation Ltd
('a1000000-0000-0000-0000-000000000001', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'e0000000-0000-0000-0000-000000000001', 'b0000000-0000-0000-0000-000000000002',
 'Client prefers email communication. Best contact time: mornings before 11am.', NOW() - INTERVAL '30 days'),
('a1000000-0000-0000-0000-000000000002', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'e0000000-0000-0000-0000-000000000001', 'b0000000-0000-0000-0000-000000000001',
 'Discussed VAT threshold - client approaching limit. Consider voluntary registration.', NOW() - INTERVAL '15 days'),
('a1000000-0000-0000-0000-000000000003', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'e0000000-0000-0000-0000-000000000001', 'b0000000-0000-0000-0000-000000000002',
 'Year-end meeting scheduled for next month. Prepare dividend planning options.', NOW() - INTERVAL '5 days'),
-- TechStart Solutions
('a1000000-0000-0000-0000-000000000004', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'e0000000-0000-0000-0000-000000000002', 'b0000000-0000-0000-0000-000000000003',
 'Startup client - R&D tax credits eligibility to be reviewed.', NOW() - INTERVAL '20 days'),
('a1000000-0000-0000-0000-000000000005', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'e0000000-0000-0000-0000-000000000002', 'b0000000-0000-0000-0000-000000000001',
 'Investor funding round expected Q4 - will need due diligence support.', NOW() - INTERVAL '10 days'),
-- Smith & Co Ltd
('a1000000-0000-0000-0000-000000000006', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'e0000000-0000-0000-0000-000000000016', 'b0000000-0000-0000-0000-000000000002',
 'New client onboarding complete. All AML checks passed.', NOW() - INTERVAL '45 days'),
('a1000000-0000-0000-0000-000000000007', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'e0000000-0000-0000-0000-000000000016', 'b0000000-0000-0000-0000-000000000002',
 'Client asked about pension contributions - schedule follow-up call.', NOW() - INTERVAL '3 days'),
-- Jones Trading Ltd
('a1000000-0000-0000-0000-000000000008', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'e0000000-0000-0000-0000-000000000017', 'b0000000-0000-0000-0000-000000000003',
 'Import/export business - check MTD compliance for customs declarations.', NOW() - INTERVAL '25 days');

-- =============================================================================
-- 19. SERVICE_REQUIREMENTS (what docs each service type needs)
-- =============================================================================
INSERT INTO service_requirements (id, tenant_id, service_type_id, document_type_id, is_mandatory) VALUES
-- VAT Return requires: Bank Statement, Invoices, Receipts
('5e000000-0000-0000-0000-000000000001', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'c0000000-0000-0000-0000-000000000001', 'd0000000-0000-0000-0000-000000000001', true),
('5e000000-0000-0000-0000-000000000002', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'c0000000-0000-0000-0000-000000000001', 'd0000000-0000-0000-0000-000000000002', true),
('5e000000-0000-0000-0000-000000000003', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'c0000000-0000-0000-0000-000000000001', 'd0000000-0000-0000-0000-000000000003', true),
-- CT600 Filing requires: Bank Statement, Invoices, Annual Accounts
('5e000000-0000-0000-0000-000000000004', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'c0000000-0000-0000-0000-000000000002', 'd0000000-0000-0000-0000-000000000001', true),
('5e000000-0000-0000-0000-000000000005', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'c0000000-0000-0000-0000-000000000002', 'd0000000-0000-0000-0000-000000000002', true),
-- Self Assessment requires: P60, Bank Statement, Dividend Voucher
('5e000000-0000-0000-0000-000000000006', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'c0000000-0000-0000-0000-000000000003', 'd0000000-0000-0000-0000-000000000006', true),
('5e000000-0000-0000-0000-000000000007', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'c0000000-0000-0000-0000-000000000003', 'd0000000-0000-0000-0000-000000000001', true),
('5e000000-0000-0000-0000-000000000008', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'c0000000-0000-0000-0000-000000000003', 'd0000000-0000-0000-0000-000000000010', false),
-- Payroll requires: Bank Statement
('5e000000-0000-0000-0000-000000000009', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'c0000000-0000-0000-0000-000000000004', 'd0000000-0000-0000-0000-000000000001', true),
-- Annual Accounts requires: Bank Statement, Invoices, Receipts
('5e000000-0000-0000-0000-000000000010', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'c0000000-0000-0000-0000-000000000005', 'd0000000-0000-0000-0000-000000000001', true),
('5e000000-0000-0000-0000-000000000011', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'c0000000-0000-0000-0000-000000000005', 'd0000000-0000-0000-0000-000000000002', true),
('5e000000-0000-0000-0000-000000000012', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'c0000000-0000-0000-0000-000000000005', 'd0000000-0000-0000-0000-000000000003', false),
-- ID Verification requires: Passport, Utility Bill
('5e000000-0000-0000-0000-000000000013', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'c0000000-0000-0000-0000-000000000006', 'd0000000-0000-0000-0000-000000000005', true),
('5e000000-0000-0000-0000-000000000014', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'c0000000-0000-0000-0000-000000000006', 'd0000000-0000-0000-0000-000000000009', true);

-- =============================================================================
-- 20. DOCUMENT_ACCESS (who can access which documents)
-- =============================================================================
INSERT INTO document_access (tenant_id, document_id, staff_id, granted_by) VALUES
-- Sarah can access Acme docs (aa...001-004)
('a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'aa000000-0000-0000-0000-000000000001', 'b0000000-0000-0000-0000-000000000002', 'b0000000-0000-0000-0000-000000000001'),
('a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'aa000000-0000-0000-0000-000000000002', 'b0000000-0000-0000-0000-000000000002', 'b0000000-0000-0000-0000-000000000001'),
('a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'aa000000-0000-0000-0000-000000000005', 'b0000000-0000-0000-0000-000000000002', 'b0000000-0000-0000-0000-000000000001'),
('a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'aa000000-0000-0000-0000-000000000006', 'b0000000-0000-0000-0000-000000000002', 'b0000000-0000-0000-0000-000000000001'),
-- Mike can access some docs
('a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'aa000000-0000-0000-0000-000000000001', 'b0000000-0000-0000-0000-000000000003', 'b0000000-0000-0000-0000-000000000001'),
('a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'aa000000-0000-0000-0000-000000000007', 'b0000000-0000-0000-0000-000000000003', 'b0000000-0000-0000-0000-000000000001'),
-- Emma can access uploaded docs
('a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'aa000000-0000-0000-0000-000000000019', 'b0000000-0000-0000-0000-000000000004', 'b0000000-0000-0000-0000-000000000001'),
('a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'aa000000-0000-0000-0000-000000000020', 'b0000000-0000-0000-0000-000000000004', 'b0000000-0000-0000-0000-000000000001');

-- =============================================================================
-- 21. EMAIL_ACCOUNTS (connected mailboxes)
-- =============================================================================
INSERT INTO email_accounts (id, tenant_id, user_id, email, type, auth_method, provider, imap_host, imap_password, oauth_access_token, status, last_sync_at) VALUES
-- Shared inbox for the firm (IMAP)
('ea000000-0000-0000-0000-000000000001', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 NULL, 'info@demo-accounting.co.uk', 'shared', 'imap', 'imap', 'mail.demo-accounting.co.uk', 'encrypted_password_here', NULL, 'active', NOW() - INTERVAL '1 hour'),
-- Sarah's personal inbox (OAuth)
('ea000000-0000-0000-0000-000000000002', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'b0000000-0000-0000-0000-000000000002', 'sarah@demo-accounting.co.uk', 'personal', 'oauth', 'google', NULL, NULL, 'encrypted_oauth_token_here', 'active', NOW() - INTERVAL '30 minutes'),
-- Mike's inbox with error (OAuth)
('ea000000-0000-0000-0000-000000000003', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'b0000000-0000-0000-0000-000000000003', 'mike@demo-accounting.co.uk', 'personal', 'oauth', 'microsoft', NULL, NULL, 'expired_oauth_token', 'error', NOW() - INTERVAL '2 days');

-- =============================================================================
-- 22. EMAIL_THREADS (conversation threads)
-- =============================================================================
INSERT INTO email_threads (id, tenant_id, thread_key, client_id, subject, participants, last_message_at, message_count, ai_summary) VALUES
-- Thread with Acme about VAT
('eb000000-0000-0000-0000-000000000001', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'thread-acme-vat-001', 'e0000000-0000-0000-0000-000000000001',
 'RE: VAT Return Q2 - Documents Required',
 '[{"email": "john@acme.co.uk", "name": "John Smith"}, {"email": "sarah@demo-accounting.co.uk", "name": "Sarah Johnson"}]',
 NOW() - INTERVAL '2 days', 4,
 'Discussion about missing VAT receipts. Client promised to send by Friday.'),
-- Thread with TechStart about R&D
('eb000000-0000-0000-0000-000000000002', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'thread-techstart-rd-001', 'e0000000-0000-0000-0000-000000000002',
 'R&D Tax Credits - Initial Assessment',
 '[{"email": "alice@techstart.io", "name": "Alice Brown"}, {"email": "info@demo-accounting.co.uk", "name": "Demo Accounting"}]',
 NOW() - INTERVAL '5 days', 3,
 'Initial discussion about R&D eligibility. Development costs reviewed.'),
-- Thread with Smith & Co onboarding
('eb000000-0000-0000-0000-000000000003', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'thread-smithco-onboard-001', 'e0000000-0000-0000-0000-000000000016',
 'Welcome to Demo Accounting Firm',
 '[{"email": "peter@smithco.co.uk", "name": "Peter Smith"}, {"email": "info@demo-accounting.co.uk", "name": "Demo Accounting"}]',
 NOW() - INTERVAL '10 days', 2,
 'Welcome email and AML verification request. Client completed all checks.');

-- =============================================================================
-- 23. REMINDERS (scheduled alerts)
-- =============================================================================
INSERT INTO reminders (id, tenant_id, user_id, client_id, service_id, document_id, remind_at, reason, status) VALUES
-- Reminder to follow up on VAT docs
('ec000000-0000-0000-0000-000000000001', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'b0000000-0000-0000-0000-000000000002', 'e0000000-0000-0000-0000-000000000001',
 'f0000000-0000-0000-0000-000000000001', NULL,
 NOW() + INTERVAL '2 hours', 'Follow up on missing VAT receipts', 'pending'),
-- Reminder to review uploaded doc
('ec000000-0000-0000-0000-000000000002', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'b0000000-0000-0000-0000-000000000001', 'e0000000-0000-0000-0000-000000000001',
 NULL, 'aa000000-0000-0000-0000-000000000001',
 NOW() + INTERVAL '1 day', 'Review bank statement with AI summary', 'pending'),
-- Reminder about CT600 deadline
('ec000000-0000-0000-0000-000000000003', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'b0000000-0000-0000-0000-000000000003', 'e0000000-0000-0000-0000-000000000001',
 'f0000000-0000-0000-0000-000000000002', NULL,
 NOW() - INTERVAL '1 day', 'CT600 deadline passed - urgent action required', 'sent'),
-- Reminder for client call
('ec000000-0000-0000-0000-000000000004', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'b0000000-0000-0000-0000-000000000002', 'e0000000-0000-0000-0000-000000000002',
 NULL, NULL,
 NOW() + INTERVAL '3 days', 'Schedule R&D tax credits consultation call', 'pending'),
-- Dismissed reminder
('ec000000-0000-0000-0000-000000000005', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'b0000000-0000-0000-0000-000000000001', 'e0000000-0000-0000-0000-000000000016',
 NULL, NULL,
 NOW() - INTERVAL '5 days', 'AML check reminder - completed', 'dismissed');

-- =============================================================================
-- 24. E_SIGN_REQUESTS (electronic signature requests)
-- =============================================================================
INSERT INTO e_sign_requests (id, tenant_id, client_id, document_id, template_type, status, signer_email, signer_name, sent_at, signed_at, expires_at, signature_data) VALUES
-- Pending engagement letter for Smith & Co
('ed000000-0000-0000-0000-000000000001', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'e0000000-0000-0000-0000-000000000016', NULL, 'engagement', 'pending',
 'peter@smithco.co.uk', 'Peter Smith',
 NOW() - INTERVAL '2 days', NULL, NOW() + INTERVAL '12 days', NULL),
-- Signed GDPR consent for Acme
('ed000000-0000-0000-0000-000000000002', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'e0000000-0000-0000-0000-000000000001', NULL, 'gdpr_consent', 'signed',
 'john@acme.co.uk', 'John Smith',
 NOW() - INTERVAL '30 days', NOW() - INTERVAL '28 days', NOW() + INTERVAL '60 days',
 '{"signature": "data:image/png;base64,iVBORw...", "ip": "192.168.1.100", "timestamp": "2026-08-06T10:30:00Z"}'),
-- Expired service agreement for TechStart
('ed000000-0000-0000-0000-000000000003', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'e0000000-0000-0000-0000-000000000002', NULL, 'service_agreement', 'expired',
 'alice@techstart.io', 'Alice Brown',
 NOW() - INTERVAL '20 days', NULL, NOW() - INTERVAL '6 days', NULL),
-- Pending engagement for Jones Trading
('ed000000-0000-0000-0000-000000000004', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'e0000000-0000-0000-0000-000000000017', NULL, 'engagement', 'pending',
 'tom@jonestrading.co.uk', 'Tom Jones',
 NOW() - INTERVAL '1 day', NULL, NOW() + INTERVAL '13 days', NULL);

-- =============================================================================
-- 25. AI_JOBS (AI processing history - OCR, classification, summaries)
-- =============================================================================
INSERT INTO ai_jobs (id, tenant_id, user_id, type, status, payload, result, error, document_id, email_id, service_id, client_id, created_at, started_at, completed_at) VALUES
-- Completed OCR job for bank statement
('ee000000-0000-0000-0000-000000000001', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'b0000000-0000-0000-0000-000000000001', 'ocr', 'completed',
 '{"document_path": "uploads/bank_statement_q2.pdf", "pages": 3}',
 '{"text_extracted": true, "pages_processed": 3, "confidence": 0.95}',
 NULL, 'aa000000-0000-0000-0000-000000000001', NULL, NULL, 'e0000000-0000-0000-0000-000000000001',
 NOW() - INTERVAL '2 hours', NOW() - INTERVAL '2 hours', NOW() - INTERVAL '1 hour 55 minutes'),
-- Completed classification job
('ee000000-0000-0000-0000-000000000002', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'b0000000-0000-0000-0000-000000000001', 'classification', 'completed',
 '{"document_id": "aa000000-0000-0000-0000-000000000001", "text_sample": "HSBC Bank Statement..."}',
 '{"document_type": "bank_statement", "confidence": 0.92, "suggested_service": "accounts"}',
 NULL, 'aa000000-0000-0000-0000-000000000001', NULL, NULL, 'e0000000-0000-0000-0000-000000000001',
 NOW() - INTERVAL '1 hour 50 minutes', NOW() - INTERVAL '1 hour 50 minutes', NOW() - INTERVAL '1 hour 48 minutes'),
-- Completed summary job for bank statement
('ee000000-0000-0000-0000-000000000003', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'b0000000-0000-0000-0000-000000000001', 'summary', 'completed',
 '{"document_id": "aa000000-0000-0000-0000-000000000001", "full_text": "Bank statement showing..."}',
 '{"summary": "Q2 bank statement showing balance £12,450 with 15 transactions. Interest earned: £34.50", "key_figures": {"balance": 12450, "transactions": 15}}',
 NULL, 'aa000000-0000-0000-0000-000000000001', NULL, NULL, 'e0000000-0000-0000-0000-000000000001',
 NOW() - INTERVAL '1 hour 45 minutes', NOW() - INTERVAL '1 hour 45 minutes', NOW() - INTERVAL '1 hour 40 minutes'),
-- Completed OCR+summary for VAT receipts
('ee000000-0000-0000-0000-000000000004', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'b0000000-0000-0000-0000-000000000002', 'ocr', 'completed',
 '{"document_path": "uploads/vat_receipts_june.pdf", "pages": 8}',
 '{"text_extracted": true, "pages_processed": 8, "confidence": 0.89}',
 NULL, 'aa000000-0000-0000-0000-000000000002', NULL, NULL, 'e0000000-0000-0000-0000-000000000001',
 NOW() - INTERVAL '3 hours', NOW() - INTERVAL '3 hours', NOW() - INTERVAL '2 hours 50 minutes'),
-- Completed email classification
('ee000000-0000-0000-0000-000000000005', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 NULL, 'email_classification', 'completed',
 '{"email_subject": "RE: VAT Return Q2 - Documents Required", "body_preview": "Hi Sarah, Please find attached..."}',
 '{"category": "client_response", "priority": "high", "suggested_client": "e0000000-0000-0000-0000-000000000001", "sentiment": "positive"}',
 NULL, NULL, 'e2000000-0000-0000-0000-000000000001', NULL, 'e0000000-0000-0000-0000-000000000001',
 NOW() - INTERVAL '2 days', NOW() - INTERVAL '2 days', NOW() - INTERVAL '2 days' + INTERVAL '30 seconds'),
-- Processing job (in progress)
('ee000000-0000-0000-0000-000000000006', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'b0000000-0000-0000-0000-000000000001', 'ocr', 'processing',
 '{"document_path": "uploads/p11d_benefits.pdf", "pages": 5}',
 NULL, NULL, 'aa000000-0000-0000-0000-000000000003', NULL, NULL, 'e0000000-0000-0000-0000-000000000001',
 NOW() - INTERVAL '5 minutes', NOW() - INTERVAL '4 minutes', NULL),
-- Failed job (for testing error handling)
('ee000000-0000-0000-0000-000000000007', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'b0000000-0000-0000-0000-000000000003', 'ocr', 'failed',
 '{"document_path": "uploads/corrupted_file.pdf", "pages": 0}',
 NULL, 'PDF parsing failed: File appears to be corrupted or password protected',
 NULL, NULL, NULL, 'e0000000-0000-0000-0000-000000000002',
 NOW() - INTERVAL '1 day', NOW() - INTERVAL '1 day', NOW() - INTERVAL '1 day' + INTERVAL '10 seconds'),
-- Pending job (queued)
('ee000000-0000-0000-0000-000000000008', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'b0000000-0000-0000-0000-000000000001', 'summary', 'pending',
 '{"document_id": "aa000000-0000-0000-0000-000000000004", "full_text": "Employment contract details..."}',
 NULL, NULL, 'aa000000-0000-0000-0000-000000000004', NULL, NULL, 'e0000000-0000-0000-0000-000000000001',
 NOW() - INTERVAL '1 minute', NULL, NULL);

-- =============================================================================
-- 26. TENANT_SUBSCRIPTIONS (billing - enterprise plan active)
-- =============================================================================
INSERT INTO tenant_subscriptions (id, tenant_id, stripe_customer_id, stripe_subscription_id, plan, status, current_period_start, current_period_end, created_at, updated_at) VALUES
('ef000000-0000-0000-0000-000000000001', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'cus_demo_accounting_001', 'sub_demo_enterprise_001',
 'enterprise', 'active',
 DATE_TRUNC('month', NOW()), DATE_TRUNC('month', NOW()) + INTERVAL '1 month',
 NOW() - INTERVAL '6 months', NOW() - INTERVAL '1 day');

-- =============================================================================
-- 27. TENANT_INVOICES (payment history)
-- =============================================================================
INSERT INTO tenant_invoices (id, tenant_id, stripe_invoice_id, amount_cents, currency, status, invoice_pdf_url, period_start, period_end, created_at) VALUES
-- Current month (most recent)
('fa000000-0000-0000-0000-000000000001', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'in_demo_202609_001', 29900, 'GBP', 'paid',
 'https://stripe.com/invoices/demo/202609.pdf',
 DATE_TRUNC('month', NOW()), DATE_TRUNC('month', NOW()) + INTERVAL '1 month',
 DATE_TRUNC('month', NOW())),
-- Previous months
('fa000000-0000-0000-0000-000000000002', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'in_demo_202608_001', 29900, 'GBP', 'paid',
 'https://stripe.com/invoices/demo/202608.pdf',
 DATE_TRUNC('month', NOW()) - INTERVAL '1 month', DATE_TRUNC('month', NOW()),
 DATE_TRUNC('month', NOW()) - INTERVAL '1 month'),
('fa000000-0000-0000-0000-000000000003', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'in_demo_202607_001', 29900, 'GBP', 'paid',
 'https://stripe.com/invoices/demo/202607.pdf',
 DATE_TRUNC('month', NOW()) - INTERVAL '2 months', DATE_TRUNC('month', NOW()) - INTERVAL '1 month',
 DATE_TRUNC('month', NOW()) - INTERVAL '2 months'),
('fa000000-0000-0000-0000-000000000004', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'in_demo_202606_001', 29900, 'GBP', 'paid',
 'https://stripe.com/invoices/demo/202606.pdf',
 DATE_TRUNC('month', NOW()) - INTERVAL '3 months', DATE_TRUNC('month', NOW()) - INTERVAL '2 months',
 DATE_TRUNC('month', NOW()) - INTERVAL '3 months');

-- =============================================================================
-- VERIFICATION QUERIES
-- =============================================================================
DO $$
DECLARE
    tenant_count INT;
    user_count INT;
    client_user_count INT;
    client_count INT;
    service_count INT;
    document_count INT;
    pending_doc_count INT;
    email_count INT;
    chase_count INT;
    notification_count INT;
    audit_count INT;
BEGIN
    SELECT COUNT(*) INTO tenant_count FROM tenants WHERE id = 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11';
    SELECT COUNT(*) INTO user_count FROM users WHERE tenant_id = 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11';
    SELECT COUNT(*) INTO client_user_count FROM users WHERE tenant_id = 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11' AND role = 'client';
    SELECT COUNT(*) INTO client_count FROM clients WHERE tenant_id = 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11';
    SELECT COUNT(*) INTO service_count FROM services WHERE tenant_id = 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11';
    SELECT COUNT(*) INTO document_count FROM documents WHERE tenant_id = 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11';
    SELECT COUNT(*) INTO pending_doc_count FROM documents WHERE tenant_id = 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11' AND status = 'pending_review';
    SELECT COUNT(*) INTO email_count FROM emails WHERE tenant_id = 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11';
    SELECT COUNT(*) INTO chase_count FROM chase_logs WHERE tenant_id = 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11';
    SELECT COUNT(*) INTO notification_count FROM notifications WHERE tenant_id = 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11';
    SELECT COUNT(*) INTO audit_count FROM audit_logs WHERE tenant_id = 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11';

    RAISE NOTICE '';
    RAISE NOTICE '════════════════════════════════════════════════════════════════';
    RAISE NOTICE '  ENHANCED SEED DATA SUMMARY (E2E Ready)';
    RAISE NOTICE '════════════════════════════════════════════════════════════════';
    RAISE NOTICE '';
    RAISE NOTICE '  CORE DATA:';
    RAISE NOTICE '    Tenant:           % (expected: 1)', tenant_count;
    RAISE NOTICE '    Users (total):    % (expected: 12)', user_count;
    RAISE NOTICE '    Users (clients):  % (expected: 4) ← NEW: Portal users', client_user_count;
    RAISE NOTICE '    Clients:          % (expected: 17)', client_count;
    RAISE NOTICE '    Services:         % (expected: 25)', service_count;
    RAISE NOTICE '    Documents:        % (expected: 22)', document_count;
    RAISE NOTICE '';
    RAISE NOTICE '  NEW E2E DATA:';
    RAISE NOTICE '    Emails:           % (expected: 8)', email_count;
    RAISE NOTICE '    Chase Logs:       % (expected: 3)', chase_count;
    RAISE NOTICE '    Notifications:    % (expected: 5)', notification_count;
    RAISE NOTICE '    Audit Logs:       % (expected: 30)', audit_count;
    RAISE NOTICE '';
    RAISE NOTICE '════════════════════════════════════════════════════════════════';
    RAISE NOTICE '  TODAY DASHBOARD DATA:';
    RAISE NOTICE '════════════════════════════════════════════════════════════════';
    RAISE NOTICE '';
    RAISE NOTICE '  DO FIRST:';
    RAISE NOTICE '    Pending Docs:     % (spec: 4)', pending_doc_count;
    RAISE NOTICE '    Overdue Services: %', (SELECT COUNT(*) FROM services WHERE tenant_id = 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11' AND deadline < CURRENT_DATE AND status NOT IN ('completed', 'cancelled'));
    RAISE NOTICE '';
    RAISE NOTICE '  LATER TODAY:';
    RAISE NOTICE '    Due Today:        %', (SELECT COUNT(*) FROM services WHERE tenant_id = 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11' AND deadline = CURRENT_DATE AND status NOT IN ('completed', 'cancelled'));
    RAISE NOTICE '';
    RAISE NOTICE '  ALERTS:';
    RAISE NOTICE '    Quiet Clients:    %', (SELECT COUNT(*) FROM clients WHERE tenant_id = 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11' AND status = 'active' AND last_contact_at < NOW() - INTERVAL '14 days');
    RAISE NOTICE '    Unread Notifs:    %', (SELECT COUNT(*) FROM notifications WHERE tenant_id = 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11' AND is_read = false);
    RAISE NOTICE '';
    RAISE NOTICE '════════════════════════════════════════════════════════════════';
    RAISE NOTICE '  ✓ Enhanced seed data inserted successfully!';
    RAISE NOTICE '  ✓ Ready for Today Dashboard E2E testing';
    RAISE NOTICE '';
    RAISE NOTICE '  LOGIN CREDENTIALS:';
    RAISE NOTICE '    Admin:  admin@test.com / Test123!';
    RAISE NOTICE '    Client: john@acme.co.uk / Test123!';
    RAISE NOTICE '════════════════════════════════════════════════════════════════';
END $$;
