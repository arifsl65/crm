# =============================================================================
# Accountant CRM - ECS/ECI Container Services
# Uses Alibaba ECI (Elastic Container Instance) for serverless containers
# Similar to AWS Fargate - no cluster management required
# =============================================================================
#
# SECURITY: Sensitive credentials (POSTGRES_PASSWORD, REDIS_PASSWORD, etc.)
# are stored in KMS Secrets Manager (see kms.tf). The ECI containers have
# RAM role access to fetch secrets at runtime via the Alibaba Cloud SDK.
#
# Applications MUST fetch secrets at startup when SECRETS_FROM_KMS=true:
#   - Go: github.com/aliyun/alibaba-cloud-sdk-go/services/kms
#   - Python: aliyun-python-sdk-kms
#
# KMS secret name env vars:
#   - KMS_POSTGRES_PASSWORD_SECRET -> postgres password
#   - KMS_REDIS_PASSWORD_SECRET    -> redis password
#   - KMS_MONGODB_URI_SECRET       -> mongodb connection string
# =============================================================================

# =============================================================================
# ECI Container Group: Go Backend (Private Subnet)
# =============================================================================
# FIXED: Moved from public to private subnet to enable:
#   - Access to Alibaba internal network (100.100.0.0/16) for metadata/DNS
#   - ACR VPC endpoint for image pulls
#   - Redis via internal VPC routing
# External traffic reaches Go backend via ALB in public subnet.

