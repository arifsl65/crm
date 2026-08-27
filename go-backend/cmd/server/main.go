// Package main is the entrypoint for the Accountant CRM Go backend service.
package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/accountant-crm/go-backend/internal/ai"
	"github.com/accountant-crm/go-backend/internal/cache"
	"github.com/accountant-crm/go-backend/internal/config"
	"github.com/accountant-crm/go-backend/internal/database"
	"github.com/accountant-crm/go-backend/internal/middleware"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to load configuration")
	}

	// Configure logging
	setupLogging(cfg.App)

	log.Info().
		Str("app", cfg.App.Name).
		Str("env", cfg.App.Env).
		Msg("Starting Accountant CRM Backend")

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize database connection
	db, err := database.NewPool(ctx, cfg.Postgres)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to PostgreSQL")
	}
	defer db.Close()

	// Initialize Redis client
	redis, err := cache.NewClient(ctx, cfg.Redis)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to Redis")
	}
	defer redis.Close()

	// Initialize AI client (for Python AI service communication)
	aiClient, err := ai.NewClient(cfg.PythonAI, cfg.MTLS)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create AI client")
	}
	defer aiClient.Close()

	// Setup Gin router
	router := setupRouter(cfg, db, redis, aiClient)

	// Create HTTP server
	srv := &http.Server{
		Addr:         fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port),
		Handler:      router,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}

	// Start server in goroutine
	go func() {
		log.Info().
			Str("addr", srv.Addr).
			Msg("HTTP server listening")

		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal().Err(err).Msg("HTTP server failed")
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info().Msg("Shutting down server...")

	// Graceful shutdown with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error().Err(err).Msg("Server forced to shutdown")
	}

	log.Info().Msg("Server exited gracefully")
}

// setupLogging configures zerolog based on environment.
func setupLogging(cfg config.AppConfig) {
	// Set log level
	level, err := zerolog.ParseLevel(cfg.LogLevel)
	if err != nil {
		level = zerolog.InfoLevel
	}
	zerolog.SetGlobalLevel(level)

	// Use pretty logging for development
	if cfg.Env == "development" {
		log.Logger = log.Output(zerolog.ConsoleWriter{
			Out:        os.Stdout,
			TimeFormat: time.RFC3339,
		})
	} else {
		// Production: JSON logging with timestamps
		zerolog.TimeFieldFormat = time.RFC3339Nano
	}
}

// setupRouter configures the Gin router with all routes.
func setupRouter(cfg *config.Config, db *database.Pool, redis *cache.Client, aiClient *ai.Client) *gin.Engine {
	// Set Gin mode
	if cfg.App.Env == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()

	// Middleware
	router.Use(gin.Recovery())
	router.Use(middleware.RequestID()) // Fix #20: X-Request-ID propagation
	router.Use(requestLogger())
	router.Use(corsMiddleware(cfg.CORS))

	// Rate limiting (applied to all routes except health checks)
	if cfg.RateLimit.Enabled {
		router.Use(rateLimiter(cfg.RateLimit, redis))
	}

	// Health endpoints (no auth required)
	router.GET("/health", healthHandler())
	router.GET("/ready", readyHandler(db, redis, aiClient))

	// Metrics endpoint - protected with bearer token (Fix #12)
	// Only accessible with X-Metrics-Token header matching METRICS_TOKEN env var
	router.GET("/metrics", metricsAuthMiddleware(cfg.App), metricsHandler(db, redis))

	// API v1 routes
	v1 := router.Group("/api/v1")
	{
		// Auth routes (to be implemented)
		auth := v1.Group("/auth")
		{
			auth.POST("/login", notImplementedHandler())
			auth.POST("/refresh", notImplementedHandler())
			auth.POST("/logout", notImplementedHandler())
		}

		// Tenant routes (to be implemented)
		tenants := v1.Group("/tenants")
		{
			tenants.GET("", notImplementedHandler())
			tenants.POST("", notImplementedHandler())
			tenants.GET("/:id", middleware.ValidateUUID("id"), notImplementedHandler())
			tenants.PATCH("/:id", middleware.ValidateUUID("id"), notImplementedHandler())
			tenants.DELETE("/:id", middleware.ValidateUUID("id"), notImplementedHandler())
		}

		// User routes (to be implemented)
		users := v1.Group("/users")
		{
			users.GET("", notImplementedHandler())
			users.POST("", notImplementedHandler())
			users.GET("/:id", middleware.ValidateUUID("id"), notImplementedHandler())
			users.PATCH("/:id", middleware.ValidateUUID("id"), notImplementedHandler())
			users.DELETE("/:id", middleware.ValidateUUID("id"), notImplementedHandler())
		}
	}

	return router
}

