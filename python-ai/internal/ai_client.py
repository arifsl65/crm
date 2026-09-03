"""
AI Client for document processing.

Provides AI-powered document analysis using Groq (primary) and Claude (fallback).
"""

import base64
import json
from typing import Any, Dict, List, Optional

import structlog
from groq import Groq, AsyncGroq

try:
    import anthropic
    ANTHROPIC_AVAILABLE = True
except ImportError:
    ANTHROPIC_AVAILABLE = False

from .config import get_settings

logger = structlog.get_logger(__name__)


class GroqClient:
    """AI client for document processing tasks with Groq primary and Claude fallback."""

    def __init__(self):
        """Initialize AI clients with settings."""
        self._client: Optional[AsyncGroq] = None
        self._sync_client: Optional[Groq] = None
        self._anthropic_client: Optional[anthropic.AsyncAnthropic] = None

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

    @property
    def anthropic_client(self) -> Optional[anthropic.AsyncAnthropic]:
        """Get or create async Anthropic client for fallback."""
        if not ANTHROPIC_AVAILABLE:
            return None
        if self._anthropic_client is None:
            settings = get_settings()
            if settings.anthropic_api_key:
                self._anthropic_client = anthropic.AsyncAnthropic(
                    api_key=settings.anthropic_api_key
                )
        return self._anthropic_client

    def is_configured(self) -> bool:
        """Check if at least one AI provider is configured."""
        settings = get_settings()
        return bool(settings.groq_api_key) or bool(settings.anthropic_api_key)

    def has_fallback(self) -> bool:
        """Check if Claude fallback is available."""
        settings = get_settings()
        return ANTHROPIC_AVAILABLE and bool(settings.anthropic_api_key)

    async def _call_with_fallback(
        self,
        system_prompt: str,
        user_prompt: str,
        max_tokens: int = 1024,
        temperature: float = 0.1,
        json_response: bool = True,
    ) -> str:
        """
        Call AI with Groq primary and Claude Haiku fallback.

        Args:
            system_prompt: System message for context.
            user_prompt: User message/query.
            max_tokens: Maximum tokens in response.
            temperature: Sampling temperature.
            json_response: Whether to request JSON format.

        Returns:
            Raw response text from the model.

        Raises:
            Exception: If both providers fail.
        """
        settings = get_settings()
        last_error = None

        # Try Groq first
        if settings.groq_api_key:
            try:
                kwargs = {
                    "model": settings.groq_model,
                    "messages": [
                        {"role": "system", "content": system_prompt},
                        {"role": "user", "content": user_prompt},
                    ],
                    "max_tokens": max_tokens,
                    "temperature": temperature,
                }
                if json_response:
                    kwargs["response_format"] = {"type": "json_object"}

                response = await self.client.chat.completions.create(**kwargs)
                return response.choices[0].message.content

            except Exception as e:
                last_error = e
                logger.warning(
                    "Groq request failed, trying Claude fallback",
                    error=str(e),
                )

        # Fallback to Claude Haiku
        if self.anthropic_client:
            try:
                # Claude doesn't have response_format, add JSON instruction to system prompt
                claude_system = system_prompt
                if json_response and "JSON" not in system_prompt:
                    claude_system += "\n\nIMPORTANT: Respond with valid JSON only."

                response = await self.anthropic_client.messages.create(
                    model=settings.anthropic_model,
                    max_tokens=max_tokens,
                    system=claude_system,
                    messages=[{"role": "user", "content": user_prompt}],
                )
                logger.info("Claude fallback successful")
                return response.content[0].text

            except Exception as e:
                logger.error("Claude fallback also failed", error=str(e))
                last_error = e

        if last_error:
            raise last_error
        raise ValueError("No AI provider configured")

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

        user_prompt = f"Filename: {filename}\n\nDocument text:\n{text[:8000]}"

        try:
            result_text = await self._call_with_fallback(
                system_prompt=system_prompt,
                user_prompt=user_prompt,
                max_tokens=1024,
                temperature=settings.groq_temperature,
                json_response=True,
            )
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
            result_text = await self._call_with_fallback(
                system_prompt=system_prompt,
                user_prompt=user_prompt,
                max_tokens=settings.groq_max_tokens,
                temperature=settings.groq_temperature,
                json_response=True,
            )
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

        sys_prompt = system_prompt or default_system

        # Build full user prompt with context
        full_prompt = ""
        if context:
            for msg in context:
                role = msg.get("role", "user")
                content = msg.get("content", "")
                full_prompt += f"{role}: {content}\n"
        full_prompt += f"user: {message}"

        try:
            result = await self._call_with_fallback(
                system_prompt=sys_prompt,
                user_prompt=full_prompt,
                max_tokens=settings.groq_max_tokens,
                temperature=0.7,
                json_response=False,
            )

            logger.info(
                "Chat completion generated",
                message_length=len(message),
                response_length=len(result),
            )

            return {
                "response": result,
                "model": "groq/claude-fallback",
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
            result_text = await self._call_with_fallback(
                system_prompt=system_prompt,
                user_prompt=f"Document:\n{text[:8000]}",
                max_tokens=1024,
                temperature=settings.groq_temperature,
                json_response=True,
            )
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

    # =========================================================================
    # Email AI Methods
    # =========================================================================

    async def summarize_email(
        self, subject: str, body: str, sender: str = "", recipient: str = ""
    ) -> Dict[str, Any]:
        """
        Summarize an email for quick review.

        Args:
            subject: Email subject line.
            body: Email body text.
            sender: Sender email/name.
            recipient: Recipient email/name.

        Returns:
            Summary with key points and action items.
        """
        settings = get_settings()

        system_prompt = """You are an email summarization expert for an accounting firm.
Create a concise summary of the email focusing on:
- Main purpose/request
- Key information (dates, amounts, deadlines)
- Required actions
- Urgency level

Respond with JSON:
{
    "summary": "1-2 sentence summary of the email",
    "key_points": ["point 1", "point 2"],
    "action_required": true/false,
    "action_items": ["action 1", "action 2"],
    "deadline": "extracted deadline or null",
    "urgency": "high/medium/low",
    "category": "inquiry/request/notification/follow_up/complaint/other"
}"""

        email_text = f"From: {sender}\nTo: {recipient}\nSubject: {subject}\n\n{body}"

        try:
            result_text = await self._call_with_fallback(
                system_prompt=system_prompt,
                user_prompt=f"Email:\n{email_text[:6000]}",
                max_tokens=1024,
                temperature=settings.groq_temperature,
                json_response=True,
            )
            result = json.loads(result_text)

            logger.info(
                "Email summarized",
                urgency=result.get("urgency"),
                action_required=result.get("action_required"),
            )

            return result

        except json.JSONDecodeError as e:
            logger.error("Failed to parse email summary response", error=str(e))
            return {"error": "Failed to parse AI response", "summary": ""}
        except Exception as e:
            logger.error("Email summarization failed", error=str(e))
            raise

    async def analyze_email_sentiment(
        self, subject: str, body: str, sender: str = ""
    ) -> Dict[str, Any]:
        """
        Analyze the sentiment of an email.

        Args:
            subject: Email subject line.
            body: Email body text.
            sender: Sender email/name.

        Returns:
            Sentiment analysis with score and indicators.
        """
        settings = get_settings()

        system_prompt = """You are a sentiment analysis expert for an accounting firm.
Analyze the emotional tone and sentiment of this email.

Consider:
- Overall tone (professional, friendly, frustrated, urgent, etc.)
- Client satisfaction indicators
- Potential concerns or complaints
- Relationship health signals

Respond with JSON:
{
    "sentiment": "positive/neutral/negative",
    "sentiment_score": 0.75,
    "tone": "professional/friendly/frustrated/urgent/formal/informal",
    "emotions": ["satisfied", "concerned"],
    "satisfaction_level": "high/medium/low/unknown",
    "risk_indicators": ["complaint", "deadline pressure"],
    "requires_attention": true/false,
    "confidence": 0.85
}"""

        email_text = f"From: {sender}\nSubject: {subject}\n\n{body}"

        try:
            result_text = await self._call_with_fallback(
                system_prompt=system_prompt,
                user_prompt=f"Email:\n{email_text[:6000]}",
                max_tokens=1024,
                temperature=settings.groq_temperature,
                json_response=True,
            )
            result = json.loads(result_text)

            logger.info(
                "Email sentiment analyzed",
                sentiment=result.get("sentiment"),
                requires_attention=result.get("requires_attention"),
            )

            return result

        except json.JSONDecodeError as e:
            logger.error("Failed to parse sentiment response", error=str(e))
            return {"error": "Failed to parse AI response", "sentiment": "unknown"}
        except Exception as e:
            logger.error("Email sentiment analysis failed", error=str(e))
            raise

    async def extract_email_promises(
        self, subject: str, body: str, sender: str = "", recipient: str = ""
    ) -> Dict[str, Any]:
        """
        Extract promised documents and actions from an email.

        Args:
            subject: Email subject line.
            body: Email body text.
            sender: Sender email/name.
            recipient: Recipient email/name.

        Returns:
            List of promised documents and actions with deadlines.
        """
        settings = get_settings()

        system_prompt = """You are an expert at extracting commitments from emails for an accounting firm.
Identify all promises, commitments, and expected deliverables mentioned in the email.

Look for:
- Documents promised to be sent (invoices, statements, receipts, contracts)
- Actions committed to (payments, reviews, approvals, calls)
- Deadlines mentioned (explicit dates or relative timing like "next week")
- Requests for documents or information

Respond with JSON:
{
    "promised_documents": [
        {
            "document_type": "invoice/receipt/statement/contract/other",
            "description": "Q3 invoice for services",
            "promised_by": "sender/recipient",
            "deadline": "2026-09-10 or null",
            "deadline_text": "by end of week"
        }
    ],
    "promised_actions": [
        {
            "action": "Review and approve",
            "responsible_party": "sender/recipient",
            "deadline": "2026-09-15 or null",
            "deadline_text": "before the meeting"
        }
    ],
    "requested_documents": [
        {
            "document_type": "bank_statement",
            "description": "Last 3 months bank statements",
            "requested_from": "recipient"
        }
    ],
    "has_commitments": true,
    "urgency": "high/medium/low",
    "follow_up_date": "suggested follow-up date or null"
}"""

        email_text = f"From: {sender}\nTo: {recipient}\nSubject: {subject}\n\n{body}"

        try:
            result_text = await self._call_with_fallback(
                system_prompt=system_prompt,
                user_prompt=f"Email:\n{email_text[:6000]}",
                max_tokens=2048,
                temperature=settings.groq_temperature,
                json_response=True,
            )
            result = json.loads(result_text)

            total_promises = len(result.get("promised_documents", [])) + len(result.get("promised_actions", []))
            logger.info(
                "Email promises extracted",
                total_promises=total_promises,
                has_commitments=result.get("has_commitments"),
            )

            return result

        except json.JSONDecodeError as e:
            logger.error("Failed to parse promises response", error=str(e))
            return {"error": "Failed to parse AI response", "has_commitments": False}
        except Exception as e:
            logger.error("Email promise extraction failed", error=str(e))
            raise

    # =========================================================================
    # Risk Analysis AI Methods
    # =========================================================================

    async def analyze_client_risk(
        self,
        client_id: str,
        client_name: str,
        client_data: Dict[str, Any],
    ) -> Dict[str, Any]:
        """
        Analyze client churn risk based on their data and interactions.

        Args:
            client_id: UUID of the client.
            client_name: Name of the client.
            client_data: Dictionary containing client metrics:
                - services: List of active services
                - last_contact_days: Days since last contact
                - outstanding_invoices: Number of unpaid invoices
                - outstanding_amount: Total unpaid amount
                - email_sentiment_history: Recent email sentiments
                - missed_deadlines: Number of missed deadlines
                - payment_delays_avg: Average payment delay in days
                - relationship_length_months: How long they've been a client

        Returns:
            Risk analysis with score, factors, and recommendations.
        """
        settings = get_settings()

        system_prompt = """You are a client relationship risk analyst for an accounting firm.
Analyze the client data to assess their churn risk and identify potential issues.

Consider:
- Communication frequency and recency
- Payment behavior and outstanding amounts
- Service engagement and satisfaction signals
- Email sentiment trends
- Deadline adherence

Respond with JSON:
{
    "risk_level": "high/medium/low",
    "risk_score": 0.75,
    "churn_probability": 0.35,
    "risk_factors": [
        {
            "factor": "Payment delays",
            "severity": "high/medium/low",
            "description": "Average payment delay of 45 days",
            "weight": 0.3
        }
    ],
    "positive_indicators": ["Long-term client", "Multiple services"],
    "recommended_actions": [
        {
            "action": "Schedule a check-in call",
            "priority": "high/medium/low",
            "reason": "No contact in 60 days"
        }
    ],
    "next_contact_urgency": "immediate/soon/normal/low",
    "confidence": 0.85
}"""

        client_summary = f"""Client ID: {client_id}
Client Name: {client_name}

Client Metrics:
{json.dumps(client_data, indent=2, default=str)}"""

        try:
            result_text = await self._call_with_fallback(
                system_prompt=system_prompt,
                user_prompt=f"Analyze churn risk for this client:\n\n{client_summary}",
                max_tokens=2048,
                temperature=settings.groq_temperature,
                json_response=True,
            )
            result = json.loads(result_text)

            logger.info(
                "Client risk analyzed",
                client_id=client_id,
                risk_level=result.get("risk_level"),
                churn_probability=result.get("churn_probability"),
            )

            return result

        except json.JSONDecodeError as e:
            logger.error("Failed to parse client risk response", error=str(e))
            return {"error": "Failed to parse AI response", "risk_level": "unknown"}
        except Exception as e:
            logger.error("Client risk analysis failed", error=str(e), client_id=client_id)
            raise

    async def analyze_service_risk(
        self,
        service_id: str,
        service_type: str,
        service_data: Dict[str, Any],
    ) -> Dict[str, Any]:
        """
        Analyze service deadline risk and identify potential issues.

        Args:
            service_id: UUID of the service.
            service_type: Type of service (e.g., "VAT Return", "Annual Accounts").
            service_data: Dictionary containing service metrics:
                - client_name: Name of the client
                - deadline: Service deadline date
                - days_until_deadline: Days remaining
                - status: Current status
                - documents_received: Number of documents received
                - documents_required: Total documents required
                - outstanding_queries: Number of pending client queries
                - assigned_staff: Staff member assigned
                - complexity: Service complexity rating
                - previous_delays: Whether previous services were delayed
                - client_responsiveness: How responsive the client is

        Returns:
            Risk analysis with deadline risk, blockers, and recommendations.
        """
        settings = get_settings()

        system_prompt = """You are a service delivery risk analyst for an accounting firm.
Analyze the service data to assess deadline risk and identify potential blockers.

Consider:
- Time remaining vs work complexity
- Document completeness and pending queries
- Client responsiveness patterns
- Staff workload and capacity
- Historical performance on similar services

Respond with JSON:
{
    "risk_level": "critical/high/medium/low",
    "risk_score": 0.8,
    "on_time_probability": 0.45,
    "days_buffer": -5,
    "risk_factors": [
        {
            "factor": "Missing documents",
            "severity": "critical/high/medium/low",
            "description": "3 of 7 required documents still pending",
            "impact_days": 5
        }
    ],
    "blockers": [
        {
            "blocker": "Awaiting bank statements",
            "owner": "client/staff",
            "days_blocked": 10
        }
    ],
    "recommended_actions": [
        {
            "action": "Send chase email for documents",
            "priority": "urgent/high/medium/low",
            "assigned_to": "staff/manager"
        }
    ],
    "escalation_needed": true,
    "suggested_new_deadline": "2026-09-20 or null",
    "confidence": 0.85
}"""

        service_summary = f"""Service ID: {service_id}
Service Type: {service_type}

Service Metrics:
{json.dumps(service_data, indent=2, default=str)}"""

        try:
            result_text = await self._call_with_fallback(
                system_prompt=system_prompt,
                user_prompt=f"Analyze deadline risk for this service:\n\n{service_summary}",
                max_tokens=2048,
                temperature=settings.groq_temperature,
                json_response=True,
            )
            result = json.loads(result_text)

            logger.info(
                "Service risk analyzed",
                service_id=service_id,
                service_type=service_type,
                risk_level=result.get("risk_level"),
                on_time_probability=result.get("on_time_probability"),
            )

            return result

        except json.JSONDecodeError as e:
            logger.error("Failed to parse service risk response", error=str(e))
            return {"error": "Failed to parse AI response", "risk_level": "unknown"}
        except Exception as e:
            logger.error("Service risk analysis failed", error=str(e), service_id=service_id)
            raise

    # =========================================================================
    # Form Auto-Fill AI Methods
    # =========================================================================

    async def auto_fill_vat(
        self,
        client_id: str,
        period: str,
        client_data: Dict[str, Any],
    ) -> Dict[str, Any]:
        """
        Auto-fill VAT return data based on client financial information.

        Args:
            client_id: UUID of the client.
            period: VAT period (e.g., "Q1-2026", "2026-01 to 2026-03").
            client_data: Dictionary containing:
                - client_name: Name of the client
                - vat_number: VAT registration number
                - invoices: List of sales invoices with amounts and VAT
                - purchases: List of purchase invoices with amounts and VAT
                - previous_returns: Previous VAT return data for reference
                - bank_transactions: Relevant bank transactions

        Returns:
            Pre-filled VAT return fields with calculations.
        """
        settings = get_settings()

        system_prompt = """You are a UK VAT return specialist for an accounting firm.
Analyze the client's financial data and pre-fill VAT return boxes.

UK VAT Return Boxes:
- Box 1: VAT due on sales and other outputs
- Box 2: VAT due on acquisitions from EU
- Box 3: Total VAT due (Box 1 + Box 2)
- Box 4: VAT reclaimed on purchases and other inputs
- Box 5: Net VAT to pay/reclaim (Box 3 - Box 4)
- Box 6: Total value of sales excluding VAT
- Box 7: Total value of purchases excluding VAT
- Box 8: Total value of supplies to EU excluding VAT
- Box 9: Total value of acquisitions from EU excluding VAT

Respond with JSON:
{
    "vat_return": {
        "box_1": 5000.00,
        "box_2": 0.00,
        "box_3": 5000.00,
        "box_4": 2500.00,
        "box_5": 2500.00,
        "box_6": 25000.00,
        "box_7": 12500.00,
        "box_8": 0.00,
        "box_9": 0.00
    },
    "summary": {
        "total_sales": 25000.00,
        "total_purchases": 12500.00,
        "vat_collected": 5000.00,
        "vat_reclaimed": 2500.00,
        "net_vat": 2500.00,
        "payment_due": true
    },
    "line_items": {
        "sales": [{"description": "Services", "net": 20000, "vat": 4000}],
        "purchases": [{"description": "Office supplies", "net": 5000, "vat": 1000}]
    },
    "warnings": ["Large purchase without invoice - verify"],
    "missing_data": ["Bank statement for March incomplete"],
    "confidence": 0.85
}"""

        data_summary = f"""Client ID: {client_id}
VAT Period: {period}

Financial Data:
{json.dumps(client_data, indent=2, default=str)}"""

        try:
            result_text = await self._call_with_fallback(
                system_prompt=system_prompt,
                user_prompt=f"Pre-fill VAT return for this period:\n\n{data_summary}",
                max_tokens=2048,
                temperature=settings.groq_temperature,
                json_response=True,
            )
            result = json.loads(result_text)

            logger.info(
                "VAT return auto-filled",
                client_id=client_id,
                period=period,
                net_vat=result.get("summary", {}).get("net_vat"),
            )

            return result

        except json.JSONDecodeError as e:
            logger.error("Failed to parse VAT auto-fill response", error=str(e))
            return {"error": "Failed to parse AI response"}
        except Exception as e:
            logger.error("VAT auto-fill failed", error=str(e), client_id=client_id)
            raise

    async def auto_fill_ct600(
        self,
        client_id: str,
        year: int,
        client_data: Dict[str, Any],
    ) -> Dict[str, Any]:
        """
        Auto-fill CT600 Corporation Tax return data.

        Args:
            client_id: UUID of the client.
            year: Accounting year end.
            client_data: Dictionary containing:
                - company_name: Name of the company
                - company_number: Companies House number
                - utr: Unique Taxpayer Reference
                - accounts: Profit & Loss and Balance Sheet data
                - adjustments: Tax adjustments (disallowables, capital allowances)
                - previous_returns: Previous CT600 data

        Returns:
            Pre-filled CT600 fields with tax calculations.
        """
        settings = get_settings()

        system_prompt = """You are a UK Corporation Tax specialist for an accounting firm.
Analyze the company's financial data and pre-fill CT600 return fields.

Key CT600 Sections:
- Turnover/Income
- Trading profits/losses
- Property income
- Chargeable gains
- Non-trading loan relationships
- Tax adjustments (add-backs, deductions)
- Capital allowances
- Trading losses brought forward
- Profits chargeable to Corporation Tax
- Corporation Tax calculation

Current UK Corporation Tax rates:
- Small profits rate: 19% (profits up to £50,000)
- Main rate: 25% (profits over £250,000)
- Marginal relief applies between £50,000 and £250,000

Respond with JSON:
{
    "ct600": {
        "turnover": 500000.00,
        "trading_profit": 75000.00,
        "other_income": 0.00,
        "total_profits": 75000.00,
        "tax_adjustments": {
            "add_backs": 5000.00,
            "deductions": 2000.00
        },
        "capital_allowances": 10000.00,
        "adjusted_profit": 68000.00,
        "losses_brought_forward": 0.00,
        "taxable_profit": 68000.00,
        "corporation_tax": 14960.00,
        "effective_rate": 0.22
    },
    "adjustments_detail": [
        {"description": "Depreciation add-back", "amount": 5000.00, "type": "add_back"},
        {"description": "Capital allowances", "amount": 10000.00, "type": "deduction"}
    ],
    "summary": {
        "profit_before_tax": 75000.00,
        "taxable_profit": 68000.00,
        "tax_due": 14960.00,
        "payment_deadline": "2027-10-01"
    },
    "warnings": ["R&D expenditure may qualify for relief - review"],
    "missing_data": ["Fixed asset register for capital allowances"],
    "confidence": 0.80
}"""

        data_summary = f"""Client ID: {client_id}
Accounting Year End: {year}

Financial Data:
{json.dumps(client_data, indent=2, default=str)}"""

        try:
            result_text = await self._call_with_fallback(
                system_prompt=system_prompt,
                user_prompt=f"Pre-fill CT600 Corporation Tax return:\n\n{data_summary}",
                max_tokens=2048,
                temperature=settings.groq_temperature,
                json_response=True,
            )
            result = json.loads(result_text)

            logger.info(
                "CT600 auto-filled",
                client_id=client_id,
                year=year,
                tax_due=result.get("summary", {}).get("tax_due"),
            )

            return result

        except json.JSONDecodeError as e:
            logger.error("Failed to parse CT600 auto-fill response", error=str(e))
            return {"error": "Failed to parse AI response"}
        except Exception as e:
            logger.error("CT600 auto-fill failed", error=str(e), client_id=client_id)
            raise

    async def auto_fill_sa(
        self,
        client_id: str,
        tax_year: str,
        client_data: Dict[str, Any],
    ) -> Dict[str, Any]:
        """
        Auto-fill Self Assessment tax return data.

        Args:
            client_id: UUID of the client.
            tax_year: Tax year (e.g., "2025-26").
            client_data: Dictionary containing:
                - taxpayer_name: Name of the taxpayer
                - utr: Unique Taxpayer Reference
                - ni_number: National Insurance number
                - employment_income: P60/P45 data
                - self_employment: Business income and expenses
                - property_income: Rental income and expenses
                - dividends: Dividend income
                - interest: Interest income
                - capital_gains: Capital gains data
                - pension_contributions: Pension payments
                - gift_aid: Gift Aid donations

        Returns:
            Pre-filled Self Assessment fields with tax calculations.
        """
        settings = get_settings()

        system_prompt = """You are a UK Self Assessment tax specialist for an accounting firm.
Analyze the taxpayer's data and pre-fill Self Assessment return fields.

Key Self Assessment Sections:
- Employment income (SA102)
- Self-employment income (SA103S/SA103F)
- Property income (SA105)
- Foreign income (SA106)
- Capital gains (SA108)
- Dividends and interest
- Pension contributions (tax relief)
- Gift Aid donations (tax relief)

UK Income Tax Rates (2025-26):
- Personal Allowance: £12,570 (tapers above £100,000)
- Basic rate: 20% (£12,571 - £50,270)
- Higher rate: 40% (£50,271 - £125,140)
- Additional rate: 45% (over £125,140)

Dividend rates: 8.75% / 33.75% / 39.35%
Savings rates: 0% starter rate, then normal rates

Respond with JSON:
{
    "self_assessment": {
        "employment_income": 45000.00,
        "self_employment_profit": 25000.00,
        "property_income": 8000.00,
        "dividend_income": 3000.00,
        "interest_income": 500.00,
        "total_income": 81500.00,
        "personal_allowance": 12570.00,
        "taxable_income": 68930.00,
        "pension_relief": 5000.00,
        "gift_aid_relief": 1000.00
    },
    "tax_calculation": {
        "income_tax": 15572.00,
        "dividend_tax": 262.50,
        "national_insurance": {
            "class_2": 179.40,
            "class_4": 2150.00
        },
        "total_tax_due": 18163.90,
        "tax_already_paid": 9000.00,
        "balance_due": 9163.90
    },
    "payments_on_account": {
        "first_payment": 4581.95,
        "second_payment": 4581.95,
        "due_dates": ["2027-01-31", "2027-07-31"]
    },
    "supplementary_pages_needed": ["SA103S", "SA105"],
    "warnings": ["High income - personal allowance reduction applies"],
    "missing_data": ["P60 not provided - using estimate"],
    "confidence": 0.75
}"""

        data_summary = f"""Client ID: {client_id}
Tax Year: {tax_year}

Taxpayer Data:
{json.dumps(client_data, indent=2, default=str)}"""

        try:
            result_text = await self._call_with_fallback(
                system_prompt=system_prompt,
                user_prompt=f"Pre-fill Self Assessment tax return:\n\n{data_summary}",
                max_tokens=2048,
                temperature=settings.groq_temperature,
                json_response=True,
            )
            result = json.loads(result_text)

            logger.info(
                "Self Assessment auto-filled",
                client_id=client_id,
                tax_year=tax_year,
                balance_due=result.get("tax_calculation", {}).get("balance_due"),
            )

            return result

        except json.JSONDecodeError as e:
            logger.error("Failed to parse SA auto-fill response", error=str(e))
            return {"error": "Failed to parse AI response"}
        except Exception as e:
            logger.error("SA auto-fill failed", error=str(e), client_id=client_id)
            raise

    # =========================================================================
    # Document Rename AI Method
    # =========================================================================

    async def suggest_document_name(
        self,
        text: str,
        original_filename: str = "",
        document_type: str = "",
        client_name: str = "",
    ) -> Dict[str, Any]:
        """
        Suggest a descriptive name for a document based on its content.

        Args:
            text: Extracted text from the document.
            original_filename: Original filename for reference.
            document_type: Type of document if known.
            client_name: Client name if known.

        Returns:
            Suggested filename with metadata.
        """
        settings = get_settings()

        system_prompt = """You are a document naming expert for an accounting firm.
Analyze the document content and suggest a clear, descriptive filename.

Naming conventions:
- Format: [Date]_[Type]_[Description]_[Client/Vendor].ext
- Use ISO date format: YYYY-MM-DD
- Use underscores, no spaces
- Keep it concise but descriptive
- Include key identifiers (invoice numbers, periods, etc.)

Examples:
- 2026-01-15_Invoice_INV-2026-001_Acme_Corp.pdf
- 2026-Q1_VAT_Return_Smith_Trading.pdf
- 2026-03_Bank_Statement_Barclays_Business.pdf
- 2025-26_P60_John_Smith.pdf

Respond with JSON:
{
    "suggested_name": "2026-01-15_Invoice_INV-2026-001_Acme_Corp",
    "extension": "pdf",
    "full_filename": "2026-01-15_Invoice_INV-2026-001_Acme_Corp.pdf",
    "document_type": "invoice",
    "key_date": "2026-01-15",
    "key_identifiers": {
        "invoice_number": "INV-2026-001",
        "vendor": "Acme Corp",
        "amount": "£1,500.00"
    },
    "alternative_names": [
        "Acme_Corp_Invoice_2026-001.pdf",
        "2026-01_Purchase_Invoice_Acme.pdf"
    ],
    "confidence": 0.9
}"""

        context = f"""Original filename: {original_filename}
Document type: {document_type or 'unknown'}
Client name: {client_name or 'unknown'}

Document content:
{text[:6000]}"""

        try:
            result_text = await self._call_with_fallback(
                system_prompt=system_prompt,
                user_prompt=f"Suggest a filename for this document:\n\n{context}",
                max_tokens=1024,
                temperature=settings.groq_temperature,
                json_response=True,
            )
            result = json.loads(result_text)

            logger.info(
                "Document name suggested",
                original=original_filename,
                suggested=result.get("suggested_name"),
                document_type=result.get("document_type"),
            )

            return result

        except json.JSONDecodeError as e:
            logger.error("Failed to parse document rename response", error=str(e))
            return {"error": "Failed to parse AI response", "suggested_name": ""}
        except Exception as e:
            logger.error("Document rename failed", error=str(e))
            raise


# Singleton instance
_groq_client: Optional[GroqClient] = None


def get_groq_client() -> GroqClient:
    """Get or create the Groq client singleton."""
    global _groq_client
    if _groq_client is None:
        _groq_client = GroqClient()
    return _groq_client
