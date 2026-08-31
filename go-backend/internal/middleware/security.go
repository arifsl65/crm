package middleware

import (
	"github.com/gin-gonic/gin"
)

// SecurityHeaders returns middleware that adds security-related HTTP headers.
// These headers match the exact values required by API_ENDPOINTS.md.
func SecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		// HSTS - Force HTTPS for 1 year, include subdomains, enable preload
		c.Header("Strict-Transport-Security", "max-age=31536000; includeSubDomains; preload")

		// Prevent clickjacking attacks by disallowing framing
		c.Header("X-Frame-Options", "DENY")

		// Prevent MIME type sniffing
		c.Header("X-Content-Type-Options", "nosniff")

		// Referrer policy - only send origin for cross-origin requests
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")

		// Content Security Policy - exact value from API_ENDPOINTS.md
		c.Header("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'")

		// Permissions Policy - exact value from API_ENDPOINTS.md
		c.Header("Permissions-Policy", "geolocation=(), microphone=(), camera=()")

		c.Next()
	}
}

// SecurityHeadersConfig allows customization of security headers.
type SecurityHeadersConfig struct {
	// EnableHSTS enables HSTS header (should be false for development)
	EnableHSTS bool
	// HSTSMaxAge in seconds (default 31536000 = 1 year)
	HSTSMaxAge int
	// ContentSecurityPolicy custom CSP (optional)
	ContentSecurityPolicy string
	// AllowFraming if true, sets X-Frame-Options to SAMEORIGIN instead of DENY
	AllowFraming bool
}

// SecurityHeadersWithConfig returns middleware matching API_ENDPOINTS.md.
// Config fields are kept for API compatibility but ignored; headers always match the spec.
func SecurityHeadersWithConfig(cfg SecurityHeadersConfig) gin.HandlerFunc {
	return SecurityHeaders()
}
