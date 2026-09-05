// Package ai provides a client for communicating with the Python AI service.
package ai

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/accountant-crm/go-backend/internal/config"
)

// RetryConfig defines retry behavior for HTTP requests.
type RetryConfig struct {
	MaxRetries  int
	BaseDelay   time.Duration
	MaxDelay    time.Duration
	RetryOn5xx  bool
	RetryOnConn bool
}

// DefaultRetryConfig returns sensible retry defaults.
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries:  3,
		BaseDelay:   100 * time.Millisecond,
		MaxDelay:    2 * time.Second,
		RetryOn5xx:  true,
		RetryOnConn: true,
	}
}

// Client provides methods to call the Python AI service.
type Client struct {
	baseURL     string
	httpClient  *http.Client
	retryConfig RetryConfig
}

// NewClient creates a new AI client with mTLS configuration.
func NewClient(pythonCfg config.PythonAIConfig, mtlsCfg config.MTLSConfig) (*Client, error) {
	transport := &http.Transport{
		MaxIdleConns:        10,
		IdleConnTimeout:     30 * time.Second,
		DisableCompression:  false,
		TLSHandshakeTimeout: 10 * time.Second,
	}

	// Configure mTLS if enabled
	if mtlsCfg.Enabled {
		tlsConfig, err := buildTLSConfig(mtlsCfg)
		if err != nil {
			return nil, fmt.Errorf("failed to build TLS config: %w", err)
		}
		transport.TLSClientConfig = tlsConfig
	}

	client := &Client{
		baseURL: pythonCfg.BaseURL,
		httpClient: &http.Client{
			Transport: transport,
			Timeout:   pythonCfg.Timeout,
		},
		retryConfig: DefaultRetryConfig(),
	}

	return client, nil
}

// buildTLSConfig creates a TLS configuration for mTLS.
func buildTLSConfig(cfg config.MTLSConfig) (*tls.Config, error) {
	// Load CA certificate
	caCert, err := os.ReadFile(cfg.CACert)
	if err != nil {
		return nil, fmt.Errorf("failed to read CA cert: %w", err)
	}

	caCertPool := x509.NewCertPool()
	if !caCertPool.AppendCertsFromPEM(caCert) {
		return nil, fmt.Errorf("failed to parse CA cert")
	}

	// Load client certificate
	clientCert, err := tls.LoadX509KeyPair(cfg.ClientCert, cfg.ClientKey)
	if err != nil {
		return nil, fmt.Errorf("failed to load client cert: %w", err)
	}

	return &tls.Config{
		RootCAs:      caCertPool,
		Certificates: []tls.Certificate{clientCert},
		MinVersion:   tls.VersionTLS12,
	}, nil
}

// drainAndClose drains the response body and closes it to allow connection reuse.
func drainAndClose(body io.ReadCloser) {
	if body != nil {
		_, _ = io.Copy(io.Discard, body)
		body.Close()
	}
}

// HealthCheck checks if the Python AI service is healthy.
func (c *Client) HealthCheck(ctx context.Context) error {
	resp, err := c.get(ctx, "/health")
	if err != nil {
		return err
	}
	defer drainAndClose(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health check failed: status %d", resp.StatusCode)
	}

	return nil
}

// OCRExtractRequest is the request body for OCR extraction.
type OCRExtractRequest struct {
	FileKey string `json:"file_key"`
}

// OCRExtractResponse is the response from OCR extraction.
type OCRExtractResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
	FileKey string `json:"file_key"`
	Text    string `json:"text,omitempty"`
	Error   string `json:"error,omitempty"`
}

