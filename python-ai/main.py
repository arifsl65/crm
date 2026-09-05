"""
Accountant CRM - Python AI Service

FastAPI application providing AI-powered document processing endpoints.
"""

import asyncio
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
                    # Convert PDF to images (run in thread pool to avoid blocking event loop)
                    images = await asyncio.to_thread(
                        convert_from_bytes, file_data, dpi=150, first_page=1, last_page=5
                    )
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
# Email AI Endpoints
# =============================================================================


@app.post("/api/v1/ai/emails/summarize", tags=["Email AI"])
async def summarize_email(
    subject: str,
    body: str,
    sender: str = "",
    recipient: str = "",
    email_id: str = "",
    state: AppState = Depends(get_app_state),
) -> Dict[str, Any]:
    """
    Summarize an email for quick review.

    Args:
        subject: Email subject line.
        body: Email body text.
        sender: Sender email/name.
        recipient: Recipient email/name.
        email_id: Optional email ID for reference.

    Returns:
        Summary with key points, action items, and urgency.
    """
    groq_client = get_groq_client()
    if not groq_client.is_configured():
        raise HTTPException(
            status_code=status.HTTP_503_SERVICE_UNAVAILABLE,
            detail="AI service not configured (missing GROQ_API_KEY)",
        )

    if not subject and not body:
        raise HTTPException(
            status_code=status.HTTP_400_BAD_REQUEST,
            detail="Email subject or body is required",
        )

    try:
        result = await groq_client.summarize_email(
            subject=subject,
            body=body,
            sender=sender,
            recipient=recipient,
        )
        if email_id:
            result["email_id"] = email_id
        return result
    except Exception as e:
        logger.error("Email summarization failed", error=str(e))
        raise HTTPException(
            status_code=status.HTTP_500_INTERNAL_SERVER_ERROR,
            detail=f"Email summarization failed: {str(e)}",
        )


@app.post("/api/v1/ai/emails/sentiment", tags=["Email AI"])
async def analyze_email_sentiment(
    subject: str,
    body: str,
    sender: str = "",
    email_id: str = "",
    state: AppState = Depends(get_app_state),
) -> Dict[str, Any]:
    """
    Analyze the sentiment of an email.

    Args:
        subject: Email subject line.
        body: Email body text.
        sender: Sender email/name.
        email_id: Optional email ID for reference.

    Returns:
        Sentiment analysis with score, tone, and risk indicators.
    """
    groq_client = get_groq_client()
    if not groq_client.is_configured():
        raise HTTPException(
            status_code=status.HTTP_503_SERVICE_UNAVAILABLE,
            detail="AI service not configured (missing GROQ_API_KEY)",
        )

    if not subject and not body:
        raise HTTPException(
            status_code=status.HTTP_400_BAD_REQUEST,
            detail="Email subject or body is required",
        )

    try:
        result = await groq_client.analyze_email_sentiment(
            subject=subject,
            body=body,
            sender=sender,
        )
        if email_id:
            result["email_id"] = email_id
        return result
    except Exception as e:
        logger.error("Email sentiment analysis failed", error=str(e))
        raise HTTPException(
            status_code=status.HTTP_500_INTERNAL_SERVER_ERROR,
            detail=f"Sentiment analysis failed: {str(e)}",
        )


@app.post("/api/v1/ai/emails/promises", tags=["Email AI"])
async def extract_email_promises(
    subject: str,
    body: str,
    sender: str = "",
    recipient: str = "",
    email_id: str = "",
    state: AppState = Depends(get_app_state),
) -> Dict[str, Any]:
    """
    Extract promised documents and actions from an email.

    Args:
        subject: Email subject line.
        body: Email body text.
        sender: Sender email/name.
        recipient: Recipient email/name.
        email_id: Optional email ID for reference.

    Returns:
        List of promised documents, actions, and requested items.
    """
    groq_client = get_groq_client()
    if not groq_client.is_configured():
        raise HTTPException(
            status_code=status.HTTP_503_SERVICE_UNAVAILABLE,
            detail="AI service not configured (missing GROQ_API_KEY)",
        )

    if not subject and not body:
        raise HTTPException(
            status_code=status.HTTP_400_BAD_REQUEST,
            detail="Email subject or body is required",
        )

    try:
        result = await groq_client.extract_email_promises(
            subject=subject,
            body=body,
            sender=sender,
            recipient=recipient,
        )
        if email_id:
            result["email_id"] = email_id
        return result
    except Exception as e:
        logger.error("Email promise extraction failed", error=str(e))
        raise HTTPException(
            status_code=status.HTTP_500_INTERNAL_SERVER_ERROR,
            detail=f"Promise extraction failed: {str(e)}",
        )


