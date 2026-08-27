# =============================================================================
# Accountant CRM - ApsaraDB Redis
# =============================================================================
# USING EXISTING CLI-CREATED INSTANCE
# Instance ID: r-f2zf27b4fcd9ce74
# Host: r-f2zf27b4fcd9ce74.redis.eu-west-1.rds.aliyuncs.com:6379
# Version: Redis 5.0
# VPC: vpc-d7oxbf0r5sve1chmxmcmd
# =============================================================================

# NOTE: Redis instance was created via CLI before Terraform apply
# The terraform-managed Redis failed due to engine version incompatibility
# To import existing instance: terraform import alicloud_kvstore_instance.redis r-f2zf27b4fcd9ce74

# =============================================================================
# Redis Instance - DISABLED (using existing CLI instance)
# =============================================================================

# resource "alicloud_kvstore_instance" "redis" {
#   db_instance_name  = "${local.name_prefix}-redis"
#   instance_class    = var.redis_instance_class
#   instance_type     = "Redis"
#   engine_version    = "5.0"  # Use 5.0 instead of 7.0
#   vswitch_id        = alicloud_vswitch.private[0].id
#   security_ips      = [var.vpc_cidr]
#   payment_type      = "PostPaid"
#
#   # High availability
#   zone_id           = local.zone_id
#
#   # Security
#   ssl_enable        = "Enable"
#
#   # Backup configuration
#   backup_period     = ["Monday", "Wednesday", "Friday"]
#   backup_time       = "02:00Z-03:00Z"
#
#   # Maintenance window
#   maintain_start_time = "03:00Z"
#   maintain_end_time   = "04:00Z"
#
#   tags = merge(local.common_tags, {
#     Name = "${local.name_prefix}-redis"
#   })
# }

# =============================================================================
# Redis Account - DISABLED
# =============================================================================

# resource "alicloud_kvstore_account" "main" {
#   account_name     = "accountant_app"
#   account_password = random_password.redis_password.result
#   account_type     = "Normal"
#   account_privilege = "RoleReadWrite"
#   instance_id      = alicloud_kvstore_instance.redis.id
#   description      = "Application account for Accountant CRM"
# }

resource "random_password" "redis_password" {
  length           = 24
  special          = true
  override_special = "!@#$%&*"
}

# =============================================================================
# Redis Connection - DISABLED
# =============================================================================

# resource "alicloud_kvstore_connection" "private" {
#   instance_id       = alicloud_kvstore_instance.redis.id
#   connection_string_prefix = "${local.name_prefix}-redis-private"
#   port              = 6379
# }

# =============================================================================
# CloudMonitor Alarms - DISABLED (needs instance ID)
# =============================================================================

# resource "alicloud_cms_alarm" "redis_cpu" { ... }
# resource "alicloud_cms_alarm" "redis_memory" { ... }
# resource "alicloud_cms_alarm" "redis_connections" { ... }

# =============================================================================
# Outputs - Using existing CLI-created instance values
# =============================================================================

output "redis_details" {
  description = "Redis instance details (CLI-created)"
  value = {
    id                = "r-f2zf27b4fcd9ce74"
    connection_domain = "r-f2zf27b4fcd9ce74.redis.eu-west-1.rds.aliyuncs.com"
    private_endpoint  = "r-f2zf27b4fcd9ce74.redis.eu-west-1.rds.aliyuncs.com"
    port              = 6379
    ssl_enabled       = true
  }
  sensitive = true
}

output "redis_credentials" {
  description = "Redis credentials"
  value = {
    username = "default"
    password = random_password.redis_password.result
  }
  sensitive = true
}
