// Package handlers provides HTTP handlers for the API.
package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/accountant-crm/go-backend/internal/ai"
	"github.com/accountant-crm/go-backend/internal/middleware"
)

// AIHandler handles AI service proxy endpoints.
type AIHandler struct {
	client *ai.Client
}

// NewAIHandler creates a new AI handler.
func NewAIHandler(client *ai.Client) *AIHandler {
	return &AIHandler{
		client: client,
	}
}

// ExtractDocumentRequest is the request for document text extraction.
type ExtractDocumentRequest struct {
	FileKey string `json:"file_key" binding:"required"`
}

// ExtractDocument extracts text from a document using OCR.
// POST /api/v1/ai/documents/extract
func (h *AIHandler) ExtractDocument(c *gin.Context) {
	var req ExtractDocumentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "validation_error",
			"message": "file_key is required",
		})
		return
	}

	ctx := h.contextWithRequestID(c)

	result, err := h.client.ExtractText(ctx, req.FileKey)
	if err != nil {
		log.Error().Err(err).Str("file_key", req.FileKey).Msg("Failed to extract document text")
		c.JSON(http.StatusBadGateway, gin.H{
			"error":   "ai_service_error",
			"message": "Failed to communicate with AI service",
		})
		return
	}

	if result.Error != "" {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"error":   "extraction_failed",
			"message": result.Error,
		})
		return
	}

	c.JSON(http.StatusOK, result)
}

// ClassifyDocumentRequest is the request for document classification.
type ClassifyDocumentRequest struct {
	FileKey string `json:"file_key" binding:"required"`
}

// ClassifyDocument classifies a document using AI.
// POST /api/v1/ai/documents/classify
func (h *AIHandler) ClassifyDocument(c *gin.Context) {
	var req ClassifyDocumentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "validation_error",
			"message": "file_key is required",
		})
		return
	}

	ctx := h.contextWithRequestID(c)

	result, err := h.client.ClassifyDocument(ctx, req.FileKey)
	if err != nil {
		log.Error().Err(err).Str("file_key", req.FileKey).Msg("Failed to classify document")
		c.JSON(http.StatusBadGateway, gin.H{
			"error":   "ai_service_error",
			"message": "Failed to communicate with AI service",
		})
		return
	}

	if result.Error != "" {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"error":   "classification_failed",
			"message": result.Error,
		})
		return
	}

	c.JSON(http.StatusOK, result)
}

// ChatRequest is the request for chat completion.
type ChatRequestBody struct {
	Message string `json:"message" binding:"required"`
}

// Chat handles chat completion requests.
// POST /api/v1/ai/chat
func (h *AIHandler) Chat(c *gin.Context) {
	var req ChatRequestBody
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "validation_error",
			"message": "message is required",
		})
		return
	}

	ctx := h.contextWithRequestID(c)

	result, err := h.client.ChatComplete(ctx, req.Message)
	if err != nil {
		log.Error().Err(err).Msg("Failed to complete chat")
		c.JSON(http.StatusBadGateway, gin.H{
			"error":   "ai_service_error",
			"message": "Failed to communicate with AI service",
		})
		return
	}

	if result.Error != "" {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"error":   "chat_failed",
			"message": result.Error,
		})
		return
	}

	c.JSON(http.StatusOK, result)
}

// ExtractFormDataRequest is the request for form data extraction.
type ExtractFormDataRequest struct {
	FileKey string `json:"file_key" binding:"required"`
}

// ExtractFormData extracts structured data from forms.
// POST /api/v1/ai/forms/extract
func (h *AIHandler) ExtractFormData(c *gin.Context) {
	var req ExtractFormDataRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "validation_error",
			"message": "file_key is required",
		})
		return
	}

	ctx := h.contextWithRequestID(c)

	result, err := h.client.ExtractFormData(ctx, req.FileKey)
	if err != nil {
		log.Error().Err(err).Str("file_key", req.FileKey).Msg("Failed to extract form data")
		c.JSON(http.StatusBadGateway, gin.H{
			"error":   "ai_service_error",
			"message": "Failed to communicate with AI service",
		})
		return
	}

	if result.Error != "" {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"error":   "form_extraction_failed",
			"message": result.Error,
		})
		return
	}

	c.JSON(http.StatusOK, result)
}

