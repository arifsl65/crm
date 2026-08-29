package middleware

import (
	"github.com/gin-gonic/gin"
)

// SecurityHeaders returns middleware that adds security-related HTTP headers.
// These headers help protect against common web vulnerabilities.
func SecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		// HSTS - Force HTTPS for 1 year, include subdomains
		// Only enable in production to avoid issues with local development
		c.Header("Strict-Transport-Security", "max-age=31536000; includeSubDomains")

		// Prevent clickjacking attacks by disallowing framing
		c.Header("X-Frame-Options", "DENY")

		// Prevent MIME type sniffing
		c.Header("X-Content-Type-Options", "nosniff")

		// Enable XSS filter in browsers
		c.Header("X-XSS-Protection", "1; mode=block")

		// Referrer policy - only send origin for cross-origin requests
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")

		// Content Security Policy - restrict resource loading
		// Adjust based on your frontend needs (e.g., if using inline styles/scripts)
		csp := "default-src 'self'; " +
			"script-src 'self'; " +
			"style-src 'self' 'unsafe-inline'; " + // unsafe-inline needed for some CSS frameworks
			"img-src 'self' data: https:; " +
			"font-src 'self'; " +
			"connect-src 'self'; " +
			"frame-ancestors 'none'; " +
			"base-uri 'self'; " +
			"form-action 'self'"
		c.Header("Content-Security-Policy", csp)

		// Permissions Policy - disable dangerous browser features
		permissions := "accelerometer=(), " +
			"camera=(), " +
			"geolocation=(), " +
			"gyroscope=(), " +
			"magnetometer=(), " +
			"microphone=(), " +
			"payment=(), " +
			"usb=()"
		c.Header("Permissions-Policy", permissions)

		// Cross-Origin policies for additional isolation
		c.Header("Cross-Origin-Opener-Policy", "same-origin")
		c.Header("Cross-Origin-Resource-Policy", "same-origin")

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

// SecurityHeadersWithConfig returns middleware with custom configuration.
func SecurityHeadersWithConfig(cfg SecurityHeadersConfig) gin.HandlerFunc {
	if cfg.HSTSMaxAge == 0 {
		cfg.HSTSMaxAge = 31536000
	}

	return func(c *gin.Context) {
		// HSTS - only in production
		if cfg.EnableHSTS {
			c.Header("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}

		// X-Frame-Options
		if cfg.AllowFraming {
			c.Header("X-Frame-Options", "SAMEORIGIN")
		} else {
			c.Header("X-Frame-Options", "DENY")
		}

		// Standard headers
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-XSS-Protection", "1; mode=block")
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")

		// CSP
		if cfg.ContentSecurityPolicy != "" {
			c.Header("Content-Security-Policy", cfg.ContentSecurityPolicy)
		} else {
			csp := "default-src 'self'; " +
				"script-src 'self'; " +
				"style-src 'self' 'unsafe-inline'; " +
				"img-src 'self' data: https:; " +
				"font-src 'self'; " +
				"connect-src 'self'; " +
				"frame-ancestors 'none'; " +
				"base-uri 'self'; " +
				"form-action 'self'"
			c.Header("Content-Security-Policy", csp)
		}

		// Permissions Policy
		permissions := "accelerometer=(), " +
			"camera=(), " +
			"geolocation=(), " +
			"gyroscope=(), " +
			"magnetometer=(), " +
			"microphone=(), " +
			"payment=(), " +
			"usb=()"
		c.Header("Permissions-Policy", permissions)

		// Cross-Origin policies
		c.Header("Cross-Origin-Opener-Policy", "same-origin")
		c.Header("Cross-Origin-Resource-Policy", "same-origin")

		c.Next()
	}
}
