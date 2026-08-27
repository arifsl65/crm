"""
FastAPI Dependency Injection for database connections.

Provides testable, properly scoped database connections via FastAPI's Depends() system.
This replaces global mutable state with a proper DI pattern.
"""

from typing import Optional

import asyncpg
import redis.asyncio as redis
import oss2
import structlog
from motor.motor_asyncio import AsyncIOMotorClient, AsyncIOMotorDatabase
from fastapi import Request

from .config import get_settings, Settings

logger = structlog.get_logger(__name__)


# =============================================================================
# Application State Container
# =============================================================================

class AppState:
    """
    Centralized application state container.

    Holds all connection pools and clients. Initialized during app lifespan
    and accessed via FastAPI's app.state or dependency injection.
    """

    def __init__(self):
        self.postgres_pool: Optional[asyncpg.Pool] = None
        self.mongodb_client: Optional[AsyncIOMotorClient] = None
        self.mongodb_database: Optional[AsyncIOMotorDatabase] = None
        self.oss_bucket: Optional[oss2.Bucket] = None
        self.redis_client: Optional[redis.Redis] = None
        self._settings: Optional[Settings] = None

    @property
    def settings(self) -> Settings:
        """Get cached settings."""
        if self._settings is None:
            self._settings = get_settings()
        return self._settings

    # =========================================================================
    # PostgreSQL
    # =========================================================================

    async def connect_postgres(self) -> asyncpg.Pool:
        """Create PostgreSQL connection pool."""
        if self.postgres_pool is not None:
            return self.postgres_pool

        settings = self.settings
        dsn = (
            f"postgresql://{settings.postgres_user}:{settings.postgres_password}"
            f"@{settings.postgres_host}:{settings.postgres_port}/{settings.postgres_database}"
        )

        if settings.postgres_sslmode == "require":
            dsn += "?sslmode=require"

        self.postgres_pool = await asyncpg.create_pool(
            dsn=dsn,
            min_size=settings.postgres_pool_min,
            max_size=settings.postgres_pool_max,
            command_timeout=30,
        )

        logger.info(
            "Connected to PostgreSQL",
            host=settings.postgres_host,
            database=settings.postgres_database,
            pool_min=settings.postgres_pool_min,
            pool_max=settings.postgres_pool_max,
        )

        return self.postgres_pool

    async def disconnect_postgres(self) -> None:
        """Close PostgreSQL connection pool."""
        if self.postgres_pool is not None:
            await self.postgres_pool.close()
            self.postgres_pool = None
            logger.info("Disconnected from PostgreSQL")

    async def postgres_health_check(self) -> bool:
        """Check PostgreSQL health."""
        if self.postgres_pool is None:
            return False
        try:
            async with self.postgres_pool.acquire() as conn:
                await conn.fetchval("SELECT 1")
            return True
        except Exception as e:
            logger.error("PostgreSQL health check failed", error=str(e))
            return False

    # =========================================================================
    # MongoDB
    # =========================================================================

    async def connect_mongodb(self) -> AsyncIOMotorDatabase:
        """Connect to MongoDB Atlas."""
        if self.mongodb_database is not None:
            return self.mongodb_database

        settings = self.settings

        self.mongodb_client = AsyncIOMotorClient(
            settings.mongodb_uri,
            serverSelectionTimeoutMS=5000,
            connectTimeoutMS=10000,
            socketTimeoutMS=30000,  # Fix #16: Added socket timeout
            maxPoolSize=50,
            minPoolSize=10,
        )

        # Verify connection
        await self.mongodb_client.admin.command("ping")

        self.mongodb_database = self.mongodb_client[settings.mongodb_database]

        logger.info(
            "Connected to MongoDB",
            database=settings.mongodb_database,
        )

        return self.mongodb_database

    async def disconnect_mongodb(self) -> None:
        """Close MongoDB connection."""
        if self.mongodb_client is not None:
            self.mongodb_client.close()
            self.mongodb_client = None
            self.mongodb_database = None
            logger.info("Disconnected from MongoDB")

    async def mongodb_health_check(self) -> bool:
        """Check MongoDB health."""
        if self.mongodb_client is None:
            return False
        try:
            await self.mongodb_client.admin.command("ping")
            return True
        except Exception as e:
            logger.error("MongoDB health check failed", error=str(e))
            return False

    # =========================================================================
    # OSS (Object Storage Service)
    # =========================================================================

    async def connect_oss(self) -> oss2.Bucket:
        """
        Connect to Alibaba Cloud OSS.

        Note: oss2 is synchronous, so we run blocking calls in a thread pool
        to avoid blocking the async event loop.
        """
        import asyncio

        if self.oss_bucket is not None:
            return self.oss_bucket

        settings = self.settings

        # Use V4 signature (V1 is deprecated)
        auth = oss2.AuthV4(
            settings.alibaba_access_key_id,
            settings.alibaba_access_key_secret,
        )

        self.oss_bucket = oss2.Bucket(
            auth,
            settings.oss_endpoint,
            settings.oss_bucket_uploads,
            region=settings.alibaba_region,
        )

        # Verify connection - run in thread pool to avoid blocking event loop
        await asyncio.to_thread(self.oss_bucket.get_bucket_info)

        logger.info(
            "Connected to OSS",
            endpoint=settings.oss_endpoint,
            bucket=settings.oss_bucket_uploads,
        )

        return self.oss_bucket

    async def oss_health_check(self) -> bool:
        """
        Check OSS health.

        Runs the synchronous OSS call in a thread pool to avoid blocking
        the async event loop.
        """
        import asyncio

        if self.oss_bucket is None:
            return False
        try:
            await asyncio.to_thread(self.oss_bucket.get_bucket_info)
            return True
        except Exception as e:
            logger.error("OSS health check failed", error=str(e))
            return False

    # =========================================================================
    # Redis
    # =========================================================================

    async def connect_redis(self) -> redis.Redis:
        """Connect to Redis."""
        if self.redis_client is not None:
            return self.redis_client

        settings = self.settings

        self.redis_client = redis.Redis(
            host=settings.redis_host,
            port=settings.redis_port,
            password=settings.redis_password or None,
            db=settings.redis_db,
            decode_responses=True,
            socket_timeout=30,
            socket_connect_timeout=10,
        )

        # Verify connection
        await self.redis_client.ping()

        logger.info(
            "Connected to Redis",
            host=settings.redis_host,
            port=settings.redis_port,
        )

        return self.redis_client

    async def disconnect_redis(self) -> None:
        """Close Redis connection."""
        if self.redis_client is not None:
            await self.redis_client.close()
            self.redis_client = None
            logger.info("Disconnected from Redis")

    async def redis_health_check(self) -> bool:
        """Check Redis health."""
        if self.redis_client is None:
            return False
        try:
            await self.redis_client.ping()
            return True
        except Exception as e:
            logger.error("Redis health check failed", error=str(e))
            return False

    # =========================================================================
    # Lifecycle Management
    # =========================================================================

    async def startup(self, fail_on_error: bool = True) -> None:
        """
        Initialize all connections.

        Args:
            fail_on_error: If True, raise exceptions on connection failure.
                          If False, log errors and continue (useful for dev).
        """
        errors = []

        try:
            await self.connect_mongodb()
        except Exception as e:
            errors.append(f"MongoDB: {e}")
            if fail_on_error:
                raise

        try:
            await self.connect_oss()
        except Exception as e:
            errors.append(f"OSS: {e}")
            if fail_on_error:
                raise

        try:
            await self.connect_postgres()
        except Exception as e:
            errors.append(f"PostgreSQL: {e}")
            if fail_on_error:
                raise

        try:
            await self.connect_redis()
        except Exception as e:
            errors.append(f"Redis: {e}")
            if fail_on_error:
                # In production, Redis is required for feature flags and rate limiting
                raise
            # In development, Redis failure is non-fatal
            logger.warning("Redis connection failed, feature flags will use defaults", error=str(e))

        if errors and not fail_on_error:
            logger.warning("Some connections failed during startup", errors=errors)

    async def shutdown(self) -> None:
        """Close all connections gracefully."""
        await self.disconnect_redis()
        await self.disconnect_mongodb()
        await self.disconnect_postgres()
        # OSS doesn't need explicit disconnect


