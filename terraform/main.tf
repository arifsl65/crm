# =============================================================================
# Accountant CRM - Terraform Main Configuration
# =============================================================================
# Provider: Alibaba Cloud
# Region: UK (London) - eu-west-1
# =============================================================================

terraform {
  required_version = ">= 1.5.0"

  required_providers {
    alicloud = {
      source  = "aliyun/alicloud"
      version = "~> 1.220"
    }
  }

  # Remote state storage in OSS with TableStore locking
  # Prerequisites:
  #   1. Create OSS bucket: aliyun oss mb oss://fzco-terraform-state --region eu-west-1
  #   2. Create TableStore instance and table for state locking
  #   3. Run: terraform init -migrate-state (to migrate local state to remote)
  #
  # TEMPORARILY DISABLED: OSS backend has V1 signature issue
  # TODO: Re-enable after fixing OSS signature version or bucket settings
  #
  # backend "oss" {
  #   bucket  = "fzco-terraform-state"
  #   prefix  = "accountant-crm"
  #   region  = "eu-west-1"
  #   encrypt = true
  #
  #   # State locking via TableStore (prevents concurrent modifications)
  #   tablestore_endpoint = "https://fzco-terraform-lock.eu-west-1.ots.aliyuncs.com"
  #   tablestore_table    = "terraform-state-lock"
  # }
}

# =============================================================================
# Provider Configuration
# =============================================================================

provider "alicloud" {
  region     = var.region
  access_key = var.alicloud_access_key
  secret_key = var.alicloud_secret_key
}

# =============================================================================
# Data Sources
# =============================================================================

# Get available zones
data "alicloud_zones" "available" {
  available_resource_creation = "VSwitch"
}

# Get current account info
data "alicloud_account" "current" {}

# =============================================================================
# Local Variables
# =============================================================================

locals {
  name_prefix = "${var.project_name}-${var.environment}"

  # Image tags - python_ai can have separate tag from go_backend
  python_ai_effective_tag = var.python_ai_image_tag != "" ? var.python_ai_image_tag : var.image_tag

  common_tags = {
    Project     = var.project_name
    Environment = var.environment
    ManagedBy   = "terraform"
    Team        = "engineering"
  }

  # Zone selection
  zone_id = data.alicloud_zones.available.zones[0].id
}

# =============================================================================
# Security Validations
# =============================================================================

check "production_redis_password" {
  assert {
    condition     = var.environment != "production" || length(var.redis_password) >= 8
    error_message = "Redis password is required for production environment (min 8 characters)."
  }
}

check "production_neon_password" {
  assert {
    condition     = var.environment != "production" || length(var.neon_password) >= 8
    error_message = "Neon PostgreSQL password is required for production environment."
  }
}

# Fix #31: Warn against using "latest" tag in production
check "production_image_tag" {
  assert {
    condition     = var.environment != "production" || var.image_tag != "latest"
    error_message = "Using 'latest' image tag in production is not recommended. Use a specific commit SHA or version tag."
  }
}

# =============================================================================
# VPC Network
# =============================================================================

resource "alicloud_vpc" "main" {
  vpc_name   = "${local.name_prefix}-vpc"
  cidr_block = var.vpc_cidr

  tags = merge(local.common_tags, {
    Name = "${local.name_prefix}-vpc"
  })
}

resource "alicloud_vswitch" "public" {
  count = length(var.public_subnet_cidrs)

  vswitch_name = "${local.name_prefix}-public-${count.index + 1}"
  vpc_id       = alicloud_vpc.main.id
  cidr_block   = var.public_subnet_cidrs[count.index]
  zone_id      = data.alicloud_zones.available.zones[count.index % length(data.alicloud_zones.available.zones)].id

  tags = merge(local.common_tags, {
    Name = "${local.name_prefix}-public-${count.index + 1}"
    Type = "public"
  })
}

resource "alicloud_vswitch" "private" {
  count = length(var.private_subnet_cidrs)

  vswitch_name = "${local.name_prefix}-private-${count.index + 1}"
  vpc_id       = alicloud_vpc.main.id
  cidr_block   = var.private_subnet_cidrs[count.index]
  zone_id      = data.alicloud_zones.available.zones[count.index % length(data.alicloud_zones.available.zones)].id

  tags = merge(local.common_tags, {
    Name = "${local.name_prefix}-private-${count.index + 1}"
    Type = "private"
  })
}

# =============================================================================
# Outputs from Main Module
# =============================================================================

output "vpc_id" {
  description = "VPC ID"
  value       = alicloud_vpc.main.id
}

output "public_vswitch_ids" {
  description = "Public VSwitch IDs"
  value       = alicloud_vswitch.public[*].id
}

output "private_vswitch_ids" {
  description = "Private VSwitch IDs"
  value       = alicloud_vswitch.private[*].id
}
