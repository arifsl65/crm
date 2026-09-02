#!/bin/bash
# =============================================================================
# Rotate app_user Database Password
# =============================================================================
# This script rotates the app_user password in Neon PostgreSQL.
#
# Usage:
#   ./scripts/rotate-db-password.sh
#
# Prerequisites:
#   - POSTGRES_MIGRATION_USER and POSTGRES_MIGRATION_PASSWORD in .env
#   - New password will be generated or can be provided via NEW_APP_USER_PASSWORD
# =============================================================================

set -euo pipefail

# Load environment variables
if [ -f .env.ecs ]; then
    source .env.ecs
elif [ -f .env ]; then
    source .env
else
    echo "ERROR: No .env file found"
    exit 1
fi

# Database connection for migrations (owner role)
MIGRATION_HOST="${POSTGRES_HOST:-ep-holy-moon-za23i30g-pooler.c-2.eu-west-2.aws.neon.tech}"
MIGRATION_USER="${POSTGRES_MIGRATION_USER:-neondb_owner}"
MIGRATION_PASSWORD="${POSTGRES_MIGRATION_PASSWORD}"
MIGRATION_DB="${POSTGRES_DB:-neondb}"

if [ -z "$MIGRATION_PASSWORD" ]; then
    echo "ERROR: POSTGRES_MIGRATION_PASSWORD not set"
    exit 1
fi

# Generate new password if not provided
if [ -z "${NEW_APP_USER_PASSWORD:-}" ]; then
    # Generate a secure 32-character password
    NEW_APP_USER_PASSWORD=$(openssl rand -base64 32 | tr -dc 'a-zA-Z0-9!@#$%^&*' | head -c 32)
    echo "Generated new password (save this securely!):"
    echo "NEW_APP_USER_PASSWORD=$NEW_APP_USER_PASSWORD"
    echo ""
fi

echo "Rotating app_user password..."

# Connect as migration user and change password
PGPASSWORD="$MIGRATION_PASSWORD" psql \
    "postgresql://${MIGRATION_USER}@${MIGRATION_HOST}:5432/${MIGRATION_DB}?sslmode=require" \
    -c "ALTER ROLE app_user WITH PASSWORD '$NEW_APP_USER_PASSWORD';"

if [ $? -eq 0 ]; then
    echo ""
    echo "SUCCESS: Password rotated successfully!"
    echo ""
    echo "NEXT STEPS:"
    echo "1. Update POSTGRES_PASSWORD in .env.ecs (local) and /opt/accountant-crm/.env (server)"
    echo "2. Restart the Go backend: docker restart accountant-go-backend"
    echo "3. Test the connection: curl https://api.irislondonshoes.com/health"
    echo ""
    echo "New password value for .env files:"
    echo "POSTGRES_PASSWORD=$NEW_APP_USER_PASSWORD"
else
    echo "ERROR: Failed to rotate password"
    exit 1
fi
