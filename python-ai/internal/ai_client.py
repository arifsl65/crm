"""
Groq AI Client for document processing.

Provides AI-powered document analysis using Groq's fast inference API.
"""

import base64
import json
from typing import Any, Dict, List, Optional

import structlog
from groq import Groq, AsyncGroq

from .config import get_settings

logger = structlog.get_logger(__name__)


class GroqClient:
    """Groq AI client for document processing tasks."""

    def __init__(self):
        """Initialize Groq client with settings."""
        self._client: Optional[AsyncGroq] = None
        self._sync_client: Optional[Groq] = None

    @property
    def client(self) -> AsyncGroq:
        """Get or create async Groq client."""
        if self._client is None:
            settings = get_settings()
            if not settings.groq_api_key:
                raise ValueError("GROQ_API_KEY not configured")
            self._client = AsyncGroq(api_key=settings.groq_api_key)
        return self._client

    @property
    def sync_client(self) -> Groq:
        """Get or create sync Groq client."""
        if self._sync_client is None:
            settings = get_settings()
            if not settings.groq_api_key:
                raise ValueError("GROQ_API_KEY not configured")
            self._sync_client = Groq(api_key=settings.groq_api_key)
        return self._sync_client

    def is_configured(self) -> bool:
        """Check if Groq is properly configured."""
        settings = get_settings()
        return bool(settings.groq_api_key)

    async def classify_document(self, text: str, filename: str = "") -> Dict[str, Any]:
        """
        Classify a document based on its text content.

        Args:
            text: Extracted text from the document.
            filename: Original filename for additional context.

        Returns:
            Classification result with document_type, confidence, and metadata.
        """
        settings = get_settings()

        system_prompt = """You are a document classification expert for an accounting firm.
Analyze the document text and classify it into one of these categories:
- invoice: Sales invoices, purchase invoices, bills
- receipt: Payment receipts, purchase receipts
- bank_statement: Bank account statements, transaction records
- tax_document: Tax returns, tax forms, tax certificates
- contract: Legal contracts, agreements, service agreements
- id_document: Passports, ID cards, driver's licenses
- payroll: Payslips, salary documents, P60/P45
- expense_report: Expense claims, travel expenses
- financial_statement: Balance sheets, P&L statements, annual accounts
- correspondence: Letters, emails, general correspondence
- other: Documents that don't fit other categories

Respond with JSON only:
{
    "document_type": "category_name",
    "confidence": 0.95,
    "subcategory": "optional specific type",
    "key_entities": ["company names", "dates", "amounts found"],
    "summary": "One sentence summary of the document"
}"""

        user_prompt = f"Filename: {filename}\n\nDocument text:\n{text[:8000]}"  # Limit text length

        try:
            response = await self.client.chat.completions.create(
                model=settings.groq_model,
                messages=[
                    {"role": "system", "content": system_prompt},
                    {"role": "user", "content": user_prompt},
                ],
                max_tokens=1024,
                temperature=settings.groq_temperature,
                response_format={"type": "json_object"},
            )

            result_text = response.choices[0].message.content
            result = json.loads(result_text)

            logger.info(
                "Document classified",
                document_type=result.get("document_type"),
                confidence=result.get("confidence"),
            )

            return result

        except json.JSONDecodeError as e:
            logger.error("Failed to parse classification response", error=str(e))
            return {
                "document_type": "other",
                "confidence": 0.0,
                "error": "Failed to parse AI response",
            }
        except Exception as e:
            logger.error("Document classification failed", error=str(e))
            raise

    async def extract_form_data(
        self, text: str, document_type: str = "unknown"
    ) -> Dict[str, Any]:
        """
        Extract structured data from a document.

        Args:
            text: Extracted text from the document.
            document_type: Type of document for context.

        Returns:
            Extracted fields as key-value pairs.
        """
        settings = get_settings()

        system_prompt = f"""You are a document data extraction expert for an accounting firm.
Extract structured data from this {document_type} document.

For invoices, extract: invoice_number, date, due_date, vendor_name, customer_name,
line_items (description, quantity, unit_price, total), subtotal, tax_amount, total_amount, currency

For receipts, extract: receipt_number, date, vendor_name, items, total_amount, payment_method, currency

For bank statements, extract: account_number, statement_period, opening_balance,
closing_balance, transactions (date, description, amount, type)

For tax documents, extract: tax_year, form_type, taxpayer_name, reference_number, key_amounts

For ID documents, extract: document_type, full_name, date_of_birth, document_number,
expiry_date, nationality, address

Respond with JSON containing the extracted fields. Use null for fields not found.
Include a "confidence" field (0-1) and "extracted_fields_count" integer."""

        user_prompt = f"Document text:\n{text[:8000]}"

        try:
            response = await self.client.chat.completions.create(
                model=settings.groq_model,
                messages=[
                    {"role": "system", "content": system_prompt},
                    {"role": "user", "content": user_prompt},
                ],
                max_tokens=settings.groq_max_tokens,
                temperature=settings.groq_temperature,
                response_format={"type": "json_object"},
            )

            result_text = response.choices[0].message.content
            result = json.loads(result_text)

            logger.info(
                "Form data extracted",
                document_type=document_type,
                fields_count=result.get("extracted_fields_count", 0),
            )

            return result

        except json.JSONDecodeError as e:
            logger.error("Failed to parse extraction response", error=str(e))
            return {"error": "Failed to parse AI response", "raw_response": str(e)}
        except Exception as e:
            logger.error("Form data extraction failed", error=str(e))
            raise

    async def chat_completion(
        self,
        message: str,
        context: Optional[List[Dict[str, str]]] = None,
        system_prompt: Optional[str] = None,
    ) -> Dict[str, Any]:
        """
        Process a chat message with AI.

        Args:
            message: User message.
            context: Previous conversation messages.
            system_prompt: Custom system prompt.

        Returns:
            AI response with the message content.
        """
        settings = get_settings()

        default_system = """You are a helpful AI assistant for an accounting firm CRM system.
You help staff with:
- Understanding document contents
- Answering questions about financial documents
- Explaining accounting concepts
- Helping with data entry and form completion

Be concise, accurate, and professional. If you're unsure about something, say so.
Do not make up financial figures or legal advice."""

        messages = [{"role": "system", "content": system_prompt or default_system}]

        if context:
            messages.extend(context)

        messages.append({"role": "user", "content": message})

        try:
            response = await self.client.chat.completions.create(
                model=settings.groq_model,
                messages=messages,
                max_tokens=settings.groq_max_tokens,
                temperature=0.7,  # Slightly higher for chat
            )

            result = response.choices[0].message.content

            logger.info(
                "Chat completion generated",
                message_length=len(message),
                response_length=len(result),
            )

            return {
                "response": result,
                "model": settings.groq_model,
                "usage": {
                    "prompt_tokens": response.usage.prompt_tokens,
                    "completion_tokens": response.usage.completion_tokens,
                    "total_tokens": response.usage.total_tokens,
                },
            }

        except Exception as e:
            logger.error("Chat completion failed", error=str(e))
            raise

    async def summarize_document(self, text: str) -> Dict[str, Any]:
        """
        Generate a summary of a document.

        Args:
            text: Document text to summarize.

        Returns:
            Summary with key points.
        """
        settings = get_settings()

        system_prompt = """You are a document summarization expert for an accounting firm.
Create a concise summary of the document focusing on:
- Key financial figures
- Important dates
- Parties involved
- Actions required

Respond with JSON:
{
    "summary": "Brief 2-3 sentence summary",
    "key_points": ["point 1", "point 2", "point 3"],
    "financial_highlights": {"total": "amount", "tax": "amount"},
    "action_items": ["action 1", "action 2"],
    "priority": "high/medium/low"
}"""

        try:
            response = await self.client.chat.completions.create(
                model=settings.groq_model,
                messages=[
                    {"role": "system", "content": system_prompt},
                    {"role": "user", "content": f"Document:\n{text[:8000]}"},
                ],
                max_tokens=1024,
                temperature=settings.groq_temperature,
                response_format={"type": "json_object"},
            )

            result_text = response.choices[0].message.content
            result = json.loads(result_text)

            logger.info("Document summarized", priority=result.get("priority"))

            return result

        except json.JSONDecodeError as e:
            logger.error("Failed to parse summary response", error=str(e))
            return {"error": "Failed to parse AI response"}
        except Exception as e:
            logger.error("Document summarization failed", error=str(e))
            raise

    async def extract_text_from_image(
        self, image_data: bytes, mime_type: str = "image/png", filename: str = ""
    ) -> Dict[str, Any]:
        """
        Extract text from an image using Groq Vision model (OCR).

        Args:
            image_data: Raw image bytes.
            mime_type: MIME type of the image (image/png, image/jpeg, etc.).
            filename: Original filename for context.

        Returns:
            Extracted text and metadata.
        """
        settings = get_settings()

        # Encode image to base64
        base64_image = base64.b64encode(image_data).decode("utf-8")

        system_prompt = """You are an OCR expert. Extract ALL text from this document image.

Rules:
1. Extract text exactly as it appears, preserving formatting where possible
2. Maintain paragraph structure with blank lines
3. For tables, use | to separate columns and newlines for rows
4. Include headers, footers, dates, amounts, and all visible text
5. If text is unclear, mark it as [unclear]
6. Preserve numbers and currency symbols exactly

Output JSON format:
{
    "text": "Full extracted text with formatting preserved",
    "language": "detected language (e.g., en, es, fr)",
    "document_type_hint": "invoice/receipt/letter/form/other",
    "has_tables": true/false,
    "has_handwriting": true/false,
    "confidence": 0.95,
    "page_count": 1
}"""

        user_content = [
            {
                "type": "image_url",
                "image_url": {
                    "url": f"data:{mime_type};base64,{base64_image}"
                }
            },
            {
                "type": "text",
                "text": f"Extract all text from this document image. Filename: {filename}"
            }
        ]

        try:
            response = await self.client.chat.completions.create(
                model=settings.groq_vision_model,
                messages=[
                    {"role": "system", "content": system_prompt},
                    {"role": "user", "content": user_content},
                ],
                max_tokens=settings.groq_max_tokens,
                temperature=0.1,  # Low temperature for accurate OCR
            )

            result_text = response.choices[0].message.content

            # Try to parse as JSON, fall back to plain text
            try:
                result = json.loads(result_text)
            except json.JSONDecodeError:
                # If not JSON, wrap the raw text
                result = {
                    "text": result_text,
                    "language": "unknown",
                    "document_type_hint": "other",
                    "has_tables": False,
                    "has_handwriting": False,
                    "confidence": 0.8,
                    "page_count": 1
                }

            logger.info(
                "Text extracted from image",
                text_length=len(result.get("text", "")),
                confidence=result.get("confidence"),
                document_type=result.get("document_type_hint"),
            )

            return result

        except Exception as e:
            logger.error("Image text extraction failed", error=str(e), filename=filename)
            raise


# Singleton instance
_groq_client: Optional[GroqClient] = None


def get_groq_client() -> GroqClient:
    """Get or create the Groq client singleton."""
    global _groq_client
    if _groq_client is None:
        _groq_client = GroqClient()
    return _groq_client
