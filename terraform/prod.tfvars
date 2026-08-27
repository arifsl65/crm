# =============================================================================
# Accountant CRM - Production Environment Variables
# =============================================================================

# Environment
environment = "production"

# Network
vpc_cidr             = "10.1.0.0/16"
public_subnet_cidrs  = ["10.1.1.0/24", "10.1.2.0/24"]
private_subnet_cidrs = ["10.1.10.0/24", "10.1.11.0/24"]

# ECS Configuration (production sizing)
ecs_instance_type = "ecs.g6.xlarge"
ecs_desired_count = 2
ecs_min_count     = 2
ecs_max_count     = 8

# Container Resources (production sizing)
go_backend_cpu    = 1024
go_backend_memory = 2048
python_ai_cpu     = 2048
python_ai_memory  = 4096

# Redis (production instance)
redis_instance_class = "redis.master.small.default"
redis_engine_version = "7.0"

# ALB
alb_idle_timeout = 60

# OSS Buckets
oss_frontend_bucket        = "fzco-frontend"
oss_uploads_bucket         = "fzco-uploads"
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

# Domain (production)
domain_name   = "accountant-crm.com"
api_subdomain = "api"
app_subdomain = "app"

# Monitoring (full monitoring for production)
enable_monitoring    = true
alarm_contact_groups = ["engineering", "ops-oncall"]
