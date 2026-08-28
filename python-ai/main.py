"""
Accountant CRM - Python AI Service

FastAPI application providing AI-powered document processing endpoints.
"""

import sys
from contextlib import asynccontextmanager
from typing import Any, Dict, Optional

import structlog
import re

from fastapi import FastAPI, HTTPException, Request, Response, status, Depends
from fastapi.middleware.cors import CORSMiddleware
from fastapi.responses import JSONResponse
from starlette.middleware.base import BaseHTTPMiddleware

from internal.config import get_settings
from internal.dependencies import AppState, get_app_state

# Configure structured logging with contextvars support for request ID propagation
structlog.configure(
    processors=[
        structlog.contextvars.merge_contextvars,  # Fix #20: Merge request_id from context
        structlog.stdlib.filter_by_level,
        structlog.stdlib.add_logger_name,
        structlog.stdlib.add_log_level,
        structlog.stdlib.PositionalArgumentsFormatter(),
        structlog.processors.TimeStamper(fmt="iso"),
        structlog.processors.StackInfoRenderer(),
        structlog.processors.format_exc_info,
        structlog.processors.UnicodeDecoder(),
        structlog.processors.JSONRenderer(),
    ],
    wrapper_class=structlog.make_filtering_bound_logger(0),
    logger_factory=structlog.stdlib.LoggerFactory(),
    cache_logger_on_first_use=True,
)

logger = structlog.get_logger(__name__)


# =============================================================================
# Request ID Middleware (Fix #20)
# =============================================================================

import uuid


class RequestIDMiddleware(BaseHTTPMiddleware):
    """
    Add X-Request-ID to all requests and responses for distributed tracing.

    - Accepts X-Request-ID from incoming request headers (e.g., from ALB or Go backend)
    - Generates a new UUID if not present
    - Adds X-Request-ID to response headers
    - Binds request_id to structlog context for all logs in this request
    """

    async def dispatch(self, request: Request, call_next):
        # Get or generate request ID
        request_id = request.headers.get("X-Request-ID")
        if not request_id:
            request_id = str(uuid.uuid4())

        # Store in request state for access in handlers
        request.state.request_id = request_id

        # Bind to structlog context for this request
        # Note: bind_contextvars() returns dict, not context manager - use try/finally
        structlog.contextvars.bind_contextvars(request_id=request_id)
        try:
            response = await call_next(request)
        finally:
            structlog.contextvars.unbind_contextvars("request_id")

        # Add to response headers
        response.headers["X-Request-ID"] = request_id

        return response


# =============================================================================
# Security Headers Middleware
# =============================================================================

class SecurityHeadersMiddleware(BaseHTTPMiddleware):
    """Add security headers to all responses."""

    async def dispatch(self, request: Request, call_next):
        response = await call_next(request)
        # Security headers
        response.headers["X-Content-Type-Options"] = "nosniff"
        response.headers["X-Frame-Options"] = "DENY"
        response.headers["X-XSS-Protection"] = "1; mode=block"
        response.headers["Referrer-Policy"] = "strict-origin-when-cross-origin"
        # Only add HSTS in production with HTTPS
        if get_settings().app_env == "production":
            response.headers["Strict-Transport-Security"] = "max-age=31536000; includeSubDomains"
        return response


# =============================================================================
# Input Validation Helpers
# =============================================================================

# Valid OSS key pattern: alphanumeric, hyphens, underscores, forward slashes, dots
# No ".." or absolute paths allowed
OSS_KEY_PATTERN = re.compile(r'^[a-zA-Z0-9][a-zA-Z0-9_\-./]*$')


def validate_oss_key(file_key: str) -> bool:
    """
    Validate OSS file key to prevent path traversal attacks.

    Args:
        file_key: The OSS key to validate.

    Returns:
        True if valid, False otherwise.
    """
    if not file_key or len(file_key) > 1024:
        return False
    if ".." in file_key:  # Path traversal attempt
        return False
    if file_key.startswith("/"):  # Absolute path not allowed
        return False
    return bool(OSS_KEY_PATTERN.match(file_key))


async def check_feature_flag(state: AppState, flag: str) -> bool:
    """
    Check if a feature flag is enabled.

    Args:
        state: Application state with Redis client.
        flag: Feature flag name (e.g., "ocr", "chat", "forms").

    Returns:
        bool: True if enabled.
    """
    try:
        if state.redis_client is None:
            raise RuntimeError("Redis not connected")
        value = await state.redis_client.get(f"ai:{flag}:enabled")
        return value == "true"
    except Exception as e:
        # Fix #18: Log the exception for debugging instead of swallowing it
        logger.debug(
            "Feature flag check failed, using config default",
            flag=flag,
            error=str(e),
        )
        settings = get_settings()
        return getattr(settings, f"ai_{flag}_enabled", False)