// ExtractText calls the Python AI service to extract text from a document.
func (c *Client) ExtractText(ctx context.Context, fileKey string) (*OCRExtractResponse, error) {
	path := fmt.Sprintf("/api/v1/ai/documents/extract?file_key=%s", url.QueryEscape(fileKey))

	resp, err := c.post(ctx, path, nil)
	if err != nil {
		return nil, err
	}
	defer drainAndClose(resp.Body)

	var result OCRExtractResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// ClassifyRequest is the request body for document classification.
type ClassifyRequest struct {
	FileKey string `json:"file_key"`
}

// ClassifyResponse is the response from document classification.
type ClassifyResponse struct {
	Status         string  `json:"status"`
	Message        string  `json:"message"`
	FileKey        string  `json:"file_key"`
	Classification string  `json:"classification,omitempty"`
	Confidence     float64 `json:"confidence,omitempty"`
	Error          string  `json:"error,omitempty"`
}

// ClassifyDocument calls the Python AI service to classify a document.
func (c *Client) ClassifyDocument(ctx context.Context, fileKey string) (*ClassifyResponse, error) {
	path := fmt.Sprintf("/api/v1/ai/documents/classify?file_key=%s", url.QueryEscape(fileKey))

	resp, err := c.post(ctx, path, nil)
	if err != nil {
		return nil, err
	}
	defer drainAndClose(resp.Body)

	var result ClassifyResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// ChatRequest is the request body for chat completion.
type ChatRequest struct {
	Message string `json:"message"`
}

// ChatResponse is the response from chat completion.
type ChatResponse struct {
	Status   string `json:"status"`
	Message  string `json:"message"`
	Input    string `json:"input,omitempty"`
	Response string `json:"response,omitempty"`
	Error    string `json:"error,omitempty"`
}

// ChatComplete calls the Python AI service for chat completion.
func (c *Client) ChatComplete(ctx context.Context, message string) (*ChatResponse, error) {
	path := fmt.Sprintf("/api/v1/ai/chat?message=%s", url.QueryEscape(message))

	resp, err := c.post(ctx, path, nil)
	if err != nil {
		return nil, err
	}
	defer drainAndClose(resp.Body)

	var result ChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// FormsExtractRequest is the request body for form extraction.
type FormsExtractRequest struct {
	FileKey string `json:"file_key"`
}

// FormsExtractResponse is the response from form extraction.
type FormsExtractResponse struct {
	Status  string                 `json:"status"`
	Message string                 `json:"message"`
	FileKey string                 `json:"file_key"`
	Fields  map[string]interface{} `json:"fields,omitempty"`
	Error   string                 `json:"error,omitempty"`
}

// ExtractFormData calls the Python AI service to extract form data.
func (c *Client) ExtractFormData(ctx context.Context, fileKey string) (*FormsExtractResponse, error) {
	path := fmt.Sprintf("/api/v1/ai/forms/extract?file_key=%s", url.QueryEscape(fileKey))

	resp, err := c.post(ctx, path, nil)
	if err != nil {
		return nil, err
	}
	defer drainAndClose(resp.Body)

	var result FormsExtractResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// shouldRetry determines if a request should be retried based on the error or response.
func (c *Client) shouldRetry(err error, resp *http.Response) bool {
	if err != nil {
		// Never retry on context cancellation or timeout - the caller wants to stop
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return false
		}

		// Retry on connection errors
		var netErr net.Error
		if errors.As(err, &netErr) && c.retryConfig.RetryOnConn {
			return true
		}

		return c.retryConfig.RetryOnConn
	}

	if resp != nil && c.retryConfig.RetryOn5xx {
		// Retry on 5xx errors (server errors)
		return resp.StatusCode >= 500 && resp.StatusCode < 600
	}

	return false
}

// calculateBackoff returns the delay for the given attempt using exponential backoff.
func (c *Client) calculateBackoff(attempt int) time.Duration {
	delay := c.retryConfig.BaseDelay * time.Duration(1<<uint(attempt))
	if delay > c.retryConfig.MaxDelay {
		delay = c.retryConfig.MaxDelay
	}
	return delay
}

// requestIDKeyType is a private type to prevent context key collisions.
// Fix #28: Using struct type instead of string literal for context keys.
type requestIDKeyType struct{}

// RequestIDKey is the context key for request ID propagation.
// Use this key with context.WithValue to set the request ID.
var RequestIDKey = requestIDKeyType{}

// requestIDHeader is the HTTP header name for request ID propagation.
const requestIDHeader = "X-Request-ID"

// get performs a GET request to the Python AI service with retry logic.
// Propagates X-Request-ID from context for distributed tracing.
func (c *Client) get(ctx context.Context, path string) (*http.Response, error) {
	reqURL := c.baseURL + path

	// Extract request ID from context for distributed tracing
	requestID, _ := ctx.Value(RequestIDKey).(string)

	var lastErr error
	for attempt := 0; attempt <= c.retryConfig.MaxRetries; attempt++ {
		if attempt > 0 {
			// Wait before retry with exponential backoff
			delay := c.calculateBackoff(attempt - 1)
			log.Debug().
				Int("attempt", attempt).
				Dur("delay", delay).
				Str("url", reqURL).
				Str("request_id", requestID).
				Msg("Retrying AI service request")

			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
			}
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		req.Header.Set("Content-Type", "application/json")
		// Propagate request ID for distributed tracing
		if requestID != "" {
			req.Header.Set(requestIDHeader, requestID)
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = err
			if c.shouldRetry(err, nil) && attempt < c.retryConfig.MaxRetries {
				log.Warn().Err(err).Str("url", reqURL).Int("attempt", attempt+1).Msg("AI service request failed, will retry")
				continue
			}
			log.Error().Err(err).Str("url", reqURL).Msg("AI service request failed")
			return nil, fmt.Errorf("request failed: %w", err)
		}

		// Check if we should retry based on response status
		if c.shouldRetry(nil, resp) && attempt < c.retryConfig.MaxRetries {
			// Drain and close body before retry
			_, _ = io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			lastErr = fmt.Errorf("server returned %d", resp.StatusCode)
			log.Warn().Int("status", resp.StatusCode).Str("url", reqURL).Int("attempt", attempt+1).Msg("AI service returned error, will retry")
			continue
		}

		return resp, nil
	}

	return nil, fmt.Errorf("request failed after %d retries: %w", c.retryConfig.MaxRetries, lastErr)
}

// post performs a POST request to the Python AI service with retry logic.
// post performs a POST request to the Python AI service with retry logic.
// Propagates X-Request-ID from context for distributed tracing.
func (c *Client) post(ctx context.Context, path string, body interface{}) (*http.Response, error) {
	reqURL := c.baseURL + path

	// Extract request ID from context for distributed tracing
	requestID, _ := ctx.Value(RequestIDKey).(string)

	// Pre-marshal body for retries
	var jsonBody []byte
	if body != nil {
		var err error
		jsonBody, err = json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
	}

	var lastErr error
	for attempt := 0; attempt <= c.retryConfig.MaxRetries; attempt++ {
		if attempt > 0 {
			// Wait before retry with exponential backoff
			delay := c.calculateBackoff(attempt - 1)
			log.Debug().
				Int("attempt", attempt).
				Dur("delay", delay).
				Str("url", reqURL).
				Str("request_id", requestID).
				Msg("Retrying AI service request")

			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
			}
		}

		var reqBody io.Reader
		if jsonBody != nil {
			reqBody = bytes.NewReader(jsonBody)
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, reqBody)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		req.Header.Set("Content-Type", "application/json")
		// Propagate request ID for distributed tracing
		if requestID != "" {
			req.Header.Set(requestIDHeader, requestID)
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = err
			if c.shouldRetry(err, nil) && attempt < c.retryConfig.MaxRetries {
				log.Warn().Err(err).Str("url", reqURL).Int("attempt", attempt+1).Msg("AI service request failed, will retry")
				continue
			}
			log.Error().Err(err).Str("url", reqURL).Msg("AI service request failed")
			return nil, fmt.Errorf("request failed: %w", err)
		}

		// Check if we should retry based on response status
		if c.shouldRetry(nil, resp) && attempt < c.retryConfig.MaxRetries {
			// Drain and close body before retry
			_, _ = io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			lastErr = fmt.Errorf("server returned %d", resp.StatusCode)
			log.Warn().Int("status", resp.StatusCode).Str("url", reqURL).Int("attempt", attempt+1).Msg("AI service returned error, will retry")
			continue
		}

		return resp, nil
	}

	return nil, fmt.Errorf("request failed after %d retries: %w", c.retryConfig.MaxRetries, lastErr)
}

// =============================================================================
// Email AI Methods
// =============================================================================

// EmailSummarizeRequest is the request body for email summarization.
type EmailSummarizeRequest struct {
	Subject   string `json:"subject"`
	Body      string `json:"body"`
	Sender    string `json:"sender,omitempty"`
	Recipient string `json:"recipient,omitempty"`
	EmailID   string `json:"email_id,omitempty"`
}

// EmailSummarizeResponse is the response from email summarization.
type EmailSummarizeResponse struct {
	Summary        string   `json:"summary"`
	KeyPoints      []string `json:"key_points"`
	ActionRequired bool     `json:"action_required"`
	ActionItems    []string `json:"action_items"`
	Deadline       *string  `json:"deadline"`
	Urgency        string   `json:"urgency"`
	Category       string   `json:"category"`
	EmailID        string   `json:"email_id,omitempty"`
	Error          string   `json:"error,omitempty"`
}

// SummarizeEmail calls the Python AI service to summarize an email.
func (c *Client) SummarizeEmail(ctx context.Context, req EmailSummarizeRequest) (*EmailSummarizeResponse, error) {
	path := fmt.Sprintf("/api/v1/ai/emails/summarize?subject=%s&body=%s&sender=%s&recipient=%s&email_id=%s",
		url.QueryEscape(req.Subject),
		url.QueryEscape(req.Body),
		url.QueryEscape(req.Sender),
		url.QueryEscape(req.Recipient),
		url.QueryEscape(req.EmailID),
	)

	resp, err := c.post(ctx, path, nil)
	if err != nil {
		return nil, err
	}
	defer drainAndClose(resp.Body)

	var result EmailSummarizeResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// EmailSentimentRequest is the request body for email sentiment analysis.
type EmailSentimentRequest struct {
	Subject string `json:"subject"`
	Body    string `json:"body"`
	Sender  string `json:"sender,omitempty"`
	EmailID string `json:"email_id,omitempty"`
}

// EmailSentimentResponse is the response from email sentiment analysis.
type EmailSentimentResponse struct {
	Sentiment         string   `json:"sentiment"`
	SentimentScore    float64  `json:"sentiment_score"`
	Tone              string   `json:"tone"`
	Emotions          []string `json:"emotions"`
	SatisfactionLevel string   `json:"satisfaction_level"`
	RiskIndicators    []string `json:"risk_indicators"`
	RequiresAttention bool     `json:"requires_attention"`
	Confidence        float64  `json:"confidence"`
	EmailID           string   `json:"email_id,omitempty"`
	Error             string   `json:"error,omitempty"`
}

// AnalyzeEmailSentiment calls the Python AI service to analyze email sentiment.
func (c *Client) AnalyzeEmailSentiment(ctx context.Context, req EmailSentimentRequest) (*EmailSentimentResponse, error) {
	path := fmt.Sprintf("/api/v1/ai/emails/sentiment?subject=%s&body=%s&sender=%s&email_id=%s",
		url.QueryEscape(req.Subject),
		url.QueryEscape(req.Body),
		url.QueryEscape(req.Sender),
		url.QueryEscape(req.EmailID),
	)

	resp, err := c.post(ctx, path, nil)
	if err != nil {
		return nil, err
	}
	defer drainAndClose(resp.Body)

	var result EmailSentimentResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// EmailPromisesRequest is the request body for email promise extraction.
type EmailPromisesRequest struct {
	Subject   string `json:"subject"`
	Body      string `json:"body"`
	Sender    string `json:"sender,omitempty"`
	Recipient string `json:"recipient,omitempty"`
	EmailID   string `json:"email_id,omitempty"`
}

// PromisedDocument represents a document promised in an email.
type PromisedDocument struct {
	DocumentType string  `json:"document_type"`
	Description  string  `json:"description"`
	PromisedBy   string  `json:"promised_by"`
	Deadline     *string `json:"deadline"`
	DeadlineText string  `json:"deadline_text"`
}

// PromisedAction represents an action promised in an email.
type PromisedAction struct {
	Action           string  `json:"action"`
	ResponsibleParty string  `json:"responsible_party"`
	Deadline         *string `json:"deadline"`
	DeadlineText     string  `json:"deadline_text"`
}

// RequestedDocument represents a document requested in an email.
type RequestedDocument struct {
	DocumentType  string `json:"document_type"`
	Description   string `json:"description"`
	RequestedFrom string `json:"requested_from"`
}

// EmailPromisesResponse is the response from email promise extraction.
type EmailPromisesResponse struct {
	PromisedDocuments  []PromisedDocument  `json:"promised_documents"`
	PromisedActions    []PromisedAction    `json:"promised_actions"`
	RequestedDocuments []RequestedDocument `json:"requested_documents"`
	HasCommitments     bool                `json:"has_commitments"`
	Urgency            string              `json:"urgency"`
	FollowUpDate       *string             `json:"follow_up_date"`
	EmailID            string              `json:"email_id,omitempty"`
	Error              string              `json:"error,omitempty"`
}

// ExtractEmailPromises calls the Python AI service to extract promises from an email.
func (c *Client) ExtractEmailPromises(ctx context.Context, req EmailPromisesRequest) (*EmailPromisesResponse, error) {
	path := fmt.Sprintf("/api/v1/ai/emails/promises?subject=%s&body=%s&sender=%s&recipient=%s&email_id=%s",
		url.QueryEscape(req.Subject),
		url.QueryEscape(req.Body),
		url.QueryEscape(req.Sender),
		url.QueryEscape(req.Recipient),
		url.QueryEscape(req.EmailID),
	)

	resp, err := c.post(ctx, path, nil)
	if err != nil {
		return nil, err
	}
	defer drainAndClose(resp.Body)

	var result EmailPromisesResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// =============================================================================
// Email Draft AI Methods
// =============================================================================

// EmailDraftRequest is the request body for AI email drafting.
type EmailDraftRequest struct {
	Context                string `json:"context"`
	Tone                   string `json:"tone,omitempty"`
	Intent                 string `json:"intent,omitempty"`
	ClientName             string `json:"client_name,omitempty"`
	StaffName              string `json:"staff_name,omitempty"`
	OriginalSubject        string `json:"original_subject,omitempty"`
	OriginalBody           string `json:"original_body,omitempty"`
	OriginalSender         string `json:"original_sender,omitempty"`
	AdditionalInstructions string `json:"additional_instructions,omitempty"`
	EmailID                string `json:"email_id,omitempty"`
}

// EmailDraftResponse is the response from AI email drafting.
type EmailDraftResponse struct {
	Subject            string   `json:"subject"`
	Body               string   `json:"body"`
	Suggestions        []string `json:"suggestions"`
	ToneAchieved       string   `json:"tone_achieved"`
	WordCount          int      `json:"word_count"`
	ReadingTimeSeconds int      `json:"reading_time_seconds"`
	CallsToAction      []string `json:"calls_to_action"`
	EmailID            string   `json:"email_id,omitempty"`
	Error              string   `json:"error,omitempty"`
}

// DraftEmail calls the Python AI service to generate an email draft.
func (c *Client) DraftEmail(ctx context.Context, req EmailDraftRequest) (*EmailDraftResponse, error) {
	path := fmt.Sprintf("/api/v1/ai/emails/draft?context=%s&tone=%s&intent=%s&client_name=%s&staff_name=%s&original_subject=%s&original_body=%s&original_sender=%s&additional_instructions=%s&email_id=%s",
		url.QueryEscape(req.Context),
		url.QueryEscape(req.Tone),
		url.QueryEscape(req.Intent),
		url.QueryEscape(req.ClientName),
		url.QueryEscape(req.StaffName),
		url.QueryEscape(req.OriginalSubject),
		url.QueryEscape(req.OriginalBody),
		url.QueryEscape(req.OriginalSender),
		url.QueryEscape(req.AdditionalInstructions),
		url.QueryEscape(req.EmailID),
	)

	resp, err := c.post(ctx, path, nil)
	if err != nil {
		return nil, err
	}
	defer drainAndClose(resp.Body)

	var result EmailDraftResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// EmailMatchClientRequest is the request body for matching email to client.
type EmailMatchClientRequest struct {
	SenderEmail  string `json:"sender_email"`
	SenderName   string `json:"sender_name,omitempty"`
	EmailContent string `json:"email_content,omitempty"`
	KnownClients string `json:"known_clients,omitempty"` // JSON string of clients
	EmailID      string `json:"email_id,omitempty"`
}

// AlternateMatch represents an alternative client match.
type AlternateMatch struct {
	ClientID   string  `json:"client_id"`
	ClientName string  `json:"client_name"`
	Confidence float64 `json:"confidence"`
	Reason     string  `json:"reason"`
}

// EmailMatchClientResponse is the response from email-to-client matching.
type EmailMatchClientResponse struct {
	Matched          bool             `json:"matched"`
	ClientID         *string          `json:"client_id"`
	ClientName       *string          `json:"client_name"`
	Confidence       float64          `json:"confidence"`
	MatchReasons     []string         `json:"match_reasons"`
	IsNewContact     bool             `json:"is_new_contact"`
	SuggestedAction  string           `json:"suggested_action"`
	AlternateMatches []AlternateMatch `json:"alternate_matches"`
	EmailID          string           `json:"email_id,omitempty"`
	Error            string           `json:"error,omitempty"`
}

// MatchEmailToClient calls the Python AI service to match an email sender to a client.
func (c *Client) MatchEmailToClient(ctx context.Context, req EmailMatchClientRequest) (*EmailMatchClientResponse, error) {
	path := fmt.Sprintf("/api/v1/ai/emails/match-client?sender_email=%s&sender_name=%s&email_content=%s&known_clients=%s&email_id=%s",
		url.QueryEscape(req.SenderEmail),
		url.QueryEscape(req.SenderName),
		url.QueryEscape(req.EmailContent),
		url.QueryEscape(req.KnownClients),
		url.QueryEscape(req.EmailID),
	)

	resp, err := c.post(ctx, path, nil)
	if err != nil {
		return nil, err
	}
	defer drainAndClose(resp.Body)

	var result EmailMatchClientResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// EmailThreadSummaryRequest is the request body for email thread summarization.
type EmailThreadSummaryRequest struct {
	Emails   string `json:"emails"` // JSON string of emails
	Focus    string `json:"focus,omitempty"`
	ThreadID string `json:"thread_id,omitempty"`
}

// ThreadActionItem represents an action item from a thread.
type ThreadActionItem struct {
	Action  string `json:"action"`
	Owner   string `json:"owner"`
	DueDate string `json:"due_date,omitempty"`
	Status  string `json:"status"`
}

// ThreadDateRange represents the date range of a thread.
type ThreadDateRange struct {
	Start string `json:"start"`
	End   string `json:"end"`
}

// EmailThreadSummaryResponse is the response from email thread summarization.
type EmailThreadSummaryResponse struct {
	ThreadSubject       string             `json:"thread_subject"`
	Summary             string             `json:"summary"`
	Participants        []string           `json:"participants"`
	MessageCount        int                `json:"message_count"`
	DateRange           ThreadDateRange    `json:"date_range"`
	KeyPoints           []string           `json:"key_points"`
	DecisionsMade       []string           `json:"decisions_made"`
	ActionItems         []ThreadActionItem `json:"action_items"`
	DocumentsMentioned  []string           `json:"documents_mentioned"`
	UnresolvedQuestions []string           `json:"unresolved_questions"`
	CurrentStatus       string             `json:"current_status"`
	RecommendedNextStep string             `json:"recommended_next_step"`
	ThreadID            string             `json:"thread_id,omitempty"`
	Error               string             `json:"error,omitempty"`
}

// SummarizeEmailThread calls the Python AI service to summarize an email thread.
func (c *Client) SummarizeEmailThread(ctx context.Context, req EmailThreadSummaryRequest) (*EmailThreadSummaryResponse, error) {
	path := fmt.Sprintf("/api/v1/ai/emails/thread-summary?emails=%s&focus=%s&thread_id=%s",
		url.QueryEscape(req.Emails),
		url.QueryEscape(req.Focus),
		url.QueryEscape(req.ThreadID),
	)

	resp, err := c.post(ctx, path, nil)
	if err != nil {
		return nil, err
	}
	defer drainAndClose(resp.Body)

	var result EmailThreadSummaryResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// FindAlternateEmailRequest is the request body for finding alternate emails.
type FindAlternateEmailRequest struct {
	BouncedEmail  string `json:"bounced_email"`
	ClientName    string `json:"client_name,omitempty"`
	CompanyName   string `json:"company_name,omitempty"`
	KnownContacts string `json:"known_contacts,omitempty"` // JSON string
	CompanyDomain string `json:"company_domain,omitempty"`
	EmailID       string `json:"email_id,omitempty"`
}

// SuggestedContact represents an alternate contact suggestion.
type SuggestedContact struct {
	Name       string  `json:"name"`
	Email      string  `json:"email"`
	Role       string  `json:"role"`
	Confidence float64 `json:"confidence"`
	Reason     string  `json:"reason"`
}

// AlternateAction represents a recommended action for bounced emails.
type AlternateAction struct {
	Action   string `json:"action"`
	Priority string `json:"priority"`
	Details  string `json:"details"`
}

// FindAlternateEmailResponse is the response from finding alternate emails.
type FindAlternateEmailResponse struct {
	SuggestedContacts       []SuggestedContact `json:"suggested_contacts"`
	EmailPatternSuggestions []string           `json:"email_pattern_suggestions"`
	PossibleReasons         []string           `json:"possible_reasons"`
	RecommendedActions      []AlternateAction  `json:"recommended_actions"`
	ShouldFlagForReview     bool               `json:"should_flag_for_review"`
	Urgency                 string             `json:"urgency"`
	EmailID                 string             `json:"email_id,omitempty"`
	Error                   string             `json:"error,omitempty"`
}

// FindAlternateEmail calls the Python AI service to find alternate emails.
func (c *Client) FindAlternateEmail(ctx context.Context, req FindAlternateEmailRequest) (*FindAlternateEmailResponse, error) {
	path := fmt.Sprintf("/api/v1/ai/emails/find-alternate?bounced_email=%s&client_name=%s&company_name=%s&known_contacts=%s&company_domain=%s&email_id=%s",
		url.QueryEscape(req.BouncedEmail),
		url.QueryEscape(req.ClientName),
		url.QueryEscape(req.CompanyName),
		url.QueryEscape(req.KnownContacts),
		url.QueryEscape(req.CompanyDomain),
		url.QueryEscape(req.EmailID),
	)

	resp, err := c.post(ctx, path, nil)
	if err != nil {
		return nil, err
	}
	defer drainAndClose(resp.Body)

	var result FindAlternateEmailResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// =============================================================================
// Risk Analysis AI Methods
// =============================================================================

// ClientRiskRequest is the request body for client risk analysis.
type ClientRiskRequest struct {
	ClientID                 string   `json:"client_id"`
	ClientName               string   `json:"client_name,omitempty"`
	Services                 []string `json:"services,omitempty"`
	LastContactDays          int      `json:"last_contact_days,omitempty"`
	OutstandingInvoices      int      `json:"outstanding_invoices,omitempty"`
	OutstandingAmount        float64  `json:"outstanding_amount,omitempty"`
	EmailSentimentHistory    []string `json:"email_sentiment_history,omitempty"`
	MissedDeadlines          int      `json:"missed_deadlines,omitempty"`
	PaymentDelaysAvg         int      `json:"payment_delays_avg,omitempty"`
	RelationshipLengthMonths int      `json:"relationship_length_months,omitempty"`
}

// RiskFactor represents a factor contributing to risk.
type RiskFactor struct {
	Factor      string  `json:"factor"`
	Severity    string  `json:"severity"`
	Description string  `json:"description"`
	Weight      float64 `json:"weight,omitempty"`
	ImpactDays  int     `json:"impact_days,omitempty"`
}

// RecommendedAction represents a recommended action to mitigate risk.
type RecommendedAction struct {
	Action     string `json:"action"`
	Priority   string `json:"priority"`
	Reason     string `json:"reason,omitempty"`
	AssignedTo string `json:"assigned_to,omitempty"`
}

// ClientRiskResponse is the response from client risk analysis.
type ClientRiskResponse struct {
	RiskLevel          string              `json:"risk_level"`
	RiskScore          float64             `json:"risk_score"`
	ChurnProbability   float64             `json:"churn_probability"`
	RiskFactors        []RiskFactor        `json:"risk_factors"`
	PositiveIndicators []string            `json:"positive_indicators"`
	RecommendedActions []RecommendedAction `json:"recommended_actions"`
	NextContactUrgency string              `json:"next_contact_urgency"`
	Confidence         float64             `json:"confidence"`
	ClientID           string              `json:"client_id,omitempty"`
	Error              string              `json:"error,omitempty"`
}

// AnalyzeClientRisk calls the Python AI service to analyze client churn risk.
func (c *Client) AnalyzeClientRisk(ctx context.Context, req ClientRiskRequest) (*ClientRiskResponse, error) {
	// Build query string
	services := ""
	if len(req.Services) > 0 {
		for i, s := range req.Services {
			if i > 0 {
				services += ","
			}
			services += s
		}
	}

	sentiments := ""
	if len(req.EmailSentimentHistory) > 0 {
		for i, s := range req.EmailSentimentHistory {
			if i > 0 {
				sentiments += ","
			}
			sentiments += s
		}
	}

	path := fmt.Sprintf("/api/v1/ai/risk/client?client_id=%s&client_name=%s&services=%s&last_contact_days=%d&outstanding_invoices=%d&outstanding_amount=%f&email_sentiment_history=%s&missed_deadlines=%d&payment_delays_avg=%d&relationship_length_months=%d",
		url.QueryEscape(req.ClientID),
		url.QueryEscape(req.ClientName),
		url.QueryEscape(services),
		req.LastContactDays,
		req.OutstandingInvoices,
		req.OutstandingAmount,
		url.QueryEscape(sentiments),
		req.MissedDeadlines,
		req.PaymentDelaysAvg,
		req.RelationshipLengthMonths,
	)

	resp, err := c.post(ctx, path, nil)
	if err != nil {
		return nil, err
	}
	defer drainAndClose(resp.Body)

	var result ClientRiskResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// ServiceRiskRequest is the request body for service risk analysis.
type ServiceRiskRequest struct {
	ServiceID            string `json:"service_id"`
	ServiceType          string `json:"service_type,omitempty"`
	ClientName           string `json:"client_name,omitempty"`
	Deadline             string `json:"deadline,omitempty"`
	DaysUntilDeadline    int    `json:"days_until_deadline,omitempty"`
	Status               string `json:"status,omitempty"`
	DocumentsReceived    int    `json:"documents_received,omitempty"`
	DocumentsRequired    int    `json:"documents_required,omitempty"`
	OutstandingQueries   int    `json:"outstanding_queries,omitempty"`
	AssignedStaff        string `json:"assigned_staff,omitempty"`
	Complexity           string `json:"complexity,omitempty"`
	PreviousDelays       bool   `json:"previous_delays,omitempty"`
	ClientResponsiveness string `json:"client_responsiveness,omitempty"`
}

// Blocker represents something blocking service progress.
type Blocker struct {
	Blocker     string `json:"blocker"`
	Owner       string `json:"owner"`
	DaysBlocked int    `json:"days_blocked"`
}

// ServiceRiskResponse is the response from service risk analysis.
type ServiceRiskResponse struct {
	RiskLevel            string              `json:"risk_level"`
	RiskScore            float64             `json:"risk_score"`
	OnTimeProbability    float64             `json:"on_time_probability"`
	DaysBuffer           int                 `json:"days_buffer"`
	RiskFactors          []RiskFactor        `json:"risk_factors"`
	Blockers             []Blocker           `json:"blockers"`
	RecommendedActions   []RecommendedAction `json:"recommended_actions"`
	EscalationNeeded     bool                `json:"escalation_needed"`
	SuggestedNewDeadline *string             `json:"suggested_new_deadline"`
	Confidence           float64             `json:"confidence"`
	ServiceID            string              `json:"service_id,omitempty"`
	Error                string              `json:"error,omitempty"`
}

// AnalyzeServiceRisk calls the Python AI service to analyze service deadline risk.
func (c *Client) AnalyzeServiceRisk(ctx context.Context, req ServiceRiskRequest) (*ServiceRiskResponse, error) {
	path := fmt.Sprintf("/api/v1/ai/risk/service?service_id=%s&service_type=%s&client_name=%s&deadline=%s&days_until_deadline=%d&service_status=%s&documents_received=%d&documents_required=%d&outstanding_queries=%d&assigned_staff=%s&complexity=%s&previous_delays=%t&client_responsiveness=%s",
		url.QueryEscape(req.ServiceID),
		url.QueryEscape(req.ServiceType),
		url.QueryEscape(req.ClientName),
		url.QueryEscape(req.Deadline),
		req.DaysUntilDeadline,
		url.QueryEscape(req.Status),
		req.DocumentsReceived,
		req.DocumentsRequired,
		req.OutstandingQueries,
		url.QueryEscape(req.AssignedStaff),
		url.QueryEscape(req.Complexity),
		req.PreviousDelays,
		url.QueryEscape(req.ClientResponsiveness),
	)

	resp, err := c.post(ctx, path, nil)
	if err != nil {
		return nil, err
	}
	defer drainAndClose(resp.Body)

	var result ServiceRiskResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// =============================================================================
// Form Auto-Fill AI Methods
// =============================================================================

// VATAutoFillRequest is the request body for VAT auto-fill.
type VATAutoFillRequest struct {
	ClientID       string  `json:"client_id"`
	Period         string  `json:"period"`
	ClientName     string  `json:"client_name,omitempty"`
	VATNumber      string  `json:"vat_number,omitempty"`
	TotalSales     float64 `json:"total_sales,omitempty"`
	TotalPurchases float64 `json:"total_purchases,omitempty"`
	VATOnSales     float64 `json:"vat_on_sales,omitempty"`
	VATOnPurchases float64 `json:"vat_on_purchases,omitempty"`
	EUAcquisitions float64 `json:"eu_acquisitions,omitempty"`
	EUSupplies     float64 `json:"eu_supplies,omitempty"`
}

// VATReturn represents the VAT return boxes.
type VATReturn struct {
	Box1 float64 `json:"box_1"`
	Box2 float64 `json:"box_2"`
	Box3 float64 `json:"box_3"`
	Box4 float64 `json:"box_4"`
	Box5 float64 `json:"box_5"`
	Box6 float64 `json:"box_6"`
	Box7 float64 `json:"box_7"`
	Box8 float64 `json:"box_8"`
	Box9 float64 `json:"box_9"`
}

// VATSummary represents the VAT return summary.
type VATSummary struct {
	TotalSales     float64 `json:"total_sales"`
	TotalPurchases float64 `json:"total_purchases"`
	VATCollected   float64 `json:"vat_collected"`
	VATReclaimed   float64 `json:"vat_reclaimed"`
	NetVAT         float64 `json:"net_vat"`
	PaymentDue     bool    `json:"payment_due"`
}

// VATAutoFillResponse is the response from VAT auto-fill.
type VATAutoFillResponse struct {
	VATReturn   VATReturn  `json:"vat_return"`
	Summary     VATSummary `json:"summary"`
	Warnings    []string   `json:"warnings"`
	MissingData []string   `json:"missing_data"`
	Confidence  float64    `json:"confidence"`
	ClientID    string     `json:"client_id,omitempty"`
	Period      string     `json:"period,omitempty"`
	Error       string     `json:"error,omitempty"`
}

// AutoFillVAT calls the Python AI service to auto-fill VAT return.
func (c *Client) AutoFillVAT(ctx context.Context, req VATAutoFillRequest) (*VATAutoFillResponse, error) {
	path := fmt.Sprintf("/api/v1/ai/forms/vat?client_id=%s&period=%s&client_name=%s&vat_number=%s&total_sales=%f&total_purchases=%f&vat_on_sales=%f&vat_on_purchases=%f&eu_acquisitions=%f&eu_supplies=%f",
		url.QueryEscape(req.ClientID),
		url.QueryEscape(req.Period),
		url.QueryEscape(req.ClientName),
		url.QueryEscape(req.VATNumber),
		req.TotalSales,
		req.TotalPurchases,
		req.VATOnSales,
		req.VATOnPurchases,
		req.EUAcquisitions,
		req.EUSupplies,
	)

	resp, err := c.post(ctx, path, nil)
	if err != nil {
		return nil, err
	}
	defer drainAndClose(resp.Body)

	var result VATAutoFillResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// CT600AutoFillRequest is the request body for CT600 auto-fill.
type CT600AutoFillRequest struct {
	ClientID         string  `json:"client_id"`
	Year             int     `json:"year"`
	CompanyName      string  `json:"company_name,omitempty"`
	CompanyNumber    string  `json:"company_number,omitempty"`
	UTR              string  `json:"utr,omitempty"`
	Turnover         float64 `json:"turnover,omitempty"`
	CostOfSales      float64 `json:"cost_of_sales,omitempty"`
	GrossProfit      float64 `json:"gross_profit,omitempty"`
	AdminExpenses    float64 `json:"admin_expenses,omitempty"`
	Depreciation     float64 `json:"depreciation,omitempty"`
	InterestReceived float64 `json:"interest_received,omitempty"`
	InterestPaid     float64 `json:"interest_paid,omitempty"`
	OtherIncome      float64 `json:"other_income,omitempty"`
}

// CT600Data represents the CT600 return data.
type CT600Data struct {
	Turnover          float64                `json:"turnover"`
	TradingProfit     float64                `json:"trading_profit"`
	OtherIncome       float64                `json:"other_income"`
	TotalProfits      float64                `json:"total_profits"`
	TaxAdjustments    map[string]float64     `json:"tax_adjustments"`
	CapitalAllowances float64                `json:"capital_allowances"`
	AdjustedProfit    float64                `json:"adjusted_profit"`
	LossesBroughtFwd  float64                `json:"losses_brought_forward"`
	TaxableProfit     float64                `json:"taxable_profit"`
	CorporationTax    float64                `json:"corporation_tax"`
	EffectiveRate     float64                `json:"effective_rate"`
}

// CT600Summary represents the CT600 summary.
type CT600Summary struct {
	ProfitBeforeTax float64 `json:"profit_before_tax"`
	TaxableProfit   float64 `json:"taxable_profit"`
	TaxDue          float64 `json:"tax_due"`
	PaymentDeadline string  `json:"payment_deadline"`
}

// CT600AutoFillResponse is the response from CT600 auto-fill.
type CT600AutoFillResponse struct {
	CT600       CT600Data    `json:"ct600"`
	Summary     CT600Summary `json:"summary"`
	Warnings    []string     `json:"warnings"`
	MissingData []string     `json:"missing_data"`
	Confidence  float64      `json:"confidence"`
	ClientID    string       `json:"client_id,omitempty"`
	Year        int          `json:"year,omitempty"`
	Error       string       `json:"error,omitempty"`
}

// AutoFillCT600 calls the Python AI service to auto-fill CT600 return.
func (c *Client) AutoFillCT600(ctx context.Context, req CT600AutoFillRequest) (*CT600AutoFillResponse, error) {
	path := fmt.Sprintf("/api/v1/ai/forms/ct600?client_id=%s&year=%d&company_name=%s&company_number=%s&utr=%s&turnover=%f&cost_of_sales=%f&gross_profit=%f&admin_expenses=%f&depreciation=%f&interest_received=%f&interest_paid=%f&other_income=%f",
		url.QueryEscape(req.ClientID),
		req.Year,
		url.QueryEscape(req.CompanyName),
		url.QueryEscape(req.CompanyNumber),
		url.QueryEscape(req.UTR),
		req.Turnover,
		req.CostOfSales,
		req.GrossProfit,
		req.AdminExpenses,
		req.Depreciation,
		req.InterestReceived,
		req.InterestPaid,
		req.OtherIncome,
	)

	resp, err := c.post(ctx, path, nil)
	if err != nil {
		return nil, err
	}
	defer drainAndClose(resp.Body)

	var result CT600AutoFillResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// SAAutoFillRequest is the request body for Self Assessment auto-fill.
type SAAutoFillRequest struct {
	ClientID               string  `json:"client_id"`
	TaxYear                string  `json:"tax_year"`
	TaxpayerName           string  `json:"taxpayer_name,omitempty"`
	UTR                    string  `json:"utr,omitempty"`
	NINumber               string  `json:"ni_number,omitempty"`
	EmploymentIncome       float64 `json:"employment_income,omitempty"`
	SelfEmploymentIncome   float64 `json:"self_employment_income,omitempty"`
	SelfEmploymentExpenses float64 `json:"self_employment_expenses,omitempty"`
	PropertyIncome         float64 `json:"property_income,omitempty"`
	PropertyExpenses       float64 `json:"property_expenses,omitempty"`
	DividendIncome         float64 `json:"dividend_income,omitempty"`
	InterestIncome         float64 `json:"interest_income,omitempty"`
	PensionContributions   float64 `json:"pension_contributions,omitempty"`
	GiftAid                float64 `json:"gift_aid,omitempty"`
}

// SelfAssessmentData represents the Self Assessment return data.
type SelfAssessmentData struct {
	EmploymentIncome    float64 `json:"employment_income"`
	SelfEmploymentProfit float64 `json:"self_employment_profit"`
	PropertyIncome      float64 `json:"property_income"`
	DividendIncome      float64 `json:"dividend_income"`
	InterestIncome      float64 `json:"interest_income"`
	TotalIncome         float64 `json:"total_income"`
	PersonalAllowance   float64 `json:"personal_allowance"`
	TaxableIncome       float64 `json:"taxable_income"`
	PensionRelief       float64 `json:"pension_relief"`
	GiftAidRelief       float64 `json:"gift_aid_relief"`
}

// TaxCalculation represents the tax calculation.
type TaxCalculation struct {
	IncomeTax         float64            `json:"income_tax"`
	DividendTax       float64            `json:"dividend_tax"`
	NationalInsurance map[string]float64 `json:"national_insurance"`
	TotalTaxDue       float64            `json:"total_tax_due"`
	TaxAlreadyPaid    float64            `json:"tax_already_paid"`
	BalanceDue        float64            `json:"balance_due"`
}

// SAAutoFillResponse is the response from Self Assessment auto-fill.
type SAAutoFillResponse struct {
	SelfAssessment           SelfAssessmentData `json:"self_assessment"`
	TaxCalculation           TaxCalculation     `json:"tax_calculation"`
	SupplementaryPagesNeeded []string           `json:"supplementary_pages_needed"`
	Warnings                 []string           `json:"warnings"`
	MissingData              []string           `json:"missing_data"`
	Confidence               float64            `json:"confidence"`
	ClientID                 string             `json:"client_id,omitempty"`
	TaxYear                  string             `json:"tax_year,omitempty"`
	Error                    string             `json:"error,omitempty"`
}

// AutoFillSA calls the Python AI service to auto-fill Self Assessment return.
func (c *Client) AutoFillSA(ctx context.Context, req SAAutoFillRequest) (*SAAutoFillResponse, error) {
	path := fmt.Sprintf("/api/v1/ai/forms/sa?client_id=%s&tax_year=%s&taxpayer_name=%s&utr=%s&ni_number=%s&employment_income=%f&self_employment_income=%f&self_employment_expenses=%f&property_income=%f&property_expenses=%f&dividend_income=%f&interest_income=%f&pension_contributions=%f&gift_aid=%f",
		url.QueryEscape(req.ClientID),
		url.QueryEscape(req.TaxYear),
		url.QueryEscape(req.TaxpayerName),
		url.QueryEscape(req.UTR),
		url.QueryEscape(req.NINumber),
		req.EmploymentIncome,
		req.SelfEmploymentIncome,
		req.SelfEmploymentExpenses,
		req.PropertyIncome,
		req.PropertyExpenses,
		req.DividendIncome,
		req.InterestIncome,
		req.PensionContributions,
		req.GiftAid,
	)

	resp, err := c.post(ctx, path, nil)
	if err != nil {
		return nil, err
	}
	defer drainAndClose(resp.Body)

	var result SAAutoFillResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// =============================================================================
// Document Rename AI Methods
// =============================================================================

// DocumentRenameRequest is the request body for document rename suggestion.
type DocumentRenameRequest struct {
	Text             string `json:"text"`
	OriginalFilename string `json:"original_filename,omitempty"`
	DocumentType     string `json:"document_type,omitempty"`
	ClientName       string `json:"client_name,omitempty"`
	FileKey          string `json:"file_key,omitempty"`
}

// KeyIdentifiers represents key identifiers extracted from the document.
type KeyIdentifiers struct {
	InvoiceNumber string `json:"invoice_number,omitempty"`
	Vendor        string `json:"vendor,omitempty"`
	Amount        string `json:"amount,omitempty"`
	Period        string `json:"period,omitempty"`
	Reference     string `json:"reference,omitempty"`
}

// DocumentRenameResponse is the response from document rename suggestion.
type DocumentRenameResponse struct {
	SuggestedName    string         `json:"suggested_name"`
	Extension        string         `json:"extension"`
	FullFilename     string         `json:"full_filename"`
	DocumentType     string         `json:"document_type"`
	KeyDate          string         `json:"key_date"`
	KeyIdentifiers   KeyIdentifiers `json:"key_identifiers"`
	AlternativeNames []string       `json:"alternative_names"`
	Confidence       float64        `json:"confidence"`
	FileKey          string         `json:"file_key,omitempty"`
	Error            string         `json:"error,omitempty"`
}

// SuggestDocumentName calls the Python AI service to suggest a document name.
func (c *Client) SuggestDocumentName(ctx context.Context, req DocumentRenameRequest) (*DocumentRenameResponse, error) {
	path := fmt.Sprintf("/api/v1/ai/documents/rename?text=%s&original_filename=%s&document_type=%s&client_name=%s&file_key=%s",
		url.QueryEscape(req.Text),
		url.QueryEscape(req.OriginalFilename),
		url.QueryEscape(req.DocumentType),
		url.QueryEscape(req.ClientName),
		url.QueryEscape(req.FileKey),
	)

	resp, err := c.post(ctx, path, nil)
	if err != nil {
		return nil, err
	}
	defer drainAndClose(resp.Body)

	var result DocumentRenameResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// =============================================================================
// Chat History Methods (MongoDB)
// =============================================================================

// ChatHistoryRequest is the request for getting chat history.
type ChatHistoryRequest struct {
	UserID   string `json:"user_id"`
	TenantID string `json:"tenant_id,omitempty"`
	Limit    int    `json:"limit,omitempty"`
	Offset   int    `json:"offset,omitempty"`
}

// ChatMessage represents a single chat message.
type ChatMessage struct {
	Role      string                 `json:"role"`
	Content   string                 `json:"content"`
	Timestamp string                 `json:"timestamp"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// Conversation represents a chat conversation.
type Conversation struct {
	ID             string        `json:"_id,omitempty"`
	ConversationID string        `json:"conversation_id"`
	UserID         string        `json:"user_id"`
	TenantID       string        `json:"tenant_id,omitempty"`
	Messages       []ChatMessage `json:"messages"`
	CreatedAt      string        `json:"created_at"`
	UpdatedAt      string        `json:"updated_at"`
}

// ChatHistoryResponse is the response from getting chat history.
type ChatHistoryResponse struct {
	Conversations []Conversation `json:"conversations"`
	Total         int            `json:"total"`
	Limit         int            `json:"limit"`
	Offset        int            `json:"offset"`
	HasMore       bool           `json:"has_more"`
	Error         string         `json:"error,omitempty"`
}

// GetChatHistory calls the Python AI service to get chat history.
func (c *Client) GetChatHistory(ctx context.Context, req ChatHistoryRequest) (*ChatHistoryResponse, error) {
	limit := req.Limit
	if limit == 0 {
		limit = 50
	}

	path := fmt.Sprintf("/api/v1/ai/chat/history?user_id=%s&tenant_id=%s&limit=%d&offset=%d",
		url.QueryEscape(req.UserID),
		url.QueryEscape(req.TenantID),
		limit,
		req.Offset,
	)

	resp, err := c.get(ctx, path)
	if err != nil {
		return nil, err
	}
	defer drainAndClose(resp.Body)

	var result ChatHistoryResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// SaveChatMessageRequest is the request for saving a chat message.
type SaveChatMessageRequest struct {
	UserID         string `json:"user_id"`
	ConversationID string `json:"conversation_id,omitempty"`
	Role           string `json:"role"`
	Content        string `json:"content"`
	TenantID       string `json:"tenant_id,omitempty"`
	Metadata       string `json:"metadata,omitempty"`
}

// SaveChatMessageResponse is the response from saving a chat message.
type SaveChatMessageResponse struct {
	ConversationID string `json:"conversation_id"`
	MessageAdded   bool   `json:"message_added,omitempty"`
	Created        bool   `json:"created,omitempty"`
	CreatedAt      string `json:"created_at,omitempty"`
	UpdatedAt      string `json:"updated_at,omitempty"`
	Error          string `json:"error,omitempty"`
}

// SaveChatMessage calls the Python AI service to save a chat message.
func (c *Client) SaveChatMessage(ctx context.Context, req SaveChatMessageRequest) (*SaveChatMessageResponse, error) {
	path := fmt.Sprintf("/api/v1/ai/chat/history?user_id=%s&conversation_id=%s&role=%s&content=%s&tenant_id=%s&metadata=%s",
		url.QueryEscape(req.UserID),
		url.QueryEscape(req.ConversationID),
		url.QueryEscape(req.Role),
		url.QueryEscape(req.Content),
		url.QueryEscape(req.TenantID),
		url.QueryEscape(req.Metadata),
	)

	resp, err := c.post(ctx, path, nil)
	if err != nil {
		return nil, err
	}
	defer drainAndClose(resp.Body)

	var result SaveChatMessageResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// DeleteChatRequest is the request for deleting a chat conversation.
type DeleteChatRequest struct {
	ConversationID string `json:"conversation_id"`
	UserID         string `json:"user_id"`
}

// DeleteChatResponse is the response from deleting a chat conversation.
type DeleteChatResponse struct {
	ConversationID string `json:"conversation_id"`
	Deleted        bool   `json:"deleted"`
	Error          string `json:"error,omitempty"`
}

// DeleteChat calls the Python AI service to delete a chat conversation.
func (c *Client) DeleteChat(ctx context.Context, req DeleteChatRequest) (*DeleteChatResponse, error) {
	path := fmt.Sprintf("/api/v1/ai/chat/%s?user_id=%s",
		url.QueryEscape(req.ConversationID),
		url.QueryEscape(req.UserID),
	)

	resp, err := c.delete(ctx, path)
	if err != nil {
		return nil, err
	}
	defer drainAndClose(resp.Body)

	var result DeleteChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// delete performs a DELETE request to the Python AI service with retry logic.
func (c *Client) delete(ctx context.Context, path string) (*http.Response, error) {
	reqURL := c.baseURL + path

	requestID, _ := ctx.Value(RequestIDKey).(string)

	var lastErr error
	for attempt := 0; attempt <= c.retryConfig.MaxRetries; attempt++ {
		if attempt > 0 {
			delay := c.calculateBackoff(attempt - 1)
			log.Debug().
				Int("attempt", attempt).
				Dur("delay", delay).
				Str("url", reqURL).
				Str("request_id", requestID).
				Msg("Retrying AI service request")

			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
			}
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodDelete, reqURL, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		req.Header.Set("Content-Type", "application/json")
		if requestID != "" {
			req.Header.Set(requestIDHeader, requestID)
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = err
			if c.shouldRetry(err, nil) && attempt < c.retryConfig.MaxRetries {
				log.Warn().Err(err).Str("url", reqURL).Int("attempt", attempt+1).Msg("AI service request failed, will retry")
				continue
			}
			log.Error().Err(err).Str("url", reqURL).Msg("AI service request failed")
			return nil, fmt.Errorf("request failed: %w", err)
		}

		if c.shouldRetry(nil, resp) && attempt < c.retryConfig.MaxRetries {
			_, _ = io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			lastErr = fmt.Errorf("server returned %d", resp.StatusCode)
			log.Warn().Int("status", resp.StatusCode).Str("url", reqURL).Int("attempt", attempt+1).Msg("AI service returned error, will retry")
			continue
		}

		return resp, nil
	}

	return nil, fmt.Errorf("request failed after %d retries: %w", c.retryConfig.MaxRetries, lastErr)
}

// =============================================================================
// Template AI Methods
// =============================================================================

// GenerateTemplateRequest is the request body for template generation.
type GenerateTemplateRequest struct {
	Purpose             string `json:"purpose"`
	TemplateType        string `json:"template_type,omitempty"`
	Tone                string `json:"tone,omitempty"`
	IncludePlaceholders bool   `json:"include_placeholders,omitempty"`
	ExampleContext      string `json:"example_context,omitempty"`
}

// GenerateTemplateResponse is the response from template generation.
type GenerateTemplateResponse struct {
	Name                    string   `json:"name"`
	Description             string   `json:"description"`
	Subject                 string   `json:"subject"`
	Body                    string   `json:"body"`
	PlaceholdersUsed        []string `json:"placeholders_used"`
	SuggestedAttachments    []string `json:"suggested_attachments"`
	Category                string   `json:"category"`
	ToneAchieved            string   `json:"tone_achieved"`
	EstimatedReadTimeSeconds int      `json:"estimated_read_time_seconds"`
	Error                   string   `json:"error,omitempty"`
}

// GenerateTemplate calls the Python AI service to generate an email template.
func (c *Client) GenerateTemplate(ctx context.Context, req GenerateTemplateRequest) (*GenerateTemplateResponse, error) {
	includePlaceholders := "true"
	if !req.IncludePlaceholders {
		includePlaceholders = "false"
	}
	path := fmt.Sprintf("/api/v1/ai/templates/generate?purpose=%s&template_type=%s&tone=%s&include_placeholders=%s&example_context=%s",
		url.QueryEscape(req.Purpose),
		url.QueryEscape(req.TemplateType),
		url.QueryEscape(req.Tone),
		includePlaceholders,
		url.QueryEscape(req.ExampleContext),
	)

	resp, err := c.post(ctx, path, nil)
	if err != nil {
		return nil, err
	}
	defer drainAndClose(resp.Body)

	var result GenerateTemplateResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// =============================================================================
// Client AI Methods
// =============================================================================

// CheckDuplicateClientsRequest is the request body for duplicate checking.
type CheckDuplicateClientsRequest struct {
	NewClient       string `json:"new_client"`       // JSON string
	ExistingClients string `json:"existing_clients"` // JSON string
}

// PotentialMatch represents a potential duplicate match.
type PotentialMatch struct {
	ClientID       string   `json:"client_id"`
	ClientName     string   `json:"client_name"`
	MatchScore     float64  `json:"match_score"`
	MatchReasons   []string `json:"match_reasons"`
	Differences    []string `json:"differences"`
	Recommendation string   `json:"recommendation"`
}

// MergeSuggestion represents merge suggestions for duplicates.
type MergeSuggestion struct {
	PreferredRecordID string            `json:"preferred_record_id"`
	FieldsToUpdate    map[string]string `json:"fields_to_update"`
}

// CheckDuplicateClientsResponse is the response from duplicate checking.
type CheckDuplicateClientsResponse struct {
	IsDuplicate       bool             `json:"is_duplicate"`
	Confidence        float64          `json:"confidence"`
	PotentialMatches  []PotentialMatch `json:"potential_matches"`
	DuplicateType     string           `json:"duplicate_type"`
	RecommendedAction string           `json:"recommended_action"`
	FieldsToReview    []string         `json:"fields_to_review"`
	MergeSuggestions  *MergeSuggestion `json:"merge_suggestions,omitempty"`
	Error             string           `json:"error,omitempty"`
}

// CheckDuplicateClients calls the Python AI service to check for duplicate clients.
func (c *Client) CheckDuplicateClients(ctx context.Context, req CheckDuplicateClientsRequest) (*CheckDuplicateClientsResponse, error) {
	path := fmt.Sprintf("/api/v1/ai/clients/duplicate-check?new_client=%s&existing_clients=%s",
		url.QueryEscape(req.NewClient),
		url.QueryEscape(req.ExistingClients),
	)

	resp, err := c.post(ctx, path, nil)
	if err != nil {
		return nil, err
	}
	defer drainAndClose(resp.Body)

	var result CheckDuplicateClientsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// =============================================================================
// Service AI Methods
// =============================================================================

// AutoNameServiceRequest is the request body for service auto-naming.
type AutoNameServiceRequest struct {
	ServiceType       string `json:"service_type"`
	ClientName        string `json:"client_name"`
	Period            string `json:"period,omitempty"`
	Year              string `json:"year,omitempty"`
	AdditionalContext string `json:"additional_context,omitempty"`
}

// AutoNameServiceResponse is the response from service auto-naming.
type AutoNameServiceResponse struct {
	SuggestedName   string   `json:"suggested_name"`
	DisplayName     string   `json:"display_name"`
	Alternatives    []string `json:"alternatives"`
	PeriodFormatted string   `json:"period_formatted"`
	YearFormatted   string   `json:"year_formatted"`
	DeadlineHint    string   `json:"deadline_hint"`
	Category        string   `json:"category"`
	Error           string   `json:"error,omitempty"`
}

// AutoNameService calls the Python AI service to auto-generate a service name.
func (c *Client) AutoNameService(ctx context.Context, req AutoNameServiceRequest) (*AutoNameServiceResponse, error) {
	path := fmt.Sprintf("/api/v1/ai/services/auto-name?service_type=%s&client_name=%s&period=%s&year=%s&additional_context=%s",
		url.QueryEscape(req.ServiceType),
		url.QueryEscape(req.ClientName),
		url.QueryEscape(req.Period),
		url.QueryEscape(req.Year),
		url.QueryEscape(req.AdditionalContext),
	)

	resp, err := c.post(ctx, path, nil)
	if err != nil {
		return nil, err
	}
	defer drainAndClose(resp.Body)

	var result AutoNameServiceResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// CompletionSummaryRequest is the request body for completion summary generation.
type CompletionSummaryRequest struct {
	ServiceType    string `json:"service_type"`
	ClientName     string `json:"client_name"`
	CompletionData string `json:"completion_data"` // JSON string
	ServiceID      string `json:"service_id,omitempty"`
}

// KeyOutcome represents a key outcome from service completion.
type KeyOutcome struct {
	Label string `json:"label"`
	Value string `json:"value"`
	Note  string `json:"note,omitempty"`
}

// CompletionSummaryResponse is the response from completion summary generation.
type CompletionSummaryResponse struct {
	InternalSummary     string       `json:"internal_summary"`
	ClientSummary       string       `json:"client_summary"`
	SubjectLine         string       `json:"subject_line"`
	KeyOutcomes         []KeyOutcome `json:"key_outcomes"`
	FollowUpItems       []string     `json:"follow_up_items"`
	DocumentsToSend     []string     `json:"documents_to_send"`
	NextServiceReminder string       `json:"next_service_reminder"`
	ProfessionalNotes   string       `json:"professional_notes"`
	Tone                string       `json:"tone"`
	ServiceID           string       `json:"service_id,omitempty"`
	Error               string       `json:"error,omitempty"`
}

// GenerateCompletionSummary calls the Python AI service to generate completion summary.
func (c *Client) GenerateCompletionSummary(ctx context.Context, req CompletionSummaryRequest) (*CompletionSummaryResponse, error) {
	path := fmt.Sprintf("/api/v1/ai/services/completion-summary?service_type=%s&client_name=%s&completion_data=%s&service_id=%s",
		url.QueryEscape(req.ServiceType),
		url.QueryEscape(req.ClientName),
		url.QueryEscape(req.CompletionData),
		url.QueryEscape(req.ServiceID),
	)

	resp, err := c.post(ctx, path, nil)
	if err != nil {
		return nil, err
	}
	defer drainAndClose(resp.Body)

	var result CompletionSummaryResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// =============================================================================
// Dashboard AI Methods
// =============================================================================

// TroublemakersRequest is the request body for finding troublemakers.
type TroublemakersRequest struct {
	Clients               string `json:"clients"` // JSON string
	ThresholdDaysOverdue  int    `json:"threshold_days_overdue,omitempty"`
}

// TroublemakerIssue represents an issue with a troublemaker client.
type TroublemakerIssue struct {
	Type        string `json:"type"`
	Description string `json:"description"`
	Urgency     string `json:"urgency"`
}

// Troublemaker represents a problematic client.
type Troublemaker struct {
	ClientID          string              `json:"client_id"`
	ClientName        string              `json:"client_name"`
	CompanyName       string              `json:"company_name"`
	Severity          string              `json:"severity"`
	Score             int                 `json:"score"`
	Issues            []TroublemakerIssue `json:"issues"`
	RecommendedAction string              `json:"recommended_action"`
	ChaseMethod       string              `json:"chase_method"`
	ContactPreference string              `json:"contact_preference"`
	LastChase         string              `json:"last_chase"`
	EscalationNeeded  bool                `json:"escalation_needed"`
}

// TroublemakersSummary represents summary statistics.
type TroublemakersSummary struct {
	TotalTroublemakers    int `json:"total_troublemakers"`
	Critical              int `json:"critical"`
	High                  int `json:"high"`
	Medium                int `json:"medium"`
	BlockedServices       int `json:"blocked_services"`
	TotalOverdueDocuments int `json:"total_overdue_documents"`
}

// BatchAction represents a recommended batch action.
type BatchAction struct {
	Action        string `json:"action"`
	TargetClients int    `json:"target_clients"`
	Template      string `json:"template"`
}

// TroublemakersResponse is the response from troublemakers analysis.
type TroublemakersResponse struct {
	Troublemakers           []Troublemaker       `json:"troublemakers"`
	Summary                 TroublemakersSummary `json:"summary"`
	RecommendedBatchActions []BatchAction        `json:"recommended_batch_actions"`
	Error                   string               `json:"error,omitempty"`
}

// FindTroublemakers calls the Python AI service to find troublemaker clients.
func (c *Client) FindTroublemakers(ctx context.Context, req TroublemakersRequest) (*TroublemakersResponse, error) {
	threshold := 7
	if req.ThresholdDaysOverdue > 0 {
		threshold = req.ThresholdDaysOverdue
	}
	path := fmt.Sprintf("/api/v1/ai/dashboard/troublemakers?clients=%s&threshold_days_overdue=%d",
		url.QueryEscape(req.Clients),
		threshold,
	)

	resp, err := c.post(ctx, path, nil)
	if err != nil {
		return nil, err
	}
	defer drainAndClose(resp.Body)

	var result TroublemakersResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// AnomaliesRequest is the request body for anomaly detection.
type AnomaliesRequest struct {
	DataType string `json:"data_type"`
	Data     string `json:"data"` // JSON string
	Context  string `json:"context,omitempty"`
}

// Anomaly represents a detected anomaly.
type Anomaly struct {
	ID                     string  `json:"id"`
	Field                  string  `json:"field"`
	Value                  string  `json:"value"`
	ExpectedRange          string  `json:"expected_range"`
	Severity               string  `json:"severity"`
	Type                   string  `json:"type"`
	Description            string  `json:"description"`
	RecommendedAction      string  `json:"recommended_action"`
	FalsePositiveLikelihood float64 `json:"false_positive_likelihood"`
}

// Pattern represents a detected pattern.
type Pattern struct {
	Pattern         string `json:"pattern"`
	AffectedRecords int    `json:"affected_records"`
	Significance    string `json:"significance"`
}

// AnomaliesResponse is the response from anomaly detection.
type AnomaliesResponse struct {
	AnomaliesFound   bool      `json:"anomalies_found"`
	AnomalyCount     int       `json:"anomaly_count"`
	Anomalies        []Anomaly `json:"anomalies"`
	PatternsDetected []Pattern `json:"patterns_detected"`
	DataQualityScore float64   `json:"data_quality_score"`
	Summary          string    `json:"summary"`
	Error            string    `json:"error,omitempty"`
}

// DetectAnomalies calls the Python AI service to detect anomalies.
func (c *Client) DetectAnomalies(ctx context.Context, req AnomaliesRequest) (*AnomaliesResponse, error) {
	path := fmt.Sprintf("/api/v1/ai/dashboard/anomalies?data_type=%s&data=%s&context=%s",
		url.QueryEscape(req.DataType),
		url.QueryEscape(req.Data),
		url.QueryEscape(req.Context),
	)

	resp, err := c.post(ctx, path, nil)
	if err != nil {
		return nil, err
	}
	defer drainAndClose(resp.Body)

	var result AnomaliesResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// StaffActivityRequest is the request body for staff activity analysis.
type StaffActivityRequest struct {
	StaffID      string `json:"staff_id"`
	StaffName    string `json:"staff_name"`
	ActivityData string `json:"activity_data"` // JSON string
	Period       string `json:"period,omitempty"`
}

// ProductivityMetrics represents productivity statistics.
type ProductivityMetrics struct {
	ServicesCompleted     int     `json:"services_completed"`
	DocumentsProcessed    int     `json:"documents_processed"`
	EmailsHandled         int     `json:"emails_handled"`
	AverageCompletionTime string  `json:"average_completion_time"`
	EfficiencyScore       float64 `json:"efficiency_score"`
}

// ClientInteractionMetrics represents client interaction statistics.
type ClientInteractionMetrics struct {
	TotalInteractions      int    `json:"total_interactions"`
	ClientsServed          int    `json:"clients_served"`
	AverageResponseTime    string `json:"average_response_time"`
	SatisfactionIndicators string `json:"satisfaction_indicators"`
}

// TimeAllocation represents time allocation breakdown.
type TimeAllocation struct {
	ServiceWork         string `json:"service_work"`
	ClientCommunication string `json:"client_communication"`
	Admin               string `json:"admin"`
}

// StaffActivityResponse is the response from staff activity analysis.
type StaffActivityResponse struct {
	StaffID              string                   `json:"staff_id"`
	StaffName            string                   `json:"staff_name"`
	Period               string                   `json:"period"`
	Summary              string                   `json:"summary"`
	Productivity         ProductivityMetrics      `json:"productivity"`
	ClientInteractions   ClientInteractionMetrics `json:"client_interactions"`
	TimeAllocation       TimeAllocation           `json:"time_allocation"`
	Highlights           []string                 `json:"highlights"`
	AreasForImprovement  []string                 `json:"areas_for_improvement"`
	Recommendations      []string                 `json:"recommendations"`
	WorkloadAssessment   string                   `json:"workload_assessment"`
	Trend                string                   `json:"trend"`
	Error                string                   `json:"error,omitempty"`
}

// AnalyzeStaffActivity calls the Python AI service to analyze staff activity.
func (c *Client) AnalyzeStaffActivity(ctx context.Context, req StaffActivityRequest) (*StaffActivityResponse, error) {
	period := "last_week"
	if req.Period != "" {
		period = req.Period
	}
	path := fmt.Sprintf("/api/v1/ai/staff/activity?staff_id=%s&staff_name=%s&activity_data=%s&period=%s",
		url.QueryEscape(req.StaffID),
		url.QueryEscape(req.StaffName),
		url.QueryEscape(req.ActivityData),
		url.QueryEscape(period),
	)

	resp, err := c.post(ctx, path, nil)
	if err != nil {
		return nil, err
	}
	defer drainAndClose(resp.Body)

	var result StaffActivityResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// Close closes the client and releases resources.
func (c *Client) Close() {
	c.httpClient.CloseIdleConnections()
}
