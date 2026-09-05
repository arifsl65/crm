-- =============================================================================
-- SEED EDGE CASES - Phase 2: UI Triggers & Edge Cases
-- =============================================================================
-- PURPOSE: Additional data for Super Admin, Document versions, Email AI,
--          Branding, Security, and extended audit logs
-- RUN AFTER: seed-demo.sql
-- =============================================================================

-- Bypass RLS for seeding
SET app.env = 'development';

-- =============================================================================
-- 1. SUPER ADMIN PORTAL - Multiple Tenants
-- =============================================================================

-- Tenant 2: Tech Bros Ltd (starter, active)
INSERT INTO tenants (id, name, domain, plan, timezone, is_active) VALUES
('b1000000-0000-0000-0000-000000000001', 'Tech Bros Ltd', 'techbros', 'starter', 'Europe/London', true);

-- Tenant 3: Global Finance (pro, past_due subscription)
INSERT INTO tenants (id, name, domain, plan, timezone, is_active) VALUES
('b2000000-0000-0000-0000-000000000001', 'Global Finance', 'globalfin', 'pro', 'America/New_York', true);

-- Tenant 4: Old Firm Accounting (enterprise, suspended)
INSERT INTO tenants (id, name, domain, plan, timezone, is_active, deleted_at) VALUES
('b3000000-0000-0000-0000-000000000001', 'Old Firm Accounting', 'oldfirm', 'enterprise', 'Europe/London', false, NOW() - INTERVAL '7 days');

-- Admin users for new tenants
INSERT INTO users (id, tenant_id, email, password, name, role, status) VALUES
-- Tech Bros admin
('c1000000-0000-0000-0000-000000000001', 'b1000000-0000-0000-0000-000000000001',
 'admin@techbros.io', '$argon2id$v=19$m=65536,t=3,p=4$c29tZXNhbHQ$RdescudvJCsgt3ub+b+dWRWJTmaaJObG',
 'Jake Developer', 'tenant_admin', 'active'),
-- Global Finance admin
('c2000000-0000-0000-0000-000000000001', 'b2000000-0000-0000-0000-000000000001',
 'admin@globalfin.com', '$argon2id$v=19$m=65536,t=3,p=4$c29tZXNhbHQ$RdescudvJCsgt3ub+b+dWRWJTmaaJObG',
 'Maria Finance', 'tenant_admin', 'active'),
-- Old Firm admin (suspended)
('c3000000-0000-0000-0000-000000000001', 'b3000000-0000-0000-0000-000000000001',
 'admin@oldfirm.co.uk', '$argon2id$v=19$m=65536,t=3,p=4$c29tZXNhbHQ$RdescudvJCsgt3ub+b+dWRWJTmaaJObG',
 'Robert Old', 'tenant_admin', 'inactive');

-- Clients for new tenants (usage stats)
INSERT INTO clients (id, tenant_id, company_name, contact_name, email, status) VALUES
-- Tech Bros clients (3)
('c1100000-0000-0000-0000-000000000001', 'b1000000-0000-0000-0000-000000000001', 'Startup Alpha', 'Alex Startup', 'contact@startupalpha.io', 'active'),
('c1100000-0000-0000-0000-000000000002', 'b1000000-0000-0000-0000-000000000001', 'Beta Corp', 'Betty Corp', 'info@betacorp.io', 'active'),
('c1100000-0000-0000-0000-000000000003', 'b1000000-0000-0000-0000-000000000001', 'Gamma Labs', 'Gary Labs', 'hello@gammalabs.io', 'active'),
-- Global Finance clients (5)
('c2100000-0000-0000-0000-000000000001', 'b2000000-0000-0000-0000-000000000001', 'Wall Street Inc', 'Walter Street', 'info@wallstreet.com', 'active'),
('c2100000-0000-0000-0000-000000000002', 'b2000000-0000-0000-0000-000000000001', 'Hedge Fund Alpha', 'Harry Hedge', 'contact@hfalpha.com', 'active'),
('c2100000-0000-0000-0000-000000000003', 'b2000000-0000-0000-0000-000000000001', 'Private Equity Co', 'Paula Equity', 'pe@privateequity.com', 'active'),
('c2100000-0000-0000-0000-000000000004', 'b2000000-0000-0000-0000-000000000001', 'Venture Capital LLC', 'Victor Capital', 'vc@venturecap.com', 'active'),
('c2100000-0000-0000-0000-000000000005', 'b2000000-0000-0000-0000-000000000001', 'Investment Trust', 'Ian Trust', 'trust@investment.com', 'active'),
-- Old Firm clients (2 - dormant)
('c3100000-0000-0000-0000-000000000001', 'b3000000-0000-0000-0000-000000000001', 'Legacy Corp', 'Leo Legacy', 'old@legacy.co.uk', 'inactive'),
('c3100000-0000-0000-0000-000000000002', 'b3000000-0000-0000-0000-000000000001', 'Defunct Ltd', 'Dan Defunct', 'info@defunct.co.uk', 'inactive');

