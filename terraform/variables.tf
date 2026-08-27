# =============================================================================
# Accountant CRM - Terraform Variables
# =============================================================================

# =============================================================================
# Provider Configuration
# =============================================================================

variable "alicloud_access_key" {
  description = "Alibaba Cloud Access Key ID"
  type        = string
  sensitive   = true
}

variable "alicloud_secret_key" {
  description = "Alibaba Cloud Access Key Secret"
  type        = string
  sensitive   = true
}

variable "region" {
  description = "Alibaba Cloud region"
  type        = string
  default     = "eu-west-1" # UK (London)
}

# =============================================================================
# Project Configuration
# =============================================================================

variable "project_name" {
  description = "Project name used for resource naming"
  type        = string
  default     = "accountant-crm"
}

variable "environment" {
  description = "Environment (staging, production)"
  type        = string
  validation {
    condition     = contains(["staging", "production"], var.environment)
    error_message = "Environment must be staging or production."
  }
}

# =============================================================================
# Network Configuration
# =============================================================================

variable "vpc_cidr" {
  description = "CIDR block for VPC"
  type        = string
  default     = "10.0.0.0/16"
}

variable "public_subnet_cidrs" {
  description = "CIDR blocks for public subnets"
  type        = list(string)
  default     = ["10.0.1.0/24", "10.0.2.0/24"]
}

variable "private_subnet_cidrs" {
  description = "CIDR blocks for private subnets"
  type        = list(string)
  default     = ["10.0.10.0/24", "10.0.11.0/24"]
}

# =============================================================================
# ECS Configuration
# =============================================================================

variable "ecs_instance_type" {
  description = "ECS instance type"
  type        = string
  default     = "ecs.g6.large"
}

variable "ecs_desired_count" {
  description = "Desired number of ECS instances"
  type        = number
  default     = 2
}

variable "ecs_min_count" {
  description = "Minimum number of ECS instances"
  type        = number
  default     = 1
}

variable "ecs_max_count" {
  description = "Maximum number of ECS instances"
  type        = number
  default     = 4
}

# =============================================================================
# Container Configuration
# =============================================================================

# Fix #31: image_tag validation - "latest" is dangerous in production
variable "image_tag" {
  description = "Docker image tag to deploy (do not use 'latest' in production)"
  type        = string
  default     = "latest"

  validation {
    condition     = var.image_tag != ""
    error_message = "Image tag cannot be empty."
  }

  # Note: "latest" is allowed but will trigger a warning via check block in main.tf
}

variable "acr_username" {
  description = "ACR registry username for image pulls"
  type        = string
  sensitive   = true
}

variable "acr_password" {
  description = "ACR registry password for image pulls"
  type        = string
  sensitive   = true
}

variable "go_backend_cpu" {
  description = "CPU units for Go backend container"
  type        = number
  default     = 512
}

variable "go_backend_memory" {
  description = "Memory (MB) for Go backend container"
  type        = number
  default     = 1024
}

variable "python_ai_cpu" {
  description = "CPU units for Python AI container"
  type        = number
  default     = 1024
}

variable "python_ai_memory" {
  description = "Memory (MB) for Python AI container"
  type        = number
  default     = 2048
}

# =============================================================================
# Redis Configuration
# =============================================================================

variable "redis_instance_class" {
  description = "Redis instance class"
  type        = string
  default     = "redis.master.small.default"
}

variable "redis_engine_version" {
  description = "Redis engine version"
  type        = string
  default     = "7.0"
}

# =============================================================================
# ALB Configuration
# =============================================================================

variable "alb_idle_timeout" {
  description = "ALB idle timeout in seconds"
  type        = number
  default     = 60
}

variable "ssl_certificate_id" {
  description = "Alibaba Cloud SSL certificate ID (if empty, uses local certs from terraform/certs/)"
  type        = string
  default     = ""
}

# =============================================================================
# OSS Configuration
# =============================================================================

variable "oss_frontend_bucket" {
  description = "OSS bucket name for frontend assets"
  type        = string
  default     = "fzco-frontend"
}

variable "oss_uploads_bucket" {
  description = "OSS bucket name for uploads"
  type        = string
  default     = "fzco-uploads"
}

variable "oss_uploads_staging_bucket" {
  description = "OSS bucket name for staging uploads"
  type        = string
  default     = "fzco-uploads-stg"
}

# =============================================================================
# MNS Configuration
# =============================================================================

variable "mns_topics" {
  description = "List of MNS topic names (must match workflow spec, use hyphens not dots)"
  type        = list(string)
  default = [
    "doc-uploaded",
    "doc-processed",
    "email-received",
    "service-at-risk",
    "chase-due",
    "user-deleted"
  ]
}

# =============================================================================
# Domain Configuration
# =============================================================================

variable "domain_name" {
  description = "Primary domain name"
  type        = string
  default     = "accountant-crm.com"
}

variable "api_subdomain" {
  description = "API subdomain"
  type        = string
  default     = "api"
}

variable "app_subdomain" {
  description = "Application subdomain"
  type        = string
  default     = "app"
}

# =============================================================================
# Monitoring Configuration
# =============================================================================

variable "enable_monitoring" {
  description = "Enable CloudMonitor integration"
  type        = bool
  default     = true
}

variable "alarm_contact_groups" {
  description = "Contact groups for alarms"
  type        = list(string)
  default     = ["engineering"]
}

# =============================================================================
# External Database Configuration
# =============================================================================

variable "neon_host" {
  description = "Neon PostgreSQL host"
  type        = string
}

variable "neon_user" {
  description = "Neon PostgreSQL username"
  type        = string
}

variable "neon_password" {
  description = "Neon PostgreSQL password"
  type        = string
  sensitive   = true
}

variable "neon_database" {
  description = "Neon PostgreSQL database name"
  type        = string
  default     = "neondb"
}

variable "redis_password" {
  description = "Redis instance password (required for production)"
  type        = string
  sensitive   = true

  validation {
    condition     = length(var.redis_password) >= 8 || length(var.redis_password) == 0
    error_message = "Redis password must be at least 8 characters if provided."
  }
}

variable "jwt_secret_key" {
  description = "JWT secret key for signing tokens (min 32 chars for HS256)"
  type        = string
  sensitive   = true

  validation {
    condition     = length(var.jwt_secret_key) >= 32
    error_message = "JWT secret key must be at least 32 characters for HS256."
  }
}

variable "mongodb_uri" {
  description = "MongoDB Atlas connection URI"
  type        = string
  sensitive   = true
}

# =============================================================================
# mTLS Configuration
# =============================================================================

variable "mtls_ca_cert" {
  description = "mTLS CA certificate (PEM format)"
  type        = string
  sensitive   = true
  default     = ""
}

variable "mtls_server_cert" {
  description = "mTLS server certificate (PEM format)"
  type        = string
  sensitive   = true
  default     = ""
}

variable "mtls_server_key" {
  description = "mTLS server private key (PEM format)"
  type        = string
  sensitive   = true
  default     = ""
}

variable "mtls_client_cert" {
  description = "mTLS client certificate (PEM format)"
  type        = string
  sensitive   = true
  default     = ""
}

variable "mtls_client_key" {
  description = "mTLS client private key (PEM format)"
  type        = string
  sensitive   = true
  default     = ""
}
