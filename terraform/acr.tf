# =============================================================================
# Accountant CRM - Container Registry (ACR)
# =============================================================================

# =============================================================================
# ACR Enterprise Instance
# =============================================================================

resource "alicloud_cr_ee_instance" "main" {
  instance_type  = "Basic"    # Terraform schema value (actual: Enterprise_Economy)
  instance_name  = "fzco-acr" # Existing instance created via console
  payment_type   = "Subscription"
  period         = 1
  renewal_status = "AutoRenewal"
  renew_period   = 1

  # Note: Instance already created via console and imported into Terraform
  # Economy edition (cost-efficient) for staging environment

  lifecycle {
    ignore_changes = [
      instance_type, # Actual type is Enterprise_Economy, Terraform schema uses Basic/Standard/Advanced
      payment_type,
      period,
      renewal_status,
      renew_period
    ]
  }
}

# =============================================================================
# Namespace
# =============================================================================

resource "alicloud_cr_ee_namespace" "main" {
  instance_id        = alicloud_cr_ee_instance.main.id
  name               = "accountant-crm"
  auto_create        = false
  default_visibility = "PRIVATE"
}

# =============================================================================
# Repositories
# =============================================================================

# Go Backend Repository
resource "alicloud_cr_ee_repo" "go_backend" {
  instance_id = alicloud_cr_ee_instance.main.id
  namespace   = alicloud_cr_ee_namespace.main.name
  name        = "go-backend"
  summary     = "Go backend service for Accountant CRM"
  detail      = "Contains the main Go API service"
  repo_type   = "PRIVATE"
}

# Python AI Repository
resource "alicloud_cr_ee_repo" "python_ai" {
  instance_id = alicloud_cr_ee_instance.main.id
  namespace   = alicloud_cr_ee_namespace.main.name
  name        = "python-ai"
  summary     = "Python AI service for Accountant CRM"
  detail      = "Contains the AI/ML service for document processing"
  repo_type   = "PRIVATE"
}

# Frontend Repository (for optional SSR builds)
resource "alicloud_cr_ee_repo" "frontend" {
  instance_id = alicloud_cr_ee_instance.main.id
  namespace   = alicloud_cr_ee_namespace.main.name
  name        = "frontend"
  summary     = "Frontend build container"
  detail      = "Contains Next.js build artifacts"
  repo_type   = "PRIVATE"
}

# =============================================================================
# Image Scanning Policy
# =============================================================================

# Note: Image scanning is enabled by default in ACR EE
# Configure vulnerability scanning triggers as needed

# =============================================================================
# Access Control
# =============================================================================

# RAM policy for CI/CD push access
resource "alicloud_ram_policy" "acr_push" {
  policy_name = "${local.name_prefix}-acr-push-policy"
  policy_document = jsonencode({
    Version = "1"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "cr:GetAuthorizationToken",
          "cr:PushRepository",
          "cr:PullRepository",
          "cr:GetRepository",
          "cr:ListRepository",
          "cr:CreateRepository",
          "cr:GetTag",
          "cr:ListTag",
          "cr:DeleteTag"
        ]
        Resource = [
          "acs:cr:${var.region}:*:repository/${alicloud_cr_ee_namespace.main.name}/*"
        ]
      }
    ]
  })
  description = "Policy for CI/CD to push images to ACR"
}

# RAM policy for pull-only access (for ECS nodes)
resource "alicloud_ram_policy" "acr_pull" {
  policy_name = "${local.name_prefix}-acr-pull-policy"
  policy_document = jsonencode({
    Version = "1"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "cr:GetAuthorizationToken",
          "cr:PullRepository",
          "cr:GetRepository",
          "cr:ListRepository",
          "cr:GetTag",
          "cr:ListTag"
        ]
        Resource = [
          "acs:cr:${var.region}:*:repository/${alicloud_cr_ee_namespace.main.name}/*"
        ]
      }
    ]
  })
  description = "Policy for ECS nodes to pull images from ACR"
}

# =============================================================================
# Build Rules (for automatic builds from source)
# =============================================================================

# Note: Build rules can be configured to trigger on Git pushes
# This requires connecting ACR to your Git provider

# Example build rule structure (configure via console or API):
# - Source: GitHub/GitLab repository
# - Branch: main for production, develop for staging
# - Dockerfile path: go-backend/Dockerfile, python-ai/Dockerfile
# - Build context: go-backend/, python-ai/

# =============================================================================
# Image Lifecycle Policy
# =============================================================================

# Retention policy for images
resource "alicloud_cr_ee_sync_rule" "cleanup" {
  count = 0 # Disabled by default, enable for cross-region sync

  instance_id           = alicloud_cr_ee_instance.main.id
  namespace_name        = alicloud_cr_ee_namespace.main.name
  name                  = "sync-to-backup"
  target_region_id      = "cn-hongkong"
  target_instance_id    = "" # Target ACR instance ID
  target_namespace_name = alicloud_cr_ee_namespace.main.name
  tag_filter            = "latest,v*"
  sync_scope            = "REPO"
  sync_trigger          = "PASSIVE"
}

# =============================================================================
# Outputs
# =============================================================================

output "acr_details" {
  description = "ACR configuration details"
  value = {
    instance_id   = alicloud_cr_ee_instance.main.id
    instance_name = alicloud_cr_ee_instance.main.instance_name
    namespace     = alicloud_cr_ee_namespace.main.name

    repositories = {
      go_backend = "${alicloud_cr_ee_namespace.main.name}/${alicloud_cr_ee_repo.go_backend.name}"
      python_ai  = "${alicloud_cr_ee_namespace.main.name}/${alicloud_cr_ee_repo.python_ai.name}"
      frontend   = "${alicloud_cr_ee_namespace.main.name}/${alicloud_cr_ee_repo.frontend.name}"
    }
  }
}