-- Subscriptions for new tenants
INSERT INTO tenant_subscriptions (id, tenant_id, stripe_customer_id, stripe_subscription_id, plan, status, current_period_start, current_period_end) VALUES
('f1000000-0000-0000-0000-000000000001', 'b1000000-0000-0000-0000-000000000001',
 'cus_techbros_001', 'sub_techbros_001', 'starter', 'active',
 DATE_TRUNC('month', NOW()), DATE_TRUNC('month', NOW()) + INTERVAL '1 month'),
('f2000000-0000-0000-0000-000000000001', 'b2000000-0000-0000-0000-000000000001',
 'cus_globalfin_001', 'sub_globalfin_001', 'pro', 'past_due',
 DATE_TRUNC('month', NOW()) - INTERVAL '1 month', DATE_TRUNC('month', NOW())),
('f3000000-0000-0000-0000-000000000001', 'b3000000-0000-0000-0000-000000000001',
 'cus_oldfirm_001', 'sub_oldfirm_001', 'enterprise', 'canceled',
 DATE_TRUNC('month', NOW()) - INTERVAL '2 months', DATE_TRUNC('month', NOW()) - INTERVAL '1 month');

-- =============================================================================
-- 2. DOCUMENT EDGE CASES
-- =============================================================================

-- 2a. Firm Documents (client_id IS NULL) - Internal docs
INSERT INTO documents (id, tenant_id, client_id, service_id, uploaded_by, type_id, name, original_name, status, file_size, mime_type, ai_summary, access) VALUES
('ab000000-0000-0000-0000-000000000001', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 NULL, NULL, 'b0000000-0000-0000-0000-000000000001', NULL,
 'Staff_Handbook_2024.pdf', 'Staff_Handbook_2024.pdf', 'approved', 520000, 'application/pdf',
 'Internal staff handbook covering policies, procedures, and compliance requirements.',
 'admin'),
('ab000000-0000-0000-0000-000000000002', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 NULL, NULL, 'b0000000-0000-0000-0000-000000000001', NULL,
 'GDPR_Policy.pdf', 'GDPR_Policy.pdf', 'approved', 180000, 'application/pdf',
 'GDPR compliance policy document for firm operations.',
 'all_staff'),
('ab000000-0000-0000-0000-000000000003', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 NULL, NULL, 'b0000000-0000-0000-0000-000000000001', NULL,
 'Fee_Schedule_2024.xlsx', 'Fee_Schedule_2024.xlsx', 'approved', 45000, 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet',
 'Current fee schedule for all service types.',
 'admin'),
('ab000000-0000-0000-0000-000000000004', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 NULL, NULL, 'b0000000-0000-0000-0000-000000000002', NULL,
 'AML_Checklist_Template.pdf', 'AML_Checklist_Template.pdf', 'approved', 92000, 'application/pdf',
 'Anti-money laundering checklist template for new client onboarding.',
 'all_staff'),
('ab000000-0000-0000-0000-000000000005', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 NULL, NULL, 'b0000000-0000-0000-0000-000000000001', NULL,
 'Board_Minutes_Aug_2024.pdf', 'Board_Minutes_Aug_2024.pdf', 'approved', 78000, 'application/pdf',
 'Board meeting minutes from August 2024.',
 'admin');

