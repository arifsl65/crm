"""
PostgreSQL connection management for Python AI service.

Uses asyncpg for async connection pooling to the shared Neon database.
"""

from typing import Any, Optional

import asyncpg
import structlog

from internal.config import get_settings

logger = structlog.get_logger(__name__)

# Global connection pool
_pool: Optional[asyncpg.Pool] = None


async def connect() -> asyncpg.Pool:
    """
    Create and return the PostgreSQL connection pool.

    Returns:
        asyncpg.Pool: The connection pool.

    Raises:
        Exception: If connection fails.
    """
    global _pool

    if _pool is not None:
        return _pool

    settings = get_settings()

    dsn = (
        f"postgresql://{settings.postgres_user}:{settings.postgres_password}"
        f"@{settings.postgres_host}:{settings.postgres_port}/{settings.postgres_database}"
    )

    # Add SSL mode for Neon
    if settings.postgres_sslmode == "require":
        dsn += "?sslmode=require"

    try:
        _pool = await asyncpg.create_pool(
            dsn=dsn,
            min_size=settings.postgres_pool_min,
            max_size=settings.postgres_pool_max,
            command_timeout=30,
        )
        logger.info(
            "Connected to PostgreSQL",
            host=settings.postgres_host,
            database=settings.postgres_database,
        )
        return _pool
    except Exception as e:
        logger.error("Failed to connect to PostgreSQL", error=str(e))
        raise


async def disconnect() -> None:
    """Close the PostgreSQL connection pool."""
    global _pool

    if _pool is not None:
        await _pool.close()
        _pool = None
        logger.info("Disconnected from PostgreSQL")


async def get_pool() -> asyncpg.Pool:
    """
    Get the PostgreSQL connection pool.

    Returns:
        asyncpg.Pool: The connection pool.

    Raises:
        RuntimeError: If pool is not initialized.
    """
    if _pool is None:
        raise RuntimeError("PostgreSQL pool is not initialized. Call connect() first.")
    return _pool


async def health_check() -> bool:
    """
    Check if PostgreSQL is healthy.

    Returns:
        bool: True if healthy, False otherwise.
    """
    try:
        pool = await get_pool()
        async with pool.acquire() as conn:
            await conn.fetchval("SELECT 1")
        return True
    except Exception as e:
        logger.error("PostgreSQL health check failed", error=str(e))
        return False


async def set_tenant_context(conn: asyncpg.Connection, tenant_id: str) -> None:
    """
    Set the tenant context for Row Level Security.

    IMPORTANT: This should be called within a transaction block to ensure
    the tenant context is properly scoped and doesn't leak to other queries
    when the connection is returned to the pool.

    The third parameter to set_config (true) makes it transaction-local,
    so it automatically resets when the transaction ends.

    Args:
        conn: The database connection (should be within a transaction).
        tenant_id: The tenant UUID to set as current context.

    Raises:
        ValueError: If tenant_id is not a valid UUID format.
    """
    import re
    # Validate UUID format to prevent SQL injection
    uuid_pattern = re.compile(
        r'^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$',
        re.IGNORECASE
    )
    if not uuid_pattern.match(tenant_id):
        raise ValueError(f"Invalid tenant_id format: {tenant_id}")

    # Use set_config with is_local=true (third param) for transaction-scoped setting
    # This ensures the tenant context doesn't leak to other queries via connection pooling
    await conn.execute(
        "SELECT set_config('app.tenant_id', $1, true)",
        tenant_id
    )


async def fetch_tenant_by_domain(domain: str) -> Optional[dict[str, Any]]:
    """
    Fetch a tenant by domain.

    This bypasses RLS since it's used before tenant context is established.
    Uses a transaction with SET LOCAL to ensure the bypass doesn't leak
    to other queries when the connection is returned to the pool.

    Args:
        domain: The tenant domain.

    Returns:
        The tenant record or None if not found.
    """
    pool = await get_pool()

    async with pool.acquire() as conn:
        # Use transaction to scope the RLS bypass - SET LOCAL is transaction-scoped
        # and automatically resets when the transaction ends, preventing leaks
        async with conn.transaction():
            # Bypass RLS for tenant lookup (LOCAL = transaction-scoped only)
            await conn.execute("SET LOCAL app.tenant_id = ''")

            row = await conn.fetchrow(
                """
                SELECT id, name, domain, status, plan, settings, metadata, created_at, updated_at
                FROM tenants
                WHERE domain = $1 AND deleted_at IS NULL
                """,
                domain,
            )

            if row:
                return dict(row)
            return None


async def fetch_user_by_email(tenant_id: str, email: str) -> Optional[dict[str, Any]]:
    """
    Fetch a user by email within a tenant.

    Uses a transaction to ensure tenant context is properly scoped
    and doesn't leak to other queries via connection pooling.

    Args:
        tenant_id: The tenant UUID.
        email: The user's email address.

    Returns:
        The user record or None if not found.
    """
    pool = await get_pool()

    async with pool.acquire() as conn:
        # Use transaction to scope the tenant context
        async with conn.transaction():
            await set_tenant_context(conn, tenant_id)

            row = await conn.fetchrow(
                """
                SELECT id, tenant_id, email, name, password,
                       role, status, avatar_url, phone, preferences,
                       last_login_at, created_at, updated_at
                FROM users
                WHERE email = $1 AND deleted_at IS NULL
                """,
                email,
            )

            if row:
                return dict(row)
            return None
