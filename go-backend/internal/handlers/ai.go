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
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/accountant-crm/go-backend/internal/ai"
	"github.com/accountant-crm/go-backend/internal/middleware"
)

// AICircuitBreaker implements a simple circuit breaker pattern for AI service.
type AICircuitBreaker struct {
	mu            sync.RWMutex
	failures      int
	lastFailure   time.Time
	threshold     int
	resetTimeout  time.Duration
	state         AICircuitState
}

// AICircuitState represents the state of the circuit breaker.
type AICircuitState int

const (
	AICircuitClosed AICircuitState = iota
	AICircuitOpen
	AICircuitHalfOpen
)

func NewAICircuitBreaker(threshold int, resetTimeout time.Duration) *AICircuitBreaker {
	return &AICircuitBreaker{
		threshold:    threshold,
		resetTimeout: resetTimeout,
		state:        AICircuitClosed,
	}
}

func (cb *AICircuitBreaker) Allow() bool {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	switch cb.state {
	case AICircuitClosed:
		return true
	case AICircuitOpen:
		// Check if reset timeout has passed
		if time.Since(cb.lastFailure) > cb.resetTimeout {
			return true // Allow one request to test
		}
		return false
	case AICircuitHalfOpen:
		return true
	}
	return false
}

func (cb *AICircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.failures = 0
	cb.state = AICircuitClosed
}

func (cb *AICircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.failures++
	cb.lastFailure = time.Now()
	if cb.failures >= cb.threshold {
		cb.state = AICircuitOpen
	}
}

func (cb *AICircuitBreaker) State() AICircuitState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// AIHandler handles AI service proxy endpoints.
type AIHandler struct {
	client  *ai.Client
	circuit *AICircuitBreaker
}

// NewAIHandler creates a new AI handler.
func NewAIHandler(client *ai.Client) *AIHandler {
	return &AIHandler{
		client: client,
		// Circuit breaker: 5 failures -> 30 second timeout
		circuit: NewAICircuitBreaker(5, 30*time.Second),
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

	// Check circuit breaker
	if !h.circuit.Allow() {
		log.Warn().Msg("AI service circuit breaker is open")
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error":   "service_unavailable",
			"message": "AI service is temporarily unavailable, please try again later",
		})
		return
	}

	ctx := h.contextWithRequestID(c)

	result, err := h.client.ExtractText(ctx, req.FileKey)
	if err != nil {
		h.circuit.RecordFailure()
		log.Error().Err(err).Str("file_key", req.FileKey).Msg("Failed to extract document text")
		c.JSON(http.StatusBadGateway, gin.H{
			"error":   "ai_service_error",
			"message": "Failed to communicate with AI service",
		})
		return
	}

	h.circuit.RecordSuccess()

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

	// Check circuit breaker
	if !h.circuit.Allow() {
		log.Warn().Msg("AI service circuit breaker is open")
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error":   "service_unavailable",
			"message": "AI service is temporarily unavailable, please try again later",
		})
		return
	}

	ctx := h.contextWithRequestID(c)

	result, err := h.client.ClassifyDocument(ctx, req.FileKey)
	if err != nil {
		h.circuit.RecordFailure()
		log.Error().Err(err).Str("file_key", req.FileKey).Msg("Failed to classify document")
		c.JSON(http.StatusBadGateway, gin.H{
			"error":   "ai_service_error",
			"message": "Failed to communicate with AI service",
		})
		return
	}

	h.circuit.RecordSuccess()

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

	// Check circuit breaker
	if !h.circuit.Allow() {
		log.Warn().Msg("AI service circuit breaker is open")
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error":   "service_unavailable",
			"message": "AI service is temporarily unavailable, please try again later",
		})
		return
	}

	ctx := h.contextWithRequestID(c)

	result, err := h.client.ChatComplete(ctx, req.Message)
	if err != nil {
		h.circuit.RecordFailure()
		log.Error().Err(err).Msg("Failed to complete chat")
		c.JSON(http.StatusBadGateway, gin.H{
			"error":   "ai_service_error",
			"message": "Failed to communicate with AI service",
		})
		return
	}

	h.circuit.RecordSuccess()

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

	// Check circuit breaker
	if !h.circuit.Allow() {
		log.Warn().Msg("AI service circuit breaker is open")
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error":   "service_unavailable",
			"message": "AI service is temporarily unavailable, please try again later",
		})
		return
	}

	ctx := h.contextWithRequestID(c)

	result, err := h.client.ExtractFormData(ctx, req.FileKey)
	if err != nil {
		h.circuit.RecordFailure()
		log.Error().Err(err).Str("file_key", req.FileKey).Msg("Failed to extract form data")
		c.JSON(http.StatusBadGateway, gin.H{
			"error":   "ai_service_error",
			"message": "Failed to communicate with AI service",
		})
		return
	}

	h.circuit.RecordSuccess()

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
	Text             string `json:"text" binding:"required"`
	OriginalFilename string `json:"original_filename"`
	DocumentType     string `json:"document_type"`
	ClientName       string `json:"client_name"`
	FileKey          string `json:"file_key"`
}