@app.post("/api/v1/ai/emails/draft", tags=["Email AI"])
async def draft_email(
    context: str,
    tone: str = "professional",
    intent: str = "reply",
    client_name: str = "",
    staff_name: str = "",
    original_subject: str = "",
    original_body: str = "",
    original_sender: str = "",
    additional_instructions: str = "",
    email_id: str = "",
    state: AppState = Depends(get_app_state),
) -> Dict[str, Any]:
    """
    Generate an AI-drafted email reply or new email.

    Args:
        context: Context or instructions for the email.
        tone: Desired tone (professional, friendly, formal, casual).
        intent: Email intent (reply, follow_up, request_documents, chase, thank_you).
        client_name: Name of the client for personalization.
        staff_name: Name of the staff member sending the email.
        original_subject: Subject of original email (for replies).
        original_body: Body of original email (for replies).
        original_sender: Sender of original email (for replies).
        additional_instructions: Any additional instructions for the AI.
        email_id: Optional email ID for reference.

    Returns:
        Draft email with subject, body, and suggestions.
    """
    groq_client = get_groq_client()
    if not groq_client.is_configured():
        raise HTTPException(
            status_code=status.HTTP_503_SERVICE_UNAVAILABLE,
            detail="AI service not configured (missing GROQ_API_KEY)",
        )

    if not context:
        raise HTTPException(
            status_code=status.HTTP_400_BAD_REQUEST,
            detail="Context or instructions are required",
        )

    # Build original email dict if provided
    original_email = None
    if original_body or original_subject:
        original_email = {
            "subject": original_subject,
            "body": original_body,
            "sender": original_sender,
        }

    try:
        result = await groq_client.draft_email(
            context=context,
            original_email=original_email,
            tone=tone,
            intent=intent,
            client_name=client_name,
            staff_name=staff_name,
            additional_instructions=additional_instructions,
        )
        if email_id:
            result["email_id"] = email_id
        return result
    except Exception as e:
        logger.error("Email draft generation failed", error=str(e))
        raise HTTPException(
            status_code=status.HTTP_500_INTERNAL_SERVER_ERROR,
            detail=f"Email draft generation failed: {str(e)}",
        )


@app.post("/api/v1/ai/emails/match-client", tags=["Email AI"])
async def match_email_to_client(
    sender_email: str,
    sender_name: str = "",
    email_content: str = "",
    known_clients: str = "",
    email_id: str = "",
    state: AppState = Depends(get_app_state),
) -> Dict[str, Any]:
    """
    Match an unknown email sender to an existing client.

    Args:
        sender_email: Email address of the sender.
        sender_name: Display name of the sender.
        email_content: Content of the email for context clues.
        known_clients: JSON string of known clients list.
        email_id: Optional email ID for reference.

    Returns:
        Match results with confidence and reasoning.
    """
    groq_client = get_groq_client()
    if not groq_client.is_configured():
        raise HTTPException(
            status_code=status.HTTP_503_SERVICE_UNAVAILABLE,
            detail="AI service not configured (missing GROQ_API_KEY)",
        )

    if not sender_email:
        raise HTTPException(
            status_code=status.HTTP_400_BAD_REQUEST,
            detail="Sender email is required",
        )

    # Parse known clients JSON
    clients_list = []
    if known_clients:
        try:
            clients_list = json.loads(known_clients)
        except json.JSONDecodeError:
            raise HTTPException(
                status_code=status.HTTP_400_BAD_REQUEST,
                detail="Invalid known_clients format (must be JSON array)",
            )

    # Extract domain from email
    email_domain = sender_email.split("@")[-1] if "@" in sender_email else ""

    try:
        result = await groq_client.match_email_to_client(
            sender_email=sender_email,
            sender_name=sender_name,
            email_domain=email_domain,
            email_content=email_content,
            known_clients=clients_list,
        )
        if email_id:
            result["email_id"] = email_id
        return result
    except Exception as e:
        logger.error("Client matching failed", error=str(e))
        raise HTTPException(
            status_code=status.HTTP_500_INTERNAL_SERVER_ERROR,
            detail=f"Client matching failed: {str(e)}",
        )


@app.post("/api/v1/ai/emails/thread-summary", tags=["Email AI"])
async def summarize_email_thread(
    emails: str,
    focus: str = "general",
    thread_id: str = "",
    state: AppState = Depends(get_app_state),
) -> Dict[str, Any]:
    """
    Summarize an email conversation thread.

    Args:
        emails: JSON string of emails in chronological order.
        focus: What to focus on (general, action_items, decisions, timeline, documents).
        thread_id: Optional thread ID for reference.

    Returns:
        Thread summary with key points and action items.
    """
    groq_client = get_groq_client()
    if not groq_client.is_configured():
        raise HTTPException(
            status_code=status.HTTP_503_SERVICE_UNAVAILABLE,
            detail="AI service not configured (missing GROQ_API_KEY)",
        )

    if not emails:
        raise HTTPException(
            status_code=status.HTTP_400_BAD_REQUEST,
            detail="Emails list is required",
        )

    # Parse emails JSON
    try:
        emails_list = json.loads(emails)
    except json.JSONDecodeError:
        raise HTTPException(
            status_code=status.HTTP_400_BAD_REQUEST,
            detail="Invalid emails format (must be JSON array)",
        )

    if not emails_list:
        raise HTTPException(
            status_code=status.HTTP_400_BAD_REQUEST,
            detail="At least one email is required",
        )

    try:
        result = await groq_client.summarize_email_thread(
            emails=emails_list,
            focus=focus,
        )
        if thread_id:
            result["thread_id"] = thread_id
        return result
    except Exception as e:
        logger.error("Thread summarization failed", error=str(e))
        raise HTTPException(
            status_code=status.HTTP_500_INTERNAL_SERVER_ERROR,
            detail=f"Thread summarization failed: {str(e)}",
        )


