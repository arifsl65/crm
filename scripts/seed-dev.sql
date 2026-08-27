-- =============================================================================
-- Development Seed Data
-- =============================================================================
-- WARNING: This file contains test credentials. NEVER run in production.
-- Usage: psql $DATABASE_URL -f scripts/seed-dev.sql
-- =============================================================================

-- Strict environment check - FAIL if not explicitly development
DO $$
DECLARE
    current_env TEXT;
BEGIN
    current_env := current_setting('app.env', true);

    -- Fail-safe: require explicit 'development' or 'test' environment
    IF current_env IS NULL OR current_env = '' THEN
        RAISE EXCEPTION 'app.env not set. Set it explicitly: SET app.env = ''development'';';
    END IF;

    IF current_env NOT IN ('development', 'test') THEN
        RAISE EXCEPTION 'Refusing to seed: app.env = ''%'' (must be development or test)', current_env;
    END IF;

    RAISE NOTICE 'Environment check passed: app.env = %', current_env;
END $$;

-- Create a default tenant for development
INSERT INTO tenants (id, name, domain, plan)
VALUES (
    '00000000-0000-0000-0000-000000000001',
    'Development Tenant',
    'dev.localhost',
    'enterprise'
) ON CONFLICT (domain) DO NOTHING;

-- Create a default admin user
-- Password: admin123 (Argon2id hashed)
-- WARNING: Change immediately if used outside local development
INSERT INTO users (
    id,
    tenant_id,
    email,
    name,
    password,
    role
)
VALUES (
    '00000000-0000-0000-0000-000000000001',
    '00000000-0000-0000-0000-000000000001',
    'admin@dev.local',
    'Admin User',
    '$argon2id$v=19$m=65536,t=3,p=4$c29tZXNhbHQ$RdescudvJCsgt3ub+b+dWRWJTmaaJObG',
    'tenant_admin'
) ON CONFLICT DO NOTHING;

DO $$
BEGIN
    RAISE NOTICE '✓ Development seed data inserted successfully';
    RAISE NOTICE '  Tenant: dev (00000000-0000-0000-0000-000000000001)';
    RAISE NOTICE '  User: admin@dev.local / admin123';
END $$;
