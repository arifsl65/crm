"""
Alibaba Cloud KMS Secrets Manager integration.

Provides secret retrieval from KMS when running in Alibaba Cloud ECI.
Uses ECS RAM role for automatic credential management.
"""

import os
from functools import lru_cache
from typing import Optional

import structlog

logger = structlog.get_logger(__name__)

# Lazy import to avoid requiring SDK in development
_kms_client = None


def is_kms_enabled() -> bool:
    """Check if KMS-based secret fetching is enabled."""
    return os.environ.get("SECRETS_FROM_KMS", "").lower() == "true"


def _get_kms_client():
    """Get or create the KMS client (lazy initialization)."""
    global _kms_client

    if _kms_client is not None:
        return _kms_client

    try:
        from alibabacloud_kms20160120.client import Client as KmsClient
        from alibabacloud_tea_openapi import models as open_api_models
        from alibabacloud_credentials.client import Client as CredClient
    except ImportError as e:
        raise ImportError(
            "Alibaba Cloud KMS SDK not installed. "
            "Install with: pip install alibabacloud-kms20160120 alibabacloud-credentials"
        ) from e

    region = os.environ.get("KMS_REGION", "eu-west-1")

    # Use ECS RAM role credentials (automatic in ECI)
    cred_client = CredClient()

    config = open_api_models.Config(
        credential=cred_client,
        region_id=region,
        endpoint=f"kms.{region}.aliyuncs.com",
    )

    _kms_client = KmsClient(config)
    logger.info("Initialized KMS client", region=region)

    return _kms_client


@lru_cache(maxsize=32)
def get_secret(secret_name: str) -> str:
    """
    Retrieve a secret value from KMS Secrets Manager.

    Results are cached to avoid repeated API calls.

    Args:
        secret_name: The name of the secret in KMS.

    Returns:
        The secret value as a string.

    Raises:
        Exception: If secret retrieval fails.
    """
    try:
        from alibabacloud_kms20160120 import models as kms_models
    except ImportError as e:
        raise ImportError(
            "Alibaba Cloud KMS SDK not installed. "
            "Install with: pip install alibabacloud-kms20160120"
        ) from e

    client = _get_kms_client()

    request = kms_models.GetSecretValueRequest(
        secret_name=secret_name,
        version_stage="ACSCurrent",  # Get current version
    )

    response = client.get_secret_value(request)

    logger.debug("Retrieved secret from KMS", secret_name=secret_name)

    return response.body.secret_data


def get_secret_or_env(secret_name_env: str, env_var: str, default: str = "") -> str:
    """
    Get a secret from KMS if enabled, otherwise from environment variable.

    Args:
        secret_name_env: Environment variable containing the KMS secret name.
        env_var: Environment variable to fall back to if KMS is disabled.
        default: Default value if neither source provides a value.

    Returns:
        The secret value.
    """
    # First check if we have a direct env var value
    direct_value = os.environ.get(env_var, "")
    if direct_value:
        return direct_value

    # If KMS is enabled, try to fetch from KMS
    if is_kms_enabled():
        secret_name = os.environ.get(secret_name_env, "")
        if secret_name:
            try:
                return get_secret(secret_name)
            except Exception as e:
                logger.error(
                    "Failed to retrieve secret from KMS",
                    secret_name=secret_name,
                    error=str(e),
                )
                raise

    return default


def clear_cache():
    """Clear the secret cache (useful for secret rotation)."""
    get_secret.cache_clear()
    logger.info("Cleared KMS secret cache")


def load_mtls_certs_from_kms() -> tuple[str, str, str]:
    """
    Load mTLS certificates from KMS and write to temporary files.

    For the Python AI service (which acts as the mTLS server), this loads:
    - CA certificate (for verifying client certs)
    - Server certificate
    - Server private key

    Returns:
        Tuple of (ca_cert_path, server_cert_path, server_key_path)

    Raises:
        ValueError: If KMS secret names are not configured.
        Exception: If secret retrieval fails.
    """
    import tempfile
    import stat

    # Get secret names from environment
    ca_secret = os.environ.get("KMS_MTLS_CA_CERT_SECRET", "")
    server_cert_secret = os.environ.get("KMS_MTLS_SERVER_CERT_SECRET", "")
    server_key_secret = os.environ.get("KMS_MTLS_SERVER_KEY_SECRET", "")

    if not ca_secret or not server_cert_secret or not server_key_secret:
        raise ValueError(
            f"mTLS KMS secret names not configured: "
            f"CA={ca_secret!r}, Cert={server_cert_secret!r}, Key={server_key_secret!r}"
        )

    # Fetch certificates from KMS
    ca_cert = get_secret(ca_secret)
    server_cert = get_secret(server_cert_secret)
    server_key = get_secret(server_key_secret)

    # Write to temporary files with secure permissions
    def write_temp_file(prefix: str, content: str) -> str:
        fd, path = tempfile.mkstemp(prefix=prefix, suffix=".pem")
        try:
            os.chmod(path, stat.S_IRUSR | stat.S_IWUSR)  # 0600
            os.write(fd, content.encode("utf-8"))
        finally:
            os.close(fd)
        return path

    ca_cert_path = write_temp_file("mtls-ca-", ca_cert)
    server_cert_path = write_temp_file("mtls-server-cert-", server_cert)
    server_key_path = write_temp_file("mtls-server-key-", server_key)

    logger.info(
        "Loaded mTLS certificates from KMS",
        ca_cert_path=ca_cert_path,
        server_cert_path=server_cert_path,
        server_key_path=server_key_path,
    )

    return ca_cert_path, server_cert_path, server_key_path
