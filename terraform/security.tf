# =============================================================================
# Accountant CRM - Security Groups and Network ACLs
# =============================================================================

# =============================================================================
# ALB Security Group
# =============================================================================

resource "alicloud_security_group" "alb" {
  name        = "${local.name_prefix}-alb-sg"
  vpc_id      = alicloud_vpc.main.id
  description = "Security group for Application Load Balancer"

  tags = merge(local.common_tags, {
    Name = "${local.name_prefix}-alb-sg"
  })
}

# Allow HTTP from anywhere (redirect to HTTPS)
resource "alicloud_security_group_rule" "alb_http" {
  security_group_id = alicloud_security_group.alb.id
  type              = "ingress"
  ip_protocol       = "tcp"
  port_range        = "80/80"
  cidr_ip           = "0.0.0.0/0"
  description       = "Allow HTTP from anywhere"
}

# Allow HTTPS from anywhere
resource "alicloud_security_group_rule" "alb_https" {
  security_group_id = alicloud_security_group.alb.id
  type              = "ingress"
  ip_protocol       = "tcp"
  port_range        = "443/443"
  cidr_ip           = "0.0.0.0/0"
  description       = "Allow HTTPS from anywhere"
}

# Allow all outbound
resource "alicloud_security_group_rule" "alb_egress" {
  security_group_id = alicloud_security_group.alb.id
  type              = "egress"
  ip_protocol       = "all"
  port_range        = "-1/-1"
  cidr_ip           = "0.0.0.0/0"
  description       = "Allow all outbound traffic"
}

# =============================================================================
# ECS Security Group
# =============================================================================

resource "alicloud_security_group" "ecs" {
  name        = "${local.name_prefix}-ecs-sg"
  vpc_id      = alicloud_vpc.main.id
  description = "Security group for ECS instances"

  tags = merge(local.common_tags, {
    Name = "${local.name_prefix}-ecs-sg"
  })
}

# Allow Go backend port from ALB
resource "alicloud_security_group_rule" "ecs_go_backend" {
  security_group_id        = alicloud_security_group.ecs.id
  type                     = "ingress"
  ip_protocol              = "tcp"
  port_range               = "8080/8080"
  source_security_group_id = alicloud_security_group.alb.id
  description              = "Allow Go backend traffic from ALB"
}

# Allow Python AI port from ALB
resource "alicloud_security_group_rule" "ecs_python_ai" {
  security_group_id        = alicloud_security_group.ecs.id
  type                     = "ingress"
  ip_protocol              = "tcp"
  port_range               = "8000/8000"
  source_security_group_id = alicloud_security_group.alb.id
  description              = "Allow Python AI traffic from ALB"
}

# =============================================================================
# Specific Internal Service Rules (replacing overly permissive 1/65535)
# =============================================================================

# Go Backend (8080) can receive from within VPC (ALB, health checks)
resource "alicloud_security_group_rule" "ecs_go_internal" {
  security_group_id = alicloud_security_group.ecs.id
  type              = "ingress"
  ip_protocol       = "tcp"
  port_range        = "8080/8080"
  cidr_ip           = var.vpc_cidr
  description       = "Allow Go backend port from VPC"
}

# Python AI (8000) can receive from within VPC (Go backend via mTLS)
resource "alicloud_security_group_rule" "ecs_python_internal" {
  security_group_id = alicloud_security_group.ecs.id
  type              = "ingress"
  ip_protocol       = "tcp"
  port_range        = "8000/8000"
  cidr_ip           = var.vpc_cidr
  description       = "Allow Python AI port from VPC"
}

# Redis (6379) - only from within VPC
resource "alicloud_security_group_rule" "ecs_redis_internal" {
  security_group_id = alicloud_security_group.ecs.id
  type              = "ingress"
  ip_protocol       = "tcp"
  port_range        = "6379/6379"
  cidr_ip           = var.vpc_cidr
  description       = "Allow Redis port from VPC"
}

# DNS (53) for internal resolution
resource "alicloud_security_group_rule" "ecs_dns_internal" {
  security_group_id = alicloud_security_group.ecs.id
  type              = "ingress"
  ip_protocol       = "udp"
  port_range        = "53/53"
  cidr_ip           = var.vpc_cidr
  description       = "Allow DNS from VPC"
}

# Allow SSH from bastion (if needed)
resource "alicloud_security_group_rule" "ecs_ssh" {
  count = var.environment == "staging" ? 1 : 0

  security_group_id = alicloud_security_group.ecs.id
  type              = "ingress"
  ip_protocol       = "tcp"
  port_range        = "22/22"
  cidr_ip           = var.vpc_cidr
  description       = "Allow SSH from VPC (staging only)"
}

# Allow all outbound
resource "alicloud_security_group_rule" "ecs_egress" {
  security_group_id = alicloud_security_group.ecs.id
  type              = "egress"
  ip_protocol       = "all"
  port_range        = "-1/-1"
  cidr_ip           = "0.0.0.0/0"
  description       = "Allow all outbound traffic"
}