@app.post("/api/v1/ai/emails/find-alternate", tags=["Email AI"])
async def find_alternate_email(
    bounced_email: str,
    client_name: str = "",
    company_name: str = "",
    known_contacts: str = "",
    company_domain: str = "",
    email_id: str = "",
    state: AppState = Depends(get_app_state),
) -> Dict[str, Any]:
    """
    Suggest alternate email addresses when an email bounces.

    Args:
        bounced_email: The email address that bounced.
        client_name: Name of the client.
        company_name: Name of the company.
        known_contacts: JSON string of known contacts.
        company_domain: Known company email domain.
        email_id: Optional email ID for reference.

    Returns:
        Suggestions for alternate emails and actions.
    """
    groq_client = get_groq_client()
    if not groq_client.is_configured():
        raise HTTPException(
            status_code=status.HTTP_503_SERVICE_UNAVAILABLE,
            detail="AI service not configured (missing GROQ_API_KEY)",
        )

    if not bounced_email:
        raise HTTPException(
            status_code=status.HTTP_400_BAD_REQUEST,
            detail="Bounced email address is required",
        )

    # Parse known contacts JSON
    contacts_list = []
    if known_contacts:
        try:
            contacts_list = json.loads(known_contacts)
        except json.JSONDecodeError:
            raise HTTPException(
                status_code=status.HTTP_400_BAD_REQUEST,
                detail="Invalid known_contacts format (must be JSON array)",
            )

    try:
        result = await groq_client.find_alternate_email(
            bounced_email=bounced_email,
            client_name=client_name,
            company_name=company_name,
            known_contacts=contacts_list,
            company_domain=company_domain,
        )
        if email_id:
            result["email_id"] = email_id
        return result
    except Exception as e:
        logger.error("Alternate email search failed", error=str(e))
        raise HTTPException(
            status_code=status.HTTP_500_INTERNAL_SERVER_ERROR,
            detail=f"Alternate email search failed: {str(e)}",
        )


# =============================================================================
# Risk Analysis AI Endpoints
# =============================================================================


@app.post("/api/v1/ai/risk/client", tags=["Risk Analysis"])
async def analyze_client_risk(
    client_id: str,
    client_name: str = "",
    services: str = "",
    last_contact_days: int = 0,
    outstanding_invoices: int = 0,
    outstanding_amount: float = 0.0,
    email_sentiment_history: str = "",
    missed_deadlines: int = 0,
    payment_delays_avg: int = 0,
    relationship_length_months: int = 0,
    state: AppState = Depends(get_app_state),
) -> Dict[str, Any]:
    """
    Analyze client churn risk based on their data and interactions.

    Args:
        client_id: UUID of the client.
        client_name: Name of the client.
        services: Comma-separated list of active services.
        last_contact_days: Days since last contact.
        outstanding_invoices: Number of unpaid invoices.
        outstanding_amount: Total unpaid amount.
        email_sentiment_history: Comma-separated sentiment values (positive,neutral,negative).
        missed_deadlines: Number of missed deadlines.
        payment_delays_avg: Average payment delay in days.
        relationship_length_months: How long they've been a client.

    Returns:
        Risk analysis with score, factors, and recommendations.
    """
    groq_client = get_groq_client()
    if not groq_client.is_configured():
        raise HTTPException(
            status_code=status.HTTP_503_SERVICE_UNAVAILABLE,
            detail="AI service not configured (missing GROQ_API_KEY)",
        )

    if not client_id:
        raise HTTPException(
            status_code=status.HTTP_400_BAD_REQUEST,
            detail="client_id is required",
        )

    # Build client data from parameters
    client_data = {
        "services": [s.strip() for s in services.split(",") if s.strip()] if services else [],
        "last_contact_days": last_contact_days,
        "outstanding_invoices": outstanding_invoices,
        "outstanding_amount": outstanding_amount,
        "email_sentiment_history": [s.strip() for s in email_sentiment_history.split(",") if s.strip()] if email_sentiment_history else [],
        "missed_deadlines": missed_deadlines,
        "payment_delays_avg": payment_delays_avg,
        "relationship_length_months": relationship_length_months,
    }

    try:
        result = await groq_client.analyze_client_risk(
            client_id=client_id,
            client_name=client_name,
            client_data=client_data,
        )
        result["client_id"] = client_id
        return result
    except Exception as e:
        logger.error("Client risk analysis failed", error=str(e), client_id=client_id)
        raise HTTPException(
            status_code=status.HTTP_500_INTERNAL_SERVER_ERROR,
            detail=f"Client risk analysis failed: {str(e)}",
        )


@app.post("/api/v1/ai/risk/service", tags=["Risk Analysis"])
async def analyze_service_risk(
    service_id: str,
    service_type: str = "",
    client_name: str = "",
    deadline: str = "",
    days_until_deadline: int = 0,
    service_status: str = "",
    documents_received: int = 0,
    documents_required: int = 0,
    outstanding_queries: int = 0,
    assigned_staff: str = "",
    complexity: str = "medium",
    previous_delays: bool = False,
    client_responsiveness: str = "normal",
    state: AppState = Depends(get_app_state),
) -> Dict[str, Any]:
    """
    Analyze service deadline risk and identify potential issues.

    Args:
        service_id: UUID of the service.
        service_type: Type of service (e.g., "VAT Return", "Annual Accounts").
        client_name: Name of the client.
        deadline: Service deadline date (ISO format).
        days_until_deadline: Days remaining until deadline.
        service_status: Current status of the service.
        documents_received: Number of documents received from client.
        documents_required: Total documents required for completion.
        outstanding_queries: Number of pending client queries.
        assigned_staff: Staff member assigned to the service.
        complexity: Service complexity (low/medium/high).
        previous_delays: Whether previous services for this client were delayed.
        client_responsiveness: How responsive the client is (slow/normal/fast).

    Returns:
        Risk analysis with deadline risk, blockers, and recommendations.
    """
    groq_client = get_groq_client()
    if not groq_client.is_configured():
        raise HTTPException(
            status_code=status.HTTP_503_SERVICE_UNAVAILABLE,
            detail="AI service not configured (missing GROQ_API_KEY)",
        )

    if not service_id:
        raise HTTPException(
            status_code=status.HTTP_400_BAD_REQUEST,
            detail="service_id is required",
        )

    # Build service data from parameters
    service_data = {
        "client_name": client_name,
        "deadline": deadline,
        "days_until_deadline": days_until_deadline,
        "status": service_status,
        "documents_received": documents_received,
        "documents_required": documents_required,
        "outstanding_queries": outstanding_queries,
        "assigned_staff": assigned_staff,
        "complexity": complexity,
        "previous_delays": previous_delays,
        "client_responsiveness": client_responsiveness,
    }

    try:
        result = await groq_client.analyze_service_risk(
            service_id=service_id,
            service_type=service_type,
            service_data=service_data,
        )
        result["service_id"] = service_id
        return result
    except Exception as e:
        logger.error("Service risk analysis failed", error=str(e), service_id=service_id)
        raise HTTPException(
            status_code=status.HTTP_500_INTERNAL_SERVER_ERROR,
            detail=f"Service risk analysis failed: {str(e)}",
        )