// RenameDocument suggests a name for a document using AI.
// POST /api/v1/ai/documents/rename
func (h *AIHandler) RenameDocument(c *gin.Context) {
	var req RenameDocumentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "validation_error",
			"message": "Document text is required",
		})
		return
	}

	ctx := h.contextWithRequestID(c)

	result, err := h.client.SuggestDocumentName(ctx, ai.DocumentRenameRequest{
		Text:             req.Text,
		OriginalFilename: req.OriginalFilename,
		DocumentType:     req.DocumentType,
		ClientName:       req.ClientName,
		FileKey:          req.FileKey,
	})
	if err != nil {
		log.Error().Err(err).Msg("Failed to suggest document name")
		c.JSON(http.StatusBadGateway, gin.H{
			"error":   "ai_service_error",
			"message": "Failed to communicate with AI service",
		})
		return
	}

	if result.Error != "" {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"error":   "rename_failed",
			"message": result.Error,
		})
		return
	}

	c.JSON(http.StatusOK, result)
}

// SummarizeEmailRequest is the request for email summarization.
type SummarizeEmailRequest struct {
	Subject   string `json:"subject" binding:"required"`
	Body      string `json:"body" binding:"required"`
	Sender    string `json:"sender"`
	Recipient string `json:"recipient"`
	EmailID   string `json:"email_id"`
}

// SummarizeEmail summarizes an email using AI.
// POST /api/v1/ai/emails/summarize
func (h *AIHandler) SummarizeEmail(c *gin.Context) {
	var req SummarizeEmailRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "validation_error",
			"message": "subject and body are required",
		})
		return
	}

	ctx := h.contextWithRequestID(c)

	result, err := h.client.SummarizeEmail(ctx, ai.EmailSummarizeRequest{
		Subject:   req.Subject,
		Body:      req.Body,
		Sender:    req.Sender,
		Recipient: req.Recipient,
		EmailID:   req.EmailID,
	})
	if err != nil {
		log.Error().Err(err).Msg("Failed to summarize email")
		c.JSON(http.StatusBadGateway, gin.H{
			"error":   "ai_service_error",
			"message": "Failed to communicate with AI service",
		})
		return
	}

	if result.Error != "" {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"error":   "summarization_failed",
			"message": result.Error,
		})
		return
	}

	c.JSON(http.StatusOK, result)
}

// AnalyzeEmailSentimentRequest is the request for email sentiment analysis.
type AnalyzeEmailSentimentRequest struct {
	Subject string `json:"subject" binding:"required"`
	Body    string `json:"body" binding:"required"`
	Sender  string `json:"sender"`
	EmailID string `json:"email_id"`
}

// AnalyzeEmailSentiment analyzes the sentiment of an email.
// POST /api/v1/ai/emails/sentiment
func (h *AIHandler) AnalyzeEmailSentiment(c *gin.Context) {
	var req AnalyzeEmailSentimentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "validation_error",
			"message": "subject and body are required",
		})
		return
	}

	ctx := h.contextWithRequestID(c)

	result, err := h.client.AnalyzeEmailSentiment(ctx, ai.EmailSentimentRequest{
		Subject: req.Subject,
		Body:    req.Body,
		Sender:  req.Sender,
		EmailID: req.EmailID,
	})
	if err != nil {
		log.Error().Err(err).Msg("Failed to analyze email sentiment")
		c.JSON(http.StatusBadGateway, gin.H{
			"error":   "ai_service_error",
			"message": "Failed to communicate with AI service",
		})
		return
	}

	if result.Error != "" {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"error":   "sentiment_analysis_failed",
			"message": result.Error,
		})
		return
	}

	c.JSON(http.StatusOK, result)
}

