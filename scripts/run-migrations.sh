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

# Database credentials
export PGHOST="ep-holy-moon-za23i30g-pooler.c-2.eu-west-2.aws.neon.tech"
export PGPORT="5432"
export PGUSER="neondb_owner"
export PGPASSWORD="npg_2tYNgQCqbay0"
export PGDATABASE="neondb"
export PGSSLMODE="require"

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

# Get current schema version
echo -e "${YELLOW}Checking current schema version...${NC}"
CURRENT_VERSION=$(psql -t -c "SELECT COALESCE(MAX(version), 0) FROM schema_version;" 2>/dev/null || echo "0")
CURRENT_VERSION=$(echo $CURRENT_VERSION | tr -d ' ')
echo -e "Current version: ${GREEN}${CURRENT_VERSION}${NC}"

# Run migrations based on version
MIGRATIONS_DIR="$(dirname "$0")/../migrations"

if [ "$CURRENT_VERSION" -lt 1 ]; then
    echo -e "${YELLOW}Running migration 000001_initial_schema.up.sql...${NC}"
    psql -f "$MIGRATIONS_DIR/000001_initial_schema.up.sql"
    echo -e "${GREEN}✓ Migration 000001 complete${NC}"
fi

if [ "$CURRENT_VERSION" -lt 2 ]; then
    echo -e "${YELLOW}Running migration 000002_add_missing_rls_policies.up.sql...${NC}"
    psql -f "$MIGRATIONS_DIR/000002_add_missing_rls_policies.up.sql"
    echo -e "${GREEN}✓ Migration 000002 complete${NC}"
fi

if [ "$CURRENT_VERSION" -lt 3 ]; then
    echo -e "${YELLOW}Running migration 000003_add_soft_delete.up.sql...${NC}"
    psql -f "$MIGRATIONS_DIR/000003_add_soft_delete.up.sql"
    echo -e "${GREEN}✓ Migration 000003 complete${NC}"
fi

# Show final status
echo ""
echo -e "${GREEN}=== Migration Complete ===${NC}"
echo -e "${YELLOW}Verifying tables...${NC}"
psql -c "\dt" | head -50

echo ""
echo -e "${YELLOW}Schema version:${NC}"
psql -c "SELECT * FROM schema_version;"
