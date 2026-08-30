// Package middleware provides HTTP middleware for the API.
package middleware

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"

	"github.com/accountant-crm/go-backend/internal/database"
)

// CORSConfig holds CORS configuration.
type CORSConfig struct {
	// StaticOrigins are always allowed (e.g., localhost for dev)
	StaticOrigins []string
	// AllowCredentials enables the Access-Control-Allow-Credentials header
	AllowCredentials bool
	// DB is used for dynamic origin lookups from tenants table
	DB *database.Pool
	// CacheTTL is how long to cache DB lookups (default: 5 minutes)
	CacheTTL time.Duration
}

// corsCache stores cached origin lookups.
type corsCache struct {
	mu            sync.RWMutex
	allowedOrigins map[string]bool
	lastRefresh   time.Time
	ttl           time.Duration
}

func newCorsCache(ttl time.Duration) *corsCache {
	if ttl == 0 {
		ttl = 5 * time.Minute
	}
	return &corsCache{
		allowedOrigins: make(map[string]bool),
		ttl:            ttl,
	}
}

func (c *corsCache) isValid() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return time.Since(c.lastRefresh) < c.ttl
}

func (c *corsCache) isAllowed(origin string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.allowedOrigins[origin]
}

func (c *corsCache) refresh(origins map[string]bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.allowedOrigins = origins
	c.lastRefresh = time.Now()
}

// DynamicCORS returns a middleware that handles CORS with dynamic origin lookup.
// It allows origins from:
// 1. Static configuration (e.g., localhost for development)
// 2. Tenant domains from the database (domain and custom_domain)
func DynamicCORS(cfg CORSConfig) gin.HandlerFunc {
	// Build static origins map
	staticOrigins := make(map[string]bool)
	for _, origin := range cfg.StaticOrigins {
		staticOrigins[origin] = true
	}

	// Initialize cache
	cache := newCorsCache(cfg.CacheTTL)

	// Background refresh function
	refreshCache := func() {
		if cfg.DB == nil {
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		origins := make(map[string]bool)

		// Copy static origins
		for origin := range staticOrigins {
			origins[origin] = true
		}

		// Query tenant domains
		query := `
			SELECT domain, custom_domain
			FROM tenants
			WHERE is_active = true AND deleted_at IS NULL
		`
		rows, err := cfg.DB.Query(ctx, query)
		if err != nil {
			log.Error().Err(err).Msg("Failed to fetch tenant domains for CORS")
			return
		}
		defer rows.Close()

		for rows.Next() {
			var domain string
			var customDomain *string
			if err := rows.Scan(&domain, &customDomain); err != nil {
				log.Error().Err(err).Msg("Failed to scan tenant domain")
				continue
			}

			// Add tenant domain as HTTPS origin
			addDomainOrigins(origins, domain)

			// Add custom domain if set
			if customDomain != nil && *customDomain != "" {
				addDomainOrigins(origins, *customDomain)
			}
		}

		if err := rows.Err(); err != nil {
			log.Error().Err(err).Msg("Error iterating tenant domains")
			return
		}

		cache.refresh(origins)
		log.Debug().Int("count", len(origins)).Msg("CORS cache refreshed")
	}

	// Initial cache population
	refreshCache()

	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")

		// Check if origin is allowed
		if origin != "" {
			// Refresh cache if expired
			if !cache.isValid() {
				go refreshCache()
			}

			allowed := false

			// Check static origins first (fast path)
			if staticOrigins[origin] {
				setAllowedOrigin(c, origin, cfg.AllowCredentials)
				allowed = true
				log.Debug().
					Str("origin", origin).
					Str("match_type", "static").
					Msg("CORS origin allowed")
			} else if cache.isAllowed(origin) {
				// Check cached tenant origins
				setAllowedOrigin(c, origin, cfg.AllowCredentials)
				allowed = true
				log.Debug().
					Str("origin", origin).
					Str("match_type", "tenant_domain").
					Msg("CORS origin allowed")
			}

			// Log rejected origins for security auditing
			if !allowed {
				// Parse origin to extract host for logging
				parsedOrigin, err := url.Parse(origin)
				host := origin
				if err == nil && parsedOrigin.Host != "" {
					host = parsedOrigin.Host
				}

				log.Warn().
					Str("origin", origin).
					Str("host", host).
					Str("method", c.Request.Method).
					Str("path", c.Request.URL.Path).
					Str("remote_ip", c.ClientIP()).
					Msg("CORS origin rejected - not in allowed list")
			}
		}

		// Always set these headers
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Request-ID, X-Tenant-ID")
		c.Writer.Header().Set("Access-Control-Expose-Headers", "X-Request-ID, X-RateLimit-Remaining, X-RateLimit-Reset")
		c.Writer.Header().Set("Access-Control-Max-Age", "86400")

		// Handle preflight
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

// parseOriginHost extracts the host from an Origin header value.
// Returns the host without port for comparison purposes.
func parseOriginHost(origin string) string {
	parsed, err := url.Parse(origin)
	if err != nil || parsed.Host == "" {
		return ""
	}
	// Return host without port
	host := parsed.Hostname()
	return host
}

// addDomainOrigins adds a domain and its variants to the origins map.
// Handles both plain domains and full URLs, adding HTTPS variants.
func addDomainOrigins(origins map[string]bool, domain string) {
	domain = strings.TrimSpace(domain)
	if domain == "" {
		return
	}

	// If already a full URL, parse and normalize it
	if strings.HasPrefix(domain, "http://") || strings.HasPrefix(domain, "https://") {
		parsed, err := url.Parse(domain)
		if err != nil {
			log.Warn().Str("domain", domain).Msg("Invalid domain URL in tenant")
			return
		}
		// Always use HTTPS for security
		origins["https://"+parsed.Host] = true
		// Also add without www if present
		if strings.HasPrefix(parsed.Host, "www.") {
			origins["https://"+strings.TrimPrefix(parsed.Host, "www.")] = true
		}
		return
	}

	// Plain domain - add as HTTPS
	origins["https://"+domain] = true

	// Also allow without www if it has www
	if strings.HasPrefix(domain, "www.") {
		origins["https://"+strings.TrimPrefix(domain, "www.")] = true
	}

	// Also allow with www if it doesn't have www (for flexibility)
	if !strings.HasPrefix(domain, "www.") && !strings.Contains(domain, ".app.") {
		// Only add www variant for custom domains, not subdomains
		parts := strings.Split(domain, ".")
		if len(parts) == 2 {
			origins["https://www."+domain] = true
		}
	}
}

func setAllowedOrigin(c *gin.Context, origin string, allowCredentials bool) {
	c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
	c.Writer.Header().Set("Vary", "Origin")
	if allowCredentials {
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
	}
}

// SimpleCORS returns a simple CORS middleware with static configuration only.
// Use this when dynamic lookup isn't needed.
func SimpleCORS(allowedOrigins []string, allowCredentials bool) gin.HandlerFunc {
	origins := make(map[string]bool)
	for _, origin := range allowedOrigins {
		origins[origin] = true
	}

	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")

		if origin != "" && origins[origin] {
			c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
			c.Writer.Header().Set("Vary", "Origin")
			if allowCredentials {
				c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
			}
		}

		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Request-ID")
		c.Writer.Header().Set("Access-Control-Max-Age", "86400")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}
