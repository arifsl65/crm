# =============================================================================
# Accountant CRM - KMS Secrets Management
# =============================================================================
# Stores sensitive credentials in Alibaba Cloud KMS for secure access by ECI
# =============================================================================

# =============================================================================
# KMS Key for Secrets Encryption
# =============================================================================

resource "alicloud_kms_key" "secrets" {
  description            = "Key for encrypting ${local.name_prefix} secrets"
  key_usage              = "ENCRYPT/DECRYPT"
  pending_window_in_days = 7
  automatic_rotation     = "Disabled"

  tags = merge(local.common_tags, {
    Name = "${local.name_prefix}-secrets-key"
  })
}

resource "alicloud_kms_alias" "secrets" {
  alias_name = "alias/${local.name_prefix}-secrets"
  key_id     = alicloud_kms_key.secrets.id
}

# =============================================================================
# KMS Secrets for Database Credentials
# =============================================================================
# TEMPORARILY DISABLED: KMS Secrets Manager service not activated in console
# TODO: Re-enable after activating KMS Secrets Manager via:
#   https://kms.console.aliyun.com/ → Secrets Manager → Activate
# Credentials are passed directly via environment variables for now.
# =============================================================================

# resource "alicloud_kms_secret" "postgres_password" {
#   secret_name = "${local.name_prefix}/postgres-password"
#   secret_data = var.neon_password
#   version_id  = "v1"
#
#   encryption_key_id = alicloud_kms_key.secrets.id
#
#   tags = merge(local.common_tags, {
#     Name = "${local.name_prefix}-postgres-password"
#     Type = "database"
#   })
# }

# resource "alicloud_kms_secret" "redis_password" {
#   secret_name = "${local.name_prefix}/redis-password"
#   secret_data = var.redis_password
#   version_id  = "v1"
#
#   encryption_key_id = alicloud_kms_key.secrets.id
#
#   tags = merge(local.common_tags, {
#     Name = "${local.name_prefix}-redis-password"
#     Type = "cache"
#   })
# }

# resource "alicloud_kms_secret" "mongodb_uri" {
#   secret_name = "${local.name_prefix}/mongodb-uri"
#   secret_data = var.mongodb_uri
#   version_id  = "v1"
#
#   encryption_key_id = alicloud_kms_key.secrets.id
#
#   tags = merge(local.common_tags, {
#     Name = "${local.name_prefix}-mongodb-uri"
#     Type = "database"
#   })
# }

# resource "alicloud_kms_secret" "acr_password" {
#   secret_name = "${local.name_prefix}/acr-password"
#   secret_data = var.acr_password
#   version_id  = "v1"
#
#   encryption_key_id = alicloud_kms_key.secrets.id
#
#   tags = merge(local.common_tags, {
#     Name = "${local.name_prefix}-acr-password"
#     Type = "registry"
#   })
# }

# resource "alicloud_kms_secret" "jwt_secret_key" {
#   secret_name = "${local.name_prefix}/jwt-secret-key"
#   secret_data = var.jwt_secret_key
#   version_id  = "v1"
#
#   encryption_key_id = alicloud_kms_key.secrets.id
#
#   tags = merge(local.common_tags, {
#     Name = "${local.name_prefix}-jwt-secret-key"
#     Type = "auth"
#   })
# }

# =============================================================================
# KMS Secrets for mTLS Certificates (Fix #10)
# =============================================================================
# TEMPORARILY DISABLED: KMS Secrets Manager service not activated
# =============================================================================

# resource "alicloud_kms_secret" "mtls_ca_cert" {
#   count       = var.mtls_ca_cert != "" ? 1 : 0
#   secret_name = "${local.name_prefix}/mtls-ca-cert"
#   secret_data = var.mtls_ca_cert
#   version_id  = "v1"
#
#   encryption_key_id = alicloud_kms_key.secrets.id
#
#   tags = merge(local.common_tags, {
#     Name = "${local.name_prefix}-mtls-ca-cert"
#     Type = "mtls"
#   })
# }

# resource "alicloud_kms_secret" "mtls_server_cert" {
#   count       = var.mtls_server_cert != "" ? 1 : 0
#   secret_name = "${local.name_prefix}/mtls-server-cert"
#   secret_data = var.mtls_server_cert
#   version_id  = "v1"
#
#   encryption_key_id = alicloud_kms_key.secrets.id
#
#   tags = merge(local.common_tags, {
#     Name = "${local.name_prefix}-mtls-server-cert"
#     Type = "mtls"
#   })
# }