// ExtractEmailPromisesRequest is the request for extracting promises from email.
type ExtractEmailPromisesRequest struct {
	Subject   string `json:"subject" binding:"required"`
	Body      string `json:"body" binding:"required"`
	Sender    string `json:"sender"`
	Recipient string `json:"recipient"`
	EmailID   string `json:"email_id"`
}

// ExtractEmailPromises extracts promised documents/actions from an email.
// POST /api/v1/ai/emails/promises
func (h *AIHandler) ExtractEmailPromises(c *gin.Context) {
	var req ExtractEmailPromisesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "validation_error",
			"message": "subject and body are required",
		})
		return
	}

	ctx := h.contextWithRequestID(c)

	result, err := h.client.ExtractEmailPromises(ctx, ai.EmailPromisesRequest{
		Subject:   req.Subject,
		Body:      req.Body,
		Sender:    req.Sender,
		Recipient: req.Recipient,
		EmailID:   req.EmailID,
	})
	if err != nil {
		log.Error().Err(err).Msg("Failed to extract email promises")
		c.JSON(http.StatusBadGateway, gin.H{
			"error":   "ai_service_error",
			"message": "Failed to communicate with AI service",
		})
		return
	}

	if result.Error != "" {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"error":   "promise_extraction_failed",
			"message": result.Error,
		})
		return
	}

	c.JSON(http.StatusOK, result)
}

// ClientRiskRequest is the request for client risk analysis.
type ClientRiskRequest struct {
	ClientID                 string   `json:"client_id" binding:"required,uuid"`
	ClientName               string   `json:"client_name"`
	Services                 []string `json:"services"`
	LastContactDays          int      `json:"last_contact_days"`
	OutstandingInvoices      int      `json:"outstanding_invoices"`
	OutstandingAmount        float64  `json:"outstanding_amount"`
	EmailSentimentHistory    []string `json:"email_sentiment_history"`
	MissedDeadlines          int      `json:"missed_deadlines"`
	PaymentDelaysAvg         int      `json:"payment_delays_avg"`
	RelationshipLengthMonths int      `json:"relationship_length_months"`
}

// AnalyzeClientRisk analyzes client churn risk.
// POST /api/v1/ai/risk/client
func (h *AIHandler) AnalyzeClientRisk(c *gin.Context) {
	var req ClientRiskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "validation_error",
			"message": "client_id is required and must be a valid UUID",
		})
		return
	}

	// Check circuit breaker
	if !h.circuit.Allow() {
		log.Warn().Msg("AI service circuit breaker is open")
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error":   "service_unavailable",
			"message": "AI service is temporarily unavailable, please try again later",
		})
		return
	}

	ctx := h.contextWithRequestID(c)

	result, err := h.client.AnalyzeClientRisk(ctx, ai.ClientRiskRequest{
		ClientID:                 req.ClientID,
		ClientName:               req.ClientName,
		Services:                 req.Services,
		LastContactDays:          req.LastContactDays,
		OutstandingInvoices:      req.OutstandingInvoices,
		OutstandingAmount:        req.OutstandingAmount,
		EmailSentimentHistory:    req.EmailSentimentHistory,
		MissedDeadlines:          req.MissedDeadlines,
		PaymentDelaysAvg:         req.PaymentDelaysAvg,
		RelationshipLengthMonths: req.RelationshipLengthMonths,
	})
	if err != nil {
		h.circuit.RecordFailure()
		log.Error().Err(err).Str("client_id", req.ClientID).Msg("Failed to analyze client risk")
		c.JSON(http.StatusBadGateway, gin.H{
			"error":   "ai_service_error",
			"message": "Failed to communicate with AI service",
		})
		return
	}

	h.circuit.RecordSuccess()

	if result.Error != "" {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"error":   "risk_analysis_failed",
			"message": result.Error,
		})
		return
	}

	c.JSON(http.StatusOK, result)
}

// ServiceRiskRequest is the request for service risk analysis.
type ServiceRiskRequest struct {
	ServiceID            string `json:"service_id" binding:"required,uuid"`
	ServiceType          string `json:"service_type"`
	ClientName           string `json:"client_name"`
	Deadline             string `json:"deadline"`
	DaysUntilDeadline    int    `json:"days_until_deadline"`
	Status               string `json:"status"`
	DocumentsReceived    int    `json:"documents_received"`
	DocumentsRequired    int    `json:"documents_required"`
	OutstandingQueries   int    `json:"outstanding_queries"`
	AssignedStaff        string `json:"assigned_staff"`
	Complexity           string `json:"complexity"`
	PreviousDelays       bool   `json:"previous_delays"`
	ClientResponsiveness string `json:"client_responsiveness"`
}