// ProxyRequest proxies any request to the Python AI service.
// This is used for endpoints not yet implemented in the Go client.
// The path after /api/v1/ai/ is forwarded to the Python service.
func (h *AIHandler) ProxyRequest(c *gin.Context) {
	// Get the path after /api/v1/ai/
	path := c.Param("path")
	if path == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid_path",
			"message": "AI endpoint path is required",
		})
		return
	}

	// Build the target URL
	targetPath := "/api/v1/ai/" + strings.TrimPrefix(path, "/")

	// Add query parameters
	if c.Request.URL.RawQuery != "" {
		targetPath += "?" + c.Request.URL.RawQuery
	}

	// Read request body
	var body []byte
	if c.Request.Body != nil {
		var err error
		body, err = io.ReadAll(c.Request.Body)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "invalid_body",
				"message": "Failed to read request body",
			})
			return
		}
	}

	// Add tenant and user context to headers
	tenantID, _ := middleware.GetTenantID(c)
	userID, _ := middleware.GetUserID(c)
	requestID := middleware.GetRequestID(c)

	// Proxy the request
	ctx := c.Request.Context()
	if requestID != "" {
		ctx = context.WithValue(ctx, ai.RequestIDKey, requestID)
	}

	resp, err := h.proxyHTTP(ctx, c.Request.Method, targetPath, body, tenantID, userID)
	if err != nil {
		log.Error().Err(err).Str("path", targetPath).Msg("AI proxy request failed")
		c.JSON(http.StatusBadGateway, gin.H{
			"error":   "ai_service_error",
			"message": "Failed to communicate with AI service",
		})
		return
	}
	defer resp.Body.Close()

	// Copy response headers
	for key, values := range resp.Header {
		for _, value := range values {
			c.Header(key, value)
		}
	}

	// Copy response body
	c.Status(resp.StatusCode)
	_, _ = io.Copy(c.Writer, resp.Body)
}

// proxyHTTP makes an HTTP request to the AI service.
func (h *AIHandler) proxyHTTP(ctx context.Context, method, path string, body []byte, tenantID, userID uuid.UUID) (*http.Response, error) {
	// This is a simplified proxy - in production you'd want the full URL from config
	// For now, we'll use the client's internal HTTP client
	// This requires exposing the base URL and httpClient from ai.Client

	// Since we can't easily proxy with the current ai.Client design,
	// return a not implemented error for now
	return nil, fmt.Errorf("generic proxy not yet implemented - use specific AI endpoints")
}

// contextWithRequestID adds the request ID to the context for distributed tracing.
func (h *AIHandler) contextWithRequestID(c *gin.Context) context.Context {
	ctx := c.Request.Context()
	requestID := middleware.GetRequestID(c)
	if requestID != "" {
		ctx = context.WithValue(ctx, ai.RequestIDKey, requestID)
	}
	return ctx
}

// ChatStreamRequest is the request for streaming chat.
type ChatStreamRequest struct {
	Message string `json:"message" binding:"required"`
}

// ChatStream handles streaming chat requests (SSE).
// POST /api/v1/ai/chat/stream
func (h *AIHandler) ChatStream(c *gin.Context) {
	var req ChatStreamRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "validation_error",
			"message": "message is required",
		})
		return
	}

	// For SSE streaming, we need to proxy the response as it comes
	// This requires a streaming-capable proxy

	// Set SSE headers
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")

	// TODO: Implement SSE streaming proxy to Python service
	// For now, return a single message indicating streaming is not yet available

	// Write SSE format
	c.SSEvent("message", gin.H{
		"status":  "error",
		"message": "Streaming chat not yet implemented in Go proxy. Use Python service directly.",
	})
	c.SSEvent("done", nil)
}

// GetJobStatus gets the status of an async AI job.
// GET /api/v1/ai/jobs/:id
func (h *AIHandler) GetJobStatus(c *gin.Context) {
	jobID := c.Param("id")
	if _, err := uuid.Parse(jobID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "validation_error",
			"message": "Invalid job ID",
		})
		return
	}

	// TODO: Query job status from database or Python service
	// For now, return not implemented
	c.JSON(http.StatusNotImplemented, gin.H{
		"error":   "not_implemented",
		"message": "Job status polling not yet implemented. Use Python service directly.",
	})
}

// SummarizeDocumentRequest is the request for document summarization.
type SummarizeDocumentRequest struct {
	FileKey string `json:"file_key" binding:"required"`
}

