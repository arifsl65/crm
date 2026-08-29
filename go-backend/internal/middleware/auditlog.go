// Package middleware provides HTTP middleware for the API.
package middleware

import (
	"bytes"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/accountant-crm/go-backend/internal/audit"
)

// AuditLogConfig holds configuration for audit logging middleware.
type AuditLogConfig struct {
	// Logger is the audit logger instance
	Logger *audit.Logger
	// SkipPaths specifies paths that should not be audited
	SkipPaths []string
	// SkipMethods specifies HTTP methods that should not be audited
	SkipMethods []string
	// LogRequestBody enables logging of request body (use with caution)
	LogRequestBody bool
	// MaxBodyLogSize limits the size of logged request bodies
	MaxBodyLogSize int
	// LogSuccessOnly skips logging for failed requests
	LogSuccessOnly bool
}

// AuditLog returns a middleware that logs API requests for audit trail.
// This middleware logs:
// - All mutating operations (POST, PUT, PATCH, DELETE)
// - User, tenant, and request context
// - Entity type and ID when available
//
// Usage:
//
//	router.Use(middleware.AuditLog(middleware.AuditLogConfig{
//	    Logger:     auditLogger,
//	    SkipPaths:  []string{"/health", "/api/v1/auth/refresh"},
//	    LogRequestBody: false,
//	}))
func AuditLog(cfg AuditLogConfig) gin.HandlerFunc {
	// Build skip paths set
	skipPaths := make(map[string]bool)
	for _, p := range cfg.SkipPaths {
		skipPaths[p] = true
	}

	// Build skip methods set
	skipMethods := make(map[string]bool)
	for _, m := range cfg.SkipMethods {
		skipMethods[m] = true
	}
	// By default, skip GET and OPTIONS
	if len(skipMethods) == 0 {
		skipMethods["GET"] = true
		skipMethods["OPTIONS"] = true
		skipMethods["HEAD"] = true
	}

	// Default max body log size
	if cfg.MaxBodyLogSize == 0 {
		cfg.MaxBodyLogSize = 4096 // 4KB
	}

	return func(c *gin.Context) {
		// Skip if path is in skip list
		if skipPaths[c.Request.URL.Path] {
			c.Next()
			return
		}

		// Skip if method is in skip list
		if skipMethods[c.Request.Method] {
			c.Next()
			return
		}

		// Skip if logger not configured
		if cfg.Logger == nil {
			c.Next()
			return
		}

		// Capture request body if enabled
		var requestBody []byte
		if cfg.LogRequestBody {
			body, err := io.ReadAll(c.Request.Body)
			if err == nil {
				// Restore body for downstream handlers
				c.Request.Body = io.NopCloser(bytes.NewBuffer(body))
				// Truncate for logging
				if len(body) > cfg.MaxBodyLogSize {
					requestBody = body[:cfg.MaxBodyLogSize]
				} else {
					requestBody = body
				}
				// Store for idempotency middleware
				c.Set("request_body", body)
			}
		}

		// Record start time
		start := time.Now()

		// Process request
		c.Next()

		// Skip logging if request failed and LogSuccessOnly is true
		if cfg.LogSuccessOnly && c.Writer.Status() >= 400 {
			return
		}

		// Build audit entry
		ctx := c.Request.Context()
		action := methodToAction(c.Request.Method, c.Request.URL.Path)
		entityType, entityID := extractEntityInfo(c)

		var userID *uuid.UUID
		var tenantID *uuid.UUID

		if uid, exists := c.Get(AuthUserID); exists {
			if id, ok := uid.(uuid.UUID); ok {
				userID = &id
			}
		}

		if tid, exists := c.Get(AuthTenantID); exists {
			if id, ok := tid.(uuid.UUID); ok {
				tenantID = &id
			}
		}

		// Build metadata
		metadata := map[string]interface{}{
			"method":     c.Request.Method,
			"path":       c.Request.URL.Path,
			"status":     c.Writer.Status(),
			"latency_ms": time.Since(start).Milliseconds(),
		}

		if c.Request.URL.RawQuery != "" {
			metadata["query"] = c.Request.URL.RawQuery
		}

		if requestID := c.GetString("request_id"); requestID != "" {
			metadata["request_id"] = requestID
		}

		// Log the audit entry
		entry := audit.LogEntry{
			TenantID:   tenantID,
			UserID:     userID,
			Action:     action,
			EntityType: entityType,
			EntityID:   entityID,
			IPAddress:  c.ClientIP(),
			UserAgent:  c.Request.UserAgent(),
			Metadata:   metadata,
			Success:    c.Writer.Status() < 400,
		}

		if err := cfg.Logger.Log(ctx, entry); err != nil {
			log.Error().Err(err).Str("action", string(action)).Msg("Failed to write audit log")
		}
	}
}