@asynccontextmanager
async def lifespan(app: FastAPI):
    """Application lifespan manager using dependency injection."""
    settings = get_settings()

    logger.info(
        "Starting AI service",
        app=settings.app_name,
        env=settings.app_env,
    )

    # Initialize centralized application state
    app_state = AppState()
    app.state.app_state = app_state

    # Connect to all services
    # Fix #17: OSS connection is now wrapped in async-compatible method
    # Staging should mirror production - fail on connection errors in both
    try:
        await app_state.startup(fail_on_error=(settings.app_env in ("production", "staging")))
    except Exception as e:
        logger.error("Failed to start services", error=str(e))
        sys.exit(1)

    yield

    # Cleanup all connections
    await app_state.shutdown()
    logger.info("AI service stopped")


# Create FastAPI application
app = FastAPI(
    title="Accountant CRM - AI Service",
    description="AI-powered document processing for accounting workflows",
    version="1.0.0",
    lifespan=lifespan,
)

# =============================================================================
# CORS Configuration (must be at module level for FastAPI middleware)
# =============================================================================
# Note: This runs at import time, which is intentional. FastAPI middleware
# must be configured before the app starts accepting requests. Structlog is
# already configured above, so logging works properly.
_settings = get_settings()
_cors_origins = _settings.get_cors_origins()

# Fix #14: Require explicit CORS origins in production AND staging
if _settings.app_env in ("production", "staging") and not _cors_origins:
    logger.error("CORS_ALLOWED_ORIGINS must be set in production/staging")
    sys.exit(1)

# Development fallback (only for local development)
if not _cors_origins:
    if _settings.app_env == "development":
        _cors_origins = ["http://localhost:3000", "http://localhost:8080"]
        logger.warning("Using development CORS origins", origins=_cors_origins)
    else:
        logger.error("CORS_ALLOWED_ORIGINS not set for environment", env=_settings.app_env)
        sys.exit(1)

app.add_middleware(
    CORSMiddleware,
    allow_origins=_cors_origins,
    allow_credentials=True,
    allow_methods=["GET", "POST", "PUT", "DELETE", "OPTIONS"],
    allow_headers=["Authorization", "Content-Type", "X-Request-ID", "X-Tenant-ID"],
)

# Add security headers middleware
app.add_middleware(SecurityHeadersMiddleware)

# Add request ID middleware (Fix #20: X-Request-ID propagation)
app.add_middleware(RequestIDMiddleware)


# =============================================================================
# Health Endpoints
# =============================================================================


@app.get("/health", tags=["Health"])
async def health() -> Dict[str, str]:
    """
    Liveness probe endpoint.

    Returns immediately with OK status.
    """
    return {"status": "ok"}


@app.get("/ready", tags=["Health"])
async def ready(state: AppState = Depends(get_app_state)) -> Dict[str, Any]:
    """
    Readiness probe endpoint.

    Checks all external dependencies using dependency-injected state.
    """
    response: Dict[str, str] = {}
    all_healthy = True

    # Check MongoDB
    if await state.mongodb_health_check():
        response["mongodb"] = "ok"
    else:
        response["mongodb"] = "error"
        all_healthy = False

    # Check PostgreSQL
    if await state.postgres_health_check():
        response["postgres"] = "ok"
    else:
        response["postgres"] = "error"
        all_healthy = False

    # Check OSS (async to avoid blocking event loop)
    # Returns None if OSS is not configured (skip)
    oss_status = await state.oss_health_check()
    if oss_status is None:
        response["oss"] = "disabled"  # Not configured, don't affect health
    elif oss_status:
        response["oss"] = "ok"
    else:
        response["oss"] = "error"
        all_healthy = False

    # Check Redis
    if await state.redis_health_check():
        response["redis"] = "ok"
    else:
        response["redis"] = "error"
        all_healthy = False

    if not all_healthy:
        return JSONResponse(
            status_code=status.HTTP_503_SERVICE_UNAVAILABLE,
            content=response,
        )

    return response


# =============================================================================
# AI Feature Endpoints (Stubs)
# =============================================================================


