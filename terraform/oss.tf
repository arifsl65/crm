# =============================================================================
# Accountant CRM - OSS Buckets
# =============================================================================

# =============================================================================
# Frontend Static Assets Bucket
# =============================================================================

resource "alicloud_oss_bucket" "frontend" {
  bucket        = var.oss_frontend_bucket
  acl           = "public-read"
  storage_class = "Standard"

  # Enable static website hosting
  website {
    index_document = "index.html"
    error_document = "404.html"
  }

  # Enable versioning for rollbacks
  versioning {
    status = "Enabled"
  }

  # Lifecycle rules for old versions
  lifecycle_rule {
    id      = "cleanup-old-versions"
    enabled = true

    noncurrent_version_expiration {
      days = 30
    }
  }

  # CORS configuration for frontend
  # FIXED: Restricted from wildcard (*) to specific domains
  cors_rule {
    allowed_origins = [
      "https://${var.domain_name}",
      "https://www.${var.domain_name}",
      "https://app.${var.domain_name}",
      "https://staging.${var.domain_name}",
      "http://localhost:3000" # Local development
    ]
    allowed_methods = ["GET", "HEAD"]
    allowed_headers = ["*"]
    max_age_seconds = 3600
  }

  # Logging
  logging {
    target_bucket = alicloud_oss_bucket.logs.bucket
    target_prefix = "frontend-access-logs/"
  }

  tags = merge(local.common_tags, {
    Name = var.oss_frontend_bucket
    Type = "frontend"
  })
}

# =============================================================================
# Uploads Bucket (Production)
# =============================================================================

resource "alicloud_oss_bucket" "uploads" {
  bucket        = var.oss_uploads_bucket
  acl           = "private"
  storage_class = "Standard"

  # Enable versioning
  versioning {
    status = "Enabled"
  }

  # Lifecycle rules
  lifecycle_rule {
    id      = "archive-old-documents"
    enabled = true
    prefix  = "documents/"

    # Move to IA storage after 90 days
    transitions {
      days          = 90
      storage_class = "IA"
    }

    # Move to Archive after 365 days
    transitions {
      days          = 365
      storage_class = "Archive"
    }
  }

  lifecycle_rule {
    id      = "cleanup-temp-files"
    enabled = true
    prefix  = "temp/"

    expiration {
      days = 7
    }
  }

  lifecycle_rule {
    id      = "cleanup-old-versions"
    enabled = true

    noncurrent_version_expiration {
      days = 90
    }
  }

  # CORS for uploads
  cors_rule {
    allowed_origins = ["https://*.${var.domain_name}", "http://localhost:3000"]
    allowed_methods = ["GET", "PUT", "POST", "DELETE", "HEAD"]
    allowed_headers = ["*"]
    expose_headers  = ["ETag", "Content-Length"]
    max_age_seconds = 3600
  }

  # Server-side encryption
  server_side_encryption_rule {
    sse_algorithm = "AES256"
  }

  # Logging
  logging {
    target_bucket = alicloud_oss_bucket.logs.bucket
    target_prefix = "uploads-access-logs/"
  }

  tags = merge(local.common_tags, {
    Name = var.oss_uploads_bucket
    Type = "uploads"
  })
}

# =============================================================================
# Uploads Bucket (Staging)
# =============================================================================

resource "alicloud_oss_bucket" "uploads_staging" {
  count = var.environment == "staging" ? 1 : 0

  bucket        = var.oss_uploads_staging_bucket
  acl           = "private"
  storage_class = "Standard"

  # Shorter retention for staging
  lifecycle_rule {
    id      = "cleanup-staging"
    enabled = true

    expiration {
      days = 30
    }
  }

  # Server-side encryption
  server_side_encryption_rule {
    sse_algorithm = "AES256"
  }

  tags = merge(local.common_tags, {
    Name = var.oss_uploads_staging_bucket
    Type = "uploads-staging"
  })
}

# =============================================================================
# Logs Bucket
# =============================================================================

resource "alicloud_oss_bucket" "logs" {
  bucket        = "${var.project_name}-logs-${var.environment}"
  acl           = "private"
  storage_class = "IA"

  # Lifecycle for log retention
  lifecycle_rule {
    id      = "archive-old-logs"
    enabled = true

    transitions {
      days          = 30
      storage_class = "Archive"
    }

    expiration {
      days = 365
    }
  }

  tags = merge(local.common_tags, {
    Name = "${var.project_name}-logs-${var.environment}"
    Type = "logs"
  })
}

# =============================================================================
# Bucket Policies
# =============================================================================

# RAM policy for application access to uploads bucket
resource "alicloud_ram_policy" "oss_uploads_policy" {
  policy_name = "${local.name_prefix}-oss-uploads-policy"
  policy_document = jsonencode({
    Version = "1"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "oss:GetObject",
          "oss:PutObject",
          "oss:DeleteObject",
          "oss:ListObjects",
          "oss:GetObjectMeta",
          "oss:AbortMultipartUpload",
          "oss:ListMultipartUploads"
        ]
        Resource = [
          "acs:oss:*:*:${var.oss_uploads_bucket}",
          "acs:oss:*:*:${var.oss_uploads_bucket}/*"
        ]
      }
    ]
  })
  description = "Policy for application access to uploads bucket"
}

# RAM policy for frontend deployment
resource "alicloud_ram_policy" "oss_frontend_policy" {
  policy_name = "${local.name_prefix}-oss-frontend-policy"
  policy_document = jsonencode({
    Version = "1"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "oss:GetObject",
          "oss:PutObject",
          "oss:DeleteObject",
          "oss:ListObjects"
        ]
        Resource = [
          "acs:oss:*:*:${var.oss_frontend_bucket}",
          "acs:oss:*:*:${var.oss_frontend_bucket}/*"
        ]
      }
    ]
  })
  description = "Policy for frontend deployment"
}