// SummarizeDocument summarizes a document using AI.
// POST /api/v1/ai/documents/summarize
func (h *AIHandler) SummarizeDocument(c *gin.Context) {
	var req SummarizeDocumentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "validation_error",
			"message": "file_key is required",
		})
		return
	}

	ctx := h.contextWithRequestID(c)

	// Use extract endpoint with summarization option
	// The Python service should handle this
	result, err := h.proxyAIRequest(ctx, "POST", "/api/v1/ai/documents/summarize", map[string]string{
		"file_key": req.FileKey,
	})
	if err != nil {
		log.Error().Err(err).Str("file_key", req.FileKey).Msg("Failed to summarize document")
		c.JSON(http.StatusBadGateway, gin.H{
			"error":   "ai_service_error",
			"message": "Failed to communicate with AI service",
		})
		return
	}

	c.JSON(http.StatusOK, result)
}

// proxyAIRequest is a helper to proxy requests to the AI service.
func (h *AIHandler) proxyAIRequest(ctx context.Context, method, path string, body interface{}) (map[string]interface{}, error) {
	// This is a placeholder - actual implementation would use http client
	// For now, return an error indicating this endpoint needs direct Python access
	return nil, fmt.Errorf("endpoint %s not yet implemented in Go - use Python service directly", path)
}

// RenameDocumentRequest is the request for AI document renaming.
type RenameDocumentRequest struct {
	FileKey string `json:"file_key" binding:"required"`
}

// RenameDocument suggests a name for a document using AI.
// POST /api/v1/ai/documents/rename
func (h *AIHandler) RenameDocument(c *gin.Context) {
	// TODO: Implement when Python service method is available
	c.JSON(http.StatusNotImplemented, gin.H{
		"error":   "not_implemented",
		"message": "Document rename AI not yet implemented",
	})
}

// SummarizeEmailRequest is the request for email summarization.
type SummarizeEmailRequest struct {
	EmailID string `json:"email_id" binding:"required,uuid"`
}

// SummarizeEmail summarizes an email using AI.
// POST /api/v1/ai/emails/summarize
func (h *AIHandler) SummarizeEmail(c *gin.Context) {
	// TODO: Implement when Python service method is available
	c.JSON(http.StatusNotImplemented, gin.H{
		"error":   "not_implemented",
		"message": "Email summarization AI not yet implemented",
	})
}

// AnalyzeEmailSentimentRequest is the request for email sentiment analysis.
type AnalyzeEmailSentimentRequest struct {
	EmailID string `json:"email_id" binding:"required,uuid"`
}

// AnalyzeEmailSentiment analyzes the sentiment of an email.
// POST /api/v1/ai/emails/sentiment
func (h *AIHandler) AnalyzeEmailSentiment(c *gin.Context) {
	// TODO: Implement when Python service method is available
	c.JSON(http.StatusNotImplemented, gin.H{
		"error":   "not_implemented",
		"message": "Email sentiment analysis AI not yet implemented",
	})
}

// ExtractEmailPromisesRequest is the request for extracting promises from email.
type ExtractEmailPromisesRequest struct {
	EmailID string `json:"email_id" binding:"required,uuid"`
}

// ExtractEmailPromises extracts promised documents/actions from an email.
// POST /api/v1/ai/emails/promises
func (h *AIHandler) ExtractEmailPromises(c *gin.Context) {
	// TODO: Implement when Python service method is available
	c.JSON(http.StatusNotImplemented, gin.H{
		"error":   "not_implemented",
		"message": "Email promise extraction AI not yet implemented",
	})
}

// ClientRiskRequest is the request for client risk analysis.
type ClientRiskRequest struct {
	ClientID string `json:"client_id" binding:"required,uuid"`
}

// AnalyzeClientRisk analyzes client churn risk.
// POST /api/v1/ai/risk/client
func (h *AIHandler) AnalyzeClientRisk(c *gin.Context) {
	// TODO: Implement when Python service method is available
	c.JSON(http.StatusNotImplemented, gin.H{
		"error":   "not_implemented",
		"message": "Client risk analysis AI not yet implemented",
	})
}

// ServiceRiskRequest is the request for service risk analysis.
type ServiceRiskRequest struct {
	ServiceID string `json:"service_id" binding:"required,uuid"`
}

