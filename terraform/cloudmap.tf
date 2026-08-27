# =============================================================================
# Accountant CRM - Service Discovery (Private Zone DNS)
# =============================================================================

# =============================================================================
# Private Zone for Service Discovery
# Namespace: fzco.local (per workflow spec)
# =============================================================================

resource "alicloud_pvtz_zone" "main" {
  zone_name = "fzco.local"

  remark = "Private DNS zone for ${var.project_name} service discovery"
}

# Associate zone with VPC
resource "alicloud_pvtz_zone_attachment" "main" {
  zone_id = alicloud_pvtz_zone.main.id
  vpc_ids = [alicloud_vpc.main.id]
}

# =============================================================================
# Service DNS Records
# =============================================================================

# Go Backend Service (go-backend.fzco.local)
# Fix #18: Use ECI intranet_ip from Terraform instead of hardcoded IP
resource "alicloud_pvtz_zone_record" "go_backend" {
  zone_id = alicloud_pvtz_zone.main.id
  type    = "A"
  rr      = "go-backend"
  value   = alicloud_eci_container_group.go_backend.intranet_ip
  ttl     = 60
  status  = "ENABLE"
}

# Python AI Service (python-ai.fzco.local:8000)
# Go backend calls this via mTLS for internal AI operations
# Fix #18: Use ECI intranet_ip from Terraform instead of hardcoded IP
resource "alicloud_pvtz_zone_record" "python_ai" {
  zone_id = alicloud_pvtz_zone.main.id
  type    = "A"
  rr      = "python-ai"
  value   = alicloud_eci_container_group.python_ai.intranet_ip
  ttl     = 60
  status  = "ENABLE"
}

# Redis (redis.fzco.local)
resource "alicloud_pvtz_zone_record" "redis" {
  zone_id = alicloud_pvtz_zone.main.id
  type    = "CNAME"
  rr      = "redis"
  value   = local.redis_connection_domain
  ttl     = 300
  status  = "ENABLE"
}

# =============================================================================
# Service Discovery for Kubernetes
# =============================================================================

# Note: When using managed Kubernetes (ACK), CoreDNS handles internal service
# discovery automatically. The private zone above is for cross-service
# communication outside of Kubernetes or for hybrid deployments.

# Kubernetes services will be available at:
# - go-backend.fzco.svc.cluster.local
# - python-ai.fzco.svc.cluster.local
#
# The fzco.local private zone provides an alternative DNS path for:
# - go-backend.fzco.local:8080
# - python-ai.fzco.local:8000 (mTLS required)

# =============================================================================
# External DNS Records (if managing DNS in Alibaba)
# =============================================================================

# Note: Uncomment and configure if using Alibaba Cloud DNS for public domains

# resource "alicloud_dns_record" "api" {
#   name        = var.domain_name
#   host_record = var.api_subdomain
#   type        = "CNAME"
#   value       = alicloud_alb_load_balancer.main.dns_name
#   ttl         = 600
# }

# resource "alicloud_dns_record" "app" {
#   name        = var.domain_name
#   host_record = var.app_subdomain
#   type        = "CNAME"
#   value       = "${var.oss_frontend_bucket}.${var.region}.aliyuncs.com"
#   ttl         = 600
# }

# =============================================================================
# Outputs
# =============================================================================

output "service_discovery" {
  description = "Service discovery configuration"
  value = {
    zone_id   = alicloud_pvtz_zone.main.id
    zone_name = alicloud_pvtz_zone.main.zone_name

    internal_endpoints = {
      go_backend = "go-backend.fzco.local:8080"
      python_ai  = "python-ai.fzco.local:8000"
      redis      = "redis.fzco.local"
    }
  }
}
