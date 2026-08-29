// Package middleware provides HTTP middleware for the API.
package middleware

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
)

const (
	// IdempotencyKeyHeader is the header name for idempotency keys
	IdempotencyKeyHeader = "X-Idempotency-Key"
	// IdempotencyKeyMaxLength is the maximum allowed length for idempotency keys
	IdempotencyKeyMaxLength = 64
	// IdempotencyTTL is the default TTL for cached responses
	IdempotencyTTL = 24 * time.Hour
)

// IdempotencyStore defines the interface for storing idempotency records.
// Implementations can use Redis, database, or in-memory storage.
type IdempotencyStore interface {
	// Get retrieves a cached response for the given key.
	// Returns nil if not found or expired.
	Get(ctx context.Context, key string) (*IdempotencyRecord, error)

	// Set stores a response for the given key with TTL.
	Set(ctx context.Context, key string, record *IdempotencyRecord, ttl time.Duration) error

	// Lock attempts to acquire a lock for the given key.
	// Returns true if lock acquired, false if another request is processing.
	Lock(ctx context.Context, key string, ttl time.Duration) (bool, error)

	// Unlock releases the lock for the given key.
	Unlock(ctx context.Context, key string) error
}

// IdempotencyRecord stores the cached response for an idempotent request.
type IdempotencyRecord struct {
	StatusCode int               `json:"status_code"`
	Headers    map[string]string `json:"headers"`
	Body       []byte            `json:"body"`
	CreatedAt  time.Time         `json:"created_at"`
	RequestHash string           `json:"request_hash"` // Hash of request body for conflict detection
}

// IdempotencyConfig holds configuration for the idempotency middleware.
type IdempotencyConfig struct {
	Store          IdempotencyStore
	TTL            time.Duration
	// RequiredForMethods specifies HTTP methods that require idempotency keys
	RequiredForMethods []string
	// ExcludePaths specifies paths that should skip idempotency checks
	ExcludePaths []string
}

// Idempotency returns a middleware that enforces idempotent request handling.
// This is a skeleton implementation - full Redis integration will be wired
// when payment and invoice endpoints are implemented.
//
// Usage:
//
//	router.POST("/invoices", middleware.Idempotency(cfg), handlers.CreateInvoice)
//
// The middleware:
// 1. Checks for X-Idempotency-Key header
// 2. Returns cached response if key was previously processed
// 3. Locks the key while processing to prevent duplicate concurrent requests
// 4. Caches successful responses for replay
func Idempotency(cfg IdempotencyConfig) gin.HandlerFunc {
	if cfg.TTL == 0 {
		cfg.TTL = IdempotencyTTL
	}

	// Build excluded paths map
	excludedPaths := make(map[string]bool)
	for _, path := range cfg.ExcludePaths {
		excludedPaths[path] = true
	}

	// Build required methods map
	requiredMethods := make(map[string]bool)
	for _, method := range cfg.RequiredForMethods {
		requiredMethods[method] = true
	}
	// Default: POST and PATCH require idempotency keys
	if len(requiredMethods) == 0 {
		requiredMethods["POST"] = true
		requiredMethods["PATCH"] = true
	}

	return func(c *gin.Context) {
		// Skip if path is excluded
		if excludedPaths[c.Request.URL.Path] {
			c.Next()
			return
		}

		// Skip if method doesn't require idempotency
		if !requiredMethods[c.Request.Method] {
			c.Next()
			return
		}

		// Get idempotency key from header
		idempotencyKey := c.GetHeader(IdempotencyKeyHeader)
		if idempotencyKey == "" {
			// Key not required for all endpoints in skeleton mode
			// Will be enforced for payment/invoice endpoints later
			c.Next()
			return
		}

		// Validate key length
		if len(idempotencyKey) > IdempotencyKeyMaxLength {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "invalid_idempotency_key",
				"message": "Idempotency key exceeds maximum length",
			})
			c.Abort()
			return
		}

		// If no store configured, just pass through (skeleton mode)
		if cfg.Store == nil {
			log.Debug().Str("key", idempotencyKey).Msg("Idempotency store not configured, passing through")
			c.Next()
			return
		}

		ctx := c.Request.Context()

		// Build scoped key (includes user ID to prevent cross-user replay)
		userID, _ := GetUserID(c)
		scopedKey := buildScopedKey(userID.String(), c.Request.URL.Path, idempotencyKey)

		// Check for existing response
		record, err := cfg.Store.Get(ctx, scopedKey)
		if err != nil {
			log.Error().Err(err).Str("key", scopedKey).Msg("Failed to check idempotency store")
			// Continue processing on store errors
		}

		if record != nil {
			// Verify request body hash matches (prevent different requests with same key)
			currentHash := hashRequestBody(c)
			if record.RequestHash != "" && record.RequestHash != currentHash {
				c.JSON(http.StatusConflict, gin.H{
					"error":   "idempotency_conflict",
					"message": "Idempotency key was used with different request body",
				})
				c.Abort()
				return
			}

			// Replay cached response
			for k, v := range record.Headers {
				c.Header(k, v)
			}
			c.Header("X-Idempotent-Replayed", "true")
			c.Data(record.StatusCode, "application/json", record.Body)
			c.Abort()
			return
		}

		// Try to acquire lock
		locked, err := cfg.Store.Lock(ctx, scopedKey, 30*time.Second)
		if err != nil {
			log.Error().Err(err).Str("key", scopedKey).Msg("Failed to acquire idempotency lock")
		}
		if !locked {
			c.JSON(http.StatusConflict, gin.H{
				"error":   "request_in_progress",
				"message": "Another request with this idempotency key is in progress",
			})
			c.Abort()
			return
		}

		// Create response writer wrapper to capture response
		rw := &idempotencyResponseWriter{
			ResponseWriter: c.Writer,
			body:           make([]byte, 0),
		}
		c.Writer = rw

		// Process request
		c.Next()

		// Store response for replay (only for successful responses)
		if c.Writer.Status() >= 200 && c.Writer.Status() < 300 {
			newRecord := &IdempotencyRecord{
				StatusCode:  c.Writer.Status(),
				Headers:     extractResponseHeaders(c),
				Body:        rw.body,
				CreatedAt:   time.Now(),
				RequestHash: hashRequestBody(c),
			}

			if err := cfg.Store.Set(ctx, scopedKey, newRecord, cfg.TTL); err != nil {
				log.Error().Err(err).Str("key", scopedKey).Msg("Failed to store idempotency record")
			}
		}

		// Release lock
		if err := cfg.Store.Unlock(ctx, scopedKey); err != nil {
			log.Error().Err(err).Str("key", scopedKey).Msg("Failed to release idempotency lock")
		}
	}
}