# =============================================================================
# Form Auto-Fill AI Endpoints
# =============================================================================


@app.post("/api/v1/ai/forms/vat", tags=["Form Auto-Fill"])
async def auto_fill_vat(
    client_id: str,
    period: str,
    client_name: str = "",
    vat_number: str = "",
    total_sales: float = 0.0,
    total_purchases: float = 0.0,
    vat_on_sales: float = 0.0,
    vat_on_purchases: float = 0.0,
    eu_acquisitions: float = 0.0,
    eu_supplies: float = 0.0,
    state: AppState = Depends(get_app_state),
) -> Dict[str, Any]:
    """
    Auto-fill VAT return data based on client financial information.

    Args:
        client_id: UUID of the client.
        period: VAT period (e.g., "Q1-2026").
        client_name: Name of the client.
        vat_number: VAT registration number.
        total_sales: Total sales value excluding VAT.
        total_purchases: Total purchases value excluding VAT.
        vat_on_sales: VAT collected on sales.
        vat_on_purchases: VAT paid on purchases.
        eu_acquisitions: Value of EU acquisitions.
        eu_supplies: Value of EU supplies.

    Returns:
        Pre-filled VAT return boxes with calculations.
    """
    groq_client = get_groq_client()
    if not groq_client.is_configured():
        raise HTTPException(
            status_code=status.HTTP_503_SERVICE_UNAVAILABLE,
            detail="AI service not configured (missing GROQ_API_KEY)",
        )

    if not client_id or not period:
        raise HTTPException(
            status_code=status.HTTP_400_BAD_REQUEST,
            detail="client_id and period are required",
        )

    client_data = {
        "client_name": client_name,
        "vat_number": vat_number,
        "total_sales": total_sales,
        "total_purchases": total_purchases,
        "vat_on_sales": vat_on_sales,
        "vat_on_purchases": vat_on_purchases,
        "eu_acquisitions": eu_acquisitions,
        "eu_supplies": eu_supplies,
    }

    try:
        result = await groq_client.auto_fill_vat(
            client_id=client_id,
            period=period,
            client_data=client_data,
        )
        result["client_id"] = client_id
        result["period"] = period
        return result
    except Exception as e:
        logger.error("VAT auto-fill failed", error=str(e), client_id=client_id)
        raise HTTPException(
            status_code=status.HTTP_500_INTERNAL_SERVER_ERROR,
            detail=f"VAT auto-fill failed: {str(e)}",
        )


@app.post("/api/v1/ai/forms/ct600", tags=["Form Auto-Fill"])
async def auto_fill_ct600(
    client_id: str,
    year: int,
    company_name: str = "",
    company_number: str = "",
    utr: str = "",
    turnover: float = 0.0,
    cost_of_sales: float = 0.0,
    gross_profit: float = 0.0,
    admin_expenses: float = 0.0,
    depreciation: float = 0.0,
    interest_received: float = 0.0,
    interest_paid: float = 0.0,
    other_income: float = 0.0,
    state: AppState = Depends(get_app_state),
) -> Dict[str, Any]:
    """
    Auto-fill CT600 Corporation Tax return data.

    Args:
        client_id: UUID of the client.
        year: Accounting year end (e.g., 2026).
        company_name: Name of the company.
        company_number: Companies House number.
        utr: Unique Taxpayer Reference.
        turnover: Total turnover/revenue.
        cost_of_sales: Cost of sales.
        gross_profit: Gross profit.
        admin_expenses: Administrative expenses.
        depreciation: Depreciation (for add-back).
        interest_received: Interest received.
        interest_paid: Interest paid.
        other_income: Other income.

    Returns:
        Pre-filled CT600 fields with tax calculations.
    """
    groq_client = get_groq_client()
    if not groq_client.is_configured():
        raise HTTPException(
            status_code=status.HTTP_503_SERVICE_UNAVAILABLE,
            detail="AI service not configured (missing GROQ_API_KEY)",
        )

    if not client_id or not year:
        raise HTTPException(
            status_code=status.HTTP_400_BAD_REQUEST,
            detail="client_id and year are required",
        )

    client_data = {
        "company_name": company_name,
        "company_number": company_number,
        "utr": utr,
        "accounts": {
            "turnover": turnover,
            "cost_of_sales": cost_of_sales,
            "gross_profit": gross_profit,
            "admin_expenses": admin_expenses,
            "depreciation": depreciation,
            "interest_received": interest_received,
            "interest_paid": interest_paid,
            "other_income": other_income,
        },
    }

    try:
        result = await groq_client.auto_fill_ct600(
            client_id=client_id,
            year=year,
            client_data=client_data,
        )
        result["client_id"] = client_id
        result["year"] = year
        return result
    except Exception as e:
        logger.error("CT600 auto-fill failed", error=str(e), client_id=client_id)
        raise HTTPException(
            status_code=status.HTTP_500_INTERNAL_SERVER_ERROR,
            detail=f"CT600 auto-fill failed: {str(e)}",
        )


