-- ============================================================================
-- Accountant CRM - Initial Schema (v3.2)
-- Generated from db_final.md
-- 37 Tables (36 PostgreSQL + 1 MongoDB reference)
-- ============================================================================

-- Enable required extensions
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- ============================================================================
-- ENUMS
-- ============================================================================

CREATE TYPE user_role AS ENUM ('super_admin', 'tenant_admin', 'staff', 'client');
CREATE TYPE user_status AS ENUM ('pending', 'active', 'inactive');
CREATE TYPE client_status AS ENUM ('active', 'inactive', 'archived');
CREATE TYPE client_email_status AS ENUM ('active', 'unsubscribed', 'bounced', 'complained');
CREATE TYPE document_status AS ENUM ('requested', 'uploaded', 'pending_review', 'approved', 'rejected');
CREATE TYPE document_access_level AS ENUM ('admin', 'all_staff', 'specific');
CREATE TYPE service_status AS ENUM ('not_started', 'in_progress', 'review', 'waiting', 'completed', 'cancelled');
CREATE TYPE service_priority AS ENUM ('low', 'normal', 'high', 'urgent');
CREATE TYPE risk_level AS ENUM ('low', 'medium', 'high');
CREATE TYPE deadline_pattern AS ENUM ('monthly', 'quarterly', 'annual', 'custom');
CREATE TYPE email_direction AS ENUM ('inbound', 'outbound');
CREATE TYPE email_type AS ENUM ('chase', 'notification', 'invite', 'manual');
CREATE TYPE email_status AS ENUM ('queued', 'sent', 'delivered', 'opened', 'clicked', 'bounced', 'complained');
CREATE TYPE email_template_type AS ENUM ('chase', 'notification', 'welcome', 'custom');
CREATE TYPE email_account_type AS ENUM ('shared', 'personal');
CREATE TYPE email_account_provider AS ENUM ('imap', 'google', 'microsoft', 'zoho');
CREATE TYPE email_account_status AS ENUM ('active', 'error', 'disconnected');
CREATE TYPE notification_type AS ENUM ('document', 'deadline', 'email', 'system', 'reminder');
CREATE TYPE director_role AS ENUM ('director', 'secretary');
CREATE TYPE psc_ownership AS ENUM ('75%+', '50-75%', '25-50%');
CREATE TYPE e_sign_status AS ENUM ('pending', 'signed', 'expired', 'declined');
CREATE TYPE push_platform AS ENUM ('ios', 'android', 'web');
CREATE TYPE reminder_status AS ENUM ('pending', 'sent', 'dismissed');
CREATE TYPE audit_severity AS ENUM ('info', 'warning', 'critical');
CREATE TYPE subscription_status AS ENUM ('trialing', 'active', 'past_due', 'canceled', 'unpaid');
CREATE TYPE ai_job_status AS ENUM ('pending', 'processing', 'completed', 'failed');

-- ============================================================================
-- TABLE 29: tenants (must be first - referenced by all other tables)
-- ============================================================================

CREATE TABLE tenants (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(255) NOT NULL,
    domain VARCHAR(255) UNIQUE NOT NULL,
    custom_domain VARCHAR(255) UNIQUE,
    plan VARCHAR(50) DEFAULT 'starter',
    logo_url VARCHAR(500),
    favicon_url VARCHAR(500),
    primary_color VARCHAR(7),
    secondary_color VARCHAR(7),
    timezone VARCHAR(50) DEFAULT 'Europe/London',
    is_active BOOLEAN DEFAULT true,
    deleted_at TIMESTAMP,
    settings JSONB DEFAULT '{}',
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX idx_tenants_domain ON tenants(domain);
CREATE INDEX idx_tenants_custom_domain ON tenants(custom_domain) WHERE custom_domain IS NOT NULL;
CREATE INDEX idx_tenants_deleted ON tenants(deleted_at) WHERE deleted_at IS NOT NULL;
CREATE INDEX idx_tenants_is_active ON tenants(is_active) WHERE is_active = TRUE;

-- ============================================================================
-- TABLE 1: users
-- ============================================================================

CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID REFERENCES tenants(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    email VARCHAR(255) NOT NULL,
    password VARCHAR(255),
    role user_role NOT NULL DEFAULT 'client',
    status user_status DEFAULT 'pending',
    avatar_url VARCHAR(500),
    phone VARCHAR(50),
    invite_token VARCHAR(255),
    invite_expires TIMESTAMP,
    totp_secret VARCHAR(255),
    specialism VARCHAR(255),
    notes TEXT,
    reset_token VARCHAR(255),
    reset_token_expires TIMESTAMP,
    preferences JSONB DEFAULT '{}',
    last_login_at TIMESTAMP,
    failed_login_attempts INTEGER DEFAULT 0,
    locked_until TIMESTAMP,
    deleted_at TIMESTAMP,
    anonymized_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),

    CONSTRAINT chk_role_values CHECK (role IN ('super_admin', 'tenant_admin', 'staff', 'client')),
    CONSTRAINT chk_role_tenant_consistency CHECK (
        (role = 'super_admin' AND tenant_id IS NULL) OR
        (role != 'super_admin' AND tenant_id IS NOT NULL)
    )
);

CREATE UNIQUE INDEX idx_users_email_tenant ON users(tenant_id, email) WHERE tenant_id IS NOT NULL;
CREATE UNIQUE INDEX idx_users_email_super ON users(email) WHERE tenant_id IS NULL;
CREATE INDEX idx_users_tenant_role ON users(tenant_id, role) WHERE tenant_id IS NOT NULL;

-- ============================================================================
-- TABLE 2: clients
-- ============================================================================

