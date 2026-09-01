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
	"github.com/accountant-crm/go-backend/internal/audit"
	"github.com/accountant-crm/go-backend/internal/auth"
	"github.com/accountant-crm/go-backend/internal/cache"
	"github.com/accountant-crm/go-backend/internal/config"
	"github.com/accountant-crm/go-backend/internal/database"
	"github.com/accountant-crm/go-backend/internal/email"
	"github.com/accountant-crm/go-backend/internal/handlers"
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

	// Initialize JWT manager
	jwtManager := auth.NewJWTManager(
		cfg.JWT.SecretKey,
		cfg.JWT.AccessTokenExpire,
		cfg.JWT.RefreshTokenExpire,
		cfg.JWT.Issuer,
	)

	// Initialize session manager for token revocation
	sessionManager := auth.NewSessionManager(db, cfg.JWT.RefreshTokenExpire)

	// Initialize email client (optional - can be nil if not configured)
	var emailClient *email.Client
	if cfg.Email.APIKey != "" {
		emailClient = email.NewClient(email.Config{
			APIKey:    cfg.Email.APIKey,
			FromEmail: cfg.Email.FromEmail,
			FromName:  cfg.Email.FromName,
		})
		log.Info().Msg("Email client configured (Resend)")
	} else {
		log.Warn().Msg("Email client not configured - password reset emails will not be sent")
	}

	// Initialize auth rate limiter
	authRateLimiter := middleware.NewAuthRateLimiter(redis)

	// Initialize audit logger
	auditLogger := audit.NewLogger(db)

	// Initialize handlers
	authHandler := handlers.NewAuthHandler(db, jwtManager, sessionManager, emailClient, cfg.FrontendURL, authRateLimiter, auditLogger, redis)
	tenantHandler := handlers.NewTenantHandler(db)
	userHandler := handlers.NewUserHandler(db)
	clientHandler := handlers.NewClientHandler(db, auditLogger)
	serviceHandler := handlers.NewServiceHandler(db, auditLogger)
	documentHandler := handlers.NewDocumentHandler(db, auditLogger)
	dashboardHandler := handlers.NewDashboardHandler(db)
	serviceTypeHandler := handlers.NewServiceTypeHandler(db, auditLogger)
	documentTypeHandler := handlers.NewDocumentTypeHandler(db, auditLogger)
	aiHandler := handlers.NewAIHandler(aiClient)
	companiesHouseHandler := handlers.NewCompaniesHouseHandler(db, redis, cfg.CompaniesHouse)

	// Setup Gin router
	router := setupRouter(cfg, db, redis, aiClient, jwtManager, authHandler, tenantHandler, userHandler, clientHandler, serviceHandler, documentHandler, dashboardHandler, serviceTypeHandler, documentTypeHandler, aiHandler, companiesHouseHandler, auditLogger)

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
func setupRouter(cfg *config.Config, db *database.Pool, redis *cache.Client, aiClient *ai.Client, jwtManager *auth.JWTManager, authHandler *handlers.AuthHandler, tenantHandler *handlers.TenantHandler, userHandler *handlers.UserHandler, clientHandler *handlers.ClientHandler, serviceHandler *handlers.ServiceHandler, documentHandler *handlers.DocumentHandler, dashboardHandler *handlers.DashboardHandler, serviceTypeHandler *handlers.ServiceTypeHandler, documentTypeHandler *handlers.DocumentTypeHandler, aiHandler *handlers.AIHandler, companiesHouseHandler *handlers.CompaniesHouseHandler, auditLogger *audit.Logger) *gin.Engine {
	// Set Gin mode
	if cfg.App.Env == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()

	// Middleware
	router.Use(gin.Recovery())
	router.Use(middleware.RequestID()) // Fix #20: X-Request-ID propagation
	router.Use(requestLogger())
	router.Use(middleware.DynamicCORS(middleware.CORSConfig{
		StaticOrigins:    cfg.CORS.AllowedOrigins,
		AllowCredentials: cfg.CORS.AllowCredentials,
		DB:               db,
		CacheTTL:         5 * time.Minute,
	}))

	// Security headers (exact values from API_ENDPOINTS.md)
	router.Use(middleware.SecurityHeaders())

	// Rate limiting (applied to all routes except health checks)
	if cfg.RateLimit.Enabled {
		router.Use(rateLimiter(cfg.RateLimit, redis))
	}

	// Health endpoints (no auth required)
	router.GET("/health", healthHandler())
	router.HEAD("/health", healthHandler())
	router.GET("/ready", readyHandler(db, redis, aiClient))

	// Metrics endpoint - protected with bearer token (Fix #12)
	// Only accessible with X-Metrics-Token header matching METRICS_TOKEN env var
	router.GET("/metrics", metricsAuthMiddleware(cfg.App), metricsHandler(db, redis))

	// API v1 routes
	v1 := router.Group("/api/v1")
	{
		// Auth routes (public)
		authRoutes := v1.Group("/auth")
		{
			authRoutes.POST("/login", authHandler.Login)
			authRoutes.POST("/register", authHandler.Register)
			authRoutes.POST("/refresh", authHandler.Refresh)
			authRoutes.POST("/reset-password", authHandler.ForgotPassword)
			authRoutes.POST("/reset-password/confirm", authHandler.ResetPassword)
			authRoutes.POST("/magic-link", authHandler.SendMagicLink)
			authRoutes.GET("/magic-link", authHandler.VerifyMagicLink)
			authRoutes.POST("/invite-accept", authHandler.InviteAccept) // Accept invite and set password
			// 2FA backup code verification (public - for lockout recovery)
			authRoutes.POST("/2fa/backup-codes/verify", authHandler.VerifyBackupCode)
		}

		// Auth routes (protected)
		authProtected := v1.Group("/auth")
		authProtected.Use(middleware.JWTAuth(jwtManager, redis))
		authProtected.Use(middleware.TenantRLS(db)) // Wire RLS context
		authProtected.Use(middleware.AuditLog(middleware.AuditLogConfig{
			Logger:    auditLogger,
			SkipPaths: []string{"/api/v1/auth/refresh"},
		}))
		{
			authProtected.POST("/logout", authHandler.Logout)
			authProtected.GET("/me", authHandler.GetMe)
			authProtected.PATCH("/me", authHandler.UpdateMe)
			authProtected.PATCH("/password", authHandler.ChangePassword)
			authProtected.GET("/sessions", authHandler.GetSessions)
			// Token family revocation (for theft detection)
			authProtected.POST("/refresh/revoke-family", authHandler.RevokeTokenFamily)
			// 2FA endpoints
			authProtected.POST("/2fa/setup", authHandler.Setup2FA)
			authProtected.POST("/2fa/verify", authHandler.Verify2FA)
			authProtected.DELETE("/2fa", authHandler.Disable2FA)
			authProtected.POST("/2fa/backup-codes", authHandler.GenerateBackupCodes)
		}

		// Protected routes (require authentication)
		protected := v1.Group("")
		protected.Use(middleware.JWTAuth(jwtManager, redis))
		protected.Use(middleware.TenantRLS(db)) // Wire RLS context for tenant isolation
		protected.Use(middleware.AuditLog(middleware.AuditLogConfig{
			Logger: auditLogger,
		}))

		// Admin routes (super_admin operations)
		admin := protected.Group("/admin")
		{
			// Tenant management routes
			tenants := admin.Group("/tenants")
			{
				tenants.GET("", tenantHandler.List)
				tenants.POST("", middleware.RequireRole("super_admin"), tenantHandler.Create)
				tenants.GET("/:id", middleware.ValidateUUID("id"), tenantHandler.Get)
				tenants.PATCH("/:id", middleware.ValidateUUID("id"), middleware.RequireRole("super_admin", "tenant_admin"), tenantHandler.Update)
				tenants.DELETE("/:id", middleware.ValidateUUID("id"), middleware.RequireRole("super_admin"), tenantHandler.Delete)
			}
		}

		// User routes
		users := protected.Group("/users")
		{
			users.GET("", userHandler.List)
			users.POST("", middleware.RequireRole("super_admin", "tenant_admin"), userHandler.Create)
			users.GET("/:id", middleware.ValidateUUID("id"), userHandler.Get)
			users.PATCH("/:id", middleware.ValidateUUID("id"), userHandler.Update)
			users.DELETE("/:id", middleware.ValidateUUID("id"), middleware.RequireRole("super_admin", "tenant_admin"), userHandler.Delete)
			users.POST("/:id/restore", middleware.ValidateUUID("id"), middleware.RequireRole("super_admin", "tenant_admin"), userHandler.Restore)
			users.DELETE("/:id/2fa", middleware.ValidateUUID("id"), middleware.RequireRole("super_admin", "tenant_admin"), userHandler.Reset2FA)
		}

		// Client routes
		clients := protected.Group("/clients")
		{
			clients.GET("", clientHandler.List)
			clients.GET("/suppressed", clientHandler.ListSuppressed)
			clients.POST("/bulk-reassign", middleware.RequireRole("super_admin", "tenant_admin"), clientHandler.BulkReassign)
			clients.POST("", middleware.RequireRole("super_admin", "tenant_admin", "staff"), clientHandler.Create)
			clients.GET("/:id", middleware.ValidateUUID("id"), clientHandler.Get)
			clients.PATCH("/:id", middleware.ValidateUUID("id"), middleware.RequireRole("super_admin", "tenant_admin", "staff"), clientHandler.Update)
			clients.DELETE("/:id", middleware.ValidateUUID("id"), middleware.RequireRole("super_admin", "tenant_admin"), clientHandler.Delete)
			clients.POST("/:id/restore", middleware.ValidateUUID("id"), middleware.RequireRole("super_admin", "tenant_admin"), clientHandler.Restore)
			clients.GET("/:id/documents", middleware.ValidateUUID("id"), clientHandler.GetDocuments)
			clients.GET("/:id/services", middleware.ValidateUUID("id"), clientHandler.GetServices)
			clients.GET("/:id/emails", middleware.ValidateUUID("id"), clientHandler.GetEmails)
			clients.POST("/:id/assign", middleware.ValidateUUID("id"), middleware.RequireRole("super_admin", "tenant_admin"), clientHandler.AssignStaff)
			// Client Notes
			clients.GET("/:id/notes", middleware.ValidateUUID("id"), clientHandler.ListNotes)
			clients.POST("/:id/notes", middleware.ValidateUUID("id"), clientHandler.CreateNote)
			clients.PATCH("/:id/notes/:noteId", middleware.ValidateUUID("id"), middleware.ValidateUUID("noteId"), clientHandler.UpdateNote)
			clients.DELETE("/:id/notes/:noteId", middleware.ValidateUUID("id"), middleware.ValidateUUID("noteId"), clientHandler.DeleteNote)
		}

		// Service routes
		services := protected.Group("/services")
		{
			services.GET("", serviceHandler.List)
			services.POST("", middleware.RequireRole("super_admin", "tenant_admin", "staff"), serviceHandler.Create)
			services.GET("/deadlines", serviceHandler.GetDeadlines)
			services.GET("/alerts", serviceHandler.GetAlerts)
			services.POST("/bulk-update", middleware.RequireRole("super_admin", "tenant_admin"), serviceHandler.BulkUpdate)
			services.PATCH("/reorder", serviceHandler.Reorder)
			services.GET("/:id", middleware.ValidateUUID("id"), serviceHandler.Get)
			services.PATCH("/:id", middleware.ValidateUUID("id"), middleware.RequireRole("super_admin", "tenant_admin", "staff"), serviceHandler.Update)
			services.PATCH("/:id/status", middleware.ValidateUUID("id"), serviceHandler.UpdateStatus)
			services.POST("/:id/complete", middleware.ValidateUUID("id"), serviceHandler.Complete)
			services.POST("/:id/hmrc-mark", middleware.ValidateUUID("id"), serviceHandler.MarkHMRC)
			services.DELETE("/:id", middleware.ValidateUUID("id"), middleware.RequireRole("super_admin", "tenant_admin"), serviceHandler.Delete)
		}

		// Document routes
		documents := protected.Group("/documents")
		{
			documents.GET("", documentHandler.List)
			documents.POST("", middleware.RequireRole("super_admin", "tenant_admin", "staff"), documentHandler.Create)
			documents.POST("/bulk-request", middleware.RequireRole("super_admin", "tenant_admin", "staff"), documentHandler.BulkRequest)
			documents.POST("/bulk-approve", middleware.RequireRole("super_admin", "tenant_admin"), documentHandler.BulkApprove)
			documents.GET("/firm", documentHandler.ListFirm)
			documents.POST("/upload-url", middleware.RequireRole("super_admin", "tenant_admin", "staff"), documentHandler.GenerateUploadURL)
			documents.POST("/qr", middleware.RequireRole("super_admin", "tenant_admin", "staff"), documentHandler.GenerateQRToken)
			documents.GET("/:id", middleware.ValidateUUID("id"), documentHandler.Get)
			documents.PATCH("/:id", middleware.ValidateUUID("id"), middleware.RequireRole("super_admin", "tenant_admin", "staff"), documentHandler.Update)
			documents.POST("/:id/approve", middleware.ValidateUUID("id"), middleware.RequireRole("super_admin", "tenant_admin"), documentHandler.Approve)
			documents.POST("/:id/reject", middleware.ValidateUUID("id"), middleware.RequireRole("super_admin", "tenant_admin"), documentHandler.Reject)
			documents.GET("/:id/versions", middleware.ValidateUUID("id"), documentHandler.GetVersions)
			documents.POST("/:id/versions/:versionId/restore", middleware.ValidateUUID("id"), documentHandler.RestoreVersion)
			documents.POST("/:id/upload", middleware.ValidateUUID("id"), documentHandler.Upload)
			documents.GET("/:id/download", middleware.ValidateUUID("id"), documentHandler.Download)
		}

		// QR upload routes (public - no auth required)
		qr := v1.Group("/documents/qr")
		{
			qr.GET("/:token", documentHandler.GetQRToken)
			qr.POST("/:token/upload", documentHandler.UploadViaQR)
		}

		// Dashboard routes
		dashboard := protected.Group("/dashboard")
		{
			dashboard.GET("/stats", dashboardHandler.GetStats)
			dashboard.GET("/deadlines", dashboardHandler.GetDeadlines)
			dashboard.GET("/pending-documents", dashboardHandler.GetPendingDocuments)
			dashboard.GET("/workload", middleware.RequireRole("super_admin", "tenant_admin"), dashboardHandler.GetClientWorkload)
			dashboard.GET("/recent-clients", dashboardHandler.GetRecentClients)
			dashboard.GET("/kanban", dashboardHandler.GetKanban)
		}

		// Service Type routes
		serviceTypes := protected.Group("/service-types")
		{
			serviceTypes.GET("", serviceTypeHandler.List)
			serviceTypes.GET("/categories", serviceTypeHandler.GetCategories)
			serviceTypes.POST("", middleware.RequireRole("super_admin", "tenant_admin"), serviceTypeHandler.Create)
			serviceTypes.PATCH("/reorder", middleware.RequireRole("super_admin", "tenant_admin"), serviceTypeHandler.Reorder)
			serviceTypes.GET("/:id", middleware.ValidateUUID("id"), serviceTypeHandler.Get)
			serviceTypes.PATCH("/:id", middleware.ValidateUUID("id"), middleware.RequireRole("super_admin", "tenant_admin"), serviceTypeHandler.Update)
			serviceTypes.DELETE("/:id", middleware.ValidateUUID("id"), middleware.RequireRole("super_admin", "tenant_admin"), serviceTypeHandler.Delete)
			serviceTypes.POST("/:id/clone", middleware.ValidateUUID("id"), middleware.RequireRole("super_admin", "tenant_admin"), serviceTypeHandler.Clone)
			// Service Requirements
			serviceTypes.GET("/:id/requirements", middleware.ValidateUUID("id"), serviceTypeHandler.GetRequirements)
			serviceTypes.POST("/:id/requirements", middleware.ValidateUUID("id"), middleware.RequireRole("super_admin", "tenant_admin"), serviceTypeHandler.AddRequirement)
			serviceTypes.DELETE("/:id/requirements/:docTypeId", middleware.ValidateUUID("id"), middleware.ValidateUUID("docTypeId"), middleware.RequireRole("super_admin", "tenant_admin"), serviceTypeHandler.RemoveRequirement)
		}

		// Document Type routes
		documentTypes := protected.Group("/document-types")
		{
			documentTypes.GET("", documentTypeHandler.List)
			documentTypes.GET("/categories", documentTypeHandler.GetCategories)
			documentTypes.POST("", middleware.RequireRole("super_admin", "tenant_admin"), documentTypeHandler.Create)
			documentTypes.GET("/:id", middleware.ValidateUUID("id"), documentTypeHandler.Get)
			documentTypes.PATCH("/:id", middleware.ValidateUUID("id"), middleware.RequireRole("super_admin", "tenant_admin"), documentTypeHandler.Update)
			documentTypes.DELETE("/:id", middleware.ValidateUUID("id"), middleware.RequireRole("super_admin", "tenant_admin"), documentTypeHandler.Delete)
		}

		// AI routes (proxy to Python AI service)
		aiRoutes := protected.Group("/ai")
		{
			// Chat AI
			aiRoutes.POST("/chat", aiHandler.Chat)
			aiRoutes.POST("/chat/stream", aiHandler.ChatStream)
			aiRoutes.GET("/chat/history", aiHandler.GetChatHistory)
			aiRoutes.DELETE("/chat/:id", aiHandler.DeleteChat)

			// Document AI
			aiRoutes.POST("/documents/extract", aiHandler.ExtractDocument)
			aiRoutes.POST("/documents/classify", aiHandler.ClassifyDocument)
			aiRoutes.POST("/documents/summarize", aiHandler.SummarizeDocument)
			aiRoutes.POST("/documents/rename", aiHandler.RenameDocument)

			// Email AI
			aiRoutes.POST("/emails/summarize", aiHandler.SummarizeEmail)
			aiRoutes.POST("/emails/sentiment", aiHandler.AnalyzeEmailSentiment)
			aiRoutes.POST("/emails/promises", aiHandler.ExtractEmailPromises)
			aiRoutes.POST("/emails/draft", aiHandler.NotImplementedAI)
			aiRoutes.POST("/emails/match-client", aiHandler.NotImplementedAI)
			aiRoutes.POST("/emails/thread-summary", aiHandler.NotImplementedAI)
			aiRoutes.POST("/emails/find-alternate", aiHandler.NotImplementedAI)

			// Form AI
			aiRoutes.POST("/forms/extract", aiHandler.ExtractFormData)
			aiRoutes.POST("/forms/vat", aiHandler.AutoFillVAT)
			aiRoutes.POST("/forms/ct600", aiHandler.AutoFillCT600)
			aiRoutes.POST("/forms/sa", aiHandler.AutoFillSA)

			// Risk AI
			aiRoutes.POST("/risk/client", aiHandler.AnalyzeClientRisk)
			aiRoutes.POST("/risk/service", aiHandler.AnalyzeServiceRisk)

			// Template AI
			aiRoutes.POST("/templates/generate", aiHandler.NotImplementedAI)

			// Client AI
			aiRoutes.POST("/clients/duplicate-check", aiHandler.NotImplementedAI)

			// Service AI
			aiRoutes.POST("/services/auto-name", aiHandler.NotImplementedAI)
			aiRoutes.POST("/services/completion-summary", aiHandler.NotImplementedAI)

			// Dashboard AI
			aiRoutes.POST("/dashboard/troublemakers", aiHandler.NotImplementedAI)
			aiRoutes.POST("/dashboard/anomalies", aiHandler.NotImplementedAI)
			aiRoutes.POST("/staff/activity", aiHandler.NotImplementedAI)

			// AI Jobs
			aiRoutes.GET("/jobs/:id", middleware.ValidateUUID("id"), aiHandler.GetJobStatus)
		}

		// Companies House routes
		chRoutes := protected.Group("/ch")
		{
			chRoutes.GET("/search", companiesHouseHandler.Search)
			chRoutes.GET("/company/:number", companiesHouseHandler.GetCompany)
			chRoutes.POST("/sync/:clientId", middleware.ValidateUUID("clientId"), companiesHouseHandler.SyncClient)
			chRoutes.GET("/status", companiesHouseHandler.Status)
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
				c.Header("X-RateLimit-Limit", strconv.Itoa(totalLimit))
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

			c.Header("X-RateLimit-Limit", strconv.Itoa(totalLimit))
			c.Header("X-RateLimit-Remaining", strconv.Itoa(totalLimit-count))
			c.Next()
			return
		}

		// Set rate limit headers
		remaining := totalLimit - result.CurrentCount
		if remaining < 0 {
			remaining = 0
		}
		c.Header("X-RateLimit-Limit", strconv.Itoa(totalLimit))
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
