# =============================================================================
# Accountant CRM - Terraform Outputs
# =============================================================================
# Updated: 2026-08-29
# Architecture: Single ECS + Docker Compose (~$17/month)
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
# Summary Output
# =============================================================================

output "summary" {
  description = "Deployment summary"
  value       = <<-EOT

    ============================================
    Accountant CRM - ${var.environment} Deployment
    ============================================

    Architecture: Single ECS + Docker Compose
    Region: ${var.region}
    VPC ID: ${alicloud_vpc.main.id}

    Live Endpoints (per cloud.md):
    - Frontend: https://crm.irislondonshoes.com
    - API: https://api.irislondonshoes.com

    Docker Services (on ECS at 8.211.195.17):
    - nginx:alpine (ports 80, 443)
    - accountant-go-backend:latest (port 8080)
    - accountant-python-ai:latest (port 8000)
    - redis:7-alpine (port 6379)

    External Services (FREE tier):
    - PostgreSQL: Neon
    - MongoDB: Atlas
    - DNS: Cloudflare
    - SSL: Let's Encrypt

    ============================================
  EOT
}