@app.post("/api/v1/ai/forms/sa", tags=["Form Auto-Fill"])
async def auto_fill_sa(
    client_id: str,
    tax_year: str,
    taxpayer_name: str = "",
    utr: str = "",
    ni_number: str = "",
    employment_income: float = 0.0,
    self_employment_income: float = 0.0,
    self_employment_expenses: float = 0.0,
    property_income: float = 0.0,
    property_expenses: float = 0.0,
    dividend_income: float = 0.0,
    interest_income: float = 0.0,
    pension_contributions: float = 0.0,
    gift_aid: float = 0.0,
    state: AppState = Depends(get_app_state),
) -> Dict[str, Any]:
    """
    Auto-fill Self Assessment tax return data.

    Args:
        client_id: UUID of the client.
        tax_year: Tax year (e.g., "2025-26").
        taxpayer_name: Name of the taxpayer.
        utr: Unique Taxpayer Reference.
        ni_number: National Insurance number.
        employment_income: Employment income (from P60).
        self_employment_income: Self-employment income.
        self_employment_expenses: Self-employment expenses.
        property_income: Property/rental income.
        property_expenses: Property expenses.
        dividend_income: Dividend income.
        interest_income: Interest income.
        pension_contributions: Pension contributions (for relief).
        gift_aid: Gift Aid donations (for relief).

    Returns:
        Pre-filled Self Assessment fields with tax calculations.
    """
    groq_client = get_groq_client()
    if not groq_client.is_configured():
        raise HTTPException(
            status_code=status.HTTP_503_SERVICE_UNAVAILABLE,
            detail="AI service not configured (missing GROQ_API_KEY)",
        )

    if not client_id or not tax_year:
        raise HTTPException(
            status_code=status.HTTP_400_BAD_REQUEST,
            detail="client_id and tax_year are required",
        )

    client_data = {
        "taxpayer_name": taxpayer_name,
        "utr": utr,
        "ni_number": ni_number,
        "employment_income": employment_income,
        "self_employment": {
            "income": self_employment_income,
            "expenses": self_employment_expenses,
            "profit": self_employment_income - self_employment_expenses,
        },
        "property_income": {
            "income": property_income,
            "expenses": property_expenses,
            "profit": property_income - property_expenses,
        },
        "dividend_income": dividend_income,
        "interest_income": interest_income,
        "pension_contributions": pension_contributions,
        "gift_aid": gift_aid,
    }

    try:
        result = await groq_client.auto_fill_sa(
            client_id=client_id,
            tax_year=tax_year,
            client_data=client_data,
        )
        result["client_id"] = client_id
        result["tax_year"] = tax_year
        return result
    except Exception as e:
        logger.error("SA auto-fill failed", error=str(e), client_id=client_id)
        raise HTTPException(
            status_code=status.HTTP_500_INTERNAL_SERVER_ERROR,
            detail=f"Self Assessment auto-fill failed: {str(e)}",
        )


# =============================================================================
# Document Rename AI Endpoint
# =============================================================================


@app.post("/api/v1/ai/documents/rename", tags=["Documents"])
async def suggest_document_name(
    text: str,
    original_filename: str = "",
    document_type: str = "",
    client_name: str = "",
    file_key: str = "",
    state: AppState = Depends(get_app_state),
) -> Dict[str, Any]:
    """
    Suggest a descriptive name for a document based on its content.

    Args:
        text: Extracted text from the document.
        original_filename: Original filename for reference.
        document_type: Type of document if known.
        client_name: Client name if known.
        file_key: OSS file key for reference.

    Returns:
        Suggested filename with alternatives and metadata.
    """
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
        result = await groq_client.suggest_document_name(
            text=text,
            original_filename=original_filename,
            document_type=document_type,
            client_name=client_name,
        )
        if file_key:
            result["file_key"] = file_key
        return result
    except Exception as e:
        logger.error("Document rename failed", error=str(e))
        raise HTTPException(
            status_code=status.HTTP_500_INTERNAL_SERVER_ERROR,
            detail=f"Document rename failed: {str(e)}",
        )


# =============================================================================
# Chat History Endpoints (MongoDB)
# =============================================================================

from datetime import datetime, timezone


@app.get("/api/v1/ai/chat/history", tags=["Chat"])
async def get_chat_history(
    user_id: str,
    tenant_id: str = "",
    limit: int = 50,
    offset: int = 0,
    state: AppState = Depends(get_app_state),
) -> Dict[str, Any]:
    """
    Get chat history for a user.

    Args:
        user_id: UUID of the user.
        tenant_id: UUID of the tenant (optional filter).
        limit: Maximum number of conversations to return.
        offset: Number of conversations to skip.

    Returns:
        List of chat conversations with messages.
    """
    if not user_id:
        raise HTTPException(
            status_code=status.HTTP_400_BAD_REQUEST,
            detail="user_id is required",
        )

    if state.mongodb_database is None:
        raise HTTPException(
            status_code=status.HTTP_503_SERVICE_UNAVAILABLE,
            detail="MongoDB not available",
        )

    try:
        collection = state.mongodb_database["ai_conversations"]

        # Build query filter
        query: Dict[str, Any] = {"user_id": user_id}
        if tenant_id:
            query["tenant_id"] = tenant_id

        # Get conversations sorted by last updated
        cursor = collection.find(query).sort("updated_at", -1).skip(offset).limit(limit)
        conversations = await cursor.to_list(length=limit)

        # Get total count
        total = await collection.count_documents(query)

        # Convert ObjectId to string for JSON serialization
        for conv in conversations:
            conv["_id"] = str(conv["_id"])

        return {
            "conversations": conversations,
            "total": total,
            "limit": limit,
            "offset": offset,
            "has_more": (offset + len(conversations)) < total,
        }

    except Exception as e:
        logger.error("Failed to get chat history", error=str(e), user_id=user_id)
        raise HTTPException(
            status_code=status.HTTP_500_INTERNAL_SERVER_ERROR,
            detail=f"Failed to get chat history: {str(e)}",
        )


