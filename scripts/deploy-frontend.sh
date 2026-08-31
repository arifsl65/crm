#!/usr/bin/env bash
# =============================================================================
# Accountant CRM - Frontend Deployment Script
# =============================================================================
# Usage: ./deploy-frontend.sh [staging|production]
#
# Deploys the frontend to the ECS server via rsync.
# Architecture: Single ECS server with nginx serving static files.
# See cloud.md for infrastructure details.
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

# Server configuration (from cloud.md)
ECS_HOST="8.211.195.17"
ECS_USER="root"
REMOTE_PATH="/opt/accountant-crm/out"
SSH_KEY="${SSH_KEY:-$HOME/.ssh/id_rsa}"

# Environment-specific settings
declare -A API_URLS=(
    ["staging"]="https://api.irislondonshoes.com"
    ["production"]="https://api.irislondonshoes.com"
)

declare -A FRONTEND_URLS=(
    ["staging"]="https://crm.irislondonshoes.com"
    ["production"]="https://crm.irislondonshoes.com"
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
    local deps=("node" "npm" "rsync" "ssh")
    for dep in "${deps[@]}"; do
        if ! command -v "$dep" &> /dev/null; then
            log_error "Required command '$dep' not found"
            exit 1
        fi
    done

    # Check SSH key exists
    if [[ ! -f "$SSH_KEY" ]]; then
        log_error "SSH key not found at $SSH_KEY"
        log_error "Set SSH_KEY environment variable or ensure ~/.ssh/id_rsa exists"
        exit 1
    fi
}

validate_environment() {
    local env=$1
    if [[ ! "${API_URLS[$env]+isset}" ]]; then
        log_error "Invalid environment: $env"
        echo "Usage: $0 [staging|production]"
        exit 1
    fi
}

test_ssh_connection() {
    log_info "Testing SSH connection to $ECS_HOST..."

    if ! ssh -i "$SSH_KEY" -o ConnectTimeout=10 -o BatchMode=yes "$ECS_USER@$ECS_HOST" "echo 'SSH OK'" &> /dev/null; then
        log_error "Cannot connect to $ECS_HOST via SSH"
        log_error "Ensure SSH key is authorized on the server"
        exit 1
    fi

    log_info "SSH connection successful"
}

build_frontend() {
    local env=$1
    log_info "Building frontend for $env..."

    cd "$FRONTEND_DIR"

    # Install dependencies
    log_info "Installing dependencies..."
    npm ci --silent

    # Set environment variables for build
    export NEXT_PUBLIC_API_URL="${API_URLS[$env]}"
    export NEXT_PUBLIC_AI_API_URL="${API_URLS[$env]}"
    export NODE_ENV="production"

    # Build static export (outputs to out/ directory)
    log_info "Running Next.js build..."
    npm run build

    # Verify build output exists
    if [[ ! -d "out" ]]; then
        log_error "Build failed: out/ directory not found"
        exit 1
    fi

    local file_count
    file_count=$(find out -type f | wc -l)
    log_info "Build complete: $file_count files in out/"
}

deploy_to_server() {
    local env=$1

    log_info "Deploying to $ECS_HOST:$REMOTE_PATH..."

    cd "$FRONTEND_DIR"

    # Ensure remote directory exists
    ssh -i "$SSH_KEY" "$ECS_USER@$ECS_HOST" "mkdir -p $REMOTE_PATH"

    # Rsync the build output
    # --delete: Remove files on remote that don't exist locally
    # --checksum: Use checksum instead of time/size for comparison
    # --compress: Compress during transfer
    rsync -avz --delete --checksum \
        --exclude ".git" \
        --exclude ".DS_Store" \
        --exclude "*.map" \
        -e "ssh -i $SSH_KEY" \
        out/ \
        "$ECS_USER@$ECS_HOST:$REMOTE_PATH/"

    log_info "Deployment complete"
}

set_permissions() {
    log_info "Setting file permissions..."

    ssh -i "$SSH_KEY" "$ECS_USER@$ECS_HOST" "
        chown -R www-data:www-data $REMOTE_PATH 2>/dev/null || chown -R nginx:nginx $REMOTE_PATH 2>/dev/null || true
        chmod -R 755 $REMOTE_PATH
    "

    log_info "Permissions set"
}

verify_deployment() {
    local env=$1
    local url="${FRONTEND_URLS[$env]}"

    log_info "Verifying deployment at $url..."

    # Wait a moment for nginx to pick up changes
    sleep 2

    # Check if site is accessible
    local status_code
    status_code=$(curl -s -o /dev/null -w "%{http_code}" --max-time 10 "$url/" || echo "000")

    if [[ "$status_code" == "200" ]]; then
        log_info "Deployment verified successfully!"
        echo ""
        echo -e "${GREEN}Frontend deployed to: $url${NC}"
    else
        log_warn "Site returned status $status_code - please verify manually"
        log_warn "This may be normal if SSL certificate is self-signed"
    fi
}

confirm_production() {
    echo ""
    echo -e "${YELLOW}WARNING: You are about to deploy to PRODUCTION${NC}"
    echo -e "Server: $ECS_USER@$ECS_HOST"
    echo -e "Path: $REMOTE_PATH"
    echo ""
    read -p "Type 'production' to confirm: " confirmation

    if [[ "$confirmation" != "production" ]]; then
        log_error "Deployment cancelled"
        exit 1
    fi
}

show_usage() {
    echo "Usage: $0 [staging|production]"
    echo ""
    echo "Environment variables:"
    echo "  SSH_KEY    Path to SSH private key (default: ~/.ssh/id_rsa)"
    echo "  ECS_HOST   Server hostname/IP (default: 8.211.195.17)"
    echo "  ECS_USER   SSH user (default: root)"
    echo ""
    echo "Examples:"
    echo "  $0 production"
    echo "  SSH_KEY=~/.ssh/ecs_key $0 production"
}

main() {
    local env="${1:-}"

    if [[ -z "$env" || "$env" == "-h" || "$env" == "--help" ]]; then
        show_usage
        exit 0
    fi

    echo ""
    echo "========================================"
    echo "  Frontend Deployment - $env"
    echo "========================================"
    echo ""

    # Validate
    validate_environment "$env"
    check_dependencies
    test_ssh_connection

    # Production confirmation
    if [[ "$env" == "production" ]]; then
        confirm_production
    fi

    # Deploy
    build_frontend "$env"
    deploy_to_server "$env"
    set_permissions
    verify_deployment "$env"

    echo ""
    log_info "Deployment complete!"
}

# Run main function
main "$@"