-- Document access for firm docs (specific staff)
INSERT INTO document_access (tenant_id, document_id, staff_id, granted_by) VALUES
('a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'ab000000-0000-0000-0000-000000000002', 'b0000000-0000-0000-0000-000000000002', 'b0000000-0000-0000-0000-000000000001'),
('a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'ab000000-0000-0000-0000-000000000002', 'b0000000-0000-0000-0000-000000000003', 'b0000000-0000-0000-0000-000000000001'),
('a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'ab000000-0000-0000-0000-000000000004', 'b0000000-0000-0000-0000-000000000002', 'b0000000-0000-0000-0000-000000000001'),
('a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'ab000000-0000-0000-0000-000000000004', 'b0000000-0000-0000-0000-000000000003', 'b0000000-0000-0000-0000-000000000001'),
('a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'ab000000-0000-0000-0000-000000000004', 'b0000000-0000-0000-0000-000000000004', 'b0000000-0000-0000-0000-000000000001');

-- 2b. Version History Chain (Doc V1 -> V2 -> V3)
-- V1 marked as rejected (superseded), V2 marked as rejected (superseded), V3 is approved
INSERT INTO documents (id, tenant_id, client_id, service_id, uploaded_by, type_id, name, original_name, status, file_size, mime_type, ai_summary, version, parent_id) VALUES
-- V1 (original - superseded)
('ac000000-0000-0000-0000-000000000001', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'e0000000-0000-0000-0000-000000000001', 'f0000000-0000-0000-0000-000000000002',
 'b0000000-0000-0000-0000-000000000101', 'd0000000-0000-0000-0000-000000000002',
 'Annual_Accounts_Draft_V1.pdf', 'Annual_Accounts_Draft.pdf', 'rejected', 450000, 'application/pdf',
 'Initial draft of annual accounts. Needs review of depreciation schedule.',
 1, NULL),
-- V2 (updated - superseded)
('ac000000-0000-0000-0000-000000000002', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'e0000000-0000-0000-0000-000000000001', 'f0000000-0000-0000-0000-000000000002',
 'b0000000-0000-0000-0000-000000000002', 'd0000000-0000-0000-0000-000000000002',
 'Annual_Accounts_Draft_V2.pdf', 'Annual_Accounts_Draft_V2.pdf', 'rejected', 455000, 'application/pdf',
 'Second draft with corrected depreciation. Client requested minor changes to notes.',
 2, 'ac000000-0000-0000-0000-000000000001'),
-- V3 (current - approved)
('ac000000-0000-0000-0000-000000000003', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'e0000000-0000-0000-0000-000000000001', 'f0000000-0000-0000-0000-000000000002',
 'b0000000-0000-0000-0000-000000000002', 'd0000000-0000-0000-0000-000000000002',
 'Annual_Accounts_Final_V3.pdf', 'Annual_Accounts_Final.pdf', 'approved', 460000, 'application/pdf',
 'Final version approved by client. Ready for filing.',
 3, 'ac000000-0000-0000-0000-000000000002');

-- 2c. Expiring Documents
INSERT INTO documents (id, tenant_id, client_id, service_id, uploaded_by, type_id, name, original_name, status, file_size, mime_type, ai_summary, expiry_date) VALUES
-- Expired yesterday
('ad000000-0000-0000-0000-000000000001', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'e0000000-0000-0000-0000-000000000002', NULL,
 'b0000000-0000-0000-0000-000000000104', 'd0000000-0000-0000-0000-000000000005',
 'Passport_Expired.jpg', 'Passport_Copy.jpg', 'approved', 280000, 'image/jpeg',
 'UK passport - EXPIRED. Renewal required for ongoing AML compliance.',
 CURRENT_DATE - INTERVAL '1 day'),
-- Expires in 3 days
('ad000000-0000-0000-0000-000000000002', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'e0000000-0000-0000-0000-000000000016', NULL,
 'b0000000-0000-0000-0000-000000000102', 'd0000000-0000-0000-0000-000000000009',
 'Utility_Bill_Expiring.pdf', 'Utility_Bill_Aug.pdf', 'approved', 85000, 'application/pdf',
 'Utility bill for address verification. Valid for 3 months from issue.',
 CURRENT_DATE + INTERVAL '3 days'),
-- Expires in 7 days
('ad000000-0000-0000-0000-000000000003', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'e0000000-0000-0000-0000-000000000003', NULL,
 'b0000000-0000-0000-0000-000000000001', 'd0000000-0000-0000-0000-000000000004',
 'Driving_License_Expiring.jpg', 'Driving_License.jpg', 'approved', 195000, 'image/jpeg',
 'UK driving license expiring soon. Client notified.',
 CURRENT_DATE + INTERVAL '7 days');

-- 2d. AI Extracted Data (populate ai_extracted JSONB)
UPDATE documents SET ai_extracted = '{
  "account_number": "12345678",
  "sort_code": "40-50-60",
  "account_holder": "Acme Corporation Ltd",
  "statement_period": "2024-04-01 to 2024-06-30",
  "opening_balance": 8180.00,
  "closing_balance": 14450.00,
  "total_credits": 12500.00,
  "total_debits": 6230.00,
  "transaction_count": 45
}'::jsonb WHERE id = 'aa000000-0000-0000-0000-000000000001';

UPDATE documents SET ai_extracted = '{
  "receipt_count": 12,
  "total_amount": 4250.00,
  "vat_amount": 708.33,
  "categories": ["office_supplies", "travel", "entertainment", "software"],
  "date_range": "2024-06-01 to 2024-06-30",
  "largest_receipt": {"vendor": "Adobe", "amount": 599.00}
}'::jsonb WHERE id = 'aa000000-0000-0000-0000-000000000002';

UPDATE documents SET ai_extracted = '{
  "document_type": "passport",
  "document_number": "123456789",
  "full_name": "Peter James Smith",
  "date_of_birth": "1980-03-15",
  "nationality": "British",
  "issue_date": "2018-05-20",
  "expiry_date": "2028-05-19",
  "mrz_valid": true
}'::jsonb WHERE id = 'aa000000-0000-0000-0000-000000000003';

UPDATE documents SET ai_extracted = '{
  "utility_type": "electricity",
  "provider": "British Gas",
  "account_number": "789456123",
  "billing_address": "45 High Street, London EC1A 1BB",
  "amount_due": 156.40,
  "billing_period": "2024-06-01 to 2024-06-30",
  "address_verified": true
}'::jsonb WHERE id = 'aa000000-0000-0000-0000-000000000004';

UPDATE documents SET ai_extracted = '{
  "tax_year": "2023-24",
  "employer": "Acme Corporation Ltd",
  "employee_name": "John Smith",
  "ni_number": "AB123456C",
  "total_pay": 45000.00,
  "tax_paid": 7500.00,
  "ni_contributions": 4200.00,
  "student_loan": 0
}'::jsonb WHERE id = 'aa000000-0000-0000-0000-000000000005';