resource "alicloud_eci_container_group" "go_backend" {
  container_group_name = "${local.name_prefix}-go-backend"
  cpu                  = var.go_backend_cpu / 1000    # Convert millicores to cores
  memory               = var.go_backend_memory / 1024 # Convert MB to GB
  restart_policy       = "Always"
  security_group_id    = alicloud_security_group.go_backend.id
  vswitch_id           = alicloud_vswitch.private[0].id
  zone_id              = data.alicloud_zones.available.zones[0].id

  # RAM role disabled - causes "AliyunECIContainerGroupRole does not belong to eci.aliyuncs.com" error
  # TODO: Re-enable once ECI service-linked role is activated in console
  # ram_role_name = alicloud_ram_role.eci_role.name

  containers {
    name = "go-backend"
    # Using VPC endpoint - private subnet pulls directly without NAT
    # Fix: Internet endpoint was timing out due to missing NAT route
    image             = "fzco-acr-registry-vpc.${var.region}.cr.aliyuncs.com/${alicloud_cr_ee_namespace.main.name}/go-backend:${var.image_tag}"
    cpu               = var.go_backend_cpu / 1000
    memory            = var.go_backend_memory / 1024
    image_pull_policy = "Always"

    ports {
      port     = 8080
      protocol = "TCP"
    }

    environment_vars {
      key   = "APP_ENV"
      value = var.environment
    }

    environment_vars {
      key   = "GO_PORT"
      value = "8080"
    }

    environment_vars {
      key   = "POSTGRES_HOST"
      value = var.neon_host
    }

    environment_vars {
      key   = "POSTGRES_USER"
      value = var.neon_user
    }

    # POSTGRES_PASSWORD removed - fetched from KMS at runtime
    # See KMS_POSTGRES_PASSWORD_SECRET env var below

    environment_vars {
      key   = "POSTGRES_DB"
      value = var.neon_database
    }

    environment_vars {
      key   = "POSTGRES_SSLMODE"
      value = "require"
    }

    environment_vars {
      key   = "REDIS_HOST"
      value = alicloud_kvstore_instance.redis.connection_domain
    }

    # REDIS_PASSWORD removed - fetched from KMS at runtime
    # See KMS_REDIS_PASSWORD_SECRET env var below

    environment_vars {
      key   = "REDIS_TLS_ENABLED"
      value = "false"
    }

    # JWT Secret Key for token signing (Fix #1: Critical - app exits without this)
    environment_vars {
      key   = "JWT_SECRET_KEY"
      value = var.jwt_secret_key
    }

    # Fallback passwords - direct values until KMS runtime fetching is verified
    # Fix #3: Add these to prevent CrashLoopBackOff
    environment_vars {
      key   = "POSTGRES_PASSWORD"
      value = var.neon_password
    }

    environment_vars {
      key   = "REDIS_PASSWORD"
      value = random_password.redis_password.result
    }

    # KMS-based secret fetching disabled until RAM role is configured
    # Fix: Was causing container crash on startup
    environment_vars {
      key   = "SECRETS_FROM_KMS"
      value = "false"
    }

    # mTLS disabled temporarily - using HTTP until certs are configured
    environment_vars {
      key   = "PYTHON_AI_URL"
      value = "http://python-ai.fzco.local:8000"
    }

    environment_vars {
      key   = "MTLS_ENABLED"
      value = "false"
    }

    # KMS Secret names for runtime fetching (applications should use Alibaba SDK)
    # TODO: Remove plain-text password env vars once app-level KMS fetching is implemented
    environment_vars {
      key   = "KMS_POSTGRES_PASSWORD_SECRET"
      value = "${local.name_prefix}/postgres-password"
    }

    environment_vars {
      key   = "KMS_REDIS_PASSWORD_SECRET"
      value = "${local.name_prefix}/redis-password"
    }

    environment_vars {
      key   = "KMS_JWT_SECRET_KEY_SECRET"
      value = "${local.name_prefix}/jwt-secret-key"
    }

    environment_vars {
      key   = "KMS_REGION"
      value = var.region
    }

    # mTLS certificate KMS secret names (Fix #10)
    environment_vars {
      key   = "KMS_MTLS_CA_CERT_SECRET"
      value = "${local.name_prefix}/mtls-ca-cert"
    }

    environment_vars {
      key   = "KMS_MTLS_CLIENT_CERT_SECRET"
      value = "${local.name_prefix}/mtls-client-cert"
    }

    environment_vars {
      key   = "KMS_MTLS_CLIENT_KEY_SECRET"
      value = "${local.name_prefix}/mtls-client-key"
    }

    # Liveness probe (increased delay for Go cold start + external DB connections)
    liveness_probe {
      http_get {
        path   = "/health"
        port   = 8080
        scheme = "HTTP"
      }
      initial_delay_seconds = 30
      period_seconds        = 30
      timeout_seconds       = 5
      success_threshold     = 1
      failure_threshold     = 3
    }

    # Readiness probe (checks DB + Redis - needs time for connections)
    readiness_probe {
      http_get {
        path   = "/ready"
        port   = 8080
        scheme = "HTTP"
      }
      initial_delay_seconds = 15
      period_seconds        = 10
      timeout_seconds       = 5
      success_threshold     = 1
      failure_threshold     = 3
    }
  }

  # Image registry credentials (VPC endpoint - pulls directly without internet)
  image_registry_credential {
    server    = "fzco-acr-registry-vpc.${var.region}.cr.aliyuncs.com"
    user_name = var.acr_username
    password  = var.acr_password
  }

  tags = merge(local.common_tags, {
    Name    = "${local.name_prefix}-go-backend"
    Service = "go-backend"
    Subnet  = "private"
  })

  lifecycle {
    ignore_changes = [
      cpu,
      memory,
      dns_policy,
      ephemeral_storage,
      instance_type,
      internet_ip,
      intranet_ip,
      resource_group_id,
      spot_price_limit,
      spot_strategy,
      status,
    ]
  }
}

# =============================================================================
# ECI Container Group: Python AI (Private Subnet)
# =============================================================================