// requestLogger returns a Gin middleware for request logging.
// Includes X-Request-ID for distributed tracing (Fix #20).
func requestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path

		c.Next()

		latency := time.Since(start)
		status := c.Writer.Status()
		requestID := middleware.GetRequestID(c)

		log.Info().
			Str("request_id", requestID).
			Str("method", c.Request.Method).
			Str("path", path).
			Int("status", status).
			Dur("latency", latency).
			Str("ip", c.ClientIP()).
			Msg("Request")
	}
}

// corsMiddleware returns a Gin middleware for CORS handling.
func corsMiddleware(cfg config.CORSConfig) gin.HandlerFunc {
	// Build allowed origins map for O(1) lookup
	allowedOrigins := make(map[string]bool)
	for _, origin := range cfg.AllowedOrigins {
		allowedOrigins[origin] = true
	}

	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")

		// Check if origin is allowed
		if origin != "" && allowedOrigins[origin] {
			c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
			c.Writer.Header().Set("Vary", "Origin")

			if cfg.AllowCredentials {
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

// In-memory fallback rate limiter (Fix #11)
// Used when Redis is unavailable to prevent unlimited requests
type memoryRateLimiter struct {
	counts map[string]int
	resets map[string]time.Time
	mu     sync.Mutex
}

var fallbackLimiter = &memoryRateLimiter{
	counts: make(map[string]int),
	resets: make(map[string]time.Time),
}

func (m *memoryRateLimiter) check(ip string, limit int, window time.Duration) (allowed bool, count int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	resetTime, exists := m.resets[ip]

	// Reset if window expired
	if !exists || now.After(resetTime) {
		m.counts[ip] = 1
		m.resets[ip] = now.Add(window)
		return true, 1
	}

	// Increment and check
	m.counts[ip]++
	count = m.counts[ip]
	return count <= limit, count
}

// rateLimiter returns a Gin middleware for rate limiting using Redis.
// Uses atomic Lua script for distributed rate limiting - no race conditions.
// Fix #11: Falls back to in-memory rate limiting when Redis is down.
func rateLimiter(cfg config.RateLimitConfig, redis *cache.Client) gin.HandlerFunc {
	// Total limit includes burst allowance
	totalLimit := cfg.RequestsPerIP + cfg.BurstSize
	windowSeconds := int(cfg.Window.Seconds())

	return func(c *gin.Context) {
		// Skip rate limiting for health endpoints
		path := c.Request.URL.Path
		if path == "/health" || path == "/ready" {
			c.Next()
			return
		}

		// Get client IP
		clientIP := c.ClientIP()
		key := "ratelimit:" + clientIP

		ctx := c.Request.Context()

		// Atomic check-and-increment using Lua script
		result, err := redis.RateLimitCheck(ctx, key, totalLimit, windowSeconds)
		if err != nil {
			// Redis error - use in-memory fallback (Fix #11)
			log.Warn().Err(err).Str("ip", clientIP).Msg("Rate limiter Redis error, using fallback")

			allowed, count := fallbackLimiter.check(clientIP, totalLimit, cfg.Window)
			if !allowed {
				c.Header("X-RateLimit-Limit", strconv.Itoa(cfg.RequestsPerIP))
				c.Header("X-RateLimit-Remaining", "0")
				c.Header("Retry-After", strconv.Itoa(windowSeconds))

				log.Warn().
					Str("ip", clientIP).
					Int("count", count).
					Int("limit", totalLimit).
					Msg("Rate limit exceeded (fallback)")

				c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
					"error":   "rate_limit_exceeded",
					"message": "Too many requests. Please try again later.",
				})
				return
			}

			c.Header("X-RateLimit-Limit", strconv.Itoa(cfg.RequestsPerIP))
			c.Header("X-RateLimit-Remaining", strconv.Itoa(totalLimit-count))
			c.Next()
			return
		}

		// Set rate limit headers
		remaining := totalLimit - result.CurrentCount
		if remaining < 0 {
			remaining = 0
		}
		c.Header("X-RateLimit-Limit", strconv.Itoa(cfg.RequestsPerIP))
		c.Header("X-RateLimit-Remaining", strconv.Itoa(remaining))

		// Check if over limit (already incremented atomically)
		if !result.Allowed {
			c.Header("Retry-After", strconv.Itoa(result.TTLSeconds))

			log.Warn().
				Str("ip", clientIP).
				Int("count", result.CurrentCount).
				Int("limit", totalLimit).
				Msg("Rate limit exceeded")

			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error":   "rate_limit_exceeded",
				"message": "Too many requests. Please try again later.",
			})
			return
		}

		c.Next()
	}
}

