#!/usr/bin/env bash
# =============================================================================
# Accountant CRM - Frontend Deployment Script
# =============================================================================
# Usage: ./deploy-frontend.sh [staging|production]
#
# Deploys the frontend to Alibaba Cloud OSS and invalidates CDN cache.
# =============================================================================

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
FRONTEND_DIR="$PROJECT_ROOT/frontend"

# Environment-specific settings
declare -A BUCKETS=(
    ["staging"]="fzco-frontend-staging"
    ["production"]="fzco-frontend"
)

declare -A API_URLS=(
    ["staging"]="https://api.staging.accountant-crm.com"
    ["production"]="https://api.accountant-crm.com"
)

declare -A CDN_DOMAINS=(
    ["staging"]="staging.accountant-crm.com"
    ["production"]="app.accountant-crm.com"
)

# Functions
log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

check_dependencies() {
    local deps=("node" "npm" "aliyun")
    for dep in "${deps[@]}"; do
        if ! command -v "$dep" &> /dev/null; then
            log_error "Required command '$dep' not found"
            exit 1
        fi
    done
}

validate_environment() {
    local env=$1
    if [[ ! "${BUCKETS[$env]+isset}" ]]; then
        log_error "Invalid environment: $env"
        echo "Usage: $0 [staging|production]"
        exit 1
    fi
}

build_frontend() {
    local env=$1
    log_info "Building frontend for $env..."

    cd "$FRONTEND_DIR"

    # Install dependencies
    npm ci --silent

    # Set environment variables
    export NEXT_PUBLIC_API_URL="${API_URLS[$env]}"
    export NEXT_PUBLIC_AI_API_URL="${API_URLS[$env]}/ai"
    export NODE_ENV="production"

    # Build
    npm run build

    log_info "Build complete"
}

deploy_to_oss() {
    local env=$1
    local bucket="${BUCKETS[$env]}"

    log_info "Deploying to OSS bucket: $bucket..."

    cd "$FRONTEND_DIR"

    # Sync to OSS
    aliyun oss sync out/ "oss://$bucket/" \
        --delete \
        --exclude ".git/*" \
        --exclude ".DS_Store" \
        --exclude "*.map"

    # Set cache headers for static assets
    aliyun oss set-meta "oss://$bucket/_next/static/" \
        --update \
        --recursive \
        Cache-Control:"public, max-age=31536000, immutable"

    # Set no-cache for HTML files (use --include with --recursive)
    aliyun oss set-meta "oss://$bucket/" \
        --update \
        --recursive \
        --include "*.html" \
        Cache-Control:"no-cache, no-store, must-revalidate"

    log_info "Deployment to OSS complete"
}

invalidate_cdn() {
    local env=$1
    local domain="${CDN_DOMAINS[$env]}"

    log_info "Invalidating CDN cache for $domain..."

    # Invalidate entire domain
    aliyun cdn RefreshObjectCaches \
        --ObjectPath "https://$domain/" \
        --ObjectType Directory || {
            log_warn "CDN invalidation failed, may need manual refresh"
        }

    log_info "CDN invalidation requested"
}

verify_deployment() {
    local env=$1
    local domain="${CDN_DOMAINS[$env]}"

    log_info "Verifying deployment..."

    # Wait for CDN propagation
    sleep 10

    # Check if site is accessible
    local status_code
    status_code=$(curl -s -o /dev/null -w "%{http_code}" "https://$domain/")

    if [[ "$status_code" == "200" ]]; then
        log_info "Deployment verified successfully!"
        echo ""
        echo -e "${GREEN}Frontend deployed to: https://$domain/${NC}"
    else
        log_warn "Site returned status $status_code - please verify manually"
    fi
}

confirm_production() {
    echo ""
    echo -e "${YELLOW}⚠️  WARNING: You are about to deploy to PRODUCTION${NC}"
    echo ""
    read -p "Type 'production' to confirm: " confirmation

    if [[ "$confirmation" != "production" ]]; then
        log_error "Deployment cancelled"
        exit 1
    fi
}

main() {
    local env="${1:-staging}"

    echo ""
    echo "========================================"
    echo "  Frontend Deployment - $env"
    echo "========================================"
    echo ""

    # Validate
    validate_environment "$env"
    check_dependencies

    # Production confirmation
    if [[ "$env" == "production" ]]; then
        confirm_production
    fi

    # Deploy
    build_frontend "$env"
    deploy_to_oss "$env"
    invalidate_cdn "$env"
    verify_deployment "$env"

    echo ""
    log_info "Deployment complete!"
}

# Run main function
main "$@"