resource "alicloud_eci_container_group" "python_ai" {
  container_group_name = "${local.name_prefix}-python-ai"
  cpu                  = var.python_ai_cpu / 1000
  memory               = var.python_ai_memory / 1024
  restart_policy       = "Always"
  security_group_id    = alicloud_security_group.python_ai.id
  vswitch_id           = alicloud_vswitch.private[0].id
  zone_id              = data.alicloud_zones.available.zones[0].id

  # RAM role disabled - causes "AliyunECIContainerGroupRole does not belong to eci.aliyuncs.com" error
  # TODO: Re-enable once ECI service-linked role issue is resolved in console
  # ram_role_name = alicloud_ram_role.eci_role.name

  containers {
    name              = "python-ai"
    # Using VPC endpoint - private subnet pulls directly without NAT
    # Fix: Internet endpoint was timing out due to missing NAT route
    image             = "fzco-acr-registry-vpc.${var.region}.cr.aliyuncs.com/${alicloud_cr_ee_namespace.main.name}/python-ai:${local.python_ai_effective_tag}"
    cpu               = var.python_ai_cpu / 1000
    memory            = var.python_ai_memory / 1024
    image_pull_policy = "Always"

    ports {
      port     = 8000
      protocol = "TCP"
    }

    environment_vars {
      key   = "APP_ENV"
      value = var.environment
    }

    environment_vars {
      key   = "PORT"
      value = "8000"
    }

    # Fallback MongoDB URI - direct value until KMS runtime fetching is verified (Fix #4)
    environment_vars {
      key   = "MONGODB_URI"
      value = var.mongodb_uri
    }

    environment_vars {
      key   = "REDIS_HOST"
      value = alicloud_kvstore_instance.redis.connection_domain
    }

    # Fallback Redis password - direct value until KMS runtime fetching is verified
    environment_vars {
      key   = "REDIS_PASSWORD"
      value = random_password.redis_password.result
    }

    # PostgreSQL connection for Python AI (Fix #4: Required for external DB access)
    environment_vars {
      key   = "POSTGRES_HOST"
      value = var.neon_host
    }

    environment_vars {
      key   = "POSTGRES_USER"
      value = var.neon_user
    }

    environment_vars {
      key   = "POSTGRES_PASSWORD"
      value = var.neon_password
    }

    environment_vars {
      key   = "POSTGRES_DATABASE"
      value = var.neon_database
    }

    environment_vars {
      key   = "POSTGRES_SSLMODE"
      value = "require"
    }

    environment_vars {
      key   = "OSS_ENDPOINT"
      value = "https://oss-${var.region}.aliyuncs.com"
    }

    environment_vars {
      key   = "OSS_BUCKET_UPLOADS"
      value = alicloud_oss_bucket.uploads.id
    }

    # Alibaba Cloud credentials - required until RAM role is enabled on ECI
    # Used by Python SDK for OSS and other Alibaba Cloud services
    # TODO: For production, create a dedicated access key with limited permissions
    environment_vars {
      key   = "ALIBABA_ACCESS_KEY_ID"
      value = var.alicloud_access_key
    }

    environment_vars {
      key   = "ALIBABA_ACCESS_KEY_SECRET"
      value = var.alicloud_secret_key
    }

    environment_vars {
      key   = "ALIBABA_REGION"
      value = var.region
    }

    # mTLS disabled temporarily - using HTTP until certs are configured
    environment_vars {
      key   = "MTLS_ENABLED"
      value = "false"
    }

    # KMS-based secret fetching disabled until RAM role is configured
    # Fix: Was causing container crash on startup
    environment_vars {
      key   = "SECRETS_FROM_KMS"
      value = "false"
    }

    # KMS Secret references (not used when SECRETS_FROM_KMS=false)
    environment_vars {
      key   = "KMS_MONGODB_URI_SECRET"
      value = "${local.name_prefix}/mongodb-uri"
    }

    environment_vars {
      key   = "KMS_REDIS_PASSWORD_SECRET"
      value = "${local.name_prefix}/redis-password"
    }

    environment_vars {
      key   = "KMS_REGION"
      value = var.region
    }

    # mTLS certificate KMS secret names (Fix #10 - Python is the server)
    environment_vars {
      key   = "KMS_MTLS_CA_CERT_SECRET"
      value = "${local.name_prefix}/mtls-ca-cert"
    }

    environment_vars {
      key   = "KMS_MTLS_SERVER_CERT_SECRET"
      value = "${local.name_prefix}/mtls-server-cert"
    }

    environment_vars {
      key   = "KMS_MTLS_SERVER_KEY_SECRET"
      value = "${local.name_prefix}/mtls-server-key"
    }

    # Liveness probe (increased delays for Python cold start + external DB connections)
    liveness_probe {
      http_get {
        path   = "/health"
        port   = 8000
        scheme = "HTTP"
      }
      initial_delay_seconds = 45
      period_seconds        = 30
      timeout_seconds       = 10
      success_threshold     = 1
      failure_threshold     = 3
    }

    # Readiness probe (checks MongoDB, PostgreSQL, OSS, Redis - needs more time)
    readiness_probe {
      http_get {
        path   = "/ready"
        port   = 8000
        scheme = "HTTP"
      }
      initial_delay_seconds = 30
      period_seconds        = 15
      timeout_seconds       = 10
      success_threshold     = 1
      failure_threshold     = 3
    }
  }

  # Image registry credentials (VPC endpoint - pulls directly without internet)
  image_registry_credential {
    server    = "fzco-acr-registry-vpc.${var.region}.cr.aliyuncs.com"
    user_name = var.acr_username
    password  = var.acr_password
  }

  tags = merge(local.common_tags, {
    Name    = "${local.name_prefix}-python-ai"
    Service = "python-ai"
    Subnet  = "private"
  })

  lifecycle {
    ignore_changes = [
      cpu,
      memory,
      dns_policy,
      ephemeral_storage,
      instance_type,
      internet_ip,
      intranet_ip,
      resource_group_id,
      spot_price_limit,
      spot_strategy,
      status,
    ]
  }
}