@app.post("/api/v1/ai/documents/extract", tags=["Documents"])
async def extract_text(
    file_key: str,
    response: Response,
    state: AppState = Depends(get_app_state),
) -> Dict[str, Any]:
    """
    Extract text from a document using OCR.

    Args:
        file_key: OSS key of the document to process.

    Returns:
        Extracted text and metadata.
    """
    # FIXED: Validate file_key to prevent path traversal
    if not validate_oss_key(file_key):
        raise HTTPException(
            status_code=status.HTTP_400_BAD_REQUEST,
            detail="Invalid file_key format",
        )

    if not await check_feature_flag(state, "ocr"):
        response.status_code = status.HTTP_503_SERVICE_UNAVAILABLE
        response.headers["Retry-After"] = "300"
        return {"error": "service_unavailable", "message": "OCR feature is currently disabled"}

    # TODO: Implement OCR extraction
    return {
        "status": "not_implemented",
        "message": "OCR extraction will be implemented in Week 3",
        "file_key": file_key,
    }


@app.post("/api/v1/ai/documents/classify", tags=["Documents"])
async def classify_document(
    file_key: str,
    response: Response,
    state: AppState = Depends(get_app_state),
) -> Dict[str, Any]:
    """
    Classify a document type.

    Args:
        file_key: OSS key of the document to classify.

    Returns:
        Document classification result.
    """
    # FIXED: Validate file_key to prevent path traversal
    if not validate_oss_key(file_key):
        raise HTTPException(
            status_code=status.HTTP_400_BAD_REQUEST,
            detail="Invalid file_key format",
        )

    if not await check_feature_flag(state, "classification"):
        response.status_code = status.HTTP_503_SERVICE_UNAVAILABLE
        response.headers["Retry-After"] = "300"
        return {"error": "service_unavailable", "message": "Classification feature is currently disabled"}

    # TODO: Implement document classification
    return {
        "status": "not_implemented",
        "message": "Document classification will be implemented in Week 3",
        "file_key": file_key,
    }


@app.post("/api/v1/ai/chat", tags=["Chat"])
async def chat_completion(
    message: str,
    response: Response,
    state: AppState = Depends(get_app_state),
) -> Dict[str, Any]:
    """
    Process a chat message with AI.

    Args:
        message: User message.

    Returns:
        AI response.
    """
    if not await check_feature_flag(state, "chat"):
        response.status_code = status.HTTP_503_SERVICE_UNAVAILABLE
        response.headers["Retry-After"] = "300"
        return {"error": "service_unavailable", "message": "Chat feature is currently disabled"}

    # TODO: Implement chat completion
    return {
        "status": "not_implemented",
        "message": "Chat completion will be implemented in Week 4",
        "input": message,
    }


@app.post("/api/v1/ai/forms/extract", tags=["Forms"])
async def extract_form_data(
    file_key: str,
    response: Response,
    state: AppState = Depends(get_app_state),
) -> Dict[str, Any]:
    """
    Extract structured form data from a document.

    Args:
        file_key: OSS key of the document to process.

    Returns:
        Extracted form fields.
    """
    # FIXED: Validate file_key to prevent path traversal
    if not validate_oss_key(file_key):
        raise HTTPException(
            status_code=status.HTTP_400_BAD_REQUEST,
            detail="Invalid file_key format",
        )

    if not await check_feature_flag(state, "forms"):
        response.status_code = status.HTTP_503_SERVICE_UNAVAILABLE
        response.headers["Retry-After"] = "300"
        return {"error": "service_unavailable", "message": "Form extraction feature is currently disabled"}

    # TODO: Implement form extraction
    return {
        "status": "not_implemented",
        "message": "Form extraction will be implemented in Week 3",
        "file_key": file_key,
    }


# =============================================================================
# Error Handlers
# =============================================================================


@app.exception_handler(Exception)
async def global_exception_handler(request, exc: Exception):
    """Handle uncaught exceptions."""
    logger.error(
        "Unhandled exception",
        path=request.url.path,
        method=request.method,
        error=str(exc),
    )
    return JSONResponse(
        status_code=status.HTTP_500_INTERNAL_SERVER_ERROR,
        content={
            "error": "internal_server_error",
            "message": "An unexpected error occurred",
        },
    )


# =============================================================================
# Main Entry Point
# =============================================================================

if __name__ == "__main__":
    import uvicorn

    settings = get_settings()

    # Build uvicorn config with optional SSL/mTLS
    uvicorn_config = {
        "app": "main:app",
        "host": settings.host,
        "port": settings.port,
        "reload": settings.app_env == "development",
        "workers": 1 if settings.app_env == "development" else settings.workers,
    }

    # Enable SSL/mTLS if configured
    if settings.mtls_enabled:
        uvicorn_config["ssl_keyfile"] = settings.mtls_server_key
        uvicorn_config["ssl_certfile"] = settings.mtls_server_cert
        uvicorn_config["ssl_ca_certs"] = settings.mtls_ca_cert
        # Require client certificate for mTLS
        uvicorn_config["ssl_cert_reqs"] = 2  # ssl.CERT_REQUIRED

    uvicorn.run(**uvicorn_config)
