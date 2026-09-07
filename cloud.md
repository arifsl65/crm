# Accountant CRM - Backend Architecture & Database Documentation

> Deep analysis of the Go backend, database schema, and cloud deployment configuration.
> Generated: 2026-09-07

---

## Table of Contents

1. [Architecture Overview](#architecture-overview)
2. [Go Backend Structure](#go-backend-structure)
3. [Database Schema](#database-schema)
4. [Row-Level Security (RLS)](#row-level-security)
5. [API Endpoints](#api-endpoints)
6. [GraphQL API](#graphql-api)
7. [Configuration & Environment Variables](#configuration)
8. [Cloud Infrastructure](#cloud-infrastructure)
9. [Database Access Patterns](#database-access-patterns)
10. [Security Features](#security-features)

---

## Architecture Overview

### Technology Stack

| Component | Technology | Purpose |
|-----------|------------|---------|
| **Go Backend** | Go 1.21+ / Chi Router | REST API, WebSocket, GraphQL |
| **Python AI** | FastAPI | AI/ML services (OCR, classification, chat) |
| **PostgreSQL** | Neon (Serverless) | Primary database with connection pooling |
| **Redis** | Alibaba ApsaraDB | Caching, rate limiting, pub/sub |
| **MongoDB** | Atlas | AI chat history storage |
| **Object Storage** | Alibaba Cloud OSS | Document/file uploads |
| **Email** | Resend | Transactional emails |
| **Secrets** | Alibaba Cloud KMS | Production secret management |

### System Architecture

```
┌─────────────────────────────────────────────────────────────────────────┐
│                           CLIENTS                                        │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐                │
│  │  Web UI  │  │  Mobile  │  │  Portal  │  │   API    │                │
│  │ (Next.js)│  │   App    │  │ (Client) │  │ Consumers│                │
│  └────┬─────┘  └────┬─────┘  └────┬─────┘  └────┬─────┘                │
└───────┼─────────────┼─────────────┼─────────────┼───────────────────────┘
        │             │             │             │
        └─────────────┴─────────────┴─────────────┘
                              │
                    ┌─────────▼─────────┐
                    │    ALB (HTTPS)    │
                    │   Load Balancer   │
                    └─────────┬─────────┘
                              │
        ┌─────────────────────┼─────────────────────┐
        │                     │                     │
┌───────▼───────┐    ┌────────▼────────┐    ┌──────▼──────┐
│  Go Backend   │◄───│   Python AI     │    │  WebSocket  │
│   :8080       │    │    :8000        │    │    Hub      │
│               │    │                 │    │             │
│ • REST API    │    │ • OCR           │    │ • Real-time │
│ • GraphQL     │    │ • Chat (LLM)    │    │   notifications│
│ • Auth/JWT    │    │ • Classification│    │ • Events    │
└───────┬───────┘    └────────┬────────┘    └──────┬──────┘
        │                     │                    │
        └─────────────────────┼────────────────────┘
                              │
    ┌─────────────────────────┼─────────────────────────┐
    │                         │                         │
┌───▼───┐  ┌────────┐  ┌──────▼──────┐  ┌──────────┐  ┌──────▼───────┐
│ Neon  │  │ Redis  │  │  MongoDB    │  │   OSS    │  │   Resend     │
│ (PG)  │  │ Cache  │  │  (AI Chat)  │  │ (Files)  │  │  (Email)     │
└───────┘  └────────┘  └─────────────┘  └──────────┘  └──────────────┘
```

---

## Go Backend Structure

### Directory Layout

```
go-backend/
├── cmd/
│   ├── hashpwd/                 # Password hashing utility
│   └── server/                  # Main application entry point
│       └── main.go              # 1,143 lines - Application bootstrap & routing
├── internal/
│   ├── ai/                      # Python AI service client
│   │   └── client.go            # mTLS-enabled HTTP client for Python AI calls
│   ├── audit/                   # Audit logging
│   │   └── logger.go            # Tenant-aware audit trail logging
│   ├── auth/                    # Authentication & Authorization
│   │   ├── jwt.go               # JWT token generation/validation
│   │   ├── password.go          # Bcrypt password hashing
│   │   └── session.go           # Session & token revocation management
│   ├── cache/                   # Redis caching layer
│   │   ├── redis.go             # Redis client wrapper with TLS support
│   │   └── pubsub.go            # Pub/Sub for real-time notifications
│   ├── config/                  # Configuration management
│   │   └── config.go            # Environment variable loading & KMS integration
│   ├── crypto/                  # AES-256-GCM encryption
│   │   └── crypto.go            # Credential encryption (IMAP passwords, OAuth tokens)
│   ├── database/                # PostgreSQL connection pooling
│   │   └── postgres.go          # pgx connection pool with health checks
│   ├── email/                   # Email service
│   │   └── resend.go            # Resend email provider integration
│   ├── graphql/                 # GraphQL API
│   │   ├── handler.go           # GraphQL HTTP handler
│   │   ├── resolver.go          # GraphQL resolvers
│   │   ├── schema.resolvers.go  # Generated schema implementation
│   │   ├── schema.graphqls      # GraphQL schema definition (567 lines)
│   │   ├── generated.go         # Generated code by gqlgen
│   │   ├── dataloader/          # DataLoader for N+1 query prevention
│   │   ├── model/               # GraphQL type definitions
│   │   └── security/            # GraphQL security rules
│   │       ├── complexity.go    # Query complexity limiting
│   │       ├── depth.go         # Query depth limiting
│   │       ├── ratelimit.go     # GraphQL rate limiting
│   │       └── timeout.go       # Query timeout enforcement
│   ├── handlers/                # HTTP request handlers (26 files)
│   │   ├── auth.go              # Authentication & 2FA
│   │   ├── clients.go           # Client management
│   │   ├── documents.go         # Document upload/approval workflow
│   │   ├── services.go          # Service tracking
│   │   ├── emails.go            # Email management & threading
│   │   ├── email_accounts.go    # IMAP/OAuth email account connections
│   │   ├── email_templates.go   # Email template management
│   │   ├── chase_logs.go        # Chase/follow-up logging
│   │   ├── dashboard.go         # Analytics dashboards
│   │   ├── notifications.go     # Real-time notifications
│   │   ├── reminders.go         # Task reminders
│   │   ├── ai.go                # AI service proxy endpoints
│   │   ├── companies_house.go   # UK Companies House API integration
│   │   ├── settings.go          # Tenant settings & branding
│   │   ├── subscriptions.go     # Billing & subscriptions
│   │   ├── portal.go            # Client portal access
│   │   ├── export.go            # Data export (CSV)
│   │   ├── audit_logs.go        # Audit log queries
│   │   ├── search.go            # Global search
│   │   ├── e_sign.go            # E-signature workflows
│   │   ├── rebalance.go         # Staff workload rebalancing
│   │   └── ...                  # Additional handlers
│   ├── middleware/              # HTTP middleware (12 types)
│   │   ├── auth.go              # JWT authentication middleware
│   │   ├── tenant.go            # Tenant RLS context injection
│   │   ├── cors.go              # Dynamic CORS policy
│   │   ├── ratelimit.go         # Rate limiting middleware
│   │   ├── auditlog.go          # Audit trail middleware
│   │   ├── request_id.go        # Distributed tracing
│   │   ├── security.go          # Security headers
│   │   ├── uuid.go              # UUID validation
│   │   ├── staffscope.go        # Staff-specific filtering
│   │   ├── idempotency.go       # Idempotency key handling
│   │   └── magicbyte.go         # File type validation
│   ├── oauth/                   # OAuth 2.0 providers
│   │   ├── oauth.go             # Common OAuth logic
│   │   ├── google.go            # Google OAuth implementation
│   │   └── microsoft.go         # Microsoft OAuth implementation
│   ├── secrets/                 # Secret management
│   │   └── kms.go               # Alibaba Cloud KMS integration
│   ├── storage/                 # Cloud storage
│   │   └── oss.go               # Alibaba Cloud OSS (file uploads)
│   ├── websocket/               # Real-time WebSocket
│   │   ├── hub.go               # WebSocket connection hub
│   │   └── handler.go           # WebSocket HTTP handler
│   └── worker/                  # Background workers
│       └── outbox.go            # Transactional outbox pattern for emails
├── gqlgen.yml                   # GraphQL code generation config
├── Dockerfile                   # Multi-stage production build
└── go.mod                       # Go module definition
```

### Application Entry Point

**File:** `cmd/server/main.go` (1,143 lines)

```go
type Application struct {
    Config      *config.Config
    DB          *database.Pool          // PostgreSQL
    Redis       *cache.Client           // Cache & rate limiting
    AIClient    *ai.Client              // Python AI service
    JWT         *auth.JWTManager        // Token management
    Audit       *audit.Logger           // Audit trail
    Handlers    *Handlers               // 26 handler instances
    WSHub       *websocket.Hub          // Real-time notifications
    RateLimiter *middleware.AuthRateLimiter
}
```

### Initialization Flow

1. Load configuration from environment / KMS
2. Connect to PostgreSQL (pgx pool)
3. Connect to Redis (with fallback rate limiter)
4. Initialize AI service client (Python service at http://python-ai:8000)
5. Setup JWT manager + session management
6. Initialize email client (Resend)
7. Setup OAuth services (Google, Microsoft)
8. Initialize WebSocket hub
9. Setup HTTP router with middleware stack
10. Start background workers (token cleanup, email outbox)
11. Graceful shutdown with signal handling

---

## Database Schema

### Overview

- **Database:** PostgreSQL 16 (Neon Serverless)
- **Connection Pooling:** pgbouncer (Neon managed)
- **Total Tables:** 37+ (including monthly partitions)
- **Migrations:** 8 versioned migration files
- **RLS:** Row-Level Security enabled on all tenant-scoped tables

### Enum Types

```sql
-- User & Auth
CREATE TYPE user_role AS ENUM ('super_admin', 'tenant_admin', 'staff', 'client');
CREATE TYPE user_status AS ENUM ('pending', 'active', 'inactive');

-- Client
CREATE TYPE client_status AS ENUM ('active', 'inactive', 'archived');
CREATE TYPE client_email_status AS ENUM ('active', 'unsubscribed', 'bounced', 'complained');

-- Documents
CREATE TYPE document_status AS ENUM ('requested', 'uploaded', 'pending_review', 'approved', 'rejected');
CREATE TYPE document_access_level AS ENUM ('admin', 'all_staff', 'specific');

-- Services
CREATE TYPE service_status AS ENUM ('not_started', 'in_progress', 'review', 'waiting', 'completed', 'cancelled');
CREATE TYPE service_priority AS ENUM ('low', 'normal', 'high', 'urgent');
CREATE TYPE risk_level AS ENUM ('low', 'medium', 'high');
CREATE TYPE deadline_pattern AS ENUM ('monthly', 'quarterly', 'annual', 'custom');

-- Email
CREATE TYPE email_direction AS ENUM ('inbound', 'outbound');
CREATE TYPE email_type AS ENUM ('chase', 'notification', 'invite', 'manual');
CREATE TYPE email_status AS ENUM ('queued', 'sent', 'delivered', 'opened', 'clicked', 'bounced', 'complained');
CREATE TYPE email_template_type AS ENUM ('chase', 'notification', 'welcome', 'custom');
CREATE TYPE email_account_type AS ENUM ('shared', 'personal');
CREATE TYPE email_account_provider AS ENUM ('imap', 'google', 'microsoft', 'zoho');
CREATE TYPE email_account_status AS ENUM ('active', 'error', 'disconnected');

-- Other
CREATE TYPE notification_type AS ENUM ('document', 'deadline', 'email', 'system', 'reminder');
CREATE TYPE director_role AS ENUM ('director', 'secretary');
CREATE TYPE psc_ownership AS ENUM ('75%+', '50-75%', '25-50%');
CREATE TYPE e_sign_status AS ENUM ('pending', 'signed', 'expired', 'declined');
CREATE TYPE push_platform AS ENUM ('ios', 'android', 'web');
CREATE TYPE reminder_status AS ENUM ('pending', 'sent', 'dismissed');
CREATE TYPE audit_severity AS ENUM ('info', 'warning', 'critical');
CREATE TYPE subscription_status AS ENUM ('trialing', 'active', 'past_due', 'canceled', 'unpaid');
CREATE TYPE ai_job_status AS ENUM ('pending', 'processing', 'completed', 'failed');
```

### Core Tables

#### Tenants (Multi-tenancy Root)

```sql
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
```

#### Users

```sql
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID REFERENCES tenants(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    email VARCHAR(255) NOT NULL,
    password VARCHAR(255),                    -- Bcrypt hashed
    role user_role NOT NULL DEFAULT 'client',
    status user_status DEFAULT 'pending',
    avatar_url VARCHAR(500),
    phone VARCHAR(50),
    invite_token VARCHAR(255),
    invite_expires TIMESTAMP,
    totp_secret VARCHAR(255),                 -- 2FA TOTP secret
    specialism VARCHAR(255),
    notes TEXT,
    reset_token VARCHAR(255),
    reset_token_expires TIMESTAMP,
    preferences JSONB DEFAULT '{}',
    last_login_at TIMESTAMP,
    failed_login_attempts INTEGER DEFAULT 0,
    locked_until TIMESTAMP,
    deleted_at TIMESTAMP,                     -- Soft delete
    anonymized_at TIMESTAMP,                  -- GDPR compliance
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),

    -- Role validation constraints
    CONSTRAINT chk_role_values CHECK (role IN ('super_admin', 'tenant_admin', 'staff', 'client')),
    CONSTRAINT chk_role_tenant_consistency CHECK (
        (role = 'super_admin' AND tenant_id IS NULL) OR
        (role != 'super_admin' AND tenant_id IS NOT NULL)
    )
);

-- Unique email per tenant (allows same email across tenants)
CREATE UNIQUE INDEX idx_users_email_tenant ON users(tenant_id, email) WHERE tenant_id IS NOT NULL;
CREATE UNIQUE INDEX idx_users_email_super ON users(email) WHERE tenant_id IS NULL;
```

#### Clients

```sql
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
    utr VARCHAR(20),                          -- UK Unique Taxpayer Reference
    company_number VARCHAR(20),               -- Companies House number
    company_type VARCHAR(50),
    incorporation_date DATE,
    sic_codes JSONB,                          -- Industry codes
    vat_number VARCHAR(20),
    vat_quarter VARCHAR(10),
    status client_status DEFAULT 'active',
    risk_score INTEGER,                       -- AI-calculated risk (0-100)
    tags JSONB DEFAULT '[]',
    email_status client_email_status DEFAULT 'active',
    email_status_at TIMESTAMP,
    alternate_emails JSONB DEFAULT '[]',
    ni_number_encrypted BYTEA,                -- AES-256-GCM encrypted
    bank_details_encrypted BYTEA,             -- AES-256-GCM encrypted
    anonymized_at TIMESTAMP,                  -- GDPR compliance
    last_contact_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);
```

#### Services (Work Items)

```sql
CREATE TABLE services (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    client_id UUID REFERENCES clients(id) ON DELETE CASCADE,
    staff_id UUID REFERENCES users(id) ON DELETE SET NULL,
    type_id UUID REFERENCES service_types(id) ON DELETE SET NULL,
    name VARCHAR(255) NOT NULL,
    period VARCHAR(50),                       -- e.g., "2024-25", "Q1 2024"
    status service_status DEFAULT 'not_started',
    priority service_priority DEFAULT 'normal',
    risk_level risk_level DEFAULT 'low',
    deadline DATE,
    kanban_position INTEGER DEFAULT 0,        -- For drag-and-drop ordering
    docs_required INTEGER DEFAULT 0,
    docs_received INTEGER DEFAULT 0,          -- Counter updated by trigger
    hmrc_reference VARCHAR(100),
    hmrc_data JSONB,                          -- HMRC submission data
    filed_at TIMESTAMP,
    completed_at TIMESTAMP,
    completion_notes TEXT,
    version INTEGER DEFAULT 1,                -- Optimistic locking
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),

    CONSTRAINT chk_docs_received CHECK (docs_received >= 0),
    CONSTRAINT chk_docs_required CHECK (docs_required >= 0),
    CONSTRAINT chk_docs_balance CHECK (docs_received <= docs_required)
);
```

#### Documents

```sql
CREATE TABLE documents (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    client_id UUID REFERENCES clients(id) ON DELETE CASCADE,
    service_id UUID REFERENCES services(id) ON DELETE SET NULL,
    uploaded_by UUID REFERENCES users(id) ON DELETE SET NULL,
    type_id UUID REFERENCES document_types(id) ON DELETE SET NULL,
    name VARCHAR(255) NOT NULL,
    original_name VARCHAR(255) NOT NULL,
    file_path VARCHAR(500),                   -- OSS key
    file_size INTEGER,
    mime_type VARCHAR(100),
    status document_status DEFAULT 'uploaded',
    access document_access_level DEFAULT 'all_staff',
    version INTEGER DEFAULT 1,
    parent_id UUID REFERENCES documents(id) ON DELETE SET NULL,  -- Version chain
    requested_at TIMESTAMP,
    expiry_date DATE,
    request_note TEXT,
    upload_note TEXT,
    review_note TEXT,
    reviewed_by UUID REFERENCES users(id) ON DELETE SET NULL,
    reviewed_at TIMESTAMP,
    chase_count INTEGER DEFAULT 0,
    last_chased_at TIMESTAMP,
    ai_summary TEXT,                          -- AI-generated summary
    ai_extracted JSONB,                       -- AI-extracted data
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);
```

#### Emails (Partitioned Table)

```sql
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
    raw_oss_uri VARCHAR(500),                 -- Original email stored in OSS
    type email_type DEFAULT 'manual',
    status email_status DEFAULT 'queued',
    resend_id VARCHAR(100),
    is_read BOOLEAN DEFAULT false,
    claimed_by UUID,
    claimed_at TIMESTAMP,
    promised_docs TEXT[],                     -- AI-extracted document promises
    promised_date DATE,
    ai_summary TEXT,
    ai_tags JSONB DEFAULT '[]',
    sentiment VARCHAR(20),                    -- AI sentiment analysis
    action_needed TEXT,
    opened_at TIMESTAMP,
    clicked_at TIMESTAMP,
    bounced_at TIMESTAMP,
    bounce_reason TEXT,
    sent_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),

    PRIMARY KEY (id, created_at)
) PARTITION BY RANGE (created_at);

-- Monthly partitions (2026-2027)
CREATE TABLE emails_2026_01 PARTITION OF emails FOR VALUES FROM ('2026-01-01') TO ('2026-02-01');
-- ... (12 partitions per year)
```

#### Audit Logs (Partitioned Table)

```sql
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
```

### Complete Table List

| Category | Tables |
|----------|--------|
| **Multi-Tenancy** | `tenants`, `tenant_subscriptions`, `tenant_invoices` |
| **Users & Auth** | `users`, `sessions`, `refresh_tokens`, `totp_backup_codes`, `magic_link_tokens` |
| **Clients** | `clients`, `staff_clients`, `client_notes`, `directors`, `psc` |
| **Documents** | `documents`, `document_types`, `document_access`, `upload_tokens` |
| **Services** | `services`, `service_types`, `service_requirements` |
| **Email** | `emails` (partitioned), `email_threads`, `email_accounts`, `email_templates` |
| **Communication** | `chase_logs`, `chase_log_clients`, `e_sign_requests` |
| **Notifications** | `notifications`, `reminders`, `push_tokens` |
| **Settings** | `company_settings` |
| **Audit** | `audit_logs` (partitioned), `deletion_audit` |
| **Background Jobs** | `ai_jobs`, `outbox`, `webhook_idempotency` |
| **Migrations** | `schema_migrations` |

### Triggers & Functions

```sql
-- Auto-update updated_at timestamp
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Applied to: users, clients, documents, services, tenants, company_settings,
--             email_templates, email_accounts, email_threads, tenant_subscriptions,
--             reminders, client_notes

-- Update service docs_received counter
CREATE OR REPLACE FUNCTION update_service_docs_count()
RETURNS TRIGGER AS $$ ... $$;

-- Update email thread message_count
CREATE OR REPLACE FUNCTION update_thread_message_count()
RETURNS TRIGGER AS $$ ... $$;

-- Validate staff and client belong to same tenant
CREATE OR REPLACE FUNCTION check_staff_client_same_tenant()
RETURNS TRIGGER AS $$ ... $$;
```

### Indexes

```sql
-- Full-text search indexes
CREATE INDEX idx_clients_search ON clients
    USING gin(to_tsvector('english', company_name || ' ' || COALESCE(contact_name, '')));
CREATE INDEX idx_documents_search ON documents
    USING gin(to_tsvector('english', name || ' ' || COALESCE(ai_summary, '')));
CREATE INDEX idx_emails_search ON emails
    USING gin(to_tsvector('english', subject || ' ' || COALESCE(body_text, '')));

-- JSONB indexes
CREATE INDEX idx_clients_tags ON clients USING GIN(tags);
CREATE INDEX idx_clients_alternate_emails ON clients USING GIN(alternate_emails);
CREATE INDEX idx_documents_ai_extracted ON documents USING GIN(ai_extracted);
CREATE INDEX idx_emails_ai_tags ON emails USING GIN(ai_tags);

-- Composite indexes for common queries
CREATE INDEX idx_services_kanban ON services(staff_id, status, kanban_position);
CREATE INDEX idx_services_tenant_deadline ON services(tenant_id, staff_id, deadline, status);
CREATE INDEX idx_emails_tenant_client ON emails(tenant_id, client_id, direction, created_at);
```

---

## Row-Level Security

### RLS Overview

PostgreSQL Row-Level Security (RLS) enforces tenant isolation at the database level. Every tenant-scoped table has RLS enabled with policies that filter data based on session variables.

### Session Variables

```sql
-- Set by application before queries
SET app.tenant_id = '<tenant-uuid>';
SET app.role = 'tenant_admin';  -- super_admin, tenant_admin, staff, client
SET app.user_id = '<user-uuid>';
```

### Policy Pattern

```sql
-- Standard tenant isolation policy
CREATE POLICY tenant_isolation_<table> ON <table> FOR ALL
    USING (
        tenant_id = current_setting('app.tenant_id', true)::uuid
        OR current_setting('app.role', true) = 'super_admin'
    )
    WITH CHECK (
        tenant_id = current_setting('app.tenant_id', true)::uuid
        OR current_setting('app.role', true) = 'super_admin'
    );
```

### Force RLS

```sql
-- Critical: Force RLS even for table owner (neondb_owner)
ALTER TABLE users FORCE ROW LEVEL SECURITY;
ALTER TABLE clients FORCE ROW LEVEL SECURITY;
ALTER TABLE documents FORCE ROW LEVEL SECURITY;
ALTER TABLE services FORCE ROW LEVEL SECURITY;
-- ... (all 34 tenant-scoped tables)
```

### Application Role (No BYPASSRLS)

```sql
-- Create app_user role without BYPASSRLS privilege
CREATE ROLE app_user WITH LOGIN PASSWORD '...' NOBYPASSRLS;

-- Grant permissions
GRANT CONNECT ON DATABASE neondb TO app_user;
GRANT USAGE ON SCHEMA public TO app_user;
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO app_user;
```

### Go Backend RLS Integration

```go
// internal/database/postgres.go

// TenantTransaction executes with RLS context
func (p *Pool) TenantTransaction(ctx context.Context, tenantID, role string, fn func(tx pgx.Tx) error) error {
    // Validate inputs (prevent SQL injection)
    if tenantID != "" {
        if _, err := uuid.Parse(tenantID); err != nil {
            return fmt.Errorf("invalid tenant_id format")
        }
    }
    if role != "" && !validRoles[role] {
        return fmt.Errorf("invalid role")
    }

    tx, err := p.Begin(ctx)
    // ...

    // Set RLS context using parameterized set_config()
    _, err = tx.Exec(ctx, "SELECT set_config('app.tenant_id', $1, false)", tenantID)
    _, err = tx.Exec(ctx, "SELECT set_config('app.role', $1, false)", role)

    // Execute function
    if err := fn(tx); err != nil {
        tx.Rollback(ctx)
        return err
    }

    return tx.Commit(ctx)
}
```

---

## API Endpoints

### Health & Metrics

| Method | Path | Description |
|--------|------|-------------|
| GET | `/health` | Liveness probe |
| HEAD | `/health` | Health check (ALB compatible) |
| GET | `/ready` | Readiness probe (checks DB, Redis, AI) |
| GET | `/metrics` | Connection pool metrics (protected) |

### Authentication (Public)

| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/v1/auth/login` | Email/password login |
| POST | `/api/v1/auth/register` | User registration |
| POST | `/api/v1/auth/refresh` | Token refresh |
| POST | `/api/v1/auth/reset-password` | Forgot password request |
| POST | `/api/v1/auth/reset-password/confirm` | Reset password confirmation |
| POST | `/api/v1/auth/magic-link` | Magic link generation |
| GET | `/api/v1/auth/magic-link` | Magic link verification |
| POST | `/api/v1/auth/invite-accept` | Accept invite & set password |
| POST | `/api/v1/auth/2fa/backup-codes/verify` | 2FA backup code verification |

### Authentication (Protected)

| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/v1/auth/logout` | Logout & token revocation |
| GET | `/api/v1/auth/me` | Current user profile |
| PATCH | `/api/v1/auth/me` | Update profile |
| PATCH | `/api/v1/auth/password` | Change password |
| GET | `/api/v1/auth/sessions` | List active sessions |
| POST | `/api/v1/auth/2fa/setup` | Enable 2FA (TOTP) |
| POST | `/api/v1/auth/2fa/verify` | Verify 2FA code |
| DELETE | `/api/v1/auth/2fa` | Disable 2FA |

### Clients

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/clients` | List clients |
| POST | `/api/v1/clients` | Create client |
| GET | `/api/v1/clients/:id` | Get client details |
| PATCH | `/api/v1/clients/:id` | Update client |
| DELETE | `/api/v1/clients/:id` | Delete client (soft) |
| POST | `/api/v1/clients/:id/restore` | Restore deleted client |
| GET | `/api/v1/clients/:id/documents` | Get client's documents |
| GET | `/api/v1/clients/:id/services` | Get client's services |
| GET | `/api/v1/clients/:id/emails` | Get client's emails |
| POST | `/api/v1/clients/:id/assign` | Assign client to staff |
| GET | `/api/v1/clients/:id/notes` | List client notes |
| POST | `/api/v1/clients/:id/notes` | Create client note |

### Services

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/services` | List services |
| POST | `/api/v1/services` | Create service |
| GET | `/api/v1/services/deadlines` | Get upcoming deadlines |
| GET | `/api/v1/services/alerts` | Get service alerts |
| POST | `/api/v1/services/bulk-update` | Bulk update services |
| PATCH | `/api/v1/services/reorder` | Reorder services (kanban) |
| GET | `/api/v1/services/:id` | Get service details |
| PATCH | `/api/v1/services/:id` | Update service |
| PATCH | `/api/v1/services/:id/status` | Update service status |
| POST | `/api/v1/services/:id/complete` | Mark service complete |
| POST | `/api/v1/services/:id/hmrc-mark` | Mark as HMRC submission |

### Documents

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/documents` | List documents |
| POST | `/api/v1/documents` | Create document |
| POST | `/api/v1/documents/bulk-request` | Bulk request documents |
| POST | `/api/v1/documents/bulk-approve` | Bulk approve documents |
| POST | `/api/v1/documents/upload-url` | Generate S3 upload URL |
| POST | `/api/v1/documents/qr` | Generate QR token for client upload |
| GET | `/api/v1/documents/:id` | Get document details |
| POST | `/api/v1/documents/:id/approve` | Approve document |
| POST | `/api/v1/documents/:id/reject` | Reject document |
| GET | `/api/v1/documents/:id/download` | Download document |
| GET | `/api/v1/documents/expiring` | List expiring documents |

### AI Endpoints

| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/v1/ai/chat` | Send chat message |
| POST | `/api/v1/ai/chat/stream` | Stream chat response |
| POST | `/api/v1/ai/documents/extract` | Extract document data |
| POST | `/api/v1/ai/documents/classify` | Classify document type |
| POST | `/api/v1/ai/documents/summarize` | Summarize document |
| POST | `/api/v1/ai/emails/summarize` | Summarize email |
| POST | `/api/v1/ai/emails/sentiment` | Analyze sentiment |
| POST | `/api/v1/ai/emails/draft` | Draft email response |
| POST | `/api/v1/ai/risk/client` | Analyze client risk |
| POST | `/api/v1/ai/dashboard/troublemakers` | Find problematic clients |
| POST | `/api/v1/ai/dashboard/anomalies` | Detect anomalies |

### Companies House (UK)

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/ch/search` | Search companies |
| GET | `/api/v1/ch/company/:number` | Get company info |
| GET | `/api/v1/ch/company/:number/filings` | Get filings |
| GET | `/api/v1/ch/company/:number/officers` | Get officers |
| POST | `/api/v1/ch/sync/:clientId` | Sync client with Companies House |

### Total: 150+ REST endpoints

---

## GraphQL API

### Schema Overview

```graphql
type Query {
    # Dashboard (aggregated - primary GraphQL use case)
    dashboard: Dashboard!

    # Entities with Relay-style pagination
    clients(filter: ClientFilter, first: Int, after: String): ClientConnection!
    documents(filter: DocumentFilter, first: Int, after: String): DocumentConnection!
    services(filter: ServiceFilter, first: Int, after: String): ServiceConnection!

    # Single entity lookups
    client(id: UUID!): Client
    document(id: UUID!): Document
    service(id: UUID!): Service

    # AI features
    troublemaker_clients(limit: Int): [TroublemakerClient!]!
    anomalies(limit: Int): [AnomalyItem!]!

    # Search
    search(query: String!, limit: Int): SearchResults!
}

type Mutation {
    # Quick actions (most mutations use REST)
    mark_notification_read(id: UUID!): Notification!
    approve_document(id: UUID!, note: String): Document!
    reject_document(id: UUID!, note: String!): Document!
    update_service_status(id: UUID!, status: ServiceStatus!): Service!
    claim_email(id: UUID!): Email!
}

type Subscription {
    notification_received: Notification!
    document_processed(client_id: UUID): Document!
    service_at_risk: Service!
}
```

### Security Controls

- **Query Depth Limiting:** Max 10 levels
- **Query Complexity Limiting:** Calculated per field
- **Rate Limiting:** Per-tenant, per-user
- **Query Timeout:** 30 seconds max
- **DataLoaders:** N+1 query prevention

---

## Configuration

### Environment Variables

```bash
# =============================================================================
# Application
# =============================================================================
APP_ENV=development|staging|production
APP_NAME=accountant-crm
APP_DEBUG=true|false
LOG_LEVEL=debug|info|warn|error

# =============================================================================
# Server
# =============================================================================
GO_HOST=0.0.0.0
GO_PORT=8080
GO_READ_TIMEOUT=30s
GO_WRITE_TIMEOUT=30s
GO_SHUTDOWN_TIMEOUT=10s

# =============================================================================
# PostgreSQL (Neon)
# =============================================================================
POSTGRES_HOST=your-project.neon.tech
POSTGRES_PORT=5432
POSTGRES_USER=app_user
POSTGRES_PASSWORD=secure-password
POSTGRES_DB=neondb
POSTGRES_SSLMODE=require
POSTGRES_POOL_MIN=2
POSTGRES_POOL_MAX=8                        # Neon has 10-connection limit

# =============================================================================
# Redis (Alibaba ApsaraDB)
# =============================================================================
REDIS_HOST=r-xxxxx.redis.rds.aliyuncs.com
REDIS_PORT=6379
REDIS_PASSWORD=secure-password
REDIS_DB=0
REDIS_TLS_ENABLED=true|false
REDIS_POOL_SIZE=10

# =============================================================================
# JWT Authentication
# =============================================================================
JWT_SECRET_KEY=base64-32-chars-minimum     # REQUIRED in production
JWT_ACCESS_TOKEN_EXPIRE=15m
JWT_REFRESH_TOKEN_EXPIRE=7d
JWT_ISSUER=accountant-crm

# =============================================================================
# Email (Resend)
# =============================================================================
RESEND_API_KEY=re_your-api-key
EMAIL_FROM=noreply@domain.com
EMAIL_FROM_NAME=Accountant CRM

# =============================================================================
# Frontend
# =============================================================================
FRONTEND_URL=https://crm.domain.com
COOKIE_DOMAIN=.domain.com                  # Cross-subdomain auth

# =============================================================================
# Python AI Service
# =============================================================================
PYTHON_AI_URL=http://python-ai:8000
PYTHON_AI_TIMEOUT=30s

# =============================================================================
# CORS
# =============================================================================
CORS_ALLOWED_ORIGINS=http://localhost:3000,https://crm.domain.com
CORS_ALLOW_CREDENTIALS=true

# =============================================================================
# Rate Limiting
# =============================================================================
RATE_LIMIT_ENABLED=true
RATE_LIMIT_REQUESTS_PER_IP=100
RATE_LIMIT_BURST_SIZE=20
RATE_LIMIT_WINDOW=1m

# =============================================================================
# Companies House API (UK)
# =============================================================================
COMPANIES_HOUSE_API_KEY=your-api-key
COMPANIES_HOUSE_BASE_URL=https://api.company-information.service.gov.uk
COMPANIES_HOUSE_TIMEOUT=10s
COMPANIES_HOUSE_CACHE_TTL=1h

# =============================================================================
# Alibaba Cloud OSS
# =============================================================================
OSS_ENABLED=true|false
ALIBABA_ACCESS_KEY_ID=your-key-id
ALIBABA_ACCESS_KEY_SECRET=your-key-secret
OSS_ENDPOINT=https://oss-eu-west-1.aliyuncs.com
OSS_BUCKET_UPLOADS=fzco-uploads

# =============================================================================
# OAuth (Email Providers)
# =============================================================================
GOOGLE_CLIENT_ID=your-client-id
GOOGLE_CLIENT_SECRET=your-secret
GOOGLE_REDIRECT_URL=https://domain.com/api/v1/email-accounts/oauth/google/callback
GOOGLE_OAUTH_ENABLED=true|false

MICROSOFT_CLIENT_ID=your-client-id
MICROSOFT_CLIENT_SECRET=your-secret
MICROSOFT_REDIRECT_URL=https://domain.com/api/v1/email-accounts/oauth/microsoft/callback
MICROSOFT_OAUTH_ENABLED=true|false

# =============================================================================
# mTLS (Inter-service Communication)
# =============================================================================
MTLS_ENABLED=false
MTLS_CA_CERT=/path/to/ca.crt
MTLS_CLIENT_CERT=/path/to/client.crt
MTLS_CLIENT_KEY=/path/to/client.key

# =============================================================================
# Secrets Management (Alibaba Cloud KMS)
# =============================================================================
SECRETS_FROM_KMS=false
KMS_REGION=eu-west-1
KMS_POSTGRES_PASSWORD_SECRET=postgres-password
KMS_REDIS_PASSWORD_SECRET=redis-password
```

### Database Connection Pool

```go
// internal/database/postgres.go
poolConfig.MinConns = 2
poolConfig.MaxConns = 8
poolConfig.MaxConnLifetime = 1 * time.Hour
poolConfig.MaxConnIdleTime = 30 * time.Minute
poolConfig.HealthCheckPeriod = 1 * time.Minute

// Critical for Neon's pgbouncer compatibility
poolConfig.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol
```

---

## Cloud Infrastructure

### Provider: Alibaba Cloud

- **Region:** eu-west-1 (UK London)
- **Architecture:** Single ECS + Docker Compose (~$17/month)
- **IaC:** Terraform

### Network Architecture

```
VPC CIDR: 10.0.0.0/16

┌─────────────────────────────────────────────────────────────┐
│                         VPC                                  │
│  ┌─────────────────┐  ┌─────────────────┐                   │
│  │ Public Subnet 1 │  │ Public Subnet 2 │                   │
│  │  10.0.1.0/24    │  │  10.0.2.0/24    │                   │
│  │                 │  │                 │                   │
│  │  ┌───────────┐  │  │                 │                   │
│  │  │    ALB    │  │  │                 │                   │
│  │  └─────┬─────┘  │  │                 │                   │
│  └────────┼────────┘  └─────────────────┘                   │
│           │                                                  │
│  ┌────────┼────────┐  ┌─────────────────┐                   │
│  │ Private Subnet 1│  │ Private Subnet 2│                   │
│  │  10.0.10.0/24   │  │  10.0.11.0/24   │                   │
│  │                 │  │                 │                   │
│  │  ┌───────────┐  │  │                 │                   │
│  │  │  ECS VM   │  │  │                 │                   │
│  │  │ (Docker)  │  │  │                 │                   │
│  │  └───────────┘  │  │                 │                   │
│  └─────────────────┘  └─────────────────┘                   │
└─────────────────────────────────────────────────────────────┘

External Services:
┌───────────────┐  ┌───────────────┐  ┌───────────────┐
│ Neon (PG)     │  │ MongoDB Atlas │  │ Resend        │
│ Serverless    │  │               │  │ (Email)       │
└───────────────┘  └───────────────┘  └───────────────┘
```

### Security Groups

```hcl
# ALB Security Group
resource "alicloud_security_group_rule" "alb_http" {
  type        = "ingress"
  ip_protocol = "tcp"
  port_range  = "80/80"
  cidr_ip     = "0.0.0.0/0"
}

resource "alicloud_security_group_rule" "alb_https" {
  type        = "ingress"
  ip_protocol = "tcp"
  port_range  = "443/443"
  cidr_ip     = "0.0.0.0/0"
}

# ECS Security Group
resource "alicloud_security_group_rule" "ecs_go_backend" {
  type        = "ingress"
  ip_protocol = "tcp"
  port_range  = "8080/8080"
  cidr_ip     = var.alb_cidr  # Only from ALB
}

resource "alicloud_security_group_rule" "ecs_python_ai" {
  type        = "ingress"
  ip_protocol = "tcp"
  port_range  = "8000/8000"
  cidr_ip     = var.alb_cidr  # Only from ALB
}
```

### OSS Buckets

| Bucket | Purpose | Access |
|--------|---------|--------|
| `fzco-frontend` | Static frontend assets | Public read |
| `fzco-uploads` | User document uploads | Private |

### Terraform Validations

```hcl
# Production security checks
check "production_redis_password" {
  assert {
    condition     = var.environment != "production" || length(var.redis_password) >= 8
    error_message = "Redis password min 8 characters in production."
  }
}

check "production_neon_password" {
  assert {
    condition     = var.environment != "production" || length(var.neon_password) >= 8
    error_message = "Neon PostgreSQL password required in production."
  }
}

check "production_image_tag" {
  assert {
    condition     = var.environment != "production" || var.image_tag != "latest"
    error_message = "Don't use 'latest' tag in production."
  }
}
```

---

## Database Access Patterns

### Neon PostgreSQL Connection

```
Connection URL Format:
postgresql://app_user:password@<project>.neon.tech:5432/neondb?sslmode=require

Connection Pooling:
- Neon uses pgbouncer internally
- Use SimpleProtocol mode to avoid prepared statement conflicts
- Max 10 connections per branch (use pool_max=8 for headroom)
```

### Connection Pool Configuration

```go
type PostgresConfig struct {
    Host     string  // e.g., "ep-xxx.eu-west-1.aws.neon.tech"
    Port     int     // 5432
    User     string  // "app_user"
    Password string  // From KMS in production
    Database string  // "neondb"
    SSLMode  string  // "require"
    PoolMin  int     // 2
    PoolMax  int     // 8
}

// DSN format
func (c PostgresConfig) DSN() string {
    return fmt.Sprintf(
        "host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
        c.Host, c.Port, c.User, c.Password, c.Database, c.SSLMode,
    )
}
```

### Redis Access

```
Development:
redis://password@redis:6379/0

Production:
rediss://password@r-xxxxx.redis.rds.aliyuncs.com:6379/0  (TLS)
```

### MongoDB (AI Chat)

```
mongodb+srv://username:password@cluster.mongodb.net/accountant_ai
```

---

## Security Features

### Authentication

- **Password Hashing:** Bcrypt (cost factor 12)
- **JWT Tokens:** HS256, 15-minute access, 7-day refresh
- **2FA:** TOTP (RFC 6238) with backup codes
- **Session Management:** Token families for theft detection
- **Magic Links:** For passwordless login

### Data Encryption

- **At Rest:** Neon/MongoDB Atlas encryption
- **In Transit:** TLS 1.2+ required
- **Sensitive Fields:** AES-256-GCM (NI numbers, bank details)
- **OAuth Tokens:** Encrypted in database

### Tenant Isolation

1. **RLS Policies:** Database-level isolation
2. **FORCE ROW LEVEL SECURITY:** Applies to table owner
3. **app_user Role:** NOBYPASSRLS flag
4. **Middleware:** Sets RLS context on every request
5. **Parameterized Queries:** Prevent SQL injection

### Rate Limiting

- **Per-IP:** 100 requests/minute (configurable)
- **Burst:** 20 additional requests
- **Auth Endpoints:** Stricter limits
- **GraphQL:** Per-query complexity limits

### Audit Logging

- **All CUD Operations:** Create, Update, Delete
- **Old/New Values:** For compliance
- **Partitioned Tables:** Monthly partitions
- **Severity Levels:** info, warning, critical

### CORS & Headers

```go
// Security headers applied to all responses
w.Header().Set("X-Content-Type-Options", "nosniff")
w.Header().Set("X-Frame-Options", "DENY")
w.Header().Set("X-XSS-Protection", "1; mode=block")
w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
```

---

## Summary Statistics

| Metric | Count |
|--------|-------|
| Go Files | 70+ |
| Handler Files | 26 |
| REST API Routes | 150+ |
| Database Tables | 37+ |
| GraphQL Queries/Mutations | 50+ |
| Environment Variables | 50+ |
| Middleware Types | 12 |
| Migration Files | 8 |
| External Integrations | 8 |

### External Service Integrations

1. **Neon** - PostgreSQL database
2. **Alibaba ApsaraDB Redis** - Caching
3. **MongoDB Atlas** - AI chat history
4. **Alibaba Cloud OSS** - File storage
5. **Alibaba Cloud KMS** - Secret management
6. **Resend** - Transactional email
7. **Companies House API** - UK company data
8. **OpenRouter** - AI/LLM services

---

*Documentation generated from deep codebase analysis*