// methodToAction converts HTTP method and path to an audit action.
func methodToAction(method, path string) audit.Action {
	// Extract resource type from path
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) < 2 {
		return audit.Action(strings.ToLower(method))
	}

	// Skip "api" and version prefixes
	resourceIndex := 0
	for i, part := range parts {
		if part != "api" && !strings.HasPrefix(part, "v") {
			resourceIndex = i
			break
		}
	}

	if resourceIndex >= len(parts) {
		return audit.Action(strings.ToLower(method))
	}

	resource := parts[resourceIndex]

	// Map common resources and methods to actions
	switch method {
	case "POST":
		switch resource {
		case "users":
			return audit.ActionUserCreate
		case "clients":
			return audit.ActionClientCreate
		case "documents":
			return audit.ActionDocumentUpload
		case "invoices":
			return audit.ActionInvoiceCreate
		case "payments":
			return audit.ActionPaymentRecord
		default:
			return audit.Action(resource + "_create")
		}
	case "PUT", "PATCH":
		switch resource {
		case "users":
			return audit.ActionUserUpdate
		case "clients":
			return audit.ActionClientUpdate
		case "settings":
			return audit.ActionSettingsUpdate
		case "profile":
			return audit.ActionProfileUpdate
		default:
			return audit.Action(resource + "_update")
		}
	case "DELETE":
		switch resource {
		case "users":
			return audit.ActionUserDelete
		case "clients":
			return audit.ActionClientDelete
		case "documents":
			return audit.ActionDocumentDelete
		default:
			return audit.Action(resource + "_delete")
		}
	default:
		return audit.Action(resource + "_" + strings.ToLower(method))
	}
}

// extractEntityInfo extracts entity type and ID from the request.
func extractEntityInfo(c *gin.Context) (string, *uuid.UUID) {
	path := c.Request.URL.Path
	parts := strings.Split(strings.Trim(path, "/"), "/")

	// Look for patterns like /api/v1/users/:id or /api/v1/clients/:id
	for i := 0; i < len(parts)-1; i++ {
		part := parts[i]
		// Skip version and api prefixes
		if part == "api" || strings.HasPrefix(part, "v") {
			continue
		}

		// Check if this is a resource name followed by an ID
		if i+1 < len(parts) {
			potentialID := parts[i+1]
			if id, err := uuid.Parse(potentialID); err == nil {
				// Convert plural resource name to singular entity type
				entityType := singularize(part)
				return entityType, &id
			}
		}
	}

	// Try to get entity info from context (set by handlers)
	if entityType, exists := c.Get("audit_entity_type"); exists {
		if entityID, exists := c.Get("audit_entity_id"); exists {
			if id, ok := entityID.(uuid.UUID); ok {
				return entityType.(string), &id
			}
		}
	}

	return "", nil
}

// singularize converts plural resource names to singular.
func singularize(s string) string {
	if strings.HasSuffix(s, "ies") {
		return strings.TrimSuffix(s, "ies") + "y"
	}
	if strings.HasSuffix(s, "es") {
		return strings.TrimSuffix(s, "es")
	}
	if strings.HasSuffix(s, "s") {
		return strings.TrimSuffix(s, "s")
	}
	return s
}

// SetAuditContext is a helper for handlers to set entity context for auditing.
func SetAuditContext(c *gin.Context, entityType string, entityID uuid.UUID) {
	c.Set("audit_entity_type", entityType)
	c.Set("audit_entity_id", entityID)
}