-- =============================================================================
-- 3. EMAIL AI & EDGE CASES
-- =============================================================================

-- 3a. Bounced Emails
INSERT INTO emails (id, tenant_id, client_id, from_email, to_email, subject, body_html, body_text, direction, status, bounce_reason, bounced_at, sent_at, created_at) VALUES
('e3000000-0000-0000-0000-000000000001', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 NULL, 'info@demo-accounting.co.uk', 'wrong@invalid-domain.xyz',
 'Document Request - VAT Return', '<p>Please provide your VAT receipts...</p>', 'Please provide your VAT receipts...',
 'outbound', 'bounced', 'Mailbox not found. The email account does not exist.',
 NOW() - INTERVAL '5 days', NOW() - INTERVAL '5 days', NOW() - INTERVAL '5 days'),
('e3000000-0000-0000-0000-000000000002', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 NULL, 'info@demo-accounting.co.uk', 'old.email@closedcompany.com',
 'Annual Accounts Reminder', '<p>Your annual accounts are due...</p>', 'Your annual accounts are due...',
 'outbound', 'bounced', 'Domain not found. DNS lookup failed for closedcompany.com.',
 NOW() - INTERVAL '3 days', NOW() - INTERVAL '3 days', NOW() - INTERVAL '3 days');

-- 3b. Emails with Promised Docs
INSERT INTO emails (id, tenant_id, client_id, from_email, to_email, subject, body_html, body_text, direction, status, promised_docs, promised_date, sentiment, ai_tags, created_at) VALUES
('e3000000-0000-0000-0000-000000000003', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'e0000000-0000-0000-0000-000000000001', 'john@acme.co.uk', 'sarah@demo-accounting.co.uk',
 'RE: VAT Documents', '<p>Hi Sarah, I will send the bank statements and receipts by Friday.</p>',
 'Hi Sarah, I will send the bank statements and receipts by Friday.',
 'inbound', 'delivered', ARRAY['Bank Statement', 'VAT Receipts'], CURRENT_DATE + INTERVAL '2 days',
 'positive', '["client_response", "promise", "vat"]'::jsonb,
 NOW() - INTERVAL '1 day'),