// RequireIdempotencyKey returns a middleware that requires an idempotency key.
// Use this for endpoints that MUST have idempotency keys (e.g., payments, refunds).
func RequireIdempotencyKey() gin.HandlerFunc {
	return func(c *gin.Context) {
		idempotencyKey := c.GetHeader(IdempotencyKeyHeader)
		if idempotencyKey == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "missing_idempotency_key",
				"message": "X-Idempotency-Key header is required for this endpoint",
			})
			c.Abort()
			return
		}

		if len(idempotencyKey) > IdempotencyKeyMaxLength {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "invalid_idempotency_key",
				"message": "Idempotency key exceeds maximum length",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// buildScopedKey creates a unique key scoped to user and path
func buildScopedKey(userID, path, key string) string {
	return userID + ":" + path + ":" + key
}

// hashRequestBody creates a hash of the request body for conflict detection
func hashRequestBody(c *gin.Context) string {
	body, exists := c.Get("request_body")
	if !exists {
		return ""
	}
	bodyBytes, ok := body.([]byte)
	if !ok {
		return ""
	}
	hash := sha256.Sum256(bodyBytes)
	return hex.EncodeToString(hash[:])
}

// extractResponseHeaders extracts headers to cache
func extractResponseHeaders(c *gin.Context) map[string]string {
	headers := make(map[string]string)
	// Only cache specific headers
	headersToCache := []string{"Content-Type", "X-Request-ID"}
	for _, h := range headersToCache {
		if v := c.Writer.Header().Get(h); v != "" {
			headers[h] = v
		}
	}
	return headers
}

// idempotencyResponseWriter wraps ResponseWriter to capture response body
type idempotencyResponseWriter struct {
	gin.ResponseWriter
	body []byte
}

func (w *idempotencyResponseWriter) Write(b []byte) (int, error) {
	w.body = append(w.body, b...)
	return w.ResponseWriter.Write(b)
}

// MemoryIdempotencyStore is a simple in-memory store for development/testing.
// Use RedisIdempotencyStore in production.
type MemoryIdempotencyStore struct {
	records map[string]*memoryRecord
	locks   map[string]bool
}

type memoryRecord struct {
	record    *IdempotencyRecord
	expiresAt time.Time
}

// NewMemoryIdempotencyStore creates a new in-memory idempotency store.
func NewMemoryIdempotencyStore() *MemoryIdempotencyStore {
	return &MemoryIdempotencyStore{
		records: make(map[string]*memoryRecord),
		locks:   make(map[string]bool),
	}
}

func (s *MemoryIdempotencyStore) Get(_ context.Context, key string) (*IdempotencyRecord, error) {
	mr, exists := s.records[key]
	if !exists || time.Now().After(mr.expiresAt) {
		return nil, nil
	}
	return mr.record, nil
}

func (s *MemoryIdempotencyStore) Set(_ context.Context, key string, record *IdempotencyRecord, ttl time.Duration) error {
	s.records[key] = &memoryRecord{
		record:    record,
		expiresAt: time.Now().Add(ttl),
	}
	return nil
}

func (s *MemoryIdempotencyStore) Lock(_ context.Context, key string, _ time.Duration) (bool, error) {
	if s.locks[key] {
		return false, nil
	}
	s.locks[key] = true
	return true, nil
}

func (s *MemoryIdempotencyStore) Unlock(_ context.Context, key string) error {
	delete(s.locks, key)
	return nil
}

// MarshalJSON implements JSON marshaling for IdempotencyRecord
func (r *IdempotencyRecord) MarshalJSON() ([]byte, error) {
	type Alias IdempotencyRecord
	return json.Marshal(&struct {
		*Alias
	}{
		Alias: (*Alias)(r),
	})
}
