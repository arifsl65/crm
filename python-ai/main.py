"""
Accountant CRM - Python AI Service

FastAPI application providing AI-powered document processing endpoints.
"""

import io
import sys
from contextlib import asynccontextmanager
import json
from typing import Any, Dict, List, Optional

import structlog
import re

from fastapi import FastAPI, HTTPException, Request, Response, status, Depends, UploadFile, File
from fastapi.middleware.cors import CORSMiddleware
from fastapi.responses import JSONResponse
from starlette.middleware.base import BaseHTTPMiddleware

from internal.config import get_settings
from internal.dependencies import AppState, get_app_state
from internal.ai_client import get_groq_client, GroqClient

# PDF and image processing imports
try:
    from pypdf import PdfReader
    PDF_SUPPORT = True
except ImportError:
    PDF_SUPPORT = False

try:
    from PIL import Image
    from pdf2image import convert_from_bytes
    IMAGE_SUPPORT = True
except ImportError:
    IMAGE_SUPPORT = False

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
    response: Response,
    state: AppState = Depends(get_app_state),
    file_key: Optional[str] = None,
    file: Optional[UploadFile] = File(None),
) -> Dict[str, Any]:
    """
    Extract text from a document using OCR.

    Supports two modes:
    1. file_key: OSS key of document (requires OSS configured)
    2. file: Direct file upload (multipart/form-data)

    Args:
        file_key: OSS key of the document to process (optional).
        file: Uploaded file (optional).

    Returns:
        Extracted text and metadata.
    """
    settings = get_settings()

    # Check feature flag
    if not await check_feature_flag(state, "ocr"):
        response.status_code = status.HTTP_503_SERVICE_UNAVAILABLE
        response.headers["Retry-After"] = "300"
        return {"error": "service_unavailable", "message": "OCR feature is currently disabled"}

    # Validate input - need either file_key or file upload
    if not file_key and not file:
        raise HTTPException(
            status_code=status.HTTP_400_BAD_REQUEST,
            detail="Either file_key or file upload is required",
        )

    file_data: bytes = b""
    filename: str = ""
    mime_type: str = ""

    # Mode 1: File upload (preferred - no OSS dependency)
    if file:
        try:
            file_data = await file.read()
            filename = file.filename or "unknown"
            mime_type = file.content_type or "application/octet-stream"
            logger.info("Processing uploaded file", filename=filename, size=len(file_data))
        except Exception as e:
            logger.error("Failed to read uploaded file", error=str(e))
            raise HTTPException(
                status_code=status.HTTP_400_BAD_REQUEST,
                detail=f"Failed to read uploaded file: {str(e)}",
            )

    # Mode 2: OSS file key
    elif file_key:
        # Validate file_key to prevent path traversal
        if not validate_oss_key(file_key):
            raise HTTPException(
                status_code=status.HTTP_400_BAD_REQUEST,
                detail="Invalid file_key format",
            )

        # Check if OSS is configured
        if not settings.oss_configured:
            raise HTTPException(
                status_code=status.HTTP_503_SERVICE_UNAVAILABLE,
                detail="OSS storage not configured. Use direct file upload instead.",
            )

        # Download from OSS
        try:
            import oss2
            # Use AuthV2 signature (V1 is disabled on this bucket)
            auth = oss2.AuthV2(settings.alibaba_access_key_id, settings.alibaba_access_key_secret)
            bucket = oss2.Bucket(auth, settings.oss_endpoint, settings.oss_bucket_uploads)
            result = bucket.get_object(file_key)
            file_data = result.read()
            filename = file_key.split("/")[-1]
            # Detect mime type from extension
            if filename.lower().endswith(".pdf"):
                mime_type = "application/pdf"
            elif filename.lower().endswith((".png",)):
                mime_type = "image/png"
            elif filename.lower().endswith((".jpg", ".jpeg")):
                mime_type = "image/jpeg"
            else:
                mime_type = "application/octet-stream"
            logger.info("Downloaded file from OSS", file_key=file_key, size=len(file_data))
        except Exception as e:
            logger.error("Failed to download from OSS", file_key=file_key, error=str(e))
            raise HTTPException(
                status_code=status.HTTP_404_NOT_FOUND,
                detail=f"Failed to download file from storage: {str(e)}",
            )

    # Extract text based on file type
    extracted_text = ""
    extraction_method = ""
    metadata: Dict[str, Any] = {}

    try:
        # PDF Processing
        if mime_type == "application/pdf" or filename.lower().endswith(".pdf"):
            if not PDF_SUPPORT:
                raise HTTPException(
                    status_code=status.HTTP_503_SERVICE_UNAVAILABLE,
                    detail="PDF processing not available. Install pypdf.",
                )

            # Try to extract embedded text first (fast, no API cost)
            try:
                pdf_reader = PdfReader(io.BytesIO(file_data))
                text_parts = []
                for page in pdf_reader.pages:
                    page_text = page.extract_text() or ""
                    text_parts.append(page_text)
                extracted_text = "\n\n".join(text_parts)
                metadata["page_count"] = len(pdf_reader.pages)

                # Check if we got meaningful text
                if len(extracted_text.strip()) > 50:
                    extraction_method = "pdf_text"
                    logger.info(
                        "Extracted text from PDF",
                        pages=len(pdf_reader.pages),
                        text_length=len(extracted_text),
                    )
                else:
                    # Scanned PDF - need OCR via vision model
                    extracted_text = ""  # Reset for vision extraction
            except Exception as e:
                logger.warning("PDF text extraction failed, trying OCR", error=str(e))

            # If no text extracted, use vision model (scanned PDF)
            if not extracted_text and IMAGE_SUPPORT:
                extraction_method = "vision_ocr"
                try:
                    # Convert PDF to images
                    images = convert_from_bytes(file_data, dpi=150, first_page=1, last_page=5)
                    all_text = []

                    groq_client = get_groq_client()
                    for i, img in enumerate(images):
                        # Convert PIL Image to bytes
                        img_buffer = io.BytesIO()
                        img.save(img_buffer, format="PNG")
                        img_bytes = img_buffer.getvalue()

                        # Extract text from image using vision model
                        result = await groq_client.extract_text_from_image(
                            img_bytes, "image/png", f"{filename}_page_{i+1}"
                        )
                        if result.get("text"):
                            all_text.append(f"--- Page {i+1} ---\n{result['text']}")

                    extracted_text = "\n\n".join(all_text)
                    metadata["page_count"] = len(images)
                    metadata["ocr_pages_processed"] = len(images)
                    logger.info(
                        "Extracted text from scanned PDF via OCR",
                        pages=len(images),
                        text_length=len(extracted_text),
                    )
                except Exception as e:
                    logger.error("PDF OCR failed", error=str(e))
                    raise HTTPException(
                        status_code=status.HTTP_500_INTERNAL_SERVER_ERROR,
                        detail=f"PDF OCR processing failed: {str(e)}",
                    )

        # Image Processing (PNG, JPEG, etc.)
        elif mime_type.startswith("image/") or filename.lower().endswith((".png", ".jpg", ".jpeg", ".gif", ".webp")):
            extraction_method = "vision_ocr"
            groq_client = get_groq_client()

            if not groq_client.is_configured():
                raise HTTPException(
                    status_code=status.HTTP_503_SERVICE_UNAVAILABLE,
                    detail="AI service not configured (GROQ_API_KEY missing)",
                )

            result = await groq_client.extract_text_from_image(file_data, mime_type, filename)
            extracted_text = result.get("text", "")
            metadata.update({
                "language": result.get("language"),
                "document_type_hint": result.get("document_type_hint"),
                "has_tables": result.get("has_tables", False),
                "has_handwriting": result.get("has_handwriting", False),
                "confidence": result.get("confidence", 0.0),
                "page_count": 1,
            })
            logger.info(
                "Extracted text from image via OCR",
                text_length=len(extracted_text),
                confidence=metadata.get("confidence"),
            )

        else:
            raise HTTPException(
                status_code=status.HTTP_400_BAD_REQUEST,
                detail=f"Unsupported file type: {mime_type}. Supported: PDF, PNG, JPEG",
            )

    except HTTPException:
        raise
    except Exception as e:
        logger.error("Text extraction failed", error=str(e), filename=filename)
        raise HTTPException(
            status_code=status.HTTP_500_INTERNAL_SERVER_ERROR,
            detail=f"Text extraction failed: {str(e)}",
        )

    return {
        "status": "success",
        "text": extracted_text,
        "text_length": len(extracted_text),
        "extraction_method": extraction_method,
        "filename": filename,
        "mime_type": mime_type,
        "metadata": metadata,
    }


