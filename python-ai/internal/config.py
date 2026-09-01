"""
Configuration management for the Python AI service.

Loads settings from environment variables with validation.
When SECRETS_FROM_KMS=true, sensitive values are fetched from Alibaba Cloud KMS.
"""

import os
from functools import lru_cache
from typing import Literal, Optional

import structlog
from pydantic import Field, field_validator, model_validator
from pydantic_settings import BaseSettings

logger = structlog.get_logger(__name__)


class Settings(BaseSettings):
    """Application settings loaded from environment variables."""

    # Application
    app_env: Literal["development", "staging", "production"] = "development"
    app_name: str = "accountant-ai"
    debug: bool = True
    log_level: str = "DEBUG"

    # KMS Configuration
    secrets_from_kms: bool = False
    kms_region: str = "eu-west-1"
    kms_postgres_password_secret: str = ""
    kms_redis_password_secret: str = ""
    kms_mongodb_uri_secret: str = ""

    # Server
    host: str = "0.0.0.0"
    port: int = 8000
    workers: int = 4

    # PostgreSQL (Neon - shared with Go backend)
    postgres_host: str = "localhost"
    postgres_port: int = 5432
    postgres_user: str = "accountant"
    postgres_password: str = ""
    postgres_database: str = "accountant_crm"
    postgres_sslmode: str = "require"  # Default to require for security (Neon requires SSL)
    # Fix #32: Added validation for pool_min/max
    postgres_pool_min: int = Field(default=2, ge=1, le=20, description="Minimum pool connections (1-20)")
    postgres_pool_max: int = Field(default=5, ge=1, le=100, description="Maximum pool connections (1-100)")

    # Fix #32: Validate pool_min <= pool_max
    @model_validator(mode='after')
    def validate_pool_sizes(self) -> 'Settings':
        if self.postgres_pool_min > self.postgres_pool_max:
            raise ValueError(
                f"postgres_pool_min ({self.postgres_pool_min}) cannot be greater than "
                f"postgres_pool_max ({self.postgres_pool_max})"
            )
        return self

    @model_validator(mode='after')
    def fetch_kms_secrets(self) -> 'Settings':
        """Fetch secrets from KMS if SECRETS_FROM_KMS=true."""
        if not self.secrets_from_kms:
            return self

        try:
            from .secrets import get_secret
        except ImportError:
            logger.warning(
                "KMS SDK not available, skipping KMS secret fetching. "
                "Install with: pip install alibabacloud-kms20160120 alibabacloud-credentials"
            )
            return self

        logger.info("Fetching secrets from KMS", region=self.kms_region)

        # Fetch PostgreSQL password
        if self.kms_postgres_password_secret and not self.postgres_password:
            try:
                value = get_secret(self.kms_postgres_password_secret)
                object.__setattr__(self, 'postgres_password', value)
                logger.info("Loaded PostgreSQL password from KMS")
            except Exception as e:
                raise ValueError(f"Failed to fetch PostgreSQL password from KMS: {e}") from e

        # Fetch Redis password
        if self.kms_redis_password_secret and not self.redis_password:
            try:
                value = get_secret(self.kms_redis_password_secret)
                object.__setattr__(self, 'redis_password', value)
                logger.info("Loaded Redis password from KMS")
            except Exception as e:
                raise ValueError(f"Failed to fetch Redis password from KMS: {e}") from e

        # Fetch MongoDB URI
        if self.kms_mongodb_uri_secret and self.mongodb_uri == "mongodb://localhost:27017/":
            try:
                value = get_secret(self.kms_mongodb_uri_secret)
                object.__setattr__(self, 'mongodb_uri', value)
                logger.info("Loaded MongoDB URI from KMS")
            except Exception as e:
                raise ValueError(f"Failed to fetch MongoDB URI from KMS: {e}") from e

        # Fetch mTLS certificates from KMS if enabled
        if self.mtls_enabled and not self.mtls_server_cert:
            try:
                from .secrets import load_mtls_certs_from_kms
                ca_cert_path, server_cert_path, server_key_path = load_mtls_certs_from_kms()
                object.__setattr__(self, 'mtls_ca_cert', ca_cert_path)
                object.__setattr__(self, 'mtls_server_cert', server_cert_path)
                object.__setattr__(self, 'mtls_server_key', server_key_path)
                logger.info("Loaded mTLS certificates from KMS")
            except Exception as e:
                raise ValueError(f"Failed to fetch mTLS certificates from KMS: {e}") from e

        return self

    # MongoDB Atlas
    mongodb_uri: str = Field(
        default="mongodb://localhost:27017/",
        description="MongoDB connection URI"
    )
    mongodb_database: str = "accountant_ai"
    mongodb_collection_documents: str = "documents"
    mongodb_collection_embeddings: str = "embeddings"

    # Redis
    redis_host: str = "localhost"
    redis_port: int = 6379
    redis_password: str = ""
    redis_db: int = 0
    redis_tls_enabled: bool = False

    # Alibaba Cloud OSS
    # OSS is optional - set OSS_ENABLED=false to disable, or leave credentials empty
    oss_enabled: bool = True  # Disabled automatically if credentials are empty
    alibaba_access_key_id: str = ""
    alibaba_access_key_secret: str = ""
    alibaba_region: str = "eu-west-1"  # UK (London)
    oss_endpoint: str = "https://oss-eu-west-1.aliyuncs.com"
    oss_bucket_uploads: str = "fzco-uploads"

    @property
    def oss_configured(self) -> bool:
        """Check if OSS credentials are configured and OSS is enabled."""
        return (
            self.oss_enabled
            and bool(self.alibaba_access_key_id)
            and bool(self.alibaba_access_key_secret)
        )

    # mTLS
    mtls_enabled: bool = False
    mtls_ca_cert: str = ""
    mtls_server_cert: str = ""
    mtls_server_key: str = ""
    mtls_client_cert: str = ""
    mtls_client_key: str = ""

    # CORS Configuration
    # In production, this MUST be set to explicit origins (comma-separated)
    # Example: "https://app.accountant-crm.com,https://admin.accountant-crm.com"
    cors_allowed_origins: str = ""

    # Feature Flags (defaults)
    ai_ocr_enabled: bool = True
    ai_chat_enabled: bool = True
    ai_forms_enabled: bool = True
    ai_classification_enabled: bool = True

    # OpenAI (optional)
    openai_api_key: str = ""
    openai_model: str = "gpt-4-turbo-preview"

    # Groq AI (primary AI provider)
    groq_api_key: str = ""
    groq_model: str = "openai/gpt-oss-120b"  # General-purpose large model for document analysis
    groq_max_tokens: int = 4096
    groq_temperature: float = 0.1  # Low for consistent document analysis

    def get_cors_origins(self) -> list[str]:
        """Parse CORS origins from comma-separated string."""
        if not self.cors_allowed_origins:
            return []
        return [origin.strip() for origin in self.cors_allowed_origins.split(",") if origin.strip()]

    class Config:
        """Pydantic configuration."""

        env_file = ".env"
        env_file_encoding = "utf-8"
        case_sensitive = False
        extra = "ignore"


@lru_cache()
def get_settings() -> Settings:
    """
    Get cached settings instance.

    Returns:
        Settings: Application settings loaded from environment.
    """
    return Settings()