@app.post("/api/v1/ai/chat/history", tags=["Chat"])
async def save_chat_message(
    user_id: str,
    conversation_id: str = "",
    role: str = "user",
    content: str = "",
    tenant_id: str = "",
    metadata: str = "",
    state: AppState = Depends(get_app_state),
) -> Dict[str, Any]:
    """
    Save a chat message to history.

    Args:
        user_id: UUID of the user.
        conversation_id: UUID of the conversation (creates new if empty).
        role: Message role (user/assistant/system).
        content: Message content.
        tenant_id: UUID of the tenant.
        metadata: JSON string of additional metadata.

    Returns:
        Saved conversation with message.
    """
    if not user_id or not content:
        raise HTTPException(
            status_code=status.HTTP_400_BAD_REQUEST,
            detail="user_id and content are required",
        )

    if state.mongodb_database is None:
        raise HTTPException(
            status_code=status.HTTP_503_SERVICE_UNAVAILABLE,
            detail="MongoDB not available",
        )

    try:
        collection = state.mongodb_database["ai_conversations"]
        now = datetime.now(timezone.utc)

        # Parse metadata if provided
        meta = {}
        if metadata:
            try:
                meta = json.loads(metadata)
            except json.JSONDecodeError:
                pass

        message = {
            "role": role,
            "content": content,
            "timestamp": now.isoformat(),
            "metadata": meta,
        }

        if conversation_id:
            # Add message to existing conversation
            result = await collection.update_one(
                {"conversation_id": conversation_id, "user_id": user_id},
                {
                    "$push": {"messages": message},
                    "$set": {"updated_at": now.isoformat()},
                },
            )

            if result.modified_count == 0:
                raise HTTPException(
                    status_code=status.HTTP_404_NOT_FOUND,
                    detail="Conversation not found",
                )

            return {
                "conversation_id": conversation_id,
                "message_added": True,
                "updated_at": now.isoformat(),
            }

        else:
            # Create new conversation
            import uuid
            new_conversation_id = str(uuid.uuid4())

            conversation = {
                "conversation_id": new_conversation_id,
                "user_id": user_id,
                "tenant_id": tenant_id,
                "messages": [message],
                "created_at": now.isoformat(),
                "updated_at": now.isoformat(),
            }

            await collection.insert_one(conversation)

            return {
                "conversation_id": new_conversation_id,
                "created": True,
                "created_at": now.isoformat(),
            }

    except HTTPException:
        raise
    except Exception as e:
        logger.error("Failed to save chat message", error=str(e), user_id=user_id)
        raise HTTPException(
            status_code=status.HTTP_500_INTERNAL_SERVER_ERROR,
            detail=f"Failed to save chat message: {str(e)}",
        )


@app.delete("/api/v1/ai/chat/{conversation_id}", tags=["Chat"])
async def delete_chat(
    conversation_id: str,
    user_id: str,
    state: AppState = Depends(get_app_state),
) -> Dict[str, Any]:
    """
    Delete a chat conversation.

    Args:
        conversation_id: UUID of the conversation to delete.
        user_id: UUID of the user (for authorization).

    Returns:
        Deletion confirmation.
    """
    if not conversation_id or not user_id:
        raise HTTPException(
            status_code=status.HTTP_400_BAD_REQUEST,
            detail="conversation_id and user_id are required",
        )

    if state.mongodb_database is None:
        raise HTTPException(
            status_code=status.HTTP_503_SERVICE_UNAVAILABLE,
            detail="MongoDB not available",
        )

    try:
        collection = state.mongodb_database["ai_conversations"]

        # Delete conversation (only if owned by user)
        result = await collection.delete_one({
            "conversation_id": conversation_id,
            "user_id": user_id,
        })

        if result.deleted_count == 0:
            raise HTTPException(
                status_code=status.HTTP_404_NOT_FOUND,
                detail="Conversation not found or not owned by user",
            )

        logger.info(
            "Chat conversation deleted",
            conversation_id=conversation_id,
            user_id=user_id,
        )

        return {
            "conversation_id": conversation_id,
            "deleted": True,
        }

    except HTTPException:
        raise
    except Exception as e:
        logger.error("Failed to delete chat", error=str(e), conversation_id=conversation_id)
        raise HTTPException(
            status_code=status.HTTP_500_INTERNAL_SERVER_ERROR,
            detail=f"Failed to delete chat: {str(e)}",
        )


# =============================================================================
# Template AI Endpoints
# =============================================================================


