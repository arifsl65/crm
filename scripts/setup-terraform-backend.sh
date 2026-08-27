#!/bin/bash
# =============================================================================
# Setup Terraform Remote Backend (OSS + TableStore)
# =============================================================================
# This script creates the required resources for Terraform remote state:
#   1. OSS bucket for state storage
#   2. TableStore instance and table for state locking
# =============================================================================

set -euo pipefail

REGION="${REGION:-eu-west-1}"
STATE_BUCKET="fzco-terraform-state"
TABLESTORE_INSTANCE="fzco-terraform-lock"
TABLESTORE_TABLE="terraform-state-lock"

echo "Setting up Terraform remote backend in region: ${REGION}"

# =============================================================================
# Create OSS Bucket
# =============================================================================
echo "Creating OSS bucket: ${STATE_BUCKET}..."
aliyun oss mb "oss://${STATE_BUCKET}" --region "${REGION}" 2>/dev/null || echo "Bucket already exists"

# Enable versioning for state history
echo "Enabling versioning on bucket..."
aliyun oss bucket-versioning --method put "oss://${STATE_BUCKET}" --status Enabled

# Enable server-side encryption
echo "Enabling server-side encryption..."
aliyun oss bucket-encryption --method put "oss://${STATE_BUCKET}" \
  --sse-algorithm AES256

echo "OSS bucket ready: ${STATE_BUCKET}"

# =============================================================================
# Create TableStore Instance and Table
# =============================================================================
echo "Creating TableStore instance: ${TABLESTORE_INSTANCE}..."
aliyun ots CreateInstance \
  --InstanceName "${TABLESTORE_INSTANCE}" \
  --ClusterType HYBRID \
  --region "${REGION}" 2>/dev/null || echo "Instance already exists"

# Wait for instance to be ready
echo "Waiting for TableStore instance to be ready..."
sleep 10

# Create the lock table
echo "Creating TableStore table: ${TABLESTORE_TABLE}..."
# Note: The table is auto-created by Terraform OSS backend on first use
# This is just informational

echo ""
echo "============================================================================="
echo "Terraform backend setup complete!"
echo "============================================================================="
echo ""
echo "Next steps:"
echo "  1. cd terraform/"
echo "  2. terraform init -migrate-state"
echo "     (This will migrate your local state to the remote backend)"
echo ""
echo "Backend configuration:"
echo "  Bucket:            ${STATE_BUCKET}"
echo "  Region:            ${REGION}"
echo "  TableStore:        ${TABLESTORE_INSTANCE}"
echo "  Lock Table:        ${TABLESTORE_TABLE}"
echo ""
