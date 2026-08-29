// Package middleware provides HTTP middleware for the API.
package middleware

import (
	"context"
	"net/http"
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
			// Domain format: subdomain.app.com or full URL
			if !strings.HasPrefix(domain, "http") {
				origins["https://"+domain] = true
				// Also allow without www if it has www
				if strings.HasPrefix(domain, "www.") {
					origins["https://"+strings.TrimPrefix(domain, "www.")] = true
				}
			} else {
				origins[domain] = true
			}

			// Add custom domain if set
			if customDomain != nil && *customDomain != "" {
				if !strings.HasPrefix(*customDomain, "http") {
					origins["https://"+*customDomain] = true
				} else {
					origins[*customDomain] = true
				}
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

			// Check static origins first (fast path)
			if staticOrigins[origin] {
				setAllowedOrigin(c, origin, cfg.AllowCredentials)
			} else if cache.isAllowed(origin) {
				// Check cached tenant origins
				setAllowedOrigin(c, origin, cfg.AllowCredentials)
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