@app.post("/api/v1/ai/documents/classify", tags=["Documents"])
async def classify_document(
    file_key: str,
    text: str = "",
    response: Response = None,
    state: AppState = Depends(get_app_state),
) -> Dict[str, Any]:
    """
    Classify a document type using Groq AI.

    Args:
        file_key: OSS key of the document to classify.
        text: Extracted text from the document (if already available).

    Returns:
        Document classification result with type, confidence, and metadata.
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

    # Check if Groq is configured
    groq_client = get_groq_client()
    if not groq_client.is_configured():
        raise HTTPException(
            status_code=status.HTTP_503_SERVICE_UNAVAILABLE,
            detail="AI service not configured (missing GROQ_API_KEY)",
        )

    # If no text provided, we need to extract it first (OCR)
    if not text:
        return {
            "status": "text_required",
            "message": "Document text must be provided. Use /api/v1/ai/documents/extract first.",
            "file_key": file_key,
        }

    try:
        result = await groq_client.classify_document(text=text, filename=file_key)
        result["file_key"] = file_key
        return result
    except Exception as e:
        logger.error("Classification failed", file_key=file_key, error=str(e))
        raise HTTPException(
            status_code=status.HTTP_500_INTERNAL_SERVER_ERROR,
            detail=f"Classification failed: {str(e)}",
        )


@app.post("/api/v1/ai/chat", tags=["Chat"])
async def chat_completion(
    message: str,
    context: Optional[str] = None,
    response: Response = None,
    state: AppState = Depends(get_app_state),
) -> Dict[str, Any]:
    """
    Process a chat message with Groq AI.

    Args:
        message: User message.
        context: Optional JSON string of previous messages for context.

    Returns:
        AI response with usage statistics.
    """
    if not await check_feature_flag(state, "chat"):
        response.status_code = status.HTTP_503_SERVICE_UNAVAILABLE
        response.headers["Retry-After"] = "300"
        return {"error": "service_unavailable", "message": "Chat feature is currently disabled"}

    # Check if Groq is configured
    groq_client = get_groq_client()
    if not groq_client.is_configured():
        raise HTTPException(
            status_code=status.HTTP_503_SERVICE_UNAVAILABLE,
            detail="AI service not configured (missing GROQ_API_KEY)",
        )

    # Parse context if provided
    context_messages = None
    if context:
        try:
            context_messages = json.loads(context)
        except json.JSONDecodeError:
            raise HTTPException(
                status_code=status.HTTP_400_BAD_REQUEST,
                detail="Invalid context format (must be JSON array)",
            )

    try:
        result = await groq_client.chat_completion(
            message=message,
            context=context_messages,
        )
        return result
    except Exception as e:
        logger.error("Chat completion failed", error=str(e))
        raise HTTPException(
            status_code=status.HTTP_500_INTERNAL_SERVER_ERROR,
            detail=f"Chat completion failed: {str(e)}",
        )


@app.post("/api/v1/ai/forms/extract", tags=["Forms"])
async def extract_form_data(
    file_key: str,
    text: str = "",
    document_type: str = "unknown",
    response: Response = None,
    state: AppState = Depends(get_app_state),
) -> Dict[str, Any]:
    """
    Extract structured form data from a document using Groq AI.

    Args:
        file_key: OSS key of the document to process.
        text: Extracted text from the document.
        document_type: Type of document for better extraction.

    Returns:
        Extracted form fields as structured data.
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

    # Check if Groq is configured
    groq_client = get_groq_client()
    if not groq_client.is_configured():
        raise HTTPException(
            status_code=status.HTTP_503_SERVICE_UNAVAILABLE,
            detail="AI service not configured (missing GROQ_API_KEY)",
        )

    # If no text provided, we need to extract it first (OCR)
    if not text:
        return {
            "status": "text_required",
            "message": "Document text must be provided. Use /api/v1/ai/documents/extract first.",
            "file_key": file_key,
        }

    try:
        result = await groq_client.extract_form_data(
            text=text,
            document_type=document_type,
        )
        result["file_key"] = file_key
        return result
    except Exception as e:
        logger.error("Form extraction failed", file_key=file_key, error=str(e))
        raise HTTPException(
            status_code=status.HTTP_500_INTERNAL_SERVER_ERROR,
            detail=f"Form extraction failed: {str(e)}",
        )