# =============================================================================
# Dependency Injection Functions
# =============================================================================

def get_app_state(request: Request) -> AppState:
    """
    Get the application state from the request.

    Usage:
        @app.get("/")
        async def endpoint(state: AppState = Depends(get_app_state)):
            pool = state.postgres_pool
    """
    return request.app.state.app_state


async def get_postgres_pool(request: Request) -> asyncpg.Pool:
    """
    Get PostgreSQL connection pool.

    Usage:
        @app.get("/")
        async def endpoint(pool: asyncpg.Pool = Depends(get_postgres_pool)):
            async with pool.acquire() as conn:
                ...
    """
    state: AppState = request.app.state.app_state
    if state.postgres_pool is None:
        raise RuntimeError("PostgreSQL pool not initialized")
    return state.postgres_pool


async def get_mongodb_database(request: Request) -> AsyncIOMotorDatabase:
    """
    Get MongoDB database.

    Usage:
        @app.get("/")
        async def endpoint(db: AsyncIOMotorDatabase = Depends(get_mongodb_database)):
            collection = db["documents"]
    """
    state: AppState = request.app.state.app_state
    if state.mongodb_database is None:
        raise RuntimeError("MongoDB not initialized")
    return state.mongodb_database


async def get_oss_bucket(request: Request) -> oss2.Bucket:
    """
    Get OSS bucket.

    Usage:
        @app.get("/")
        async def endpoint(bucket: oss2.Bucket = Depends(get_oss_bucket)):
            bucket.put_object(...)
    """
    state: AppState = request.app.state.app_state
    if state.oss_bucket is None:
        raise RuntimeError("OSS not initialized")
    return state.oss_bucket


async def get_redis_client(request: Request) -> Optional[redis.Redis]:
    """
    Get Redis client (optional, may be None).

    Usage:
        @app.get("/")
        async def endpoint(redis_client: Optional[redis.Redis] = Depends(get_redis_client)):
            if redis_client:
                await redis_client.get("key")
    """
    state: AppState = request.app.state.app_state
    return state.redis_client