# =============================================================================
# Redis Security Group - DISABLED
# =============================================================================
# Redis now runs as Docker container on ECS, not managed ApsaraDB.
# The ecs_redis_internal rule above allows Redis traffic within VPC.
# This section is kept for reference if managed Redis is re-enabled later.

# resource "alicloud_security_group" "redis" {
#   name        = "${local.name_prefix}-redis-sg"
#   vpc_id      = alicloud_vpc.main.id
#   description = "Security group for Redis"
#
#   tags = merge(local.common_tags, {
#     Name = "${local.name_prefix}-redis-sg"
#   })
# }

# =============================================================================
# Network ACL for Additional Layer of Security
# =============================================================================
# Fix: Migrated from deprecated alicloud_network_acl_entries to inline rules
# Fix: Added 8080/8000 ingress for ALB health checks and internal communication

resource "alicloud_network_acl" "main" {
  vpc_id           = alicloud_vpc.main.id
  network_acl_name = "${local.name_prefix}-nacl"

  # Ingress rules (inbound traffic)
  # Rule 1: HTTP
  ingress_acl_entries {
    protocol       = "tcp"
    port           = "80/80"
    source_cidr_ip = "0.0.0.0/0"
    entry_type     = "custom"
    description    = "Allow HTTP"
    policy         = "accept"
  }

  # Rule 2: HTTPS
  ingress_acl_entries {
    protocol       = "tcp"
    port           = "443/443"
    source_cidr_ip = "0.0.0.0/0"
    entry_type     = "custom"
    description    = "Allow HTTPS"
    policy         = "accept"
  }

  # Rule 3: Go Backend port - ALB health checks and traffic
  ingress_acl_entries {
    protocol       = "tcp"
    port           = "8080/8080"
    source_cidr_ip = "0.0.0.0/0"
    entry_type     = "custom"
    description    = "Allow Go backend port for ALB health checks"
    policy         = "accept"
  }

  # Rule 4: Python AI port - Go backend calls
  ingress_acl_entries {
    protocol       = "tcp"
    port           = "8000/8000"
    source_cidr_ip = var.vpc_cidr
    entry_type     = "custom"
    description    = "Allow Python AI port from VPC"
    policy         = "accept"
  }

  # Rule 5: Ephemeral ports for return traffic (NACL is stateless)
  ingress_acl_entries {
    protocol       = "tcp"
    port           = "1024/65535"
    source_cidr_ip = "0.0.0.0/0"
    entry_type     = "custom"
    description    = "Allow ephemeral ports for return traffic"
    policy         = "accept"
  }

  # Rule 6: DNS responses (UDP)
  ingress_acl_entries {
    protocol       = "udp"
    port           = "53/53"
    source_cidr_ip = "0.0.0.0/0"
    entry_type     = "custom"
    description    = "Allow DNS responses"
    policy         = "accept"
  }

  # Egress rules (outbound traffic) - Allow all
  egress_acl_entries {
    protocol            = "all"
    port                = "-1/-1"
    destination_cidr_ip = "0.0.0.0/0"
    entry_type          = "custom"
    description         = "Allow all outbound"
    policy              = "accept"
  }

  tags = merge(local.common_tags, {
    Name = "${local.name_prefix}-nacl"
  })
}

# =============================================================================
# NACL Attachments - Managed outside Terraform
# =============================================================================
# Note: NACL attachments exist in the cloud and are working.
# The alicloud_network_acl_attachment resource doesn't support import,
# so these are managed outside Terraform. All VSwitches are attached to the NACL.
#
# If you need to modify attachments, use the Alibaba Cloud CLI:
#   aliyun vpc AssociateNetworkAcl --NetworkAclId nacl-xxx --ResourceId vsw-xxx
#   aliyun vpc UnassociateNetworkAcl --NetworkAclId nacl-xxx --ResourceId vsw-xxx

# =============================================================================
# WAF (Web Application Firewall) - Optional
# =============================================================================

# Note: WAF is recommended for production deployments
# Uncomment and configure as needed

# resource "alicloud_waf_instance" "main" {
#   count = var.environment == "production" ? 1 : 0
#
#   big_screen          = "0"
#   exclusive_ip_package = "0"
#   ext_bandwidth        = "0"
#   ext_domain_package   = "0"
#   log_storage          = "3"
#   log_time             = "180"
#   modify_protection_rules = "0"
#   package_code         = "version_3"
#   prefessional_service = "false"
#   subscription_type    = "Subscription"
#   period               = 1
#   region               = var.region
# }

# =============================================================================
# Outputs
# =============================================================================

output "security_groups" {
  description = "Security group IDs"
  value = {
    alb = alicloud_security_group.alb.id
    ecs = alicloud_security_group.ecs.id
  }
}

output "network_acl" {
  description = "Network ACL ID"
  value = {
    id = alicloud_network_acl.main.id
  }
}
