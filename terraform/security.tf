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
# Redis Security Group
# =============================================================================

resource "alicloud_security_group" "redis" {
  name        = "${local.name_prefix}-redis-sg"
  vpc_id      = alicloud_vpc.main.id
  description = "Security group for Redis"

  tags = merge(local.common_tags, {
    Name = "${local.name_prefix}-redis-sg"
  })
}

# Allow Redis port from Go Backend ECI containers
resource "alicloud_security_group_rule" "redis_from_go_backend" {
  security_group_id        = alicloud_security_group.redis.id
  type                     = "ingress"
  ip_protocol              = "tcp"
  port_range               = "6379/6379"
  source_security_group_id = alicloud_security_group.go_backend.id
  description              = "Allow Redis traffic from Go backend"
}

# Allow Redis port from Python AI ECI containers
resource "alicloud_security_group_rule" "redis_from_python_ai" {
  security_group_id        = alicloud_security_group.redis.id
  type                     = "ingress"
  ip_protocol              = "tcp"
  port_range               = "6379/6379"
  source_security_group_id = alicloud_security_group.python_ai.id
  description              = "Allow Redis traffic from Python AI"
}

# =============================================================================
# Network ACL for Additional Layer of Security
# =============================================================================

resource "alicloud_network_acl" "main" {
  vpc_id           = alicloud_vpc.main.id
  network_acl_name = "${local.name_prefix}-nacl"

  tags = merge(local.common_tags, {
    Name = "${local.name_prefix}-nacl"
  })
}

# Allow HTTP/HTTPS inbound
resource "alicloud_network_acl_entries" "inbound" {
  network_acl_id = alicloud_network_acl.main.id

  ingress {
    protocol       = "tcp"
    port           = "80/80"
    source_cidr_ip = "0.0.0.0/0"
    entry_type     = "custom"
    name           = "allow-http"
    policy         = "accept"
  }

  ingress {
    protocol       = "tcp"
    port           = "443/443"
    source_cidr_ip = "0.0.0.0/0"
    entry_type     = "custom"
    name           = "allow-https"
    policy         = "accept"
  }

  ingress {
    protocol       = "tcp"
    port           = "1024/65535"
    source_cidr_ip = "0.0.0.0/0"
    entry_type     = "custom"
    name           = "allow-ephemeral"
    policy         = "accept"
  }
}

# Allow all outbound
resource "alicloud_network_acl_entries" "outbound" {
  network_acl_id = alicloud_network_acl.main.id

  egress {
    protocol            = "all"
    port                = "-1/-1"
    destination_cidr_ip = "0.0.0.0/0"
    entry_type          = "custom"
    name                = "allow-all-outbound"
    policy              = "accept"
  }
}

# Attach NACL to public subnets
resource "alicloud_network_acl_attachment" "public" {
  count = length(alicloud_vswitch.public)

  network_acl_id = alicloud_network_acl.main.id
  resources {
    resource_id   = alicloud_vswitch.public[count.index].id
    resource_type = "VSwitch"
  }
}

# Fix #21: Attach NACL to private subnets as well
resource "alicloud_network_acl_attachment" "private" {
  count = length(alicloud_vswitch.private)

  network_acl_id = alicloud_network_acl.main.id
  resources {
    resource_id   = alicloud_vswitch.private[count.index].id
    resource_type = "VSwitch"
  }
}

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
    alb   = alicloud_security_group.alb.id
    ecs   = alicloud_security_group.ecs.id
    redis = alicloud_security_group.redis.id
  }
}

output "network_acl" {
  description = "Network ACL ID"
  value = {
    id = alicloud_network_acl.main.id
  }
}