// AnalyzeServiceRisk analyzes service deadline risk.
// POST /api/v1/ai/risk/service
func (h *AIHandler) AnalyzeServiceRisk(c *gin.Context) {
	var req ServiceRiskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "validation_error",
			"message": "service_id is required and must be a valid UUID",
		})
		return
	}

	// Check circuit breaker
	if !h.circuit.Allow() {
		log.Warn().Msg("AI service circuit breaker is open")
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error":   "service_unavailable",
			"message": "AI service is temporarily unavailable, please try again later",
		})
		return
	}

	ctx := h.contextWithRequestID(c)

	result, err := h.client.AnalyzeServiceRisk(ctx, ai.ServiceRiskRequest{
		ServiceID:            req.ServiceID,
		ServiceType:          req.ServiceType,
		ClientName:           req.ClientName,
		Deadline:             req.Deadline,
		DaysUntilDeadline:    req.DaysUntilDeadline,
		Status:               req.Status,
		DocumentsReceived:    req.DocumentsReceived,
		DocumentsRequired:    req.DocumentsRequired,
		OutstandingQueries:   req.OutstandingQueries,
		AssignedStaff:        req.AssignedStaff,
		Complexity:           req.Complexity,
		PreviousDelays:       req.PreviousDelays,
		ClientResponsiveness: req.ClientResponsiveness,
	})
	if err != nil {
		h.circuit.RecordFailure()
		log.Error().Err(err).Str("service_id", req.ServiceID).Msg("Failed to analyze service risk")
		c.JSON(http.StatusBadGateway, gin.H{
			"error":   "ai_service_error",
			"message": "Failed to communicate with AI service",
		})
		return
	}

	h.circuit.RecordSuccess()

	if result.Error != "" {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"error":   "risk_analysis_failed",
			"message": result.Error,
		})
		return
	}

	c.JSON(http.StatusOK, result)
}

// ============================================================================
// Form AI endpoints (VAT, CT600, Self-Assessment auto-fill)
// ============================================================================

// AutoFillVATRequest is the request for VAT form auto-fill.
type AutoFillVATRequest struct {
	ClientID       string  `json:"client_id" binding:"required,uuid"`
	Period         string  `json:"period" binding:"required"` // e.g., "Q1-2026"
	ClientName     string  `json:"client_name"`
	VATNumber      string  `json:"vat_number"`
	TotalSales     float64 `json:"total_sales"`
	TotalPurchases float64 `json:"total_purchases"`
	VATOnSales     float64 `json:"vat_on_sales"`
	VATOnPurchases float64 `json:"vat_on_purchases"`
	EUAcquisitions float64 `json:"eu_acquisitions"`
	EUSupplies     float64 `json:"eu_supplies"`
}

// AutoFillVAT auto-fills VAT return data.
// POST /api/v1/ai/forms/vat
func (h *AIHandler) AutoFillVAT(c *gin.Context) {
	var req AutoFillVATRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "validation_error",
			"message": "client_id and period are required",
		})
		return
	}

	ctx := h.contextWithRequestID(c)

	result, err := h.client.AutoFillVAT(ctx, ai.VATAutoFillRequest{
		ClientID:       req.ClientID,
		Period:         req.Period,
		ClientName:     req.ClientName,
		VATNumber:      req.VATNumber,
		TotalSales:     req.TotalSales,
		TotalPurchases: req.TotalPurchases,
		VATOnSales:     req.VATOnSales,
		VATOnPurchases: req.VATOnPurchases,
		EUAcquisitions: req.EUAcquisitions,
		EUSupplies:     req.EUSupplies,
	})
	if err != nil {
		log.Error().Err(err).Str("client_id", req.ClientID).Msg("Failed to auto-fill VAT")
		c.JSON(http.StatusBadGateway, gin.H{
			"error":   "ai_service_error",
			"message": "Failed to communicate with AI service",
		})
		return
	}

	if result.Error != "" {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"error":   "vat_autofill_failed",
			"message": result.Error,
		})
		return
	}

	c.JSON(http.StatusOK, result)
}