CREATE TABLE clients (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    company_name VARCHAR(255) NOT NULL,
    contact_name VARCHAR(255) NOT NULL,
    email VARCHAR(255) NOT NULL,
    phone VARCHAR(50),
    address TEXT,
    year_end DATE,
    utr VARCHAR(20),
    company_number VARCHAR(20),
    company_type VARCHAR(50),
    incorporation_date DATE,
    sic_codes JSONB,
    vat_number VARCHAR(20),
    vat_quarter VARCHAR(10),
    status client_status DEFAULT 'active',
    risk_score INTEGER,
    tags JSONB DEFAULT '[]',
    email_status client_email_status DEFAULT 'active',
    email_status_at TIMESTAMP,
    alternate_emails JSONB DEFAULT '[]',
    ni_number_encrypted BYTEA,
    bank_details_encrypted BYTEA,
    anonymized_at TIMESTAMP,
    last_contact_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX idx_clients_tenant_status ON clients(tenant_id, status);
CREATE INDEX idx_clients_email_status ON clients(email_status);

-- ============================================================================
-- TABLE 3: staff_clients
-- ============================================================================

CREATE TABLE staff_clients (
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    staff_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    client_id UUID NOT NULL REFERENCES clients(id) ON DELETE CASCADE,
    is_primary BOOLEAN DEFAULT false,
    created_at TIMESTAMP DEFAULT NOW(),

    PRIMARY KEY (staff_id, client_id)
);

CREATE UNIQUE INDEX idx_staff_clients_primary ON staff_clients(client_id) WHERE is_primary = true;
CREATE INDEX idx_staff_clients_tenant ON staff_clients(tenant_id);
CREATE INDEX idx_staff_clients_staff ON staff_clients(staff_id, is_primary);

-- ============================================================================
-- TABLE 8: document_types
-- ============================================================================

CREATE TABLE document_types (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID REFERENCES tenants(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    has_expiry BOOLEAN DEFAULT false,
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT NOW()
);

-- ============================================================================
-- TABLE 9: service_types
-- ============================================================================

CREATE TABLE service_types (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID REFERENCES tenants(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    deadline_pattern deadline_pattern,
    deadline_offset INTEGER,
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT NOW()
);

-- ============================================================================
-- TABLE 6: services
-- ============================================================================

CREATE TABLE services (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    client_id UUID REFERENCES clients(id) ON DELETE CASCADE,
    staff_id UUID REFERENCES users(id) ON DELETE SET NULL,
    type_id UUID REFERENCES service_types(id) ON DELETE SET NULL,
    name VARCHAR(255) NOT NULL,
    period VARCHAR(50),
    status service_status DEFAULT 'not_started',
    priority service_priority DEFAULT 'normal',
    risk_level risk_level DEFAULT 'low',
    deadline DATE,
    kanban_position INTEGER DEFAULT 0,
    docs_required INTEGER DEFAULT 0,
    docs_received INTEGER DEFAULT 0,
    hmrc_reference VARCHAR(100),
    hmrc_data JSONB,
    filed_at TIMESTAMP,
    completed_at TIMESTAMP,
    completion_notes TEXT,
    version INTEGER DEFAULT 1,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),

    CONSTRAINT chk_docs_received CHECK (docs_received >= 0),
    CONSTRAINT chk_docs_required CHECK (docs_required >= 0),
    CONSTRAINT chk_docs_balance CHECK (docs_received <= docs_required)
);

CREATE INDEX idx_services_kanban ON services(staff_id, status, kanban_position);
CREATE INDEX idx_services_tenant_deadline ON services(tenant_id, staff_id, deadline, status);
CREATE INDEX idx_services_client ON services(client_id, status);

-- ============================================================================
-- TABLE 4: documents
-- ============================================================================

CREATE TABLE documents (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    client_id UUID REFERENCES clients(id) ON DELETE CASCADE,
    service_id UUID REFERENCES services(id) ON DELETE SET NULL,
    uploaded_by UUID REFERENCES users(id) ON DELETE SET NULL,
    type_id UUID REFERENCES document_types(id) ON DELETE SET NULL,
    name VARCHAR(255) NOT NULL,
    original_name VARCHAR(255) NOT NULL,
    file_path VARCHAR(500),
    file_size INTEGER,
    mime_type VARCHAR(100),
    status document_status DEFAULT 'uploaded',
    access document_access_level DEFAULT 'all_staff',
    version INTEGER DEFAULT 1,
    parent_id UUID REFERENCES documents(id) ON DELETE SET NULL,
    requested_at TIMESTAMP,
    expiry_date DATE,
    request_note TEXT,
    upload_note TEXT,
    review_note TEXT,
    reviewed_by UUID REFERENCES users(id) ON DELETE SET NULL,
    reviewed_at TIMESTAMP,
    chase_count INTEGER DEFAULT 0,
    last_chased_at TIMESTAMP,
    ai_summary TEXT,
    ai_extracted JSONB,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX idx_documents_tenant_status ON documents(tenant_id, client_id, status);
CREATE INDEX idx_documents_service ON documents(service_id);
CREATE INDEX idx_documents_ai_extracted ON documents USING GIN(ai_extracted);

-- ============================================================================
-- TABLE 5: document_access
-- ============================================================================

CREATE TABLE document_access (
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    document_id UUID NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
    staff_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    granted_by UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMP DEFAULT NOW(),

    PRIMARY KEY (document_id, staff_id)
);

CREATE INDEX idx_document_access_tenant ON document_access(tenant_id);

-- ============================================================================
-- TABLE 7: sessions
-- ============================================================================

CREATE TABLE sessions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID REFERENCES tenants(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash VARCHAR(255) NOT NULL,
    token_family UUID,
    ip_address VARCHAR(45),
    user_agent TEXT,
    revoked_at TIMESTAMP,
    rotated_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT NOW(),
    expires_at TIMESTAMP NOT NULL,
    last_active_at TIMESTAMP DEFAULT NOW()
);

CREATE UNIQUE INDEX idx_sessions_token_hash ON sessions(token_hash);
CREATE INDEX idx_sessions_user_active ON sessions(user_id, expires_at) WHERE revoked_at IS NULL;
CREATE INDEX idx_sessions_token_family ON sessions(token_family) WHERE token_family IS NOT NULL;

-- ============================================================================
-- TABLE 10: service_requirements
-- ============================================================================

CREATE TABLE service_requirements (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID REFERENCES tenants(id) ON DELETE CASCADE,
    service_type_id UUID NOT NULL REFERENCES service_types(id) ON DELETE CASCADE,
    document_type_id UUID NOT NULL REFERENCES document_types(id) ON DELETE CASCADE,
    is_mandatory BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX idx_service_requirements_tenant ON service_requirements(tenant_id);

-- ============================================================================
-- TABLE 11: company_settings
-- ============================================================================

CREATE TABLE company_settings (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL UNIQUE REFERENCES tenants(id) ON DELETE CASCADE,
    firm_name VARCHAR(255) NOT NULL,
    email VARCHAR(255) NOT NULL,
    phone VARCHAR(50),
    address TEXT,
    logo_url VARCHAR(500),
    stripe_account_id VARCHAR(100),
    stripe_connected BOOLEAN DEFAULT false,
    ch_api_key_ref VARCHAR(255),
    resend_api_key_ref VARCHAR(255),
    reminder_rules JSONB DEFAULT '{"day3": true, "day7": true, "day14": true}',
    updated_by UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- ============================================================================
-- TABLE 25: email_threads
-- ============================================================================

CREATE TABLE email_threads (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    thread_key VARCHAR(255) UNIQUE NOT NULL,
    client_id UUID REFERENCES clients(id) ON DELETE SET NULL,
    first_email_id UUID,
    subject VARCHAR(500) NOT NULL,
    participants JSONB DEFAULT '[]',
    last_message_at TIMESTAMP,
    message_count INTEGER DEFAULT 1,
    ai_summary TEXT,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),

    CONSTRAINT chk_message_count CHECK (message_count >= 1)
);

-- ============================================================================
-- TABLE 13: email_templates
-- ============================================================================

CREATE TABLE email_templates (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID REFERENCES tenants(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    subject VARCHAR(500) NOT NULL,
    body_html TEXT NOT NULL,
    body_text TEXT,
    type email_template_type DEFAULT 'custom',
    category VARCHAR(100),
    placeholders JSONB DEFAULT '[]',
    is_default BOOLEAN DEFAULT false,
    is_active BOOLEAN DEFAULT true,
    created_by UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- ============================================================================
-- TABLE 12: emails (PARTITIONED)
-- ============================================================================

CREATE TABLE emails (
    id UUID NOT NULL,
    tenant_id UUID NOT NULL,
    client_id UUID,
    staff_id UUID,
    template_id UUID,
    thread_id VARCHAR(255),
    reply_to_id UUID,
    direction email_direction NOT NULL,
    to_email VARCHAR(255) NOT NULL,
    to_name VARCHAR(255),
    from_email VARCHAR(255) NOT NULL,
    subject VARCHAR(500) NOT NULL,
    body_html TEXT NOT NULL,
    body_text TEXT,
    attachments JSONB DEFAULT '[]',
    raw_oss_uri VARCHAR(500),
    type email_type DEFAULT 'manual',
    status email_status DEFAULT 'queued',
    resend_id VARCHAR(100),
    is_read BOOLEAN DEFAULT false,
    claimed_by UUID,
    claimed_at TIMESTAMP,
    promised_docs TEXT[],
    promised_date DATE,
    ai_summary TEXT,
    ai_tags JSONB DEFAULT '[]',
    sentiment VARCHAR(20),
    action_needed TEXT,
    opened_at TIMESTAMP,
    clicked_at TIMESTAMP,
    bounced_at TIMESTAMP,
    bounce_reason TEXT,
    sent_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),

    PRIMARY KEY (id, created_at)
) PARTITION BY RANGE (created_at);

-- Create initial partitions for 2026
CREATE TABLE emails_2026_01 PARTITION OF emails FOR VALUES FROM ('2026-01-01') TO ('2026-02-01');
CREATE TABLE emails_2026_02 PARTITION OF emails FOR VALUES FROM ('2026-02-01') TO ('2026-03-01');
CREATE TABLE emails_2026_03 PARTITION OF emails FOR VALUES FROM ('2026-03-01') TO ('2026-04-01');
CREATE TABLE emails_2026_04 PARTITION OF emails FOR VALUES FROM ('2026-04-01') TO ('2026-05-01');
CREATE TABLE emails_2026_05 PARTITION OF emails FOR VALUES FROM ('2026-05-01') TO ('2026-06-01');
CREATE TABLE emails_2026_06 PARTITION OF emails FOR VALUES FROM ('2026-06-01') TO ('2026-07-01');
CREATE TABLE emails_2026_07 PARTITION OF emails FOR VALUES FROM ('2026-07-01') TO ('2026-08-01');
CREATE TABLE emails_2026_08 PARTITION OF emails FOR VALUES FROM ('2026-08-01') TO ('2026-09-01');
CREATE TABLE emails_2026_09 PARTITION OF emails FOR VALUES FROM ('2026-09-01') TO ('2026-10-01');
CREATE TABLE emails_2026_10 PARTITION OF emails FOR VALUES FROM ('2026-10-01') TO ('2026-11-01');
CREATE TABLE emails_2026_11 PARTITION OF emails FOR VALUES FROM ('2026-11-01') TO ('2026-12-01');
CREATE TABLE emails_2026_12 PARTITION OF emails FOR VALUES FROM ('2026-12-01') TO ('2027-01-01');
CREATE TABLE emails_2027_01 PARTITION OF emails FOR VALUES FROM ('2027-01-01') TO ('2027-02-01');
CREATE TABLE emails_2027_02 PARTITION OF emails FOR VALUES FROM ('2027-02-01') TO ('2027-03-01');

CREATE INDEX idx_emails_tenant_client ON emails(tenant_id, client_id, direction, created_at);
CREATE INDEX idx_emails_thread ON emails(thread_id, created_at);
CREATE INDEX idx_emails_claimed ON emails(claimed_by, created_at) WHERE claimed_by IS NOT NULL;
CREATE INDEX idx_emails_ai_tags ON emails USING GIN(ai_tags);

-- ============================================================================
-- TABLE 14: email_accounts
-- ============================================================================

CREATE TABLE email_accounts (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    email VARCHAR(255) NOT NULL,
    type email_account_type DEFAULT 'shared',
    auth_method VARCHAR(10) NOT NULL,
    provider email_account_provider DEFAULT 'imap',
    imap_host VARCHAR(255),
    imap_port INTEGER DEFAULT 993,
    imap_password VARCHAR(500),
    oauth_access_token VARCHAR(2000),
    oauth_refresh_token VARCHAR(2000),
    oauth_expires_at TIMESTAMP,
    status email_account_status DEFAULT 'active',
    last_sync_at TIMESTAMP,
    error_message TEXT,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),

    CONSTRAINT chk_auth_method CHECK (
        (auth_method = 'imap' AND imap_host IS NOT NULL AND imap_password IS NOT NULL) OR
        (auth_method = 'oauth' AND oauth_access_token IS NOT NULL)
    )
);

CREATE INDEX idx_email_accounts_oauth_expires ON email_accounts(oauth_expires_at) WHERE provider != 'imap';

-- ============================================================================
-- TABLE 15: audit_logs (PARTITIONED)
-- ============================================================================

CREATE TABLE audit_logs (
    id UUID NOT NULL,
    tenant_id UUID,
    user_id UUID,
    action VARCHAR(100) NOT NULL,
    entity_type VARCHAR(50) NOT NULL,
    entity_id UUID,
    old_value JSONB,
    new_value JSONB,
    metadata JSONB DEFAULT '{}',
    ip_address VARCHAR(45),
    user_agent TEXT,
    severity audit_severity DEFAULT 'info',
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),

    PRIMARY KEY (id, created_at)
) PARTITION BY RANGE (created_at);

-- Create initial partitions for 2026
CREATE TABLE audit_logs_2026_01 PARTITION OF audit_logs FOR VALUES FROM ('2026-01-01') TO ('2026-02-01');
CREATE TABLE audit_logs_2026_02 PARTITION OF audit_logs FOR VALUES FROM ('2026-02-01') TO ('2026-03-01');
CREATE TABLE audit_logs_2026_03 PARTITION OF audit_logs FOR VALUES FROM ('2026-03-01') TO ('2026-04-01');
CREATE TABLE audit_logs_2026_04 PARTITION OF audit_logs FOR VALUES FROM ('2026-04-01') TO ('2026-05-01');
CREATE TABLE audit_logs_2026_05 PARTITION OF audit_logs FOR VALUES FROM ('2026-05-01') TO ('2026-06-01');
CREATE TABLE audit_logs_2026_06 PARTITION OF audit_logs FOR VALUES FROM ('2026-06-01') TO ('2026-07-01');
CREATE TABLE audit_logs_2026_07 PARTITION OF audit_logs FOR VALUES FROM ('2026-07-01') TO ('2026-08-01');
CREATE TABLE audit_logs_2026_08 PARTITION OF audit_logs FOR VALUES FROM ('2026-08-01') TO ('2026-09-01');
CREATE TABLE audit_logs_2026_09 PARTITION OF audit_logs FOR VALUES FROM ('2026-09-01') TO ('2026-10-01');
CREATE TABLE audit_logs_2026_10 PARTITION OF audit_logs FOR VALUES FROM ('2026-10-01') TO ('2026-11-01');
CREATE TABLE audit_logs_2026_11 PARTITION OF audit_logs FOR VALUES FROM ('2026-11-01') TO ('2026-12-01');
CREATE TABLE audit_logs_2026_12 PARTITION OF audit_logs FOR VALUES FROM ('2026-12-01') TO ('2027-01-01');
CREATE TABLE audit_logs_2027_01 PARTITION OF audit_logs FOR VALUES FROM ('2027-01-01') TO ('2027-02-01');
CREATE TABLE audit_logs_2027_02 PARTITION OF audit_logs FOR VALUES FROM ('2027-02-01') TO ('2027-03-01');

CREATE INDEX idx_audit_logs_tenant ON audit_logs(tenant_id, user_id, created_at);

-- ============================================================================
-- TABLE 16: notifications
-- ============================================================================

CREATE TABLE notifications (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    type notification_type NOT NULL,
    title VARCHAR(255) NOT NULL,
    message TEXT NOT NULL,
    entity_type VARCHAR(50),
    entity_id UUID,
    link VARCHAR(500),
    is_read BOOLEAN DEFAULT false,
    remind_at TIMESTAMP,
    dismissed_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX idx_notifications_tenant_user ON notifications(tenant_id, user_id, is_read, created_at);
CREATE INDEX idx_notifications_remind ON notifications(remind_at) WHERE remind_at IS NOT NULL;

-- ============================================================================
-- TABLE 17: directors
-- ============================================================================

CREATE TABLE directors (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    client_id UUID NOT NULL REFERENCES clients(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    role director_role DEFAULT 'director',
    appointed_date DATE,
    resigned_date DATE,
    nationality VARCHAR(100),
    dob_month INTEGER,
    dob_year INTEGER,
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX idx_directors_tenant ON directors(tenant_id, client_id);
CREATE UNIQUE INDEX idx_directors_client_name_role ON directors(client_id, name, role) WHERE is_active = true;

-- ============================================================================
-- TABLE 18: psc (Persons with Significant Control)
-- ============================================================================

CREATE TABLE psc (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    client_id UUID NOT NULL REFERENCES clients(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    ownership_percentage psc_ownership,
    voting_rights VARCHAR(50),
    notified_date DATE,
    ceased_date DATE,
    nature_of_control JSONB,
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX idx_psc_tenant ON psc(tenant_id, client_id);
CREATE UNIQUE INDEX idx_psc_client_name ON psc(client_id, name) WHERE is_active = true;

-- ============================================================================
-- TABLE 19: chase_logs
-- ============================================================================

CREATE TABLE chase_logs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    initiated_by UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    total_sent INTEGER DEFAULT 0,
    delivered INTEGER DEFAULT 0,
    opened INTEGER DEFAULT 0,
    bounced INTEGER DEFAULT 0,
    created_at TIMESTAMP DEFAULT NOW(),

    CONSTRAINT chk_total_sent CHECK (total_sent >= 0),
    CONSTRAINT chk_delivered CHECK (delivered >= 0),
    CONSTRAINT chk_opened CHECK (opened >= 0),
    CONSTRAINT chk_bounced CHECK (bounced >= 0)
);

-- ============================================================================
-- TABLE 26: chase_log_clients
-- ============================================================================

CREATE TABLE chase_log_clients (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    chase_log_id UUID NOT NULL REFERENCES chase_logs(id) ON DELETE CASCADE,
    client_id UUID NOT NULL REFERENCES clients(id) ON DELETE CASCADE,
    email_id UUID,
    created_at TIMESTAMP DEFAULT NOW()
);

-- ============================================================================
-- TABLE 20: e_sign_requests
-- ============================================================================

CREATE TABLE e_sign_requests (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    client_id UUID NOT NULL REFERENCES clients(id) ON DELETE CASCADE,
    document_id UUID REFERENCES documents(id) ON DELETE SET NULL,
    template_type VARCHAR(50) NOT NULL,
    status e_sign_status DEFAULT 'pending',
    signer_email VARCHAR(255) NOT NULL,
    signer_name VARCHAR(255),
    sent_at TIMESTAMP,
    signed_at TIMESTAMP,
    expires_at TIMESTAMP,
    signature_data JSONB,
    auto_create_service BOOLEAN DEFAULT false,
    service_type_id UUID REFERENCES service_types(id) ON DELETE SET NULL,
    created_service_id UUID REFERENCES services(id) ON DELETE SET NULL,
    created_at TIMESTAMP DEFAULT NOW()
);

-- ============================================================================
-- TABLE 21: push_tokens
-- ============================================================================

CREATE TABLE push_tokens (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token VARCHAR(500) NOT NULL,
    platform push_platform NOT NULL,
    is_active BOOLEAN DEFAULT true,
    last_used_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX idx_push_tokens_tenant ON push_tokens(tenant_id, user_id);

-- ============================================================================
-- TABLE 22: upload_tokens
-- ============================================================================

CREATE TABLE upload_tokens (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    client_id UUID NOT NULL REFERENCES clients(id) ON DELETE CASCADE,
    document_id UUID REFERENCES documents(id) ON DELETE CASCADE,
    token VARCHAR(255) UNIQUE NOT NULL,
    expires_at TIMESTAMP NOT NULL,
    used_at TIMESTAMP,
    ip_address VARCHAR(45),
    created_at TIMESTAMP DEFAULT NOW()
);

-- ============================================================================
-- TABLE 23: magic_link_tokens
-- ============================================================================

CREATE TABLE magic_link_tokens (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash VARCHAR(255) NOT NULL,
    expires_at TIMESTAMP NOT NULL,
    used_at TIMESTAMP,
    ip_address VARCHAR(45),
    created_at TIMESTAMP DEFAULT NOW()
);

-- ============================================================================
-- TABLE 24: reminders
-- ============================================================================

CREATE TABLE reminders (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    client_id UUID REFERENCES clients(id) ON DELETE CASCADE,
    email_id UUID,
    document_id UUID REFERENCES documents(id) ON DELETE CASCADE,
    service_id UUID REFERENCES services(id) ON DELETE CASCADE,
    remind_at TIMESTAMP NOT NULL,
    reason TEXT,
    status reminder_status DEFAULT 'pending',
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),

    CONSTRAINT chk_has_entity CHECK (
        client_id IS NOT NULL OR
        email_id IS NOT NULL OR
        document_id IS NOT NULL OR
        service_id IS NOT NULL
    ),
    CONSTRAINT chk_single_entity CHECK (
        (email_id IS NOT NULL)::int +
        (document_id IS NOT NULL)::int +
        (service_id IS NOT NULL)::int <= 1
    )
);

CREATE INDEX idx_reminders_tenant_pending ON reminders(tenant_id, remind_at, status) WHERE status = 'pending';

-- ============================================================================
-- TABLE 27: client_notes
-- ============================================================================

CREATE TABLE client_notes (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    client_id UUID NOT NULL REFERENCES clients(id) ON DELETE CASCADE,
    staff_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    note TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- ============================================================================
-- TABLE 30: tenant_subscriptions
-- ============================================================================

CREATE TABLE tenant_subscriptions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID UNIQUE NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    stripe_customer_id VARCHAR(100) NOT NULL,
    stripe_subscription_id VARCHAR(100),
    plan VARCHAR(50) DEFAULT 'starter',
    status subscription_status DEFAULT 'trialing',
    current_period_start TIMESTAMP,
    current_period_end TIMESTAMP,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- ============================================================================
-- TABLE 31: tenant_invoices
-- ============================================================================

CREATE TABLE tenant_invoices (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    stripe_invoice_id VARCHAR(100) UNIQUE NOT NULL,
    amount_cents INTEGER NOT NULL,
    currency VARCHAR(3) DEFAULT 'GBP',
    status VARCHAR(20) DEFAULT 'draft',
    invoice_pdf_url VARCHAR(500),
    period_start TIMESTAMP,
    period_end TIMESTAMP,
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX idx_tenant_invoices_tenant ON tenant_invoices(tenant_id, created_at);
CREATE INDEX idx_tenant_invoices_stripe ON tenant_invoices(stripe_invoice_id);

-- ============================================================================
-- TABLE 32: webhook_idempotency
-- ============================================================================

CREATE TABLE webhook_idempotency (
    provider VARCHAR(20) NOT NULL,
    event_id VARCHAR(255) NOT NULL,
    processed_at TIMESTAMP DEFAULT NOW(),
    tenant_id UUID REFERENCES tenants(id) ON DELETE CASCADE,

    PRIMARY KEY (provider, event_id)
);

CREATE INDEX idx_webhook_idempotency_cleanup ON webhook_idempotency(processed_at);

-- ============================================================================
-- TABLE 33: refresh_tokens
-- ============================================================================

CREATE TABLE refresh_tokens (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID REFERENCES tenants(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash VARCHAR(64) UNIQUE NOT NULL,
    family UUID NOT NULL,
    parent_token_hash VARCHAR(64),
    ip_address VARCHAR(45),
    user_agent TEXT,
    revoked_at TIMESTAMP,
    used_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT NOW(),
    expires_at TIMESTAMP NOT NULL
);

CREATE UNIQUE INDEX idx_refresh_tokens_hash ON refresh_tokens(token_hash);
CREATE INDEX idx_refresh_tokens_user ON refresh_tokens(user_id, revoked_at) WHERE revoked_at IS NULL;
CREATE INDEX idx_refresh_tokens_family ON refresh_tokens(family);
CREATE INDEX idx_refresh_tokens_expires ON refresh_tokens(expires_at) WHERE revoked_at IS NULL AND used_at IS NULL;

-- ============================================================================
-- TABLE 34: totp_backup_codes
-- ============================================================================

CREATE TABLE totp_backup_codes (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    code_hash VARCHAR(64) NOT NULL,
    used_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX idx_totp_backup_user ON totp_backup_codes(user_id, used_at) WHERE used_at IS NULL;

-- ============================================================================
-- TABLE 35: ai_jobs
-- ============================================================================

CREATE TABLE ai_jobs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    type VARCHAR(50) NOT NULL,
    status ai_job_status DEFAULT 'pending',
    payload JSONB NOT NULL,
    result JSONB,
    error TEXT,
    document_id UUID REFERENCES documents(id) ON DELETE SET NULL,
    email_id UUID,
    service_id UUID REFERENCES services(id) ON DELETE SET NULL,
    client_id UUID REFERENCES clients(id) ON DELETE SET NULL,
    created_at TIMESTAMP DEFAULT NOW(),
    started_at TIMESTAMP,
    completed_at TIMESTAMP
);

CREATE INDEX idx_ai_jobs_tenant_status ON ai_jobs(tenant_id, status) WHERE status IN ('pending', 'processing');
CREATE INDEX idx_ai_jobs_user ON ai_jobs(user_id, created_at DESC);

-- ============================================================================
-- TABLE 36: deletion_audit
-- ============================================================================

CREATE TABLE deletion_audit (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL,
    deleted_by UUID,
    deleted_at TIMESTAMP DEFAULT NOW(),
    reason TEXT,
    tables_affected JSONB NOT NULL,
    record_counts JSONB NOT NULL,
    confirmation_hash VARCHAR(64) NOT NULL
);

CREATE INDEX idx_deletion_audit_tenant ON deletion_audit(tenant_id);
CREATE INDEX idx_deletion_audit_date ON deletion_audit(deleted_at);

-- ============================================================================
-- TABLE 37: outbox (Transactional Event Queue)
-- ============================================================================

CREATE TABLE outbox (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    event_type VARCHAR(50) NOT NULL,
    payload JSONB NOT NULL,
    created_at TIMESTAMP DEFAULT NOW(),
    published_at TIMESTAMP,
    attempts INTEGER DEFAULT 0
);

CREATE INDEX idx_outbox_pending ON outbox(created_at) WHERE published_at IS NULL;

-- ============================================================================
-- FULL-TEXT SEARCH INDEXES
-- ============================================================================

CREATE INDEX idx_clients_search ON clients
    USING gin(to_tsvector('english', company_name || ' ' || COALESCE(contact_name, '')));
CREATE INDEX idx_documents_search ON documents
    USING gin(to_tsvector('english', name || ' ' || COALESCE(ai_summary, '')));
CREATE INDEX idx_emails_search ON emails
    USING gin(to_tsvector('english', subject || ' ' || COALESCE(body_text, '')));

-- ============================================================================
-- ADDITIONAL JSONB INDEXES
-- ============================================================================

CREATE INDEX idx_clients_tags ON clients USING GIN(tags);
CREATE INDEX idx_clients_alternate_emails ON clients USING GIN(alternate_emails);

-- ============================================================================
-- TRIGGERS & FUNCTIONS
-- ============================================================================

-- Function: Check staff and client belong to same tenant
CREATE OR REPLACE FUNCTION check_staff_client_same_tenant()
RETURNS TRIGGER AS $$
BEGIN
    IF (SELECT tenant_id FROM users WHERE id = NEW.staff_id) !=
       (SELECT tenant_id FROM clients WHERE id = NEW.client_id) THEN
        RAISE EXCEPTION 'Staff and client must belong to same tenant';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_staff_client_same_tenant
    BEFORE INSERT OR UPDATE ON staff_clients
    FOR EACH ROW EXECUTE FUNCTION check_staff_client_same_tenant();

-- Function: Update service docs_received counter
CREATE OR REPLACE FUNCTION update_service_docs_count()
RETURNS TRIGGER AS $$
BEGIN
    IF TG_OP = 'INSERT' AND NEW.service_id IS NOT NULL THEN
        UPDATE services SET docs_received = docs_received + 1
        WHERE id = NEW.service_id;
    ELSIF TG_OP = 'DELETE' AND OLD.service_id IS NOT NULL THEN
        UPDATE services SET docs_received = GREATEST(0, docs_received - 1)
        WHERE id = OLD.service_id;
    ELSIF TG_OP = 'UPDATE' AND OLD.service_id IS DISTINCT FROM NEW.service_id THEN
        IF OLD.service_id IS NOT NULL THEN
            UPDATE services SET docs_received = GREATEST(0, docs_received - 1)
            WHERE id = OLD.service_id;
        END IF;
        IF NEW.service_id IS NOT NULL THEN
            UPDATE services SET docs_received = docs_received + 1
            WHERE id = NEW.service_id;
        END IF;
    END IF;
    RETURN COALESCE(NEW, OLD);
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_update_service_docs_count
    AFTER INSERT OR UPDATE OR DELETE ON documents
    FOR EACH ROW EXECUTE FUNCTION update_service_docs_count();

-- Function: Update email thread message_count
CREATE OR REPLACE FUNCTION update_thread_message_count()
RETURNS TRIGGER AS $$
BEGIN
    IF TG_OP = 'INSERT' AND NEW.thread_id IS NOT NULL THEN
        UPDATE email_threads
        SET message_count = message_count + 1,
            last_message_at = NEW.created_at
        WHERE thread_key = NEW.thread_id;
    ELSIF TG_OP = 'DELETE' AND OLD.thread_id IS NOT NULL THEN
        UPDATE email_threads
        SET message_count = GREATEST(1, message_count - 1)
        WHERE thread_key = OLD.thread_id;
    ELSIF TG_OP = 'UPDATE' AND OLD.thread_id IS DISTINCT FROM NEW.thread_id THEN
        IF OLD.thread_id IS NOT NULL THEN
            UPDATE email_threads
            SET message_count = GREATEST(1, message_count - 1)
            WHERE thread_key = OLD.thread_id;
        END IF;
        IF NEW.thread_id IS NOT NULL THEN
            UPDATE email_threads
            SET message_count = message_count + 1,
                last_message_at = NEW.created_at
            WHERE thread_key = NEW.thread_id;
        END IF;
    END IF;
    RETURN COALESCE(NEW, OLD);
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_update_thread_message_count
    AFTER INSERT OR UPDATE OR DELETE ON emails
    FOR EACH ROW EXECUTE FUNCTION update_thread_message_count();

-- Function: Auto-update updated_at timestamp
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Apply updated_at trigger to relevant tables
CREATE TRIGGER trg_users_updated_at BEFORE UPDATE ON users FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
CREATE TRIGGER trg_clients_updated_at BEFORE UPDATE ON clients FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
CREATE TRIGGER trg_documents_updated_at BEFORE UPDATE ON documents FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
CREATE TRIGGER trg_services_updated_at BEFORE UPDATE ON services FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
CREATE TRIGGER trg_tenants_updated_at BEFORE UPDATE ON tenants FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
CREATE TRIGGER trg_company_settings_updated_at BEFORE UPDATE ON company_settings FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
CREATE TRIGGER trg_email_templates_updated_at BEFORE UPDATE ON email_templates FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
CREATE TRIGGER trg_email_accounts_updated_at BEFORE UPDATE ON email_accounts FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
CREATE TRIGGER trg_email_threads_updated_at BEFORE UPDATE ON email_threads FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
CREATE TRIGGER trg_tenant_subscriptions_updated_at BEFORE UPDATE ON tenant_subscriptions FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
CREATE TRIGGER trg_reminders_updated_at BEFORE UPDATE ON reminders FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
CREATE TRIGGER trg_client_notes_updated_at BEFORE UPDATE ON client_notes FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- ============================================================================
-- ROW-LEVEL SECURITY (RLS)
-- ============================================================================

-- Enable RLS on all tenant-scoped tables
ALTER TABLE users ENABLE ROW LEVEL SECURITY;
ALTER TABLE clients ENABLE ROW LEVEL SECURITY;
ALTER TABLE staff_clients ENABLE ROW LEVEL SECURITY;
ALTER TABLE documents ENABLE ROW LEVEL SECURITY;
ALTER TABLE document_access ENABLE ROW LEVEL SECURITY;
ALTER TABLE services ENABLE ROW LEVEL SECURITY;
ALTER TABLE sessions ENABLE ROW LEVEL SECURITY;
ALTER TABLE document_types ENABLE ROW LEVEL SECURITY;
ALTER TABLE service_types ENABLE ROW LEVEL SECURITY;
ALTER TABLE service_requirements ENABLE ROW LEVEL SECURITY;
ALTER TABLE company_settings ENABLE ROW LEVEL SECURITY;
ALTER TABLE emails ENABLE ROW LEVEL SECURITY;
ALTER TABLE email_templates ENABLE ROW LEVEL SECURITY;
ALTER TABLE email_accounts ENABLE ROW LEVEL SECURITY;
ALTER TABLE audit_logs ENABLE ROW LEVEL SECURITY;
ALTER TABLE notifications ENABLE ROW LEVEL SECURITY;
ALTER TABLE directors ENABLE ROW LEVEL SECURITY;
ALTER TABLE psc ENABLE ROW LEVEL SECURITY;
ALTER TABLE chase_logs ENABLE ROW LEVEL SECURITY;
ALTER TABLE chase_log_clients ENABLE ROW LEVEL SECURITY;
ALTER TABLE e_sign_requests ENABLE ROW LEVEL SECURITY;
ALTER TABLE push_tokens ENABLE ROW LEVEL SECURITY;
ALTER TABLE upload_tokens ENABLE ROW LEVEL SECURITY;
ALTER TABLE magic_link_tokens ENABLE ROW LEVEL SECURITY;
ALTER TABLE reminders ENABLE ROW LEVEL SECURITY;
ALTER TABLE email_threads ENABLE ROW LEVEL SECURITY;
ALTER TABLE client_notes ENABLE ROW LEVEL SECURITY;
ALTER TABLE ai_jobs ENABLE ROW LEVEL SECURITY;
ALTER TABLE tenants ENABLE ROW LEVEL SECURITY;
ALTER TABLE tenant_subscriptions ENABLE ROW LEVEL SECURITY;
ALTER TABLE tenant_invoices ENABLE ROW LEVEL SECURITY;
ALTER TABLE refresh_tokens ENABLE ROW LEVEL SECURITY;
ALTER TABLE totp_backup_codes ENABLE ROW LEVEL SECURITY;
ALTER TABLE outbox ENABLE ROW LEVEL SECURITY;

-- RLS Policies: Tenant isolation (standard tables)
CREATE POLICY tenant_isolation_clients ON clients FOR ALL
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid OR current_setting('app.role', true) = 'super_admin')
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::uuid OR current_setting('app.role', true) = 'super_admin');

CREATE POLICY tenant_isolation_documents ON documents FOR ALL
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid OR current_setting('app.role', true) = 'super_admin')
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::uuid OR current_setting('app.role', true) = 'super_admin');

CREATE POLICY tenant_isolation_services ON services FOR ALL
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid OR current_setting('app.role', true) = 'super_admin')
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::uuid OR current_setting('app.role', true) = 'super_admin');

CREATE POLICY tenant_isolation_emails ON emails FOR ALL
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid OR current_setting('app.role', true) = 'super_admin')
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::uuid OR current_setting('app.role', true) = 'super_admin');

CREATE POLICY tenant_isolation_notifications ON notifications FOR ALL
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid OR current_setting('app.role', true) = 'super_admin')
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::uuid OR current_setting('app.role', true) = 'super_admin');