('e3000000-0000-0000-0000-000000000004', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'e0000000-0000-0000-0000-000000000002', 'alice@techstart.io', 'info@demo-accounting.co.uk',
 'R&D Documentation', '<p>Will get the project timesheets to you next Monday without fail.</p>',
 'Will get the project timesheets to you next Monday without fail.',
 'inbound', 'delivered', ARRAY['Project Timesheets', 'R&D Report'], CURRENT_DATE + INTERVAL '4 days',
 'positive', '["client_response", "promise", "r&d"]'::jsonb,
 NOW() - INTERVAL '12 hours');

-- 3c. Emails with Sentiment & AI Tags
INSERT INTO emails (id, tenant_id, client_id, from_email, to_email, subject, body_html, body_text, direction, status, sentiment, ai_tags, created_at) VALUES
-- Frustrated client
('e3000000-0000-0000-0000-000000000005', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'e0000000-0000-0000-0000-000000000005', 'tom@jonestrading.co.uk', 'info@demo-accounting.co.uk',
 'URGENT: Where is my tax return?',
 '<p>I have been waiting for weeks! This is unacceptable. I need my tax return filed TODAY or I will find another accountant.</p>',
 'I have been waiting for weeks! This is unacceptable. I need my tax return filed TODAY or I will find another accountant.',
 'inbound', 'delivered', 'frustrated', '["urgent", "complaint", "tax_return", "escalation"]'::jsonb,
 NOW() - INTERVAL '4 hours'),
-- Neutral inquiry
('e3000000-0000-0000-0000-000000000006', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'e0000000-0000-0000-0000-000000000007', 'peter@smithco.co.uk', 'info@demo-accounting.co.uk',
 'Question about VAT registration',
 '<p>Hello, I wanted to ask about the VAT registration threshold and whether we need to register.</p>',
 'Hello, I wanted to ask about the VAT registration threshold and whether we need to register.',
 'inbound', 'delivered', 'neutral', '["inquiry", "vat", "registration"]'::jsonb,
 NOW() - INTERVAL '2 hours'),
-- Positive feedback
('e3000000-0000-0000-0000-000000000007', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'e0000000-0000-0000-0000-000000000002', 'alice@techstart.io', 'sarah@demo-accounting.co.uk',
 'Thank you!',
 '<p>Just wanted to say thank you for your help with the R&D claim. The refund came through and we are very happy!</p>',
 'Just wanted to say thank you for your help with the R&D claim. The refund came through and we are very happy!',
 'inbound', 'delivered', 'positive', '["thank_you", "feedback", "r&d", "success"]'::jsonb,
 NOW() - INTERVAL '6 hours');

-- 3d. Claimed Inbox Emails (Shared inbox triage)
INSERT INTO emails (id, tenant_id, client_id, from_email, to_email, subject, body_html, body_text, direction, status, claimed_by, claimed_at, created_at) VALUES
('e3000000-0000-0000-0000-000000000008', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'e0000000-0000-0000-0000-000000000004', 'bob@greenenergy.com', 'info@demo-accounting.co.uk',
 'Payroll Query',
 '<p>Hi, I have a question about the PAYE calculations for this month.</p>',
 'Hi, I have a question about the PAYE calculations for this month.',
 'inbound', 'delivered', 'b0000000-0000-0000-0000-000000000005', NOW() - INTERVAL '1 hour',
 NOW() - INTERVAL '3 hours'),
('e3000000-0000-0000-0000-000000000009', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'e0000000-0000-0000-0000-000000000009', 'mary@healthy.co.uk', 'info@demo-accounting.co.uk',
 'Invoice copies needed',
 '<p>Could you send me copies of the invoices from Q2 please?</p>',
 'Could you send me copies of the invoices from Q2 please?',
 'inbound', 'delivered', 'b0000000-0000-0000-0000-000000000002', NOW() - INTERVAL '30 minutes',
 NOW() - INTERVAL '5 hours'),
