"""
mTLS certificate validation for secure Go <-> Python communication.

Provides SSL context creation and certificate verification.
"""

import ssl
from pathlib import Path
from typing import Optional

import structlog

from .config import get_settings

logger = structlog.get_logger(__name__)


class MTLSConfig:
    """mTLS configuration and SSL context management."""

    def __init__(
        self,
        ca_cert: Optional[str] = None,
        server_cert: Optional[str] = None,
        server_key: Optional[str] = None,
        client_cert: Optional[str] = None,
        client_key: Optional[str] = None,
    ):
        """
        Initialize mTLS configuration.

        Args:
            ca_cert: Path to CA certificate file.
            server_cert: Path to server certificate file.
            server_key: Path to server private key file.
            client_cert: Path to client certificate file.
            client_key: Path to client private key file.
        """
        settings = get_settings()

        self.ca_cert = ca_cert or settings.mtls_ca_cert
        self.server_cert = server_cert or settings.mtls_server_cert
        self.server_key = server_key or settings.mtls_server_key
        self.client_cert = client_cert or settings.mtls_client_cert
        self.client_key = client_key or settings.mtls_client_key
        self.enabled = settings.mtls_enabled

    def _validate_cert_paths(self) -> bool:
        """
        Validate that all certificate files exist.

        Returns:
            bool: True if all required files exist.
        """
        required_files = [
            self.ca_cert,
            self.server_cert,
            self.server_key,
        ]

        for cert_path in required_files:
            if not cert_path or not Path(cert_path).exists():
                logger.warning(
                    "mTLS certificate file not found",
                    path=cert_path
                )
                return False

        return True

    def create_server_ssl_context(self) -> Optional[ssl.SSLContext]:
        """
        Create SSL context for server (receiving connections).

        Returns:
            Optional[ssl.SSLContext]: SSL context or None if disabled.
        """
        if not self.enabled:
            logger.info("mTLS is disabled")
            return None

        if not self._validate_cert_paths():
            logger.error("mTLS enabled but certificates not found")
            return None

        try:
            context = ssl.SSLContext(ssl.PROTOCOL_TLS_SERVER)
            context.minimum_version = ssl.TLSVersion.TLSv1_2

            # Load server certificate and key
            context.load_cert_chain(
                certfile=self.server_cert,
                keyfile=self.server_key,
            )

            # Load CA certificate for client verification
            context.load_verify_locations(cafile=self.ca_cert)

            # Require client certificate
            context.verify_mode = ssl.CERT_REQUIRED

            logger.info(
                "mTLS server SSL context created",
                ca_cert=self.ca_cert,
                server_cert=self.server_cert,
            )

            return context

        except ssl.SSLError as e:
            logger.error("Failed to create server SSL context", error=str(e))
            return None

    def create_client_ssl_context(self) -> Optional[ssl.SSLContext]:
        """
        Create SSL context for client (making connections).

        Returns:
            Optional[ssl.SSLContext]: SSL context or None if disabled.
        """
        if not self.enabled:
            return None

        if not self.client_cert or not self.client_key:
            logger.warning("Client certificates not configured")
            return None

        try:
            context = ssl.SSLContext(ssl.PROTOCOL_TLS_CLIENT)
            context.minimum_version = ssl.TLSVersion.TLSv1_2

            # Load client certificate and key
            context.load_cert_chain(
                certfile=self.client_cert,
                keyfile=self.client_key,
            )

            # Load CA certificate for server verification
            if self.ca_cert:
                context.load_verify_locations(cafile=self.ca_cert)
                context.verify_mode = ssl.CERT_REQUIRED
            else:
                context.check_hostname = False
                context.verify_mode = ssl.CERT_NONE

            logger.info(
                "mTLS client SSL context created",
                client_cert=self.client_cert,
            )

            return context

        except ssl.SSLError as e:
            logger.error("Failed to create client SSL context", error=str(e))
            return None


def get_mtls_config() -> MTLSConfig:
    """
    Get mTLS configuration instance.

    Returns:
        MTLSConfig: mTLS configuration.
    """
    return MTLSConfig()
