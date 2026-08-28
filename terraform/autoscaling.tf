# =============================================================================
# Accountant CRM - Auto Scaling Configuration
# =============================================================================
# ECI supports scaling via Alibaba Cloud Serverless Kubernetes or manual
# scaling. For ECI without K8s, we use CloudMonitor event-triggered scaling.
#
# NOTE: Full auto-scaling requires either:
#   1. Alibaba Serverless Kubernetes (ASK) - recommended for production
#   2. Manual scaling via CloudMonitor webhooks + Function Compute
#
# This file provides the foundation for scaling configuration.
# =============================================================================

# =============================================================================
# Scaling Configuration Variables
# =============================================================================

variable "go_backend_min_replicas" {
  description = "Minimum number of Go backend replicas"
  type        = number
  default     = 1
}

variable "go_backend_max_replicas" {
  description = "Maximum number of Go backend replicas"
  type        = number
  default     = 3
}

variable "python_ai_min_replicas" {
  description = "Minimum number of Python AI replicas"
  type        = number
  default     = 1
}

variable "python_ai_max_replicas" {
  description = "Maximum number of Python AI replicas"
  type        = number
  default     = 3
}

variable "scaling_cpu_threshold" {
  description = "CPU utilization percentage to trigger scale-up"
  type        = number
  default     = 70
}

variable "scaling_cooldown_seconds" {
  description = "Cooldown period between scaling actions (seconds)"
  type        = number
  default     = 300
}

# =============================================================================
# Event-Triggered Scaling (via Function Compute)
# =============================================================================
# When CPU exceeds threshold, CloudMonitor triggers a Function Compute
# that creates additional ECI instances.

# Function Compute Service for scaling actions
resource "alicloud_fc_service" "scaling" {
  count = var.enable_monitoring ? 1 : 0

  name        = "${local.name_prefix}-scaling-service"
  description = "Auto-scaling functions for ECI containers"

  role = alicloud_ram_role.scaling_function[0].arn

  log_config {
    project  = "${local.name_prefix}-logs"
    logstore = "scaling-logs"
  }
}

# RAM Role for Function Compute to manage ECI
resource "alicloud_ram_role" "scaling_function" {
  count = var.enable_monitoring ? 1 : 0

  name = "${local.name_prefix}-fc-scaling-role"
  document = jsonencode({
    Version = "1"
    Statement = [
      {
        Effect = "Allow"
        Principal = {
          Service = ["fc.aliyuncs.com"]
        }
        Action = "sts:AssumeRole"
      }
    ]
  })
  description = "Role for auto-scaling Function Compute"
}

# Policy to allow Function Compute to manage ECI
resource "alicloud_ram_policy" "scaling_eci" {
  count = var.enable_monitoring ? 1 : 0

  policy_name = "${local.name_prefix}-fc-eci-access"
  policy_document = jsonencode({
    Version = "1"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "eci:CreateContainerGroup",
          "eci:DeleteContainerGroup",
          "eci:DescribeContainerGroups",
          "eci:UpdateContainerGroup"
        ]
        Resource = "*"
      },
      {
        Effect = "Allow"
        Action = [
          "vpc:DescribeVSwitches",
          "ecs:DescribeSecurityGroups"
        ]
        Resource = "*"
      }
    ]
  })
  description = "Allow Function Compute to manage ECI for scaling"
}

resource "alicloud_ram_role_policy_attachment" "scaling_eci" {
  count = var.enable_monitoring ? 1 : 0

  role_name   = alicloud_ram_role.scaling_function[0].name
  policy_name = alicloud_ram_policy.scaling_eci[0].policy_name
  policy_type = "Custom"
}

# =============================================================================
# Scaling Metrics Configuration
# =============================================================================

locals {
  scaling_config = {
    go_backend = {
      name         = "go-backend"
      min_replicas = var.go_backend_min_replicas
      max_replicas = var.go_backend_max_replicas
      cpu          = var.go_backend_cpu
      memory       = var.go_backend_memory
      image        = "fzco-acr-registry-vpc.${var.region}.cr.aliyuncs.com/${alicloud_cr_ee_namespace.main.name}/go-backend:${var.image_tag}"
      port         = 8080
    }
    python_ai = {
      name         = "python-ai"
      min_replicas = var.python_ai_min_replicas
      max_replicas = var.python_ai_max_replicas
      cpu          = var.python_ai_cpu
      memory       = var.python_ai_memory
      image        = "fzco-acr-registry-vpc.${var.region}.cr.aliyuncs.com/${alicloud_cr_ee_namespace.main.name}/python-ai:${local.python_ai_effective_tag}"
      port         = 8000
    }
  }
}

# =============================================================================
# Outputs
# =============================================================================

output "scaling_config" {
  description = "Auto-scaling configuration"
  value = {
    go_backend = {
      min_replicas  = var.go_backend_min_replicas
      max_replicas  = var.go_backend_max_replicas
      cpu_threshold = var.scaling_cpu_threshold
      cooldown      = var.scaling_cooldown_seconds
    }
    python_ai = {
      min_replicas  = var.python_ai_min_replicas
      max_replicas  = var.python_ai_max_replicas
      cpu_threshold = var.scaling_cpu_threshold
      cooldown      = var.scaling_cooldown_seconds
    }
  }
}

# =============================================================================
# NOTE: Production Recommendation
# =============================================================================
# For production workloads, consider migrating to Alibaba Serverless
# Kubernetes (ASK) which provides:
#   - Native HPA (Horizontal Pod Autoscaler)
#   - KEDA for event-driven scaling
#   - Better integration with ALB
#   - Built-in service mesh support
#
# Migration path:
#   1. Create ASK cluster: alicloud_cs_serverless_kubernetes
#   2. Deploy containers as Kubernetes Deployments
#   3. Configure HPA with CPU/memory metrics
#   4. Use Knative for scale-to-zero capability
# =============================================================================