// AutoFillCT600Request is the request for CT600 form auto-fill.
type AutoFillCT600Request struct {
	ClientID         string  `json:"client_id" binding:"required,uuid"`
	Year             int     `json:"year" binding:"required"`
	CompanyName      string  `json:"company_name"`
	CompanyNumber    string  `json:"company_number"`
	UTR              string  `json:"utr"`
	Turnover         float64 `json:"turnover"`
	CostOfSales      float64 `json:"cost_of_sales"`
	GrossProfit      float64 `json:"gross_profit"`
	AdminExpenses    float64 `json:"admin_expenses"`
	Depreciation     float64 `json:"depreciation"`
	InterestReceived float64 `json:"interest_received"`
	InterestPaid     float64 `json:"interest_paid"`
	OtherIncome      float64 `json:"other_income"`
}

// AutoFillCT600 auto-fills CT600 corporation tax return data.
// POST /api/v1/ai/forms/ct600
func (h *AIHandler) AutoFillCT600(c *gin.Context) {
	var req AutoFillCT600Request
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "validation_error",
			"message": "client_id and year are required",
		})
		return
	}

	ctx := h.contextWithRequestID(c)

	result, err := h.client.AutoFillCT600(ctx, ai.CT600AutoFillRequest{
		ClientID:         req.ClientID,
		Year:             req.Year,
		CompanyName:      req.CompanyName,
		CompanyNumber:    req.CompanyNumber,
		UTR:              req.UTR,
		Turnover:         req.Turnover,
		CostOfSales:      req.CostOfSales,
		GrossProfit:      req.GrossProfit,
		AdminExpenses:    req.AdminExpenses,
		Depreciation:     req.Depreciation,
		InterestReceived: req.InterestReceived,
		InterestPaid:     req.InterestPaid,
		OtherIncome:      req.OtherIncome,
	})
	if err != nil {
		log.Error().Err(err).Str("client_id", req.ClientID).Msg("Failed to auto-fill CT600")
		c.JSON(http.StatusBadGateway, gin.H{
			"error":   "ai_service_error",
			"message": "Failed to communicate with AI service",
		})
		return
	}

	if result.Error != "" {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"error":   "ct600_autofill_failed",
			"message": result.Error,
		})
		return
	}

	c.JSON(http.StatusOK, result)
}

// AutoFillSARequest is the request for Self Assessment form auto-fill.
type AutoFillSARequest struct {
	ClientID               string  `json:"client_id" binding:"required,uuid"`
	TaxYear                string  `json:"tax_year" binding:"required"` // e.g., "2025-26"
	TaxpayerName           string  `json:"taxpayer_name"`
	UTR                    string  `json:"utr"`
	NINumber               string  `json:"ni_number"`
	EmploymentIncome       float64 `json:"employment_income"`
	SelfEmploymentIncome   float64 `json:"self_employment_income"`
	SelfEmploymentExpenses float64 `json:"self_employment_expenses"`
	PropertyIncome         float64 `json:"property_income"`
	PropertyExpenses       float64 `json:"property_expenses"`
	DividendIncome         float64 `json:"dividend_income"`
	InterestIncome         float64 `json:"interest_income"`
	PensionContributions   float64 `json:"pension_contributions"`
	GiftAid                float64 `json:"gift_aid"`
}

// AutoFillSA auto-fills Self Assessment tax return data.
// POST /api/v1/ai/forms/sa
func (h *AIHandler) AutoFillSA(c *gin.Context) {
	var req AutoFillSARequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "validation_error",
			"message": "client_id and tax_year are required",
		})
		return
	}

	ctx := h.contextWithRequestID(c)

	result, err := h.client.AutoFillSA(ctx, ai.SAAutoFillRequest{
		ClientID:               req.ClientID,
		TaxYear:                req.TaxYear,
		TaxpayerName:           req.TaxpayerName,
		UTR:                    req.UTR,
		NINumber:               req.NINumber,
		EmploymentIncome:       req.EmploymentIncome,
		SelfEmploymentIncome:   req.SelfEmploymentIncome,
		SelfEmploymentExpenses: req.SelfEmploymentExpenses,
		PropertyIncome:         req.PropertyIncome,
		PropertyExpenses:       req.PropertyExpenses,
		DividendIncome:         req.DividendIncome,
		InterestIncome:         req.InterestIncome,
		PensionContributions:   req.PensionContributions,
		GiftAid:                req.GiftAid,
	})
	if err != nil {
		log.Error().Err(err).Str("client_id", req.ClientID).Msg("Failed to auto-fill Self Assessment")
		c.JSON(http.StatusBadGateway, gin.H{
			"error":   "ai_service_error",
			"message": "Failed to communicate with AI service",
		})
		return
	}

	if result.Error != "" {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"error":   "sa_autofill_failed",
			"message": result.Error,
		})
		return
	}

	c.JSON(http.StatusOK, result)
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