-- RLS Policies: Users (tenant + role consistency)
CREATE POLICY users_policy ON users FOR ALL
    USING (
        current_setting('app.role', true) = 'super_admin' OR
        tenant_id = current_setting('app.tenant_id', true)::uuid
    )
    WITH CHECK (
        current_setting('app.role', true) = 'super_admin' OR
        tenant_id = current_setting('app.tenant_id', true)::uuid
    );

-- RLS Policies: Nullable tenant_id (system defaults + tenant overrides)
CREATE POLICY document_types_select ON document_types FOR SELECT
    USING (
        tenant_id IS NULL OR
        tenant_id = current_setting('app.tenant_id', true)::uuid OR
        current_setting('app.role', true) = 'super_admin'
    );

CREATE POLICY document_types_write ON document_types FOR INSERT
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::uuid);

CREATE POLICY service_types_select ON service_types FOR SELECT
    USING (
        tenant_id IS NULL OR
        tenant_id = current_setting('app.tenant_id', true)::uuid OR
        current_setting('app.role', true) = 'super_admin'
    );

CREATE POLICY service_types_write ON service_types FOR INSERT
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::uuid);

CREATE POLICY email_templates_select ON email_templates FOR SELECT
    USING (
        tenant_id IS NULL OR
        tenant_id = current_setting('app.tenant_id', true)::uuid OR
        current_setting('app.role', true) = 'super_admin'
    );

