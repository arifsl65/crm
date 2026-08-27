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

// RequestIDKey is the context key for request ID propagation
const RequestIDKey = "X-Request-ID"

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
			req.Header.Set(RequestIDKey, requestID)
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
			req.Header.Set(RequestIDKey, requestID)
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

// Close closes the client and releases resources.
func (c *Client) Close() {
	c.httpClient.CloseIdleConnections()
}