// GetChatHistoryRequest is the request for getting chat history.
type GetChatHistoryRequest struct {
	UserID   string `form:"user_id" binding:"required,uuid"`
	TenantID string `form:"tenant_id"`
	Limit    int    `form:"limit"`
	Offset   int    `form:"offset"`
}

// GetChatHistory gets chat history for the current user.
// GET /api/v1/ai/chat/history
func (h *AIHandler) GetChatHistory(c *gin.Context) {
	var req GetChatHistoryRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "validation_error",
			"message": "user_id is required",
		})
		return
	}

	// Default limit
	if req.Limit == 0 {
		req.Limit = 50
	}

	ctx := h.contextWithRequestID(c)

	result, err := h.client.GetChatHistory(ctx, ai.ChatHistoryRequest{
		UserID:   req.UserID,
		TenantID: req.TenantID,
		Limit:    req.Limit,
		Offset:   req.Offset,
	})
	if err != nil {
		log.Error().Err(err).Str("user_id", req.UserID).Msg("Failed to get chat history")
		c.JSON(http.StatusBadGateway, gin.H{
			"error":   "ai_service_error",
			"message": "Failed to communicate with AI service",
		})
		return
	}

	if result.Error != "" {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"error":   "chat_history_failed",
			"message": result.Error,
		})
		return
	}

	c.JSON(http.StatusOK, result)
}

// SaveChatHistoryRequest is the request for saving a chat message.
type SaveChatHistoryRequest struct {
	UserID         string `json:"user_id" binding:"required,uuid"`
	TenantID       string `json:"tenant_id"`
	ConversationID string `json:"conversation_id"`
	Role           string `json:"role" binding:"required,oneof=user assistant system"`
	Content        string `json:"content" binding:"required"`
	Metadata       string `json:"metadata"`
}

// SaveChatHistory saves a chat message to history.
// POST /api/v1/ai/chat/history
func (h *AIHandler) SaveChatHistory(c *gin.Context) {
	var req SaveChatHistoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "validation_error",
			"message": "user_id, role, and content are required",
		})
		return
	}

	ctx := h.contextWithRequestID(c)

	result, err := h.client.SaveChatMessage(ctx, ai.SaveChatMessageRequest{
		UserID:         req.UserID,
		TenantID:       req.TenantID,
		ConversationID: req.ConversationID,
		Role:           req.Role,
		Content:        req.Content,
		Metadata:       req.Metadata,
	})
	if err != nil {
		log.Error().Err(err).Str("user_id", req.UserID).Msg("Failed to save chat history")
		c.JSON(http.StatusBadGateway, gin.H{
			"error":   "ai_service_error",
			"message": "Failed to communicate with AI service",
		})
		return
	}

	if result.Error != "" {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"error":   "save_chat_failed",
			"message": result.Error,
		})
		return
	}

	c.JSON(http.StatusOK, result)
}

// DeleteChatQuery contains the user_id for authorization.
type DeleteChatQuery struct {
	UserID string `form:"user_id" binding:"required,uuid"`
}

// DeleteChat deletes a chat conversation.
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

	var query DeleteChatQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "validation_error",
			"message": "user_id is required for authorization",
		})
		return
	}

	ctx := h.contextWithRequestID(c)

	result, err := h.client.DeleteChat(ctx, ai.DeleteChatRequest{
		ConversationID: chatID,
		UserID:         query.UserID,
	})
	if err != nil {
		log.Error().Err(err).Str("chat_id", chatID).Msg("Failed to delete chat")
		c.JSON(http.StatusBadGateway, gin.H{
			"error":   "ai_service_error",
			"message": "Failed to communicate with AI service",
		})
		return
	}

	if result.Error != "" {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"error":   "delete_failed",
			"message": result.Error,
		})
		return
	}

	c.JSON(http.StatusOK, result)
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

// Status returns the current state of the AI service circuit breaker.
// GET /api/v1/ai/status
func (h *AIHandler) Status(c *gin.Context) {
	state := h.circuit.State()
	stateStr := "closed"
	switch state {
	case AICircuitOpen:
		stateStr = "open"
	case AICircuitHalfOpen:
		stateStr = "half-open"
	}

	c.JSON(http.StatusOK, gin.H{
		"circuit_breaker": stateStr,
		"service":         "python-ai",
	})
}

// Placeholder for unused import
var _ = bytes.NewReader