@app.post("/api/v1/ai/templates/generate", tags=["Template AI"])
async def generate_email_template(
    purpose: str,
    template_type: str = "general",
    tone: str = "professional",
    include_placeholders: bool = True,
    example_context: str = "",
    state: AppState = Depends(get_app_state),
) -> Dict[str, Any]:
    """
    Generate an email template using AI.

    Args:
        purpose: Purpose/description of the template.
        template_type: Type (document_request, chase, welcome, reminder, etc.).
        tone: Desired tone (professional, friendly, formal, casual).
        include_placeholders: Whether to include merge placeholders.
        example_context: Optional example context for better template.

    Returns:
        Generated template with subject, body, and placeholders.
    """
    groq_client = get_groq_client()
    if not groq_client.is_configured():
        raise HTTPException(
            status_code=status.HTTP_503_SERVICE_UNAVAILABLE,
            detail="AI service not configured (missing GROQ_API_KEY)",
        )

    if not purpose:
        raise HTTPException(
            status_code=status.HTTP_400_BAD_REQUEST,
            detail="Template purpose is required",
        )

    try:
        result = await groq_client.generate_email_template(
            purpose=purpose,
            template_type=template_type,
            tone=tone,
            include_placeholders=include_placeholders,
            example_context=example_context,
        )
        return result
    except Exception as e:
        logger.error("Template generation failed", error=str(e))
        raise HTTPException(
            status_code=status.HTTP_500_INTERNAL_SERVER_ERROR,
            detail=f"Template generation failed: {str(e)}",
        )


# =============================================================================
# Client AI Endpoints
# =============================================================================


@app.post("/api/v1/ai/clients/duplicate-check", tags=["Client AI"])
async def check_duplicate_clients(
    new_client: str,
    existing_clients: str = "",
    state: AppState = Depends(get_app_state),
) -> Dict[str, Any]:
    """
    Check if a new client is a duplicate of existing clients.

    Args:
        new_client: JSON string of new client data.
        existing_clients: JSON string of existing clients list.

    Returns:
        Duplicate analysis with potential matches.
    """
    groq_client = get_groq_client()
    if not groq_client.is_configured():
        raise HTTPException(
            status_code=status.HTTP_503_SERVICE_UNAVAILABLE,
            detail="AI service not configured (missing GROQ_API_KEY)",
        )

    if not new_client:
        raise HTTPException(
            status_code=status.HTTP_400_BAD_REQUEST,
            detail="New client data is required",
        )

    # Parse JSON inputs
    try:
        new_client_data = json.loads(new_client)
    except json.JSONDecodeError:
        raise HTTPException(
            status_code=status.HTTP_400_BAD_REQUEST,
            detail="Invalid new_client format (must be JSON object)",
        )

    existing_clients_list = []
    if existing_clients:
        try:
            existing_clients_list = json.loads(existing_clients)
        except json.JSONDecodeError:
            raise HTTPException(
                status_code=status.HTTP_400_BAD_REQUEST,
                detail="Invalid existing_clients format (must be JSON array)",
            )

    try:
        result = await groq_client.check_duplicate_clients(
            new_client=new_client_data,
            existing_clients=existing_clients_list,
        )
        return result
    except Exception as e:
        logger.error("Duplicate check failed", error=str(e))
        raise HTTPException(
            status_code=status.HTTP_500_INTERNAL_SERVER_ERROR,
            detail=f"Duplicate check failed: {str(e)}",
        )


# =============================================================================
# Service AI Endpoints
# =============================================================================


@app.post("/api/v1/ai/services/auto-name", tags=["Service AI"])
async def auto_name_service(
    service_type: str,
    client_name: str,
    period: str = "",
    year: str = "",
    additional_context: str = "",
    state: AppState = Depends(get_app_state),
) -> Dict[str, Any]:
    """
    Auto-generate a service name based on type and client.

    Args:
        service_type: Type of service (VAT Return, Annual Accounts, etc.).
        client_name: Name of the client.
        period: Relevant period (Q1, Q2, etc.).
        year: Tax/financial year.
        additional_context: Any additional context.

    Returns:
        Suggested service name and variations.
    """
    groq_client = get_groq_client()
    if not groq_client.is_configured():
        raise HTTPException(
            status_code=status.HTTP_503_SERVICE_UNAVAILABLE,
            detail="AI service not configured (missing GROQ_API_KEY)",
        )

    if not service_type or not client_name:
        raise HTTPException(
            status_code=status.HTTP_400_BAD_REQUEST,
            detail="service_type and client_name are required",
        )

    try:
        result = await groq_client.auto_name_service(
            service_type=service_type,
            client_name=client_name,
            period=period,
            year=year,
            additional_context=additional_context,
        )
        return result
    except Exception as e:
        logger.error("Service auto-naming failed", error=str(e))
        raise HTTPException(
            status_code=status.HTTP_500_INTERNAL_SERVER_ERROR,
            detail=f"Service auto-naming failed: {str(e)}",
        )


@app.post("/api/v1/ai/services/completion-summary", tags=["Service AI"])
async def generate_completion_summary(
    service_type: str,
    client_name: str,
    completion_data: str = "",
    service_id: str = "",
    state: AppState = Depends(get_app_state),
) -> Dict[str, Any]:
    """
    Generate a completion summary for a finished service.

    Args:
        service_type: Type of service completed.
        client_name: Name of the client.
        completion_data: JSON string of completion data.
        service_id: Optional service ID for reference.

    Returns:
        Summary suitable for client communication and internal records.
    """
    groq_client = get_groq_client()
    if not groq_client.is_configured():
        raise HTTPException(
            status_code=status.HTTP_503_SERVICE_UNAVAILABLE,
            detail="AI service not configured (missing GROQ_API_KEY)",
        )

    if not service_type or not client_name:
        raise HTTPException(
            status_code=status.HTTP_400_BAD_REQUEST,
            detail="service_type and client_name are required",
        )

    # Parse completion data
    completion_data_dict = {}
    if completion_data:
        try:
            completion_data_dict = json.loads(completion_data)
        except json.JSONDecodeError:
            raise HTTPException(
                status_code=status.HTTP_400_BAD_REQUEST,
                detail="Invalid completion_data format (must be JSON object)",
            )

    try:
        result = await groq_client.generate_completion_summary(
            service_type=service_type,
            client_name=client_name,
            completion_data=completion_data_dict,
        )
        if service_id:
            result["service_id"] = service_id
        return result
    except Exception as e:
        logger.error("Completion summary generation failed", error=str(e))
        raise HTTPException(
            status_code=status.HTTP_500_INTERNAL_SERVER_ERROR,
            detail=f"Completion summary generation failed: {str(e)}",
        )


