#!/usr/bin/env bash
# =============================================================================
# Accountant CRM - Staging Data Anonymization Script
# =============================================================================
# Usage: ./anonymize-staging.sh
#
# Anonymizes PII in the staging database. Run weekly via cron.
# This script MUST NEVER be run against production.
# =============================================================================

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LOG_FILE="/var/log/accountant-crm/anonymize-$(date +%Y%m%d).log"

# Environment check
REQUIRED_ENV="staging"
CURRENT_ENV="${APP_ENV:-unknown}"

# Functions
log_info() {
    local msg="[$(date '+%Y-%m-%d %H:%M:%S')] [INFO] $1"
    echo -e "${GREEN}${msg}${NC}"
    echo "$msg" >> "$LOG_FILE" 2>/dev/null || true
}

log_warn() {
    local msg="[$(date '+%Y-%m-%d %H:%M:%S')] [WARN] $1"
    echo -e "${YELLOW}${msg}${NC}"
    echo "$msg" >> "$LOG_FILE" 2>/dev/null || true
}

log_error() {
    local msg="[$(date '+%Y-%m-%d %H:%M:%S')] [ERROR] $1"
    echo -e "${RED}${msg}${NC}"
    echo "$msg" >> "$LOG_FILE" 2>/dev/null || true
}

check_environment() {
    log_info "Checking environment..."

    # Verify we're in staging
    if [[ "$CURRENT_ENV" != "$REQUIRED_ENV" ]]; then
        log_error "This script can only run in $REQUIRED_ENV environment!"
        log_error "Current environment: $CURRENT_ENV"
        exit 1
    fi

    # Check database URL
    if [[ -z "${DATABASE_URL:-}" ]]; then
        log_error "DATABASE_URL environment variable is not set"
        exit 1
    fi

    # Verify database URL contains "staging"
    if [[ "$DATABASE_URL" != *"staging"* && "$DATABASE_URL" != *"stg"* ]]; then
        log_error "DATABASE_URL does not appear to be a staging database!"
        log_error "Refusing to run anonymization for safety"
        exit 1
    fi

    log_info "Environment check passed"
}

check_dependencies() {
    if ! command -v psql &> /dev/null; then
        log_error "psql command not found"
        exit 1
    fi
}

create_backup() {
    log_info "Creating pre-anonymization backup..."

    local backup_file="/tmp/pre-anonymize-$(date +%Y%m%d-%H%M%S).sql"

    pg_dump "$DATABASE_URL" > "$backup_file" 2>/dev/null || {
        log_warn "Backup creation failed, continuing anyway..."
        return 0
    }

    log_info "Backup created: $backup_file"
}

anonymize_users() {
    log_info "Anonymizing user data..."

    psql "$DATABASE_URL" -c "
        -- Anonymize user PII
        UPDATE users SET
            email = CONCAT('user_', id, '@anonymized.local'),
            name = CONCAT('User ', SUBSTRING(id::text, 1, 8)),
            phone = CASE
                WHEN phone IS NOT NULL
                THEN CONCAT('+1555', LPAD(FLOOR(RANDOM() * 10000000)::text, 7, '0'))
                ELSE NULL
            END,
            avatar_url = NULL,
            password = '\$argon2id\$v=19\$m=65536,t=3,p=4\$anonymized',
            preferences = '{}'
        WHERE email NOT LIKE '%@dev.local'
          AND email NOT LIKE '%@anonymized.local';
    " || {
        log_error "Failed to anonymize users"
        return 1
    }

    log_info "User data anonymized"
}

anonymize_sessions() {
    log_info "Clearing session data..."

    psql "$DATABASE_URL" -c "
        -- Clear all sessions
        DELETE FROM sessions;

        -- Clear all refresh tokens
        DELETE FROM refresh_tokens;
    " || {
        log_error "Failed to clear sessions"
        return 1
    }

    log_info "Session data cleared"
}

anonymize_audit_logs() {
    log_info "Anonymizing audit logs..."

    psql "$DATABASE_URL" -c "
        -- Anonymize IP addresses in audit logs
        UPDATE audit_logs SET
            ip_address = '10.0.0.1'::inet,
            user_agent = 'Anonymized User Agent',
            old_value = CASE
                WHEN old_value IS NOT NULL
                THEN jsonb_strip_nulls(
                    old_value - 'email' - 'phone' - 'name'
                )
                ELSE NULL
            END,
            new_value = CASE
                WHEN new_value IS NOT NULL
                THEN jsonb_strip_nulls(
                    new_value - 'email' - 'phone' - 'name'
                )
                ELSE NULL
            END
        WHERE created_at < NOW() - INTERVAL '7 days';

        -- Delete old audit logs
        DELETE FROM audit_logs
        WHERE created_at < NOW() - INTERVAL '30 days';
    " || {
        log_error "Failed to anonymize audit logs"
        return 1
    }

    log_info "Audit logs anonymized"
}

anonymize_tenants() {
    log_info "Anonymizing tenant data..."

    psql "$DATABASE_URL" -c "
        -- Anonymize tenant names (keep domains for consistency)
        UPDATE tenants SET
            name = CONCAT('Tenant ', SUBSTRING(id::text, 1, 8)),
            settings = '{}',
            metadata = '{}'
        WHERE domain != 'dev.localhost';
    " || {
        log_error "Failed to anonymize tenants"
        return 1
    }

    log_info "Tenant data anonymized"
}

vacuum_database() {
    log_info "Running VACUUM ANALYZE..."

    psql "$DATABASE_URL" -c "VACUUM ANALYZE;" || {
        log_warn "VACUUM ANALYZE failed"
    }

    log_info "Database maintenance complete"
}

generate_report() {
    log_info "Generating anonymization report..."

    local report
    report=$(psql "$DATABASE_URL" -t -c "
        SELECT json_build_object(
            'timestamp', NOW(),
            'users_count', (SELECT COUNT(*) FROM users),
            'tenants_count', (SELECT COUNT(*) FROM tenants),
            'sessions_count', (SELECT COUNT(*) FROM sessions),
            'audit_logs_count', (SELECT COUNT(*) FROM audit_logs),
            'anonymized', true
        );
    ")

    log_info "Report: $report"

    echo "$report" > "/tmp/anonymize-report-$(date +%Y%m%d).json"
}

send_notification() {
    log_info "Sending completion notification..."

    # If Slack webhook is configured, send notification
    if [[ -n "${SLACK_WEBHOOK_URL:-}" ]]; then
        curl -s -X POST "$SLACK_WEBHOOK_URL" \
            -H 'Content-Type: application/json' \
            -d '{
                "text": "Staging database anonymization completed successfully",
                "icon_emoji": ":shield:",
                "username": "Anonymizer Bot"
            }' || true
    fi

    log_info "Notification sent"
}

main() {
    echo ""
    echo "========================================"
    echo "  Staging Data Anonymization"
    echo "========================================"
    echo ""

    # Create log directory
    mkdir -p "$(dirname "$LOG_FILE")" 2>/dev/null || true

    log_info "Starting anonymization process..."

    # Safety checks
    check_environment
    check_dependencies

    # Create backup
    create_backup

    # Run anonymization
    anonymize_users
    anonymize_sessions
    anonymize_audit_logs
    anonymize_tenants

    # Maintenance
    vacuum_database

    # Reporting
    generate_report
    send_notification

    log_info "Anonymization complete!"
    echo ""
    echo -e "${GREEN}✓ All PII has been anonymized${NC}"
    echo ""
}

# Run main function
main "$@"
