# =============================================================================
# Accountant CRM - Staging Environment Variables
# =============================================================================

# Environment
environment = "staging"

# Network
vpc_cidr             = "10.0.0.0/16"
public_subnet_cidrs  = ["10.0.1.0/24", "10.0.2.0/24"]
private_subnet_cidrs = ["10.0.10.0/24", "10.0.11.0/24"]

# ECS Configuration (smaller for staging)
ecs_instance_type = "ecs.g6.large"
ecs_desired_count = 1
ecs_min_count     = 1
ecs_max_count     = 2

# Container Resources (reduced for staging)
go_backend_cpu    = 256
go_backend_memory = 512
python_ai_cpu     = 512
python_ai_memory  = 1024

# Redis (smaller instance for staging)
redis_instance_class = "redis.master.micro.default"
redis_engine_version = "7.0"

# ALB
alb_idle_timeout = 60

# OSS Buckets
oss_frontend_bucket        = "fzco-frontend-staging"
oss_uploads_bucket         = "fzco-uploads-staging"
oss_uploads_staging_bucket = "fzco-uploads-stg"

# MNS Topics (per spec)
mns_topics = [
  "doc-uploaded",
  "doc-processed",
  "email-received",
  "service-at-risk",
  "chase-due",
  "user-deleted"
]

# Domain (staging subdomain)
domain_name   = "staging.accountant-crm.com"
api_subdomain = "api"
app_subdomain = "app"

# Monitoring (enabled but with relaxed thresholds)
enable_monitoring    = true
alarm_contact_groups = ["engineering-staging"]
