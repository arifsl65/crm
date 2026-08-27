"""
Alibaba Cloud OSS client for file storage.

Provides file upload, download, and management operations.

IMPORTANT: The oss2 library is synchronous. All blocking operations
are wrapped in asyncio.to_thread() to prevent blocking the event loop.
"""

import asyncio
import mimetypes
from io import BytesIO
from typing import BinaryIO, Optional, Tuple

import oss2
import structlog

from .config import get_settings

logger = structlog.get_logger(__name__)

# Global bucket instance
_bucket: Optional[oss2.Bucket] = None


def connect() -> oss2.Bucket:
    """
    Connect to Alibaba Cloud OSS.

    Returns:
        oss2.Bucket: Bucket instance.

    Raises:
        oss2.exceptions.OssError: If connection fails.
    """
    global _bucket

    if _bucket is not None:
        return _bucket

    settings = get_settings()

    try:
        # Use V4 signature (V1 is deprecated and forbidden)
        auth = oss2.AuthV4(
            settings.alibaba_access_key_id,
            settings.alibaba_access_key_secret,
        )

        _bucket = oss2.Bucket(
            auth,
            settings.oss_endpoint,
            settings.oss_bucket_uploads,
            region=settings.alibaba_region,
        )

        # Verify connection by checking bucket info
        _bucket.get_bucket_info()

        logger.info(
            "Connected to OSS",
            endpoint=settings.oss_endpoint,
            bucket=settings.oss_bucket_uploads,
        )

        return _bucket

    except oss2.exceptions.OssError as e:
        logger.error("Failed to connect to OSS", error=str(e))
        raise


def get_bucket() -> Optional[oss2.Bucket]:
    """
    Get the bucket instance.

    Returns:
        Optional[oss2.Bucket]: Bucket instance or None.
    """
    return _bucket


def _health_check_sync() -> bool:
    """Synchronous health check - use health_check() for async."""
    global _bucket

    if _bucket is None:
        return False

    try:
        _bucket.get_bucket_info()
        return True
    except Exception as e:
        logger.error("OSS health check failed", error=str(e))
        return False


async def health_check() -> bool:
    """
    Check OSS connection health (async).

    Runs in thread pool to avoid blocking the event loop.

    Returns:
        bool: True if healthy, False otherwise.
    """
    return await asyncio.to_thread(_health_check_sync)