# resource "alicloud_kms_secret" "mtls_server_key" {
#   count       = var.mtls_server_key != "" ? 1 : 0
#   secret_name = "${local.name_prefix}/mtls-server-key"
#   secret_data = var.mtls_server_key
#   version_id  = "v1"
#
#   encryption_key_id = alicloud_kms_key.secrets.id
#
#   tags = merge(local.common_tags, {
#     Name = "${local.name_prefix}-mtls-server-key"
#     Type = "mtls"
#   })
# }

# resource "alicloud_kms_secret" "mtls_client_cert" {
#   count       = var.mtls_client_cert != "" ? 1 : 0
#   secret_name = "${local.name_prefix}/mtls-client-cert"
#   secret_data = var.mtls_client_cert
#   version_id  = "v1"
#
#   encryption_key_id = alicloud_kms_key.secrets.id
#
#   tags = merge(local.common_tags, {
#     Name = "${local.name_prefix}-mtls-client-cert"
#     Type = "mtls"
#   })
# }

# resource "alicloud_kms_secret" "mtls_client_key" {
#   count       = var.mtls_client_key != "" ? 1 : 0
#   secret_name = "${local.name_prefix}/mtls-client-key"
#   secret_data = var.mtls_client_key
#   version_id  = "v1"
#
#   encryption_key_id = alicloud_kms_key.secrets.id
#
#   tags = merge(local.common_tags, {
#     Name = "${local.name_prefix}-mtls-client-key"
#     Type = "mtls"
#   })
# }

# =============================================================================
# RAM Policy for ECI to Access Secrets
# =============================================================================
# TEMPORARILY DISABLED: KMS Secrets not available
# =============================================================================

# resource "alicloud_ram_policy" "eci_secrets_access" {
#   policy_name = "${local.name_prefix}-eci-secrets-access"
#   policy_document = jsonencode({
#     Version = "1"
#     Statement = [
#       {
#         Effect = "Allow"
#         Action = [
#           "kms:GetSecretValue",
#           "kms:DescribeSecret"
#         ]
#         Resource = concat(
#           [
#             alicloud_kms_secret.postgres_password.arn,
#             alicloud_kms_secret.redis_password.arn,
#             alicloud_kms_secret.mongodb_uri.arn,
#             alicloud_kms_secret.acr_password.arn,
#             alicloud_kms_secret.jwt_secret_key.arn
#           ],
#           # mTLS secrets (conditional)
#           var.mtls_ca_cert != "" ? [alicloud_kms_secret.mtls_ca_cert[0].arn] : [],
#           var.mtls_server_cert != "" ? [alicloud_kms_secret.mtls_server_cert[0].arn] : [],
#           var.mtls_server_key != "" ? [alicloud_kms_secret.mtls_server_key[0].arn] : [],
#           var.mtls_client_cert != "" ? [alicloud_kms_secret.mtls_client_cert[0].arn] : [],
#           var.mtls_client_key != "" ? [alicloud_kms_secret.mtls_client_key[0].arn] : []
#         )
#       },
#       {
#         Effect = "Allow"
#         Action = [
#           "kms:Decrypt"
#         ]
#         Resource = [
#           alicloud_kms_key.secrets.arn
#         ]
#       }
#     ]
#   })
#   description = "Allow ECI containers to read secrets from KMS"
# }

# resource "alicloud_ram_role_policy_attachment" "eci_secrets" {
#   role_name   = alicloud_ram_role.eci_role.name
#   policy_name = alicloud_ram_policy.eci_secrets_access.policy_name
#   policy_type = "Custom"
# }

# =============================================================================
# Local values for secret ARNs (used by ECI)
# =============================================================================
# TEMPORARILY DISABLED: KMS Secrets not available
# =============================================================================

# locals {
#   secret_arns = {
#     postgres_password = alicloud_kms_secret.postgres_password.arn
#     redis_password    = alicloud_kms_secret.redis_password.arn
#     mongodb_uri       = alicloud_kms_secret.mongodb_uri.arn
#     acr_password      = alicloud_kms_secret.acr_password.arn
#   }
# }

# =============================================================================
# Outputs
# =============================================================================

output "kms_key_id" {
  description = "KMS key ID for secrets encryption"
  value       = alicloud_kms_key.secrets.id
}

# Outputs disabled until KMS Secrets Manager is activated
# output "secret_names" {
#   description = "KMS secret names"
#   value = {
#     postgres_password = alicloud_kms_secret.postgres_password.secret_name
#     redis_password    = alicloud_kms_secret.redis_password.secret_name
#     mongodb_uri       = alicloud_kms_secret.mongodb_uri.secret_name
#     acr_password      = alicloud_kms_secret.acr_password.secret_name
#   }
# }