CREATE POLICY email_templates_write ON email_templates FOR INSERT
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::uuid);

-- RLS Policies: Audit logs (tenant-scoped + super_admin system actions)
CREATE POLICY audit_logs_policy ON audit_logs FOR ALL
    USING (
        current_setting('app.role', true) = 'super_admin' OR
        tenant_id = current_setting('app.tenant_id', true)::uuid OR
        (tenant_id IS NULL AND user_id = current_setting('app.user_id', true)::uuid)
    );

-- RLS Policies: Tenants (super_admin sees all, others see own)
CREATE POLICY tenants_policy ON tenants FOR ALL
    USING (
        current_setting('app.role', true) = 'super_admin' OR
        id = current_setting('app.tenant_id', true)::uuid
    )
    WITH CHECK (
        current_setting('app.role', true) = 'super_admin' OR
        id = current_setting('app.tenant_id', true)::uuid
    );

-- ============================================================================
-- SCHEMA MIGRATIONS
-- ============================================================================

CREATE TABLE IF NOT EXISTS schema_migrations (
    version BIGINT PRIMARY KEY,
    dirty BOOLEAN NOT NULL DEFAULT false
);

INSERT INTO schema_migrations (version, dirty) VALUES (1, false)
ON CONFLICT (version) DO UPDATE SET dirty = false;

-- ============================================================================
-- DONE
-- ============================================================================