// healthHandler returns a handler for liveness checks.
// Fix #26: Added basic runtime checks to ensure process is healthy.
func healthHandler() gin.HandlerFunc {
	startTime := time.Now()

	return func(c *gin.Context) {
		// Basic runtime checks
		var memStats runtime.MemStats
		runtime.ReadMemStats(&memStats)

		c.JSON(http.StatusOK, gin.H{
			"status":     "ok",
			"uptime":     time.Since(startTime).String(),
			"goroutines": runtime.NumGoroutine(),
			"memory_mb":  memStats.Alloc / 1024 / 1024,
		})
	}
}

// readyHandler returns a handler for readiness checks.
func readyHandler(db *database.Pool, redis *cache.Client, aiClient *ai.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		response := gin.H{}
		status := http.StatusOK

		// Check database
		if err := db.HealthCheck(ctx); err != nil {
			response["db"] = "error"
			status = http.StatusServiceUnavailable
			log.Error().Err(err).Msg("Database health check failed")
		} else {
			response["db"] = "ok"
		}

		// Check Redis
		if err := redis.HealthCheck(ctx); err != nil {
			response["redis"] = "error"
			status = http.StatusServiceUnavailable
			log.Error().Err(err).Msg("Redis health check failed")
		} else {
			response["redis"] = "ok"
		}

		// Check AI service (Python)
		if err := aiClient.HealthCheck(ctx); err != nil {
			response["ai"] = "error"
			// AI service is non-critical, log but don't fail readiness
			log.Warn().Err(err).Msg("AI service health check failed")
		} else {
			response["ai"] = "ok"
		}

		c.JSON(status, response)
	}
}

// metricsAuthMiddleware protects the /metrics endpoint with a token.
// Fix #12: Prevent public access to internal metrics.
func metricsAuthMiddleware(cfg config.AppConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get expected token from environment
		expectedToken := os.Getenv("METRICS_TOKEN")

		// In development, allow access without token
		if cfg.Env == "development" && expectedToken == "" {
			c.Next()
			return
		}

		// Require token in production/staging
		if expectedToken == "" {
			log.Warn().Msg("METRICS_TOKEN not set, blocking metrics access")
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error":   "forbidden",
				"message": "Metrics endpoint not configured",
			})
			return
		}

		// Check X-Metrics-Token header
		token := c.GetHeader("X-Metrics-Token")
		if token != expectedToken {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error":   "unauthorized",
				"message": "Invalid or missing metrics token",
			})
			return
		}

		c.Next()
	}
}

// metricsHandler returns connection pool metrics for monitoring.
// Fix #22: Expose pool metrics for CloudMonitor/Prometheus scraping.
func metricsHandler(db *database.Pool, redis *cache.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		metrics := gin.H{
			"postgres": db.PoolMetrics(),
			"redis":    redis.PoolMetrics(ctx),
		}

		c.JSON(http.StatusOK, metrics)
	}
}

// notImplementedHandler returns a 501 Not Implemented response.
func notImplementedHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusNotImplemented, gin.H{
			"error":   "not_implemented",
			"message": "This endpoint is not yet implemented",
		})
	}
}
