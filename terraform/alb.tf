# =============================================================================
# Accountant CRM - Application Load Balancer
# =============================================================================

# =============================================================================
# ALB Instance
# =============================================================================

resource "alicloud_alb_load_balancer" "main" {
  load_balancer_name     = "${local.name_prefix}-alb"
  load_balancer_edition  = "Standard"
  vpc_id                 = alicloud_vpc.main.id
  address_type           = "Internet"
  address_allocated_mode = "Fixed"

  # Billing configuration (required)
  load_balancer_billing_config {
    pay_type = "PayAsYouGo"
  }

  # Zone mappings for high availability
  dynamic "zone_mappings" {
    for_each = alicloud_vswitch.public
    content {
      vswitch_id = zone_mappings.value.id
      zone_id    = zone_mappings.value.zone_id
    }
  }

  # Access log configuration
  access_log_config {
    log_project = alicloud_log_project.main.name
    log_store   = alicloud_log_store.alb_access.name
  }

  tags = merge(local.common_tags, {
    Name = "${local.name_prefix}-alb"
  })
}

# =============================================================================
# Server Groups
# =============================================================================

# Go Backend Server Group
# Fix #7: Added servers block to attach ECI container to server group
resource "alicloud_alb_server_group" "go_backend" {
  server_group_name = "${local.name_prefix}-go-backend"
  vpc_id            = alicloud_vpc.main.id
  protocol          = "HTTP"
  server_group_type = "Eci"

  health_check_config {
    health_check_enabled      = true
    health_check_connect_port = 8080
    health_check_host         = "$SERVER_IP"
    health_check_path         = "/health"
    health_check_protocol     = "HTTP"
    health_check_method       = "GET"
    health_check_interval     = 10
    health_check_timeout      = 5
    healthy_threshold         = 2
    unhealthy_threshold       = 3
    health_check_codes        = ["http_2xx"]
  }

  sticky_session_config {
    sticky_session_enabled = false
  }

  # Fix #7: Attach ECI container to server group
  servers {
    server_type = "Eci"
    server_id   = alicloud_eci_container_group.go_backend.id
    server_ip   = alicloud_eci_container_group.go_backend.intranet_ip
    port        = 8080
    weight      = 100
    description = "Go backend ECI container"
  }

  tags = merge(local.common_tags, {
    Name    = "${local.name_prefix}-go-backend"
    Service = "go-backend"
  })
}

# NOTE: Python AI service is NOT exposed via ALB.
# Go backend calls Python via CloudMap service discovery (python-ai.fzco.local:8000)
# with mTLS for secure internal communication.

# =============================================================================
# Listeners
# =============================================================================

# HTTP Listener (only used when HTTPS is not configured)
# When ssl_certificate_id is set, http_redirect listener handles port 80 instead
resource "alicloud_alb_listener" "http" {
  count = var.ssl_certificate_id == "" ? 1 : 0

  load_balancer_id  = alicloud_alb_load_balancer.main.id
  listener_port     = 80
  listener_protocol = "HTTP"

  default_actions {
    type = "ForwardGroup"
    forward_group_config {
      server_group_tuples {
        server_group_id = alicloud_alb_server_group.go_backend.id
      }
    }
  }
}

# HTTPS Listener (Fix #8) - Enabled when ssl_certificate_id is provided
# Set ssl_certificate_id in terraform.tfvars to enable HTTPS
resource "alicloud_alb_listener" "https" {
  count = var.ssl_certificate_id != "" ? 1 : 0

  load_balancer_id  = alicloud_alb_load_balancer.main.id
  listener_port     = 443
  listener_protocol = "HTTPS"
  idle_timeout      = var.alb_idle_timeout
  request_timeout   = 60
  gzip_enabled      = true

  # SSL configuration
  certificates {
    certificate_id = var.ssl_certificate_id
  }

  default_actions {
    type = "ForwardGroup"
    forward_group_config {
      server_group_tuples {
        server_group_id = alicloud_alb_server_group.go_backend.id
      }
    }
  }

  # XFF header configuration
  xforwarded_for_config {
    xforwardedforclientsrcportenabled = true
    xforwardedforenabled              = true
    xforwardedforprotoenabled         = true
    xforwardedforslbidenabled         = false
    xforwardedforslbportenabled       = false
  }
}