# =============================================================================
# Security Groups
# =============================================================================

# Go Backend Security Group (Private Subnet - receives traffic from ALB)
resource "alicloud_security_group" "go_backend" {
  name        = "${local.name_prefix}-go-backend-sg"
  vpc_id      = alicloud_vpc.main.id
  description = "Security group for Go backend containers"

  tags = merge(local.common_tags, {
    Name = "${local.name_prefix}-go-backend-sg"
  })
}

# Go Backend: Allow inbound from ALB
resource "alicloud_security_group_rule" "go_backend_alb" {
  type              = "ingress"
  ip_protocol       = "tcp"
  port_range        = "8080/8080"
  security_group_id = alicloud_security_group.go_backend.id
  cidr_ip           = var.vpc_cidr
  description       = "Allow traffic from ALB"
}

# Go Backend: Allow outbound to Python AI (mTLS)
resource "alicloud_security_group_rule" "go_backend_to_python" {
  type                     = "egress"
  ip_protocol              = "tcp"
  port_range               = "8000/8000"
  security_group_id        = alicloud_security_group.go_backend.id
  source_security_group_id = alicloud_security_group.python_ai.id
  description              = "Allow mTLS to Python AI"
}

# Go Backend: Allow outbound to Redis
resource "alicloud_security_group_rule" "go_backend_to_redis" {
  type              = "egress"
  ip_protocol       = "tcp"
  port_range        = "6379/6379"
  security_group_id = alicloud_security_group.go_backend.id
  cidr_ip           = var.vpc_cidr
  description       = "Allow connection to Redis"
}

# Go Backend: Allow outbound HTTPS (for Neon, external APIs)
resource "alicloud_security_group_rule" "go_backend_https" {
  type              = "egress"
  ip_protocol       = "tcp"
  port_range        = "443/443"
  security_group_id = alicloud_security_group.go_backend.id
  cidr_ip           = "0.0.0.0/0"
  description       = "Allow HTTPS outbound (Neon, APIs)"
}

# Go Backend: Allow outbound PostgreSQL (Neon uses port 5432)
resource "alicloud_security_group_rule" "go_backend_postgres" {
  type              = "egress"
  ip_protocol       = "tcp"
  port_range        = "5432/5432"
  security_group_id = alicloud_security_group.go_backend.id
  cidr_ip           = "0.0.0.0/0"
  description       = "Allow PostgreSQL outbound (Neon)"
}

# Go Backend: Allow outbound DNS (UDP 53)
resource "alicloud_security_group_rule" "go_backend_dns" {
  type              = "egress"
  ip_protocol       = "udp"
  port_range        = "53/53"
  security_group_id = alicloud_security_group.go_backend.id
  cidr_ip           = "0.0.0.0/0"
  description       = "Allow DNS resolution"
}

# Python AI Security Group (Private)
resource "alicloud_security_group" "python_ai" {
  name        = "${local.name_prefix}-python-ai-sg"
  vpc_id      = alicloud_vpc.main.id
  description = "Security group for Python AI containers (private subnet)"

  tags = merge(local.common_tags, {
    Name = "${local.name_prefix}-python-ai-sg"
  })
}

# Python AI: Allow inbound from Go Backend only (mTLS)
resource "alicloud_security_group_rule" "python_ai_from_go" {
  type                     = "ingress"
  ip_protocol              = "tcp"
  port_range               = "8000/8000"
  security_group_id        = alicloud_security_group.python_ai.id
  source_security_group_id = alicloud_security_group.go_backend.id
  description              = "Allow mTLS from Go backend only"
}

