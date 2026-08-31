#!/bin/bash
# =============================================================================
# Run Migrations Script for Accountant CRM
# Run this from your local machine or ECS server (NOT from Claude Code)
# =============================================================================

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${YELLOW}=== Accountant CRM Migration Runner ===${NC}"

# Database credentials - loaded from environment or .env.ecs file
# NEVER commit credentials to git - use .env.ecs (gitignored)
# CRITICAL: Migrations MUST use the owner role (neondb_owner), not app_user
if [ -f "$(dirname "$0")/../.env.ecs" ]; then
    echo -e "${YELLOW}Loading credentials from .env.ecs...${NC}"
    set -a
    source "$(dirname "$0")/../.env.ecs"
    set +a
fi

# Required environment variables - use migration credentials (owner role)
: "${NEON_HOST:?Error: NEON_HOST not set. Create .env.ecs or export it.}"
: "${NEON_MIGRATION_USER:?Error: NEON_MIGRATION_USER not set. Create .env.ecs or export it.}"
: "${NEON_MIGRATION_PASSWORD:?Error: NEON_MIGRATION_PASSWORD not set. Create .env.ecs or export it.}"
: "${NEON_DATABASE:?Error: NEON_DATABASE not set. Create .env.ecs or export it.}"

export PGHOST="${NEON_HOST}"
export PGPORT="5432"
export PGUSER="${NEON_MIGRATION_USER}"
export PGPASSWORD="${NEON_MIGRATION_PASSWORD}"
export PGDATABASE="${NEON_DATABASE}"
export PGSSLMODE="require"

echo -e "${YELLOW}Using migration user: ${PGUSER}${NC}"

# Check psql is installed
if ! command -v psql &> /dev/null; then
    echo -e "${RED}Error: psql is not installed${NC}"
    echo "Install with: sudo apt install postgresql-client"
    exit 1
fi

# Test connection
echo -e "${YELLOW}Testing database connection...${NC}"
if psql -c "SELECT 1;" > /dev/null 2>&1; then
    echo -e "${GREEN}✓ Connection successful${NC}"
else
    echo -e "${RED}✗ Connection failed${NC}"
    exit 1
fi

# Use schema_migrations table (golang-migrate standard)
echo -e "${YELLOW}Checking current schema version...${NC}"

# Ensure schema_migrations table exists
psql -c "CREATE TABLE IF NOT EXISTS schema_migrations (version bigint NOT NULL PRIMARY KEY, dirty boolean NOT NULL DEFAULT false);" > /dev/null 2>&1 || true

# One-time migration: copy applied versions from legacy schema_version table
psql -c "
INSERT INTO schema_migrations (version, dirty)
SELECT version, false FROM schema_version
ON CONFLICT (version) DO NOTHING;
" > /dev/null 2>&1 || true

CURRENT_VERSION=$(psql -t -c "SELECT COALESCE(MAX(version), 0) FROM schema_migrations WHERE dirty = false;" 2>/dev/null || echo "0")
CURRENT_VERSION=$(echo $CURRENT_VERSION | tr -d ' ')
echo -e "Current version: ${GREEN}${CURRENT_VERSION}${NC}"

# Run migrations based on version
MIGRATIONS_DIR="$(dirname "$0")/../migrations"

run_migration() {
    local version=$1
    local file=$2

    if [ "$CURRENT_VERSION" -lt "$version" ]; then
        echo -e "${YELLOW}Running migration ${file}...${NC}"
        psql -f "$MIGRATIONS_DIR/${file}"
        psql -c "INSERT INTO schema_migrations (version, dirty) VALUES (${version}, false) ON CONFLICT (version) DO UPDATE SET dirty = false;" > /dev/null
        echo -e "${GREEN}✓ Migration ${version} complete${NC}"
    fi
}

run_migration 1 000001_initial_schema.up.sql
run_migration 2 000002_add_missing_rls_policies.up.sql
run_migration 3 000003_add_soft_delete.up.sql
run_migration 4 000004_add_partitioned_table_fks.up.sql
run_migration 5 000005_add_counter_triggers.up.sql
run_migration 6 000006_enhance_type_tables.up.sql
run_migration 7 000007_force_rls.up.sql

# Show final status
echo ""
echo -e "${GREEN}=== Migration Complete ===${NC}"
echo -e "${YELLOW}Verifying tables...${NC}"
psql -c "\dt" | head -50

echo ""
echo -e "${YELLOW}Schema migrations:${NC}"
psql -c "SELECT * FROM schema_migrations ORDER BY version;"
