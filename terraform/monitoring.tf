# =============================================================================
# Accountant CRM - CloudMonitor Alerts
# =============================================================================
# Monitoring and alerting for ECI containers, Redis, and other resources
# Updated for alicloud provider 1.216.0+ (new escalations_critical syntax)
# =============================================================================

# =============================================================================
# Alert Contact Group
# =============================================================================

resource "alicloud_cms_alarm_contact_group" "engineering" {
  alarm_contact_group_name = "${local.name_prefix}-engineering"
  contacts                 = var.alarm_contact_groups
  describe                 = "Engineering team alerts for ${var.project_name}"
}

# =============================================================================
# ECI Container Restart Alerts
# =============================================================================

# Alert when Go backend restarts more than 3 times in 5 minutes
resource "alicloud_cms_alarm" "go_backend_restarts" {
  name               = "${local.name_prefix}-go-backend-restarts"
  project            = "acs_eci"
  metric             = "container_restart_count"
  period             = 300 # 5 minutes
  contact_groups     = [alicloud_cms_alarm_contact_group.engineering.alarm_contact_group_name]
  effective_interval = "00:00-23:59"

  dimensions = {
    containerGroupId = alicloud_eci_container_group.go_backend.id
  }

  escalations_critical {
    statistics = "Sum"
    threshold  = "3"
    times      = 1
  }

  enabled = var.enable_monitoring
}

# Alert when Python AI restarts more than 3 times in 5 minutes
resource "alicloud_cms_alarm" "python_ai_restarts" {
  name               = "${local.name_prefix}-python-ai-restarts"
  project            = "acs_eci"
  metric             = "container_restart_count"
  period             = 300 # 5 minutes
  contact_groups     = [alicloud_cms_alarm_contact_group.engineering.alarm_contact_group_name]
  effective_interval = "00:00-23:59"

  dimensions = {
    containerGroupId = alicloud_eci_container_group.python_ai.id
  }

  escalations_critical {
    statistics = "Sum"
    threshold  = "3"
    times      = 1
  }

  enabled = var.enable_monitoring
}

# =============================================================================
# ECI Container CPU Alerts
# =============================================================================

resource "alicloud_cms_alarm" "go_backend_cpu" {
  name               = "${local.name_prefix}-go-backend-cpu"
  project            = "acs_eci"
  metric             = "cpu_utilization"
  period             = 300
  contact_groups     = [alicloud_cms_alarm_contact_group.engineering.alarm_contact_group_name]
  effective_interval = "00:00-23:59"

  dimensions = {
    containerGroupId = alicloud_eci_container_group.go_backend.id
  }

  escalations_critical {
    statistics = "Average"
    threshold  = "80"
    times      = 3 # Alert after 3 consecutive periods (15 min)
  }

  enabled = var.enable_monitoring
}

resource "alicloud_cms_alarm" "python_ai_cpu" {
  name               = "${local.name_prefix}-python-ai-cpu"
  project            = "acs_eci"
  metric             = "cpu_utilization"
  period             = 300
  contact_groups     = [alicloud_cms_alarm_contact_group.engineering.alarm_contact_group_name]
  effective_interval = "00:00-23:59"

  dimensions = {
    containerGroupId = alicloud_eci_container_group.python_ai.id
  }

  escalations_critical {
    statistics = "Average"
    threshold  = "80"
    times      = 3
  }

  enabled = var.enable_monitoring
}

# =============================================================================
# ECI Container Memory Alerts
# =============================================================================

resource "alicloud_cms_alarm" "go_backend_memory" {
  name               = "${local.name_prefix}-go-backend-memory"
  project            = "acs_eci"
  metric             = "memory_utilization"
  period             = 300
  contact_groups     = [alicloud_cms_alarm_contact_group.engineering.alarm_contact_group_name]
  effective_interval = "00:00-23:59"

  dimensions = {
    containerGroupId = alicloud_eci_container_group.go_backend.id
  }

  escalations_critical {
    statistics = "Average"
    threshold  = "85"
    times      = 3
  }

  enabled = var.enable_monitoring
}

resource "alicloud_cms_alarm" "python_ai_memory" {
  name               = "${local.name_prefix}-python-ai-memory"
  project            = "acs_eci"
  metric             = "memory_utilization"
  period             = 300
  contact_groups     = [alicloud_cms_alarm_contact_group.engineering.alarm_contact_group_name]
  effective_interval = "00:00-23:59"

  dimensions = {
    containerGroupId = alicloud_eci_container_group.python_ai.id
  }

  escalations_critical {
    statistics = "Average"
    threshold  = "85"
    times      = 3
  }

  enabled = var.enable_monitoring
}

# =============================================================================
# Redis Alerts
# =============================================================================

resource "alicloud_cms_alarm" "redis_memory" {
  name               = "${local.name_prefix}-redis-memory"
  project            = "acs_kvstore"
  metric             = "MemoryUsage"  # Percentage metric (0-100), not UsedMemory (bytes)
  period             = 300
  contact_groups     = [alicloud_cms_alarm_contact_group.engineering.alarm_contact_group_name]
  effective_interval = "00:00-23:59"

  dimensions = {
    instanceId = alicloud_kvstore_instance.redis.id
  }

  escalations_critical {
    statistics = "Average"
    threshold  = "80"  # 80% memory usage
    times      = 2
  }

  enabled = var.enable_monitoring
}

resource "alicloud_cms_alarm" "redis_connections" {
  name               = "${local.name_prefix}-redis-connections"
  project            = "acs_kvstore"
  metric             = "UsedConnection"
  period             = 300
  contact_groups     = [alicloud_cms_alarm_contact_group.engineering.alarm_contact_group_name]
  effective_interval = "00:00-23:59"

  dimensions = {
    instanceId = alicloud_kvstore_instance.redis.id
  }

  escalations_critical {
    statistics = "Average"
    threshold  = "900" # Alert at 900 connections (assume 1000 max)
    times      = 2
  }

  enabled = var.enable_monitoring
}

# =============================================================================
# Outputs
# =============================================================================

output "monitoring_contact_group" {
  description = "CloudMonitor contact group name"
  value       = alicloud_cms_alarm_contact_group.engineering.alarm_contact_group_name
}

output "alarm_ids" {
  description = "CloudMonitor alarm IDs"
  value = {
    go_backend_restarts = alicloud_cms_alarm.go_backend_restarts.id
    python_ai_restarts  = alicloud_cms_alarm.python_ai_restarts.id
    go_backend_cpu      = alicloud_cms_alarm.go_backend_cpu.id
    python_ai_cpu       = alicloud_cms_alarm.python_ai_cpu.id
    go_backend_memory   = alicloud_cms_alarm.go_backend_memory.id
    python_ai_memory    = alicloud_cms_alarm.python_ai_memory.id
    redis_memory        = alicloud_cms_alarm.redis_memory.id
    redis_connections   = alicloud_cms_alarm.redis_connections.id
  }
}
