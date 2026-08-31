# =============================================================================
# Accountant CRM - Terraform Variables
# =============================================================================
# Updated: 2026-08-29
# Architecture: Single ECS + Docker Compose (~$17/month)
#
# Note: Container orchestration is now handled by Docker Compose on ECS.
# Terraform manages: VPC, VSwitches, Security Groups, OSS buckets only.
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

# =============================================================================
# Docker Image Tags (for documentation/checks only)
# =============================================================================
# Note: Actual deployment uses Docker Compose on ECS VM.
# These variables are kept for Terraform checks and documentation.

variable "image_tag" {
  description = "Docker image tag (for checks - actual deployment uses docker-compose.ecs.yml)"
  type        = string
  default     = "latest"

  validation {
    condition     = var.image_tag != ""
    error_message = "Image tag cannot be empty."
  }
}

variable "python_ai_image_tag" {
  description = "Docker image tag for Python AI service (defaults to image_tag if not set)"
  type        = string
  default     = ""
}

# =============================================================================
# External Database Configuration
# =============================================================================
# Note: These are used for Terraform checks only.
# Actual values are in .env.ecs on the ECS server.

variable "neon_host" {
  description = "Neon PostgreSQL host"
  type        = string
  default     = ""
}

variable "neon_user" {
  description = "Neon PostgreSQL username"
  type        = string
  default     = ""
}

variable "neon_password" {
  description = "Neon PostgreSQL password"
  type        = string
  sensitive   = true
  default     = ""
}

variable "neon_database" {
  description = "Neon PostgreSQL database name"
  type        = string
  default     = "neondb"
}

variable "redis_password" {
  description = "Redis password (Docker container uses this)"
  type        = string
  sensitive   = true
  default     = ""
}

variable "jwt_secret_key" {
  description = "JWT secret key (min 32 chars for HS256)"
  type        = string
  sensitive   = true
  default     = ""
}

variable "mongodb_uri" {
  description = "MongoDB Atlas connection URI"
  type        = string
  sensitive   = true
  default     = ""
}
