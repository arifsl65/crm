package middleware

import (
	"context"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/accountant-crm/go-backend/internal/ai"
)

// requestIDHeader is the HTTP header name for request ID.
// Fix #28: Separate HTTP header name from context key to avoid collisions.
const requestIDHeader = "X-Request-ID"

// ginContextKey is the Gin context key for request ID (separate from Go context).
const ginContextKey = "request_id"

// RequestID is a middleware that adds X-Request-ID to all requests and responses.
// It accepts X-Request-ID from incoming request headers (e.g., from ALB)
// or generates a new UUID if not present.
// This enables distributed tracing across services.
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get existing request ID from header or generate new one
		requestID := c.GetHeader(requestIDHeader)
		if requestID == "" {
			requestID = uuid.New().String()
		}

		// Store in Gin's context for use in handlers
		c.Set(ginContextKey, requestID)

		// CRITICAL: Also store in request's context.Context for downstream services
		// Fix #28: Use ai.RequestIDKey (private struct type) to avoid context key collisions
		ctx := context.WithValue(c.Request.Context(), ai.RequestIDKey, requestID)
		c.Request = c.Request.WithContext(ctx)

		// Add to response headers
		c.Header(requestIDHeader, requestID)

		c.Next()
	}
}

// GetRequestID retrieves the request ID from the Gin context.
// Returns empty string if not found.
func GetRequestID(c *gin.Context) string {
	if requestID, exists := c.Get(ginContextKey); exists {
		return requestID.(string)
	}
	return ""
}