-- Unclaimed emails (claimed_by IS NULL)
('e3000000-0000-0000-0000-000000000010', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 NULL, 'frank@latepayers.com', 'info@demo-accounting.co.uk',
 'Payment arrangement',
 '<p>I need to discuss a payment plan for outstanding fees.</p>',
 'I need to discuss a payment plan for outstanding fees.',
 'inbound', 'delivered', NULL, NULL,
 NOW() - INTERVAL '1 hour');

-- 3e. Unknown Sender (no matching client)
INSERT INTO emails (id, tenant_id, client_id, from_email, to_email, subject, body_html, body_text, direction, status, sentiment, ai_tags, created_at) VALUES
('e3000000-0000-0000-0000-000000000011', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 NULL, 'random.person@gmail.com', 'info@demo-accounting.co.uk',
 'New client inquiry',
 '<p>Hi, I am looking for an accountant for my small business. Can you help?</p>',
 'Hi, I am looking for an accountant for my small business. Can you help?',
 'inbound', 'delivered', 'positive', '["new_inquiry", "prospective_client"]'::jsonb,
 NOW() - INTERVAL '30 minutes'),
('e3000000-0000-0000-0000-000000000012', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 NULL, 'suspicious@unknown-domain.ru', 'info@demo-accounting.co.uk',
 'URGENT BUSINESS PROPOSAL',
 '<p>Dear Sir, I have an urgent business proposal worth millions...</p>',
 'Dear Sir, I have an urgent business proposal worth millions...',
 'inbound', 'delivered', 'neutral', '["spam_suspected", "unknown_sender"]'::jsonb,
 NOW() - INTERVAL '2 hours');

-- =============================================================================
-- 4. TENANT BRANDING (White-Label)
-- =============================================================================
UPDATE tenants SET
  logo_url = 'https://fzco-uploads.oss-eu-west-1.aliyuncs.com/a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11/branding/logo.png',
  favicon_url = 'https://fzco-uploads.oss-eu-west-1.aliyuncs.com/a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11/branding/favicon.ico',
  primary_color = '#2563EB',
  secondary_color = '#1E40AF'
WHERE id = 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11';

-- =============================================================================
-- 5. SECURITY & ACTIVE SESSIONS
-- =============================================================================

-- 5a. Active Sessions
INSERT INTO sessions (id, tenant_id, user_id, token_hash, ip_address, user_agent, expires_at, created_at) VALUES
-- Admin sessions (2 devices)
('ae000000-0000-0000-0000-000000000001', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'b0000000-0000-0000-0000-000000000001',
 'hash_admin_chrome_desktop',
 '192.168.1.100', 'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) Chrome/120.0',
 NOW() + INTERVAL '7 days', NOW() - INTERVAL '2 hours'),
('ae000000-0000-0000-0000-000000000002', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'b0000000-0000-0000-0000-000000000001',
 'hash_admin_iphone',
 '86.12.45.200', 'Mozilla/5.0 (iPhone; CPU iPhone OS 17_0) Safari/605.1',
 NOW() + INTERVAL '7 days', NOW() - INTERVAL '1 day'),
-- Sarah sessions (3 devices)
('ae000000-0000-0000-0000-000000000003', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'b0000000-0000-0000-0000-000000000002',
 'hash_sarah_chrome',
 '192.168.1.101', 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) Chrome/120.0',
 NOW() + INTERVAL '7 days', NOW() - INTERVAL '30 minutes'),
('ae000000-0000-0000-0000-000000000004', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'b0000000-0000-0000-0000-000000000002',
 'hash_sarah_firefox',
 '192.168.1.101', 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) Firefox/121.0',
 NOW() + INTERVAL '7 days', NOW() - INTERVAL '3 days'),
('ae000000-0000-0000-0000-000000000005', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'b0000000-0000-0000-0000-000000000002',
 'hash_sarah_ipad',
 '82.45.123.78', 'Mozilla/5.0 (iPad; CPU OS 17_0) Safari/605.1',
 NOW() + INTERVAL '7 days', NOW() - INTERVAL '5 hours');

-- 5b. 2FA Enabled User (Sarah)
UPDATE users SET
  totp_secret = 'JBSWY3DPEHPK3PXP'
WHERE id = 'b0000000-0000-0000-0000-000000000002';