@app.post("/api/v1/ai/documents/summarize", tags=["Documents"])
async def summarize_document(
    text: str,
    file_key: str = "",
    response: Response = None,
    state: AppState = Depends(get_app_state),
) -> Dict[str, Any]:
    """
    Generate a summary of a document using Groq AI.

    Args:
        text: Document text to summarize.
        file_key: Optional OSS key for reference.

    Returns:
        Summary with key points, financial highlights, and action items.
    """
    if file_key and not validate_oss_key(file_key):
        raise HTTPException(
            status_code=status.HTTP_400_BAD_REQUEST,
            detail="Invalid file_key format",
        )

    # Check if Groq is configured
    groq_client = get_groq_client()
    if not groq_client.is_configured():
        raise HTTPException(
            status_code=status.HTTP_503_SERVICE_UNAVAILABLE,
            detail="AI service not configured (missing GROQ_API_KEY)",
        )

    if not text:
        raise HTTPException(
            status_code=status.HTTP_400_BAD_REQUEST,
            detail="Document text is required",
        )

    try:
        result = await groq_client.summarize_document(text=text)
        if file_key:
            result["file_key"] = file_key
        return result
    except Exception as e:
        logger.error("Document summarization failed", error=str(e))
        raise HTTPException(
            status_code=status.HTTP_500_INTERNAL_SERVER_ERROR,
            detail=f"Summarization failed: {str(e)}",
        )


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