// AnalyzeServiceRisk analyzes service deadline risk.
// POST /api/v1/ai/risk/service
func (h *AIHandler) AnalyzeServiceRisk(c *gin.Context) {
	// TODO: Implement when Python service method is available
	c.JSON(http.StatusNotImplemented, gin.H{
		"error":   "not_implemented",
		"message": "Service risk analysis AI not yet implemented",
	})
}

// ============================================================================
// Form AI endpoints (VAT, CT600, Self-Assessment auto-fill)
// ============================================================================

// AutoFillVATRequest is the request for VAT form auto-fill.
type AutoFillVATRequest struct {
	ClientID string `json:"client_id" binding:"required,uuid"`
	Period   string `json:"period" binding:"required"` // e.g., "Q1-2026"
}

// AutoFillVAT auto-fills VAT return data.
// POST /api/v1/ai/forms/vat
func (h *AIHandler) AutoFillVAT(c *gin.Context) {
	// TODO: Implement when Python service method is available
	c.JSON(http.StatusNotImplemented, gin.H{
		"error":   "not_implemented",
		"message": "VAT auto-fill AI not yet implemented",
	})
}

// AutoFillCT600Request is the request for CT600 form auto-fill.
type AutoFillCT600Request struct {
	ClientID string `json:"client_id" binding:"required,uuid"`
	Year     int    `json:"year" binding:"required"`
}

// AutoFillCT600 auto-fills CT600 corporation tax return data.
// POST /api/v1/ai/forms/ct600
func (h *AIHandler) AutoFillCT600(c *gin.Context) {
	// TODO: Implement when Python service method is available
	c.JSON(http.StatusNotImplemented, gin.H{
		"error":   "not_implemented",
		"message": "CT600 auto-fill AI not yet implemented",
	})
}

// AutoFillSARequest is the request for Self Assessment form auto-fill.
type AutoFillSARequest struct {
	ClientID string `json:"client_id" binding:"required,uuid"`
	TaxYear  string `json:"tax_year" binding:"required"` // e.g., "2025-26"
}

// AutoFillSA auto-fills Self Assessment tax return data.
// POST /api/v1/ai/forms/sa
func (h *AIHandler) AutoFillSA(c *gin.Context) {
	// TODO: Implement when Python service method is available
	c.JSON(http.StatusNotImplemented, gin.H{
		"error":   "not_implemented",
		"message": "Self Assessment auto-fill AI not yet implemented",
	})
}

// ============================================================================
// Helper function to proxy to Python with body
// ============================================================================

// proxyWithBody proxies a request to the Python AI service with a JSON body.
func (h *AIHandler) proxyWithBody(ctx context.Context, method, path string, body interface{}) (*http.Response, error) {
	// Marshal body to JSON
	var jsonBody []byte
	var err error
	if body != nil {
		jsonBody, err = json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal body: %w", err)
		}
	}

	// Create request
	// Note: This requires access to the AI client's base URL and HTTP client
	// For now, this is a placeholder that returns an error
	_ = jsonBody
	return nil, fmt.Errorf("direct proxy not yet implemented")
}

// ============================================================================
// Chat History endpoints
// ============================================================================

// GetChatHistory gets chat history for the current user.
// GET /api/v1/ai/chat/history
func (h *AIHandler) GetChatHistory(c *gin.Context) {
	// TODO: Implement - query MongoDB ai_conversations collection
	c.JSON(http.StatusNotImplemented, gin.H{
		"error":   "not_implemented",
		"message": "Chat history not yet implemented - uses MongoDB",
	})
}

// DeleteChatRequest contains the chat ID to delete.
// DELETE /api/v1/ai/chat/:id
func (h *AIHandler) DeleteChat(c *gin.Context) {
	chatID := c.Param("id")
	if chatID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "validation_error",
			"message": "Chat ID is required",
		})
		return
	}

	// TODO: Implement - delete from MongoDB ai_conversations collection
	c.JSON(http.StatusNotImplemented, gin.H{
		"error":   "not_implemented",
		"message": "Chat deletion not yet implemented - uses MongoDB",
	})
}

// ============================================================================
// Helper: Build context with request ID for distributed tracing
// ============================================================================
// This is already defined above as contextWithRequestID

// NotImplementedAI is a catch-all handler for AI endpoints not yet implemented.
func (h *AIHandler) NotImplementedAI(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{
		"error":   "not_implemented",
		"message": "This AI endpoint is not yet implemented. Use Python service directly.",
	})
}

// Placeholder for unused import
var _ = bytes.NewReader