-- 5c. TOTP Backup Codes for Sarah (no tenant_id column)
INSERT INTO totp_backup_codes (id, user_id, code_hash, used_at) VALUES
('af000000-0000-0000-0000-000000000001', 'b0000000-0000-0000-0000-000000000002', 'hash_backup_code_1', NULL),
('af000000-0000-0000-0000-000000000002', 'b0000000-0000-0000-0000-000000000002', 'hash_backup_code_2', NULL),
('af000000-0000-0000-0000-000000000003', 'b0000000-0000-0000-0000-000000000002', 'hash_backup_code_3', NULL),
('af000000-0000-0000-0000-000000000004', 'b0000000-0000-0000-0000-000000000002', 'hash_backup_code_4', NULL),
('af000000-0000-0000-0000-000000000005', 'b0000000-0000-0000-0000-000000000002', 'hash_backup_code_5', NULL),
('af000000-0000-0000-0000-000000000006', 'b0000000-0000-0000-0000-000000000002', 'hash_backup_code_6', NOW() - INTERVAL '10 days'),
('af000000-0000-0000-0000-000000000007', 'b0000000-0000-0000-0000-000000000002', 'hash_backup_code_7', NULL),
('af000000-0000-0000-0000-000000000008', 'b0000000-0000-0000-0000-000000000002', 'hash_backup_code_8', NULL),
('af000000-0000-0000-0000-000000000009', 'b0000000-0000-0000-0000-000000000002', 'hash_backup_code_9', NULL),
('af000000-0000-0000-0000-000000000010', 'b0000000-0000-0000-0000-000000000002', 'hash_backup_code_10', NULL);

-- =============================================================================
-- 6. EXTENDED AUDIT LOGS (~120 more to reach 150 total)
-- =============================================================================
INSERT INTO audit_logs (id, tenant_id, user_id, action, entity_type, entity_id, old_value, new_value, ip_address, user_agent, severity, created_at)
SELECT
  uuid_generate_v4(),
  'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
  (ARRAY['b0000000-0000-0000-0000-000000000001', 'b0000000-0000-0000-0000-000000000002', 'b0000000-0000-0000-0000-000000000003', 'b0000000-0000-0000-0000-000000000004', 'b0000000-0000-0000-0000-000000000005'])[floor(random() * 5 + 1)::int]::uuid,
  (ARRAY['user.login', 'user.logout', 'document.uploaded', 'document.approved', 'document.rejected', 'service.created', 'service.updated', 'service.completed', 'client.created', 'client.updated', 'chase.sent', 'chase.bulk_sent', 'email.sent', 'email.received', 'settings.updated', 'admin.impersonate'])[floor(random() * 16 + 1)::int],
  (ARRAY['user', 'document', 'service', 'client', 'chase_log', 'email', 'settings'])[floor(random() * 7 + 1)::int],
  uuid_generate_v4(),
  NULL,
  '{"change": "audit log entry"}'::jsonb,
  '192.168.1.' || floor(random() * 255 + 1)::int,
  'Mozilla/5.0 Chrome/120.0',
  (ARRAY['info', 'info', 'info', 'warning', 'critical'])[floor(random() * 5 + 1)::int]::audit_severity,
  NOW() - (floor(random() * 30)::int || ' days')::interval - (floor(random() * 24)::int || ' hours')::interval
FROM generate_series(1, 120);

-- =============================================================================
-- 7. MORE AI JOBS (Processing & Failed states)
-- =============================================================================
INSERT INTO ai_jobs (id, tenant_id, user_id, type, status, payload, result, error, document_id, client_id, created_at, started_at, completed_at) VALUES
-- More processing jobs
('ef000000-0000-0000-0000-000000000001', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'b0000000-0000-0000-0000-000000000002', 'classification', 'processing',
 '{"document_id": "ac000000-0000-0000-0000-000000000003"}',
 NULL, NULL, 'ac000000-0000-0000-0000-000000000003', NULL,
 NOW() - INTERVAL '2 minutes', NOW() - INTERVAL '1 minute', NULL),
('ef000000-0000-0000-0000-000000000002', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'b0000000-0000-0000-0000-000000000001', 'summary', 'processing',
 '{"document_id": "ab000000-0000-0000-0000-000000000001"}',
 NULL, NULL, 'ab000000-0000-0000-0000-000000000001', NULL,
 NOW() - INTERVAL '3 minutes', NOW() - INTERVAL '2 minutes', NULL),