# HTTP Listener (when HTTPS is enabled, forwards to backend - app handles redirect)
# Note: ALB redirect requires alicloud_alb_rule with redirect action
resource "alicloud_alb_listener" "http_with_https" {
  count = var.ssl_certificate_id != "" ? 1 : 0

  load_balancer_id  = alicloud_alb_load_balancer.main.id
  listener_port     = 80
  listener_protocol = "HTTP"

  default_actions {
    type = "ForwardGroup"
    forward_group_config {
      server_group_tuples {
        server_group_id = alicloud_alb_server_group.go_backend.id
      }
    }
  }
}

# HTTP to HTTPS redirect rule (when HTTPS is enabled)
resource "alicloud_alb_rule" "http_to_https_redirect" {
  count = var.ssl_certificate_id != "" ? 1 : 0

  rule_name   = "${local.name_prefix}-http-redirect"
  listener_id = alicloud_alb_listener.http_with_https[0].id
  priority    = 1

  rule_conditions {
    type = "Path"
    path_config {
      values = ["/*"]
    }
  }

  rule_actions {
    type  = "Redirect"
    order = 1
    redirect_config {
      port      = "443"
      protocol  = "HTTPS"
      http_code = "301"
    }
  }
}

# =============================================================================
# Listener Rules
# =============================================================================

# Rule for API routes -> Go Backend
resource "alicloud_alb_rule" "api" {
  count = var.ssl_certificate_id == "" ? 1 : 0

  rule_name   = "${local.name_prefix}-api-rule"
  listener_id = alicloud_alb_listener.http[0].id
  priority    = 10

  rule_conditions {
    type = "Path"
    path_config {
      values = ["/api/*", "/health", "/ready"]
    }
  }

  rule_actions {
    type  = "ForwardGroup"
    order = 1
    forward_group_config {
      server_group_tuples {
        server_group_id = alicloud_alb_server_group.go_backend.id
      }
    }
  }
}

# NOTE: No public AI routes - Python AI is accessed only via internal CloudMap DNS

# =============================================================================
# SSL Certificate
# =============================================================================

# Option 1: Use existing Alibaba Cloud SSL certificate (recommended for production)
# Set var.ssl_certificate_id to use a pre-uploaded certificate

# Option 2: Upload certificate files (for development/testing)
# Place server.crt and server.key in terraform/certs/ directory
# Then set var.ssl_certificate_id = "" to use uploaded cert

# NOTE: Certificate upload disabled - use Alibaba Cloud SSL Certificate Service
# Set ssl_certificate_id in terraform.tfvars to use a pre-uploaded certificate
# resource "alicloud_slb_server_certificate" "main" {
#   count = var.ssl_certificate_id == "" ? 1 : 0
#
#   name               = "${local.name_prefix}-cert"
#   server_certificate = file("${path.module}/certs/server.crt")
#   private_key        = file("${path.module}/certs/server.key")
# }

locals {
  # Use provided certificate ID - HTTPS listener requires a valid cert
  # Upload certificate via Alibaba Cloud SSL Certificate Service and set ssl_certificate_id
  ssl_certificate_id = var.ssl_certificate_id
}

# =============================================================================
# Log Service for ALB
# =============================================================================

resource "alicloud_log_project" "main" {
  name        = "${local.name_prefix}-logs"
  description = "Log project for ${var.project_name}"
}

resource "alicloud_log_store" "alb_access" {
  project               = alicloud_log_project.main.name
  name                  = "alb-access-logs"
  retention_period      = 30
  shard_count           = 2
  auto_split            = true
  max_split_shard_count = 8
}

resource "alicloud_log_store" "scaling" {
  project               = alicloud_log_project.main.name
  name                  = "scaling-logs"
  retention_period      = 30
  shard_count           = 1
  auto_split            = true
  max_split_shard_count = 4
}

# =============================================================================
# Outputs
# =============================================================================

output "alb_details" {
  description = "ALB configuration details"
  value = {
    id                = alicloud_alb_load_balancer.main.id
    dns_name          = alicloud_alb_load_balancer.main.dns_name
    listener_http_id  = var.ssl_certificate_id == "" ? alicloud_alb_listener.http[0].id : null
    listener_https_id = var.ssl_certificate_id != "" ? alicloud_alb_listener.https[0].id : null
    https_enabled     = var.ssl_certificate_id != ""
    go_backend_sg_id  = alicloud_alb_server_group.go_backend.id
  }
}