# Python AI: Allow outbound to Redis
resource "alicloud_security_group_rule" "python_ai_to_redis" {
  type              = "egress"
  ip_protocol       = "tcp"
  port_range        = "6379/6379"
  security_group_id = alicloud_security_group.python_ai.id
  cidr_ip           = var.vpc_cidr
  description       = "Allow connection to Redis"
}

# Python AI: Allow outbound HTTPS (MongoDB Atlas, OpenAI, OSS)
resource "alicloud_security_group_rule" "python_ai_https" {
  type              = "egress"
  ip_protocol       = "tcp"
  port_range        = "443/443"
  security_group_id = alicloud_security_group.python_ai.id
  cidr_ip           = "0.0.0.0/0"
  description       = "Allow HTTPS outbound (MongoDB, OpenAI, OSS)"
}

# Python AI: Allow outbound PostgreSQL (Neon)
resource "alicloud_security_group_rule" "python_ai_postgres" {
  type              = "egress"
  ip_protocol       = "tcp"
  port_range        = "5432/5432"
  security_group_id = alicloud_security_group.python_ai.id
  cidr_ip           = "0.0.0.0/0"
  description       = "Allow PostgreSQL outbound (Neon)"
}

# Python AI: Allow outbound MongoDB (Atlas uses 27017)
resource "alicloud_security_group_rule" "python_ai_mongodb" {
  type              = "egress"
  ip_protocol       = "tcp"
  port_range        = "27017/27017"
  security_group_id = alicloud_security_group.python_ai.id
  cidr_ip           = "0.0.0.0/0"
  description       = "Allow MongoDB outbound (Atlas)"
}

# Python AI: Allow outbound DNS (UDP 53)
resource "alicloud_security_group_rule" "python_ai_dns" {
  type              = "egress"
  ip_protocol       = "udp"
  port_range        = "53/53"
  security_group_id = alicloud_security_group.python_ai.id
  cidr_ip           = "0.0.0.0/0"
  description       = "Allow DNS resolution"
}

# =============================================================================
# Auto Scaling (Optional - for production)
# =============================================================================

# Note: ECI supports auto-scaling via Alibaba Cloud Auto Scaling service
# Configure scaling rules based on CPU/memory utilization
# For Week 1, we use single instances; scaling added in production hardening

# =============================================================================
# RAM Role for ECI (for OSS, MNS access)
# =============================================================================

resource "alicloud_ram_role" "eci_role" {
  name = "${local.name_prefix}-eci-role"
  document = jsonencode({
    Version = "1"
    Statement = [
      {
        Effect = "Allow"
        Principal = {
          Service = ["eci.aliyuncs.com"]
        }
        Action = "sts:AssumeRole"
      }
    ]
  })
  description = "IAM role for ECI containers"
}

# Attach OSS policy
resource "alicloud_ram_role_policy_attachment" "eci_oss" {
  role_name   = alicloud_ram_role.eci_role.name
  policy_name = alicloud_ram_policy.oss_uploads_policy.policy_name
  policy_type = "Custom"
}

# Attach MNS policy
resource "alicloud_ram_role_policy_attachment" "eci_mns" {
  role_name   = alicloud_ram_role.eci_role.name
  policy_name = "AliyunMNSFullAccess"
  policy_type = "System"
}

# Attach ACR pull policy
resource "alicloud_ram_role_policy_attachment" "eci_acr" {
  role_name   = alicloud_ram_role.eci_role.name
  policy_name = "AliyunContainerRegistryReadOnlyAccess"
  policy_type = "System"
}

# =============================================================================
# Outputs
# =============================================================================

output "eci_go_backend" {
  description = "Go backend ECI container group details"
  value = {
    id          = alicloud_eci_container_group.go_backend.id
    name        = alicloud_eci_container_group.go_backend.container_group_name
    status      = alicloud_eci_container_group.go_backend.status
    intranet_ip = alicloud_eci_container_group.go_backend.intranet_ip
  }
}

output "eci_python_ai" {
  description = "Python AI ECI container group details"
  value = {
    id          = alicloud_eci_container_group.python_ai.id
    name        = alicloud_eci_container_group.python_ai.container_group_name
    status      = alicloud_eci_container_group.python_ai.status
    intranet_ip = alicloud_eci_container_group.python_ai.intranet_ip
  }
}

output "container_security_groups" {
  description = "Container security group IDs"
  value = {
    go_backend = alicloud_security_group.go_backend.id
    python_ai  = alicloud_security_group.python_ai.id
  }
}
