# =============================================================================
# Accountant CRM - Terraform Outputs
# =============================================================================

# =============================================================================
# Network Outputs
# =============================================================================

output "network" {
  description = "Network configuration"
  value = {
    vpc_id              = alicloud_vpc.main.id
    public_vswitch_ids  = alicloud_vswitch.public[*].id
    private_vswitch_ids = alicloud_vswitch.private[*].id
    nat_gateway_ids     = alicloud_nat_gateway.main[*].id
    nat_eips            = alicloud_eip_address.nat[*].ip_address
  }
}

# =============================================================================
# OSS Outputs
# =============================================================================

output "oss" {
  description = "OSS bucket configuration"
  value = {
    frontend_bucket = alicloud_oss_bucket.frontend.bucket
    frontend_domain = alicloud_oss_bucket.frontend.extranet_endpoint
    uploads_bucket  = alicloud_oss_bucket.uploads.bucket
    uploads_domain  = alicloud_oss_bucket.uploads.extranet_endpoint
  }
}

# =============================================================================
# MNS Outputs - DISABLED (MNS service not activated)
# =============================================================================

# output "mns" {
#   description = "MNS topic configuration"
#   value = {
#     topics   = { for k, v in alicloud_mns_topic.topics : k => v.name }
#     dlq_name = alicloud_mns_queue.dlq.name
#   }
# }

# =============================================================================
# Redis Outputs
# =============================================================================

output "redis" {
  description = "Redis configuration"
  value = {
    instance_id       = local.redis_instance_id
    connection_string = local.redis_connection_domain
    port              = 6379
  }
  sensitive = true
}

# =============================================================================
# ACR Outputs - DISABLED (ACR requires console activation)
# =============================================================================

# output "acr" {
#   description = "Container Registry configuration"
#   value = {
#     instance_id     = alicloud_cr_ee_instance.main.id
#     namespace       = alicloud_cr_ee_namespace.main.name
#     go_backend_repo = alicloud_cr_ee_repo.go_backend.name
#     python_ai_repo  = alicloud_cr_ee_repo.python_ai.name
#   }
# }

# =============================================================================
# ALB Outputs
# =============================================================================

output "alb" {
  description = "Application Load Balancer configuration"
  value = {
    id       = alicloud_alb_load_balancer.main.id
    dns_name = alicloud_alb_load_balancer.main.dns_name
    edition  = alicloud_alb_load_balancer.main.load_balancer_edition
  }
}

# =============================================================================
# ECS Outputs
# =============================================================================

# NOTE: Kubernetes cluster not provisioned in Week 1
# output "ecs" {
#   description = "ECS cluster configuration"
#   value = {
#     cluster_id       = alicloud_cs_managed_kubernetes.main.id
#     cluster_name     = alicloud_cs_managed_kubernetes.main.name
#     security_group_id = alicloud_security_group.ecs.id
#   }
# }

# =============================================================================
# Summary Output
# =============================================================================

output "summary" {
  description = "Deployment summary"
  value       = <<-EOT

    ============================================
    Accountant CRM - ${var.environment} Deployment
    ============================================

    Region: ${var.region}
    VPC ID: ${alicloud_vpc.main.id}

    Endpoints:
    - ALB DNS: ${alicloud_alb_load_balancer.main.dns_name}
    - Frontend: https://${var.oss_frontend_bucket}.${var.region}.aliyuncs.com
    - Redis: ${local.redis_connection_domain}:6379

    Pending Activation (Console Required):
    - ACR (Container Registry)
    - MNS (Message Service)
    - PVTZ (Private Zone / CloudMap)

    ============================================
  EOT
}