class OSSClient:
    """
    OSS file operations client with async support.

    All async methods use asyncio.to_thread() to run blocking oss2
    operations in a thread pool, preventing event loop blocking.
    """

    def __init__(self, bucket: Optional[oss2.Bucket] = None):
        """
        Initialize OSS client.

        Args:
            bucket: OSS bucket instance. Uses global if not provided.
        """
        self._bucket = bucket

    @property
    def bucket(self) -> oss2.Bucket:
        """Get the bucket instance."""
        if self._bucket is not None:
            return self._bucket

        global_bucket = get_bucket()
        if global_bucket is None:
            raise RuntimeError("OSS not connected")
        return global_bucket

    # =========================================================================
    # Synchronous methods (internal use only - prefer async versions)
    # =========================================================================

    def _upload_file_sync(
        self,
        key: str,
        file_obj: BinaryIO,
        content_type: Optional[str] = None,
    ) -> str:
        """Synchronous upload - use upload_file() for async."""
        headers = {}
        if content_type:
            headers["Content-Type"] = content_type
        else:
            guessed_type, _ = mimetypes.guess_type(key)
            if guessed_type:
                headers["Content-Type"] = guessed_type

        self.bucket.put_object(key, file_obj, headers=headers)
        logger.info("File uploaded to OSS", key=key)
        return self._get_url_sync(key)

    def _download_file_sync(self, key: str) -> Tuple[bytes, str]:
        """Synchronous download - use download_file() for async."""
        result = self.bucket.get_object(key)
        content = result.read()
        content_type = result.headers.get("Content-Type", "application/octet-stream")
        logger.info("File downloaded from OSS", key=key)
        return content, content_type

    def _delete_file_sync(self, key: str) -> bool:
        """Synchronous delete - use delete_file() for async."""
        try:
            self.bucket.delete_object(key)
            logger.info("File deleted from OSS", key=key)
            return True
        except oss2.exceptions.NoSuchKey:
            logger.warning("File not found in OSS", key=key)
            return False

    def _file_exists_sync(self, key: str) -> bool:
        """Synchronous exists check - use file_exists() for async."""
        return self.bucket.object_exists(key)

    def _get_url_sync(self, key: str, expires: int = 3600) -> str:
        """Synchronous URL generation - use get_url() for async."""
        return self.bucket.sign_url("GET", key, expires)

    def _list_files_sync(self, prefix: str = "", max_keys: int = 100) -> list:
        """Synchronous list - use list_files() for async."""
        result = []
        for obj in oss2.ObjectIterator(
            self.bucket,
            prefix=prefix,
            max_keys=max_keys,
        ):
            result.append(obj.key)
        return result

    # =========================================================================
    # Async methods (preferred - run blocking ops in thread pool)
    # =========================================================================

    async def upload_file(
        self,
        key: str,
        file_obj: BinaryIO,
        content_type: Optional[str] = None,
    ) -> str:
        """
        Upload a file to OSS (async).

        Runs in thread pool to avoid blocking the event loop.

        Args:
            key: Object key (path in bucket).
            file_obj: File-like object to upload.
            content_type: MIME type of the file.

        Returns:
            str: URL of the uploaded file.
        """
        return await asyncio.to_thread(
            self._upload_file_sync, key, file_obj, content_type
        )

    async def upload_bytes(
        self,
        key: str,
        data: bytes,
        content_type: Optional[str] = None,
    ) -> str:
        """
        Upload bytes to OSS (async).

        Args:
            key: Object key (path in bucket).
            data: Bytes to upload.
            content_type: MIME type of the data.

        Returns:
            str: URL of the uploaded file.
        """
        return await self.upload_file(key, BytesIO(data), content_type)

    async def download_file(self, key: str) -> Tuple[bytes, str]:
        """
        Download a file from OSS (async).

        Runs in thread pool to avoid blocking the event loop.

        Args:
            key: Object key.

        Returns:
            Tuple[bytes, str]: File content and content type.
        """
        return await asyncio.to_thread(self._download_file_sync, key)

    async def delete_file(self, key: str) -> bool:
        """
        Delete a file from OSS (async).

        Runs in thread pool to avoid blocking the event loop.

        Args:
            key: Object key.

        Returns:
            bool: True if deleted successfully.
        """
        return await asyncio.to_thread(self._delete_file_sync, key)

    async def file_exists(self, key: str) -> bool:
        """
        Check if a file exists in OSS (async).

        Runs in thread pool to avoid blocking the event loop.

        Args:
            key: Object key.

        Returns:
            bool: True if file exists.
        """
        return await asyncio.to_thread(self._file_exists_sync, key)

    async def get_url(self, key: str, expires: int = 3600) -> str:
        """
        Get a signed URL for a file (async).

        Note: This is fast and could be sync, but wrapped for consistency.

        Args:
            key: Object key.
            expires: URL expiration time in seconds.

        Returns:
            str: Signed URL.
        """
        return await asyncio.to_thread(self._get_url_sync, key, expires)

    async def list_files(
        self,
        prefix: str = "",
        max_keys: int = 100,
    ) -> list:
        """
        List files in the bucket (async).

        Runs in thread pool to avoid blocking the event loop.

        Args:
            prefix: Filter by key prefix.
            max_keys: Maximum number of keys to return.

        Returns:
            list: List of object keys.
        """
        return await asyncio.to_thread(self._list_files_sync, prefix, max_keys)


def get_oss_client() -> OSSClient:
    """
    Get OSS client instance.

    Returns:
        OSSClient: OSS client.
    """
    return OSSClient()