-- More failed jobs (DLQ testing)
('ef000000-0000-0000-0000-000000000003', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'b0000000-0000-0000-0000-000000000003', 'ocr', 'failed',
 '{"document_path": "uploads/encrypted.pdf"}',
 NULL, 'OCR failed: Document is password protected',
 NULL, 'e0000000-0000-0000-0000-000000000003',
 NOW() - INTERVAL '2 hours', NOW() - INTERVAL '2 hours', NOW() - INTERVAL '2 hours' + INTERVAL '15 seconds'),
('ef000000-0000-0000-0000-000000000004', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'b0000000-0000-0000-0000-000000000002', 'email_classification', 'failed',
 '{"email_id": "e3000000-0000-0000-0000-000000000011"}',
 NULL, 'Classification failed: Unable to determine client match',
 NULL, NULL,
 NOW() - INTERVAL '1 hour', NOW() - INTERVAL '1 hour', NOW() - INTERVAL '1 hour' + INTERVAL '8 seconds'),
('ef000000-0000-0000-0000-000000000005', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
 'b0000000-0000-0000-0000-000000000001', 'summary', 'failed',
 '{"document_id": "ad000000-0000-0000-0000-000000000001"}',
 NULL, 'Summary generation failed: API rate limit exceeded. Retry in 60 seconds.',
 'ad000000-0000-0000-0000-000000000001', NULL,
 NOW() - INTERVAL '45 minutes', NOW() - INTERVAL '45 minutes', NOW() - INTERVAL '45 minutes' + INTERVAL '5 seconds');

-- =============================================================================
-- VERIFICATION
-- =============================================================================
DO $$
DECLARE
  tenant_count INT;
  new_docs INT;
  new_emails INT;
  sessions_count INT;
  audit_count INT;
BEGIN
  SELECT COUNT(*) INTO tenant_count FROM tenants;
  SELECT COUNT(*) INTO new_docs FROM documents WHERE id LIKE 'a%000000-0000-0000-0000-00000000000%' AND id NOT LIKE 'aa%';
  SELECT COUNT(*) INTO new_emails FROM emails WHERE id LIKE 'e3%';
  SELECT COUNT(*) INTO sessions_count FROM sessions WHERE tenant_id = 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11';
  SELECT COUNT(*) INTO audit_count FROM audit_logs WHERE tenant_id = 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11';

  RAISE NOTICE '';
  RAISE NOTICE '════════════════════════════════════════════════════════════════';
  RAISE NOTICE '  EDGE CASES SEED DATA SUMMARY';
  RAISE NOTICE '════════════════════════════════════════════════════════════════';
  RAISE NOTICE '';
  RAISE NOTICE '  SUPER ADMIN:';
  RAISE NOTICE '    Total Tenants:     % (was 1, added 3)', tenant_count;
  RAISE NOTICE '';
  RAISE NOTICE '  DOCUMENT EDGE CASES:';
  RAISE NOTICE '    Firm Docs:         5 (client_id IS NULL)';
  RAISE NOTICE '    Version Chain:     3 (V1 -> V2 -> V3)';
  RAISE NOTICE '    Expiring Docs:     3 (yesterday, 3 days, 7 days)';
  RAISE NOTICE '    AI Extracted:      5 documents with JSONB data';
  RAISE NOTICE '';
  RAISE NOTICE '  EMAIL AI:';
  RAISE NOTICE '    Bounced:           2 emails';
  RAISE NOTICE '    Promised Docs:     2 emails';
  RAISE NOTICE '    Sentiment Tags:    3 emails (frustrated, neutral, positive)';
  RAISE NOTICE '    Claimed Inbox:     2 claimed + 1 unclaimed';
  RAISE NOTICE '    Unknown Sender:    2 emails';
  RAISE NOTICE '';
  RAISE NOTICE '  SECURITY:';
  RAISE NOTICE '    Active Sessions:   %', sessions_count;
  RAISE NOTICE '    2FA User:          Sarah (totp_secret set)';
  RAISE NOTICE '    Backup Codes:      10 (1 used)';
  RAISE NOTICE '';
  RAISE NOTICE '  AUDIT LOGS:          % (target: 150+)', audit_count;
  RAISE NOTICE '';
  RAISE NOTICE '════════════════════════════════════════════════════════════════';
  RAISE NOTICE '  ✓ Edge cases seed data inserted successfully!';
  RAISE NOTICE '════════════════════════════════════════════════════════════════';
END $$;
