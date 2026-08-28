# =============================================================================
# Accountant CRM - ApsaraDB Redis
# =============================================================================
# Creating new Redis instance in the accountant VPC (vpc-d7own9jjfj3kbyqpuqgki)
# The old CLI instance (r-f2zf27b4fcd9ce74) is in a different VPC and unreachable
# =============================================================================

# =============================================================================
# Redis Instance - NEW instance in accountant VPC
# =============================================================================

resource "alicloud_kvstore_instance" "redis" {
  db_instance_name  = "${local.name_prefix}-redis"
  instance_class    = var.redis_instance_class
  instance_type     = "Redis"
  engine_version    = "5.0"
  vswitch_id        = alicloud_vswitch.private[0].id
  security_ips      = [var.vpc_cidr]
  payment_type      = "PostPaid"
  password          = random_password.redis_password.result

  # High availability
  zone_id           = data.alicloud_zones.available.zones[0].id

  # Security - disable SSL for internal VPC traffic (simpler for staging)
  ssl_enable        = "Disable"

  # Backup configuration
  backup_period     = ["Monday", "Wednesday", "Friday"]
  backup_time       = "02:00Z-03:00Z"

  # Maintenance window
  maintain_start_time = "03:00Z"
  maintain_end_time   = "04:00Z"

  tags = merge(local.common_tags, {
    Name = "${local.name_prefix}-redis"
  })
}

# =============================================================================
# Redis Account - Not needed, using instance password directly
# =============================================================================
# The password is set on the instance via the 'password' attribute

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
# Outputs - Using Terraform-managed Redis instance
# =============================================================================

output "redis_details" {
  description = "Redis instance details"
  value = {
    id                = alicloud_kvstore_instance.redis.id
    connection_domain = alicloud_kvstore_instance.redis.connection_domain
    private_endpoint  = alicloud_kvstore_instance.redis.connection_domain
    port              = 6379
    ssl_enabled       = false
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
