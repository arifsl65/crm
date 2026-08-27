package middleware

import (
	"context"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// RequestIDKey is the context key for the request ID
const RequestIDKey = "X-Request-ID"

// RequestID is a middleware that adds X-Request-ID to all requests and responses.
// It accepts X-Request-ID from incoming request headers (e.g., from ALB)
// or generates a new UUID if not present.
// This enables distributed tracing across services.
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get existing request ID from header or generate new one
		requestID := c.GetHeader(RequestIDKey)
		if requestID == "" {
			requestID = uuid.New().String()
		}

		// Store in Gin's context for use in handlers
		c.Set(RequestIDKey, requestID)

		// CRITICAL: Also store in request's context.Context for downstream services
		// This enables ctx.Value(RequestIDKey) to work in AI client and other contexts
		ctx := context.WithValue(c.Request.Context(), RequestIDKey, requestID)
		c.Request = c.Request.WithContext(ctx)

		// Add to response headers
		c.Header(RequestIDKey, requestID)

		c.Next()
	}
}

// GetRequestID retrieves the request ID from the Gin context.
// Returns empty string if not found.
func GetRequestID(c *gin.Context) string {
	if requestID, exists := c.Get(RequestIDKey); exists {
		return requestID.(string)
	}
	return ""
}