# =============================================================================
# Dashboard AI Endpoints
# =============================================================================


@app.post("/api/v1/ai/dashboard/troublemakers", tags=["Dashboard AI"])
async def find_troublemakers(
    clients: str,
    threshold_days_overdue: int = 7,
    state: AppState = Depends(get_app_state),
) -> Dict[str, Any]:
    """
    Identify problematic clients who need attention.

    Args:
        clients: JSON string of clients with their metrics.
        threshold_days_overdue: Days before considered overdue.

    Returns:
        Ranked list of troublemakers with recommended actions.
    """
    groq_client = get_groq_client()
    if not groq_client.is_configured():
        raise HTTPException(
            status_code=status.HTTP_503_SERVICE_UNAVAILABLE,
            detail="AI service not configured (missing GROQ_API_KEY)",
        )

    if not clients:
        raise HTTPException(
            status_code=status.HTTP_400_BAD_REQUEST,
            detail="Clients data is required",
        )

    # Parse clients JSON
    try:
        clients_list = json.loads(clients)
    except json.JSONDecodeError:
        raise HTTPException(
            status_code=status.HTTP_400_BAD_REQUEST,
            detail="Invalid clients format (must be JSON array)",
        )

    try:
        result = await groq_client.find_troublemakers(
            clients=clients_list,
            threshold_days_overdue=threshold_days_overdue,
        )
        return result
    except Exception as e:
        logger.error("Troublemaker analysis failed", error=str(e))
        raise HTTPException(
            status_code=status.HTTP_500_INTERNAL_SERVER_ERROR,
            detail=f"Troublemaker analysis failed: {str(e)}",
        )


@app.post("/api/v1/ai/dashboard/anomalies", tags=["Dashboard AI"])
async def detect_anomalies(
    data_type: str,
    data: str,
    context: str = "",
    state: AppState = Depends(get_app_state),
) -> Dict[str, Any]:
    """
    Detect anomalies in various types of data.

    Args:
        data_type: Type of data (client_financials, service_metrics, etc.).
        data: JSON string of data records to analyze.
        context: Additional context about what to look for.

    Returns:
        Detected anomalies with explanations.
    """
    groq_client = get_groq_client()
    if not groq_client.is_configured():
        raise HTTPException(
            status_code=status.HTTP_503_SERVICE_UNAVAILABLE,
            detail="AI service not configured (missing GROQ_API_KEY)",
        )

    if not data_type or not data:
        raise HTTPException(
            status_code=status.HTTP_400_BAD_REQUEST,
            detail="data_type and data are required",
        )

    # Parse data JSON
    try:
        data_list = json.loads(data)
    except json.JSONDecodeError:
        raise HTTPException(
            status_code=status.HTTP_400_BAD_REQUEST,
            detail="Invalid data format (must be JSON array)",
        )

    try:
        result = await groq_client.detect_anomalies(
            data_type=data_type,
            data=data_list,
            context=context,
        )
        return result
    except Exception as e:
        logger.error("Anomaly detection failed", error=str(e))
        raise HTTPException(
            status_code=status.HTTP_500_INTERNAL_SERVER_ERROR,
            detail=f"Anomaly detection failed: {str(e)}",
        )


@app.post("/api/v1/ai/staff/activity", tags=["Staff AI"])
async def analyze_staff_activity(
    staff_id: str,
    staff_name: str,
    activity_data: str = "",
    period: str = "last_week",
    state: AppState = Depends(get_app_state),
) -> Dict[str, Any]:
    """
    Generate a staff activity report.

    Args:
        staff_id: UUID of the staff member.
        staff_name: Name of the staff member.
        activity_data: JSON string of activity metrics.
        period: Time period (last_week, last_month, custom).

    Returns:
        Comprehensive activity report with insights.
    """
    groq_client = get_groq_client()
    if not groq_client.is_configured():
        raise HTTPException(
            status_code=status.HTTP_503_SERVICE_UNAVAILABLE,
            detail="AI service not configured (missing GROQ_API_KEY)",
        )

    if not staff_id or not staff_name:
        raise HTTPException(
            status_code=status.HTTP_400_BAD_REQUEST,
            detail="staff_id and staff_name are required",
        )

    # Parse activity data
    activity_data_dict = {}
    if activity_data:
        try:
            activity_data_dict = json.loads(activity_data)
        except json.JSONDecodeError:
            raise HTTPException(
                status_code=status.HTTP_400_BAD_REQUEST,
                detail="Invalid activity_data format (must be JSON object)",
            )

    try:
        result = await groq_client.analyze_staff_activity(
            staff_id=staff_id,
            staff_name=staff_name,
            activity_data=activity_data_dict,
            period=period,
        )
        return result
    except Exception as e:
        logger.error("Staff activity analysis failed", error=str(e))
        raise HTTPException(
            status_code=status.HTTP_500_INTERNAL_SERVER_ERROR,
            detail=f"Staff activity analysis failed: {str(e)}",
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
