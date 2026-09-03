// Package main is the entrypoint for the Accountant CRM Go backend service.
package main

import (
	"context"
	"crypto/subtle"
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
	"github.com/accountant-crm/go-backend/internal/crypto"
	"github.com/accountant-crm/go-backend/internal/database"
	"github.com/accountant-crm/go-backend/internal/email"
	"github.com/accountant-crm/go-backend/internal/handlers"
	"github.com/accountant-crm/go-backend/internal/middleware"
	"github.com/accountant-crm/go-backend/internal/oauth"
	"github.com/accountant-crm/go-backend/internal/storage"
	"github.com/accountant-crm/go-backend/internal/websocket"
	"github.com/accountant-crm/go-backend/internal/worker"
)

// Application holds all the application dependencies.
// This struct reduces the number of parameters passed to setupRouter.
type Application struct {
	Config      *config.Config
	DB          *database.Pool
	Redis       *cache.Client
	AIClient    *ai.Client
	JWT         *auth.JWTManager
	Audit       *audit.Logger
	Handlers    *Handlers
	WSHub       *websocket.Hub
	RateLimiter *middleware.AuthRateLimiter
}

// Handlers holds all HTTP handler instances.
type Handlers struct {
	Auth           *handlers.AuthHandler
	Tenant         *handlers.TenantHandler
	User           *handlers.UserHandler
	Client         *handlers.ClientHandler
	Service        *handlers.ServiceHandler
	Document       *handlers.DocumentHandler
	Dashboard      *handlers.DashboardHandler
	ServiceType    *handlers.ServiceTypeHandler
	DocumentType   *handlers.DocumentTypeHandler
	AI             *handlers.AIHandler
	CompaniesHouse *handlers.CompaniesHouseHandler
	WebSocket      *websocket.Handler
	// Email module handlers
	Email         *handlers.EmailHandler
	EmailTemplate *handlers.EmailTemplateHandler
	EmailAccount  *handlers.EmailAccountHandler
	ChaseLog     *handlers.ChaseLogHandler
	Notification *handlers.NotificationHandler
	Settings     *handlers.SettingsHandler
	ESign        *handlers.ESignHandler
	Portal       *handlers.PortalHandler
	Reminder     *handlers.ReminderHandler
	Subscription *handlers.SubscriptionHandler
	PushToken    *handlers.PushTokenHandler
}

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to load configuration")
	}
	// Cleanup mTLS temp files on shutdown (only if loaded from KMS)
	defer cfg.MTLS.CleanupTempFiles()

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

	// Start background goroutine to cleanup expired tokens daily
	go func() {
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			count, err := sessionManager.CleanupExpiredTokens(context.Background())
			if err != nil {
				log.Error().Err(err).Msg("Failed to cleanup expired tokens")
			} else {
				log.Info().Int64("cleaned", count).Msg("Expired tokens cleanup")
			}
		}
	}()

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

	// Initialize OSS storage client (optional - can be nil if not configured)
	ossClient, err := storage.NewOSSClient(cfg.OSS)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to initialize OSS client")
	}
	if ossClient != nil && ossClient.IsConfigured() {
		log.Info().Msg("OSS storage configured (Alibaba Cloud)")
	} else {
		log.Warn().Msg("OSS storage not configured - file uploads will not be stored in cloud")
	}

	// Initialize auth rate limiter
	authRateLimiter := middleware.NewAuthRateLimiter(redis)

	// Initialize audit logger
	auditLogger := audit.NewLogger(db)

	// Initialize encryptor for sensitive data (IMAP passwords, OAuth tokens)
	var encryptor *crypto.Encryptor
	encryptor, err = crypto.NewEncryptorFromEnv()
	if err != nil {
		if err == crypto.ErrKeyNotConfigured {
			log.Warn().Msg("ENCRYPTION_KEY not configured - IMAP/OAuth credential storage will be disabled")
		} else {
			log.Error().Err(err).Msg("Failed to initialize encryptor")
		}
	} else {
		log.Info().Msg("Credential encryption configured (AES-256-GCM)")
	}

	// Initialize OAuth service for email account connections
	var oauthService *oauth.Service
	if cfg.OAuth.Google.Enabled || cfg.OAuth.Microsoft.Enabled {
		oauthService = oauth.NewService(redis, encryptor, cfg.OAuth)
		log.Info().
			Bool("google", cfg.OAuth.Google.Enabled).
			Bool("microsoft", cfg.OAuth.Microsoft.Enabled).
			Msg("OAuth service initialized")
	} else {
		log.Info().Msg("OAuth not configured - email OAuth connections disabled")
	}

	// Initialize WebSocket hub
	wsHub := websocket.NewHub(redis)
	go wsHub.Run()

	// Build Application with all dependencies
	app := &Application{
		Config:      cfg,
		DB:          db,
		Redis:       redis,
		AIClient:    aiClient,
		JWT:         jwtManager,
		Audit:       auditLogger,
		WSHub:       wsHub,
		RateLimiter: authRateLimiter,
		Handlers: &Handlers{
			Auth:           handlers.NewAuthHandler(db, jwtManager, sessionManager, emailClient, cfg.FrontendURL, authRateLimiter, auditLogger, redis),
			Tenant:         handlers.NewTenantHandler(db),
			User:           handlers.NewUserHandler(db, sessionManager),
			Client:         handlers.NewClientHandler(db, auditLogger),
			Service:        handlers.NewServiceHandler(db, auditLogger),
			Document:       handlers.NewDocumentHandler(db, auditLogger, redis, ossClient),
			Dashboard:      handlers.NewDashboardHandler(db),
			ServiceType:    handlers.NewServiceTypeHandler(db, auditLogger),
			DocumentType:   handlers.NewDocumentTypeHandler(db, auditLogger),
			AI:             handlers.NewAIHandler(aiClient),
			CompaniesHouse: handlers.NewCompaniesHouseHandler(db, redis, cfg.CompaniesHouse),
			WebSocket:      websocket.NewHandler(wsHub, jwtManager, redis),
			// Email module handlers
			Email:         handlers.NewEmailHandler(db, emailClient, auditLogger, authRateLimiter),
			EmailTemplate: handlers.NewEmailTemplateHandler(db, auditLogger),
			EmailAccount:  handlers.NewEmailAccountHandler(db, auditLogger, encryptor, oauthService),
			ChaseLog:     handlers.NewChaseLogHandler(db, emailClient, auditLogger, authRateLimiter),
			Notification: handlers.NewNotificationHandler(db, auditLogger),
			Settings:     handlers.NewSettingsHandler(db, auditLogger),
			ESign:        handlers.NewESignHandler(db, auditLogger, emailClient, cfg.FrontendURL),
			Portal:       handlers.NewPortalHandler(db),
			Reminder:     handlers.NewReminderHandler(db, auditLogger),
			Subscription: handlers.NewSubscriptionHandler(db),
			PushToken:    handlers.NewPushTokenHandler(db),
		},
	}

	// Initialize and start outbox worker for async email processing
	outboxWorker := worker.NewOutboxWorker(db, emailClient)
	outboxWorker.Start()

	// Setup Gin router
	router := setupRouter(app)

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

	// Stop outbox worker first
	outboxWorker.Stop()

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
func setupRouter(app *Application) *gin.Engine {
	cfg := app.Config
	h := app.Handlers

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
		DB:               app.DB,
		CacheTTL:         5 * time.Minute,
	}))

	// Security headers (exact values from API_ENDPOINTS.md)
	router.Use(middleware.SecurityHeaders())

	// SECURITY: Limit request body size to prevent DoS attacks (1MB for JSON)
	// File upload endpoints handle their own limits via http.MaxBytesReader
	router.Use(middleware.BodySizeLimit(middleware.MaxJSONBodySize))

	// Rate limiting (applied to all routes except health checks)
	if cfg.RateLimit.Enabled {
		router.Use(rateLimiter(cfg.RateLimit, app.Redis))
	}

	// Health endpoints (no auth required)
	router.GET("/health", healthHandler())
	router.HEAD("/health", healthHandler())
	router.GET("/ready", readyHandler(app.DB, app.Redis, app.AIClient))

	// Metrics endpoint - protected with bearer token (Fix #12)
	// Only accessible with X-Metrics-Token header matching METRICS_TOKEN env var
	router.GET("/metrics", metricsAuthMiddleware(cfg.App), metricsHandler(app.DB, app.Redis))

	// WebSocket endpoint (public - auth via token query param)
	router.GET("/ws", h.WebSocket.Connect)

	// API v1 routes
	v1 := router.Group("/api/v1")
	{
		// Auth routes (public)
		authRoutes := v1.Group("/auth")
		{
			authRoutes.POST("/login", h.Auth.Login)
			authRoutes.POST("/register", h.Auth.Register)
			authRoutes.POST("/refresh", h.Auth.Refresh)
			authRoutes.POST("/reset-password", h.Auth.ForgotPassword)
			authRoutes.POST("/reset-password/confirm", h.Auth.ResetPassword)
			authRoutes.POST("/magic-link", h.Auth.SendMagicLink)
			authRoutes.GET("/magic-link", h.Auth.VerifyMagicLink)
			authRoutes.POST("/invite-accept", h.Auth.InviteAccept) // Accept invite and set password
			// 2FA backup code verification (public - for lockout recovery)
			authRoutes.POST("/2fa/backup-codes/verify", h.Auth.VerifyBackupCode)
		}

		// Auth routes (protected)
		authProtected := v1.Group("/auth")
		authProtected.Use(middleware.JWTAuth(app.JWT, app.Redis))
		authProtected.Use(middleware.TenantRLS(app.DB)) // Wire RLS context
		authProtected.Use(middleware.AuditLog(middleware.AuditLogConfig{
			Logger:    app.Audit,
			SkipPaths: []string{"/api/v1/auth/refresh"},
		}))
		{
			authProtected.POST("/logout", h.Auth.Logout)
			authProtected.GET("/me", h.Auth.GetMe)
			authProtected.PATCH("/me", h.Auth.UpdateMe)
			authProtected.PATCH("/password", h.Auth.ChangePassword)
			authProtected.GET("/sessions", h.Auth.GetSessions)
			// Token family revocation (for theft detection)
			authProtected.POST("/refresh/revoke-family", h.Auth.RevokeTokenFamily)
			// 2FA endpoints
			authProtected.POST("/2fa/setup", h.Auth.Setup2FA)
			authProtected.POST("/2fa/verify", h.Auth.Verify2FA)
			authProtected.DELETE("/2fa", h.Auth.Disable2FA)
			authProtected.POST("/2fa/backup-codes", h.Auth.GenerateBackupCodes)
		}

		// Protected routes (require authentication)
		protected := v1.Group("")
		protected.Use(middleware.JWTAuth(app.JWT, app.Redis))
		protected.Use(middleware.TenantRLS(app.DB)) // Wire RLS context for tenant isolation
		protected.Use(middleware.AuditLog(middleware.AuditLogConfig{
			Logger: app.Audit,
		}))

		// Admin routes (super_admin operations)
		admin := protected.Group("/admin")
		{
			// Tenant management routes
			tenants := admin.Group("/tenants")
			{
				tenants.GET("", h.Tenant.List)
				tenants.POST("", middleware.RequireRole("super_admin"), h.Tenant.Create)
				tenants.GET("/:id", middleware.ValidateUUID("id"), h.Tenant.Get)
				tenants.PATCH("/:id", middleware.ValidateUUID("id"), middleware.RequireRole("super_admin", "tenant_admin"), h.Tenant.Update)
				tenants.DELETE("/:id", middleware.ValidateUUID("id"), middleware.RequireRole("super_admin"), h.Tenant.Delete)
			}
		}

		// User routes
		users := protected.Group("/users")
		{
			users.GET("", h.User.List)
			users.POST("", middleware.RequireRole("super_admin", "tenant_admin"), h.User.Create)
			users.GET("/:id", middleware.ValidateUUID("id"), h.User.Get)
			users.PATCH("/:id", middleware.ValidateUUID("id"), h.User.Update)
			users.DELETE("/:id", middleware.ValidateUUID("id"), middleware.RequireRole("super_admin", "tenant_admin"), h.User.Delete)
			users.POST("/:id/restore", middleware.ValidateUUID("id"), middleware.RequireRole("super_admin", "tenant_admin"), h.User.Restore)
			users.DELETE("/:id/2fa", middleware.ValidateUUID("id"), middleware.RequireRole("super_admin", "tenant_admin"), h.User.Reset2FA)
		}

		// Client routes
		clients := protected.Group("/clients")
		{
			clients.GET("", h.Client.List)
			clients.GET("/suppressed", h.Client.ListSuppressed)
			clients.POST("/bulk-reassign", middleware.RequireRole("super_admin", "tenant_admin"), h.Client.BulkReassign)
			clients.POST("", middleware.RequireRole("super_admin", "tenant_admin", "staff"), h.Client.Create)
			clients.GET("/:id", middleware.ValidateUUID("id"), h.Client.Get)
			clients.PATCH("/:id", middleware.ValidateUUID("id"), middleware.RequireRole("super_admin", "tenant_admin", "staff"), h.Client.Update)
			clients.DELETE("/:id", middleware.ValidateUUID("id"), middleware.RequireRole("super_admin", "tenant_admin"), h.Client.Delete)
			clients.POST("/:id/restore", middleware.ValidateUUID("id"), middleware.RequireRole("super_admin", "tenant_admin"), h.Client.Restore)
			clients.GET("/:id/documents", middleware.ValidateUUID("id"), h.Client.GetDocuments)
			clients.GET("/:id/services", middleware.ValidateUUID("id"), h.Client.GetServices)
			clients.GET("/:id/emails", middleware.ValidateUUID("id"), h.Client.GetEmails)
			clients.POST("/:id/assign", middleware.ValidateUUID("id"), middleware.RequireRole("super_admin", "tenant_admin"), h.Client.AssignStaff)
			// Client Notes
			clients.GET("/:id/notes", middleware.ValidateUUID("id"), h.Client.ListNotes)
			clients.POST("/:id/notes", middleware.ValidateUUID("id"), h.Client.CreateNote)
			clients.PATCH("/:id/notes/:noteId", middleware.ValidateUUID("id"), middleware.ValidateUUID("noteId"), h.Client.UpdateNote)
			clients.DELETE("/:id/notes/:noteId", middleware.ValidateUUID("id"), middleware.ValidateUUID("noteId"), h.Client.DeleteNote)
		}

		// Service routes
		services := protected.Group("/services")
		{
			services.GET("", h.Service.List)
			services.POST("", middleware.RequireRole("super_admin", "tenant_admin", "staff"), h.Service.Create)
			services.GET("/deadlines", h.Service.GetDeadlines)
			services.GET("/alerts", h.Service.GetAlerts)
			services.POST("/bulk-update", middleware.RequireRole("super_admin", "tenant_admin"), h.Service.BulkUpdate)
			services.PATCH("/reorder", h.Service.Reorder)
			services.GET("/:id", middleware.ValidateUUID("id"), h.Service.Get)
			services.PATCH("/:id", middleware.ValidateUUID("id"), middleware.RequireRole("super_admin", "tenant_admin", "staff"), h.Service.Update)
			services.PATCH("/:id/status", middleware.ValidateUUID("id"), h.Service.UpdateStatus)
			services.POST("/:id/complete", middleware.ValidateUUID("id"), h.Service.Complete)
			services.POST("/:id/hmrc-mark", middleware.ValidateUUID("id"), h.Service.MarkHMRC)
			services.DELETE("/:id", middleware.ValidateUUID("id"), middleware.RequireRole("super_admin", "tenant_admin"), h.Service.Delete)
		}

		// Document routes
		documents := protected.Group("/documents")
		{
			documents.GET("", h.Document.List)
			documents.POST("", middleware.RequireRole("super_admin", "tenant_admin", "staff"), h.Document.Create)
			documents.POST("/bulk-request", middleware.RequireRole("super_admin", "tenant_admin", "staff"), h.Document.BulkRequest)
			documents.POST("/bulk-approve", middleware.RequireRole("super_admin", "tenant_admin"), h.Document.BulkApprove)
			documents.GET("/firm", h.Document.ListFirm)
			documents.POST("/upload-url", middleware.RequireRole("super_admin", "tenant_admin", "staff"), h.Document.GenerateUploadURL)
			documents.POST("/qr", middleware.RequireRole("super_admin", "tenant_admin", "staff"), h.Document.GenerateQRToken)
			documents.GET("/:id", middleware.ValidateUUID("id"), h.Document.Get)
			documents.PATCH("/:id", middleware.ValidateUUID("id"), middleware.RequireRole("super_admin", "tenant_admin", "staff"), h.Document.Update)
			documents.DELETE("/:id", middleware.ValidateUUID("id"), middleware.RequireRole("super_admin", "tenant_admin"), h.Document.Delete)
			documents.POST("/:id/approve", middleware.ValidateUUID("id"), middleware.RequireRole("super_admin", "tenant_admin"), h.Document.Approve)
			documents.POST("/:id/reject", middleware.ValidateUUID("id"), middleware.RequireRole("super_admin", "tenant_admin"), h.Document.Reject)
			documents.GET("/:id/versions", middleware.ValidateUUID("id"), h.Document.GetVersions)
			documents.POST("/:id/versions/:versionId/restore", middleware.ValidateUUID("id"), h.Document.RestoreVersion)
			documents.POST("/:id/upload", middleware.ValidateUUID("id"), h.Document.Upload)
			documents.GET("/:id/download", middleware.ValidateUUID("id"), h.Document.Download)
			// Document renewal workflow
			documents.GET("/expiring", h.Document.ListExpiring)
			documents.POST("/:id/request-renewal", middleware.ValidateUUID("id"), h.Document.RequestRenewal)
			documents.DELETE("/:id/renewal", middleware.ValidateUUID("id"), h.Document.CancelRenewal)
		}

		// QR upload routes (public - no auth required)
		qr := v1.Group("/documents/qr")
		{
			qr.GET("/:token", h.Document.GetQRToken)
			qr.POST("/:token/upload", h.Document.UploadViaQR)
		}

		// E-Sign public routes (no auth required)
		esignPublic := v1.Group("/e-sign/sign")
		{
			esignPublic.GET("/:token", h.ESign.GetSigningPage)
			esignPublic.POST("/:token", h.ESign.SubmitSignature)
		}

		// Dashboard routes
		dashboard := protected.Group("/dashboard")
		{
			dashboard.GET("/stats", h.Dashboard.GetStats)
			dashboard.GET("/deadlines", h.Dashboard.GetDeadlines)
			dashboard.GET("/pending-documents", h.Dashboard.GetPendingDocuments)
			dashboard.GET("/workload", middleware.RequireRole("super_admin", "tenant_admin"), h.Dashboard.GetClientWorkload)
			dashboard.GET("/recent-clients", h.Dashboard.GetRecentClients)
			dashboard.GET("/kanban", h.Dashboard.GetKanban)
		}

		// Service Type routes
		serviceTypes := protected.Group("/service-types")
		{
			serviceTypes.GET("", h.ServiceType.List)
			serviceTypes.GET("/categories", h.ServiceType.GetCategories)
			serviceTypes.POST("", middleware.RequireRole("super_admin", "tenant_admin"), h.ServiceType.Create)
			serviceTypes.PATCH("/reorder", middleware.RequireRole("super_admin", "tenant_admin"), h.ServiceType.Reorder)
			serviceTypes.GET("/:id", middleware.ValidateUUID("id"), h.ServiceType.Get)
			serviceTypes.PATCH("/:id", middleware.ValidateUUID("id"), middleware.RequireRole("super_admin", "tenant_admin"), h.ServiceType.Update)
			serviceTypes.DELETE("/:id", middleware.ValidateUUID("id"), middleware.RequireRole("super_admin", "tenant_admin"), h.ServiceType.Delete)
			serviceTypes.POST("/:id/clone", middleware.ValidateUUID("id"), middleware.RequireRole("super_admin", "tenant_admin"), h.ServiceType.Clone)
			// Service Requirements
			serviceTypes.GET("/:id/requirements", middleware.ValidateUUID("id"), h.ServiceType.GetRequirements)
			serviceTypes.POST("/:id/requirements", middleware.ValidateUUID("id"), middleware.RequireRole("super_admin", "tenant_admin"), h.ServiceType.AddRequirement)
			serviceTypes.DELETE("/:id/requirements/:docTypeId", middleware.ValidateUUID("id"), middleware.ValidateUUID("docTypeId"), middleware.RequireRole("super_admin", "tenant_admin"), h.ServiceType.RemoveRequirement)
		}

		// Document Type routes
		documentTypes := protected.Group("/document-types")
		{
			documentTypes.GET("", h.DocumentType.List)
			documentTypes.GET("/categories", h.DocumentType.GetCategories)
			documentTypes.POST("", middleware.RequireRole("super_admin", "tenant_admin"), h.DocumentType.Create)
			documentTypes.GET("/:id", middleware.ValidateUUID("id"), h.DocumentType.Get)
			documentTypes.PATCH("/:id", middleware.ValidateUUID("id"), middleware.RequireRole("super_admin", "tenant_admin"), h.DocumentType.Update)
			documentTypes.DELETE("/:id", middleware.ValidateUUID("id"), middleware.RequireRole("super_admin", "tenant_admin"), h.DocumentType.Delete)
		}

		// AI routes (proxy to Python AI service)
		aiRoutes := protected.Group("/ai")
		{
			// Chat AI
			aiRoutes.POST("/chat", h.AI.Chat)
			aiRoutes.POST("/chat/stream", h.AI.ChatStream)
			aiRoutes.GET("/chat/history", h.AI.GetChatHistory)
			aiRoutes.POST("/chat/history", h.AI.SaveChatHistory)
			aiRoutes.DELETE("/chat/:id", h.AI.DeleteChat)

			// Document AI
			aiRoutes.POST("/documents/extract", h.AI.ExtractDocument)
			aiRoutes.POST("/documents/classify", h.AI.ClassifyDocument)
			aiRoutes.POST("/documents/summarize", h.AI.SummarizeDocument)
			aiRoutes.POST("/documents/rename", h.AI.RenameDocument)

			// Email AI
			aiRoutes.POST("/emails/summarize", h.AI.SummarizeEmail)
			aiRoutes.POST("/emails/sentiment", h.AI.AnalyzeEmailSentiment)
			aiRoutes.POST("/emails/promises", h.AI.ExtractEmailPromises)
			aiRoutes.POST("/emails/draft", h.AI.NotImplementedAI)
			aiRoutes.POST("/emails/match-client", h.AI.NotImplementedAI)
			aiRoutes.POST("/emails/thread-summary", h.AI.NotImplementedAI)
			aiRoutes.POST("/emails/find-alternate", h.AI.NotImplementedAI)

			// Form AI
			aiRoutes.POST("/forms/extract", h.AI.ExtractFormData)
			aiRoutes.POST("/forms/vat", h.AI.AutoFillVAT)
			aiRoutes.POST("/forms/ct600", h.AI.AutoFillCT600)
			aiRoutes.POST("/forms/sa", h.AI.AutoFillSA)

			// Risk AI
			aiRoutes.POST("/risk/client", h.AI.AnalyzeClientRisk)
			aiRoutes.POST("/risk/service", h.AI.AnalyzeServiceRisk)

			// Template AI
			aiRoutes.POST("/templates/generate", h.AI.NotImplementedAI)

			// Client AI
			aiRoutes.POST("/clients/duplicate-check", h.AI.NotImplementedAI)

			// Service AI
			aiRoutes.POST("/services/auto-name", h.AI.NotImplementedAI)
			aiRoutes.POST("/services/completion-summary", h.AI.NotImplementedAI)

			// Dashboard AI
			aiRoutes.POST("/dashboard/troublemakers", h.AI.NotImplementedAI)
			aiRoutes.POST("/dashboard/anomalies", h.AI.NotImplementedAI)
			aiRoutes.POST("/staff/activity", h.AI.NotImplementedAI)

			// AI Jobs
			aiRoutes.GET("/jobs/:id", middleware.ValidateUUID("id"), h.AI.GetJobStatus)
		}

		// Companies House routes
		chRoutes := protected.Group("/ch")
		{
			chRoutes.GET("/search", h.CompaniesHouse.Search)
			chRoutes.GET("/company/:number", h.CompaniesHouse.GetCompany)
			chRoutes.POST("/sync/:clientId", middleware.ValidateUUID("clientId"), h.CompaniesHouse.SyncClient)
			chRoutes.GET("/status", h.CompaniesHouse.Status)
		}

		// Email routes
		emails := protected.Group("/emails")
		{
			emails.GET("", h.Email.List)
			emails.GET("/stats", h.Email.GetStats)
			emails.POST("", middleware.RequireRole("super_admin", "tenant_admin", "staff"), h.Email.Send)
			emails.POST("/send-template", middleware.RequireRole("super_admin", "tenant_admin", "staff"), h.Email.SendFromTemplate)
			emails.GET("/:id", middleware.ValidateUUID("id"), h.Email.Get)
			emails.PATCH("/:id/read", middleware.ValidateUUID("id"), h.Email.MarkRead)
		}

		// Email Template routes
		emailTemplates := protected.Group("/email-templates")
		{
			emailTemplates.GET("", h.EmailTemplate.List)
			emailTemplates.POST("", middleware.RequireRole("super_admin", "tenant_admin"), h.EmailTemplate.Create)
			emailTemplates.GET("/:id", middleware.ValidateUUID("id"), h.EmailTemplate.Get)
			emailTemplates.PATCH("/:id", middleware.ValidateUUID("id"), middleware.RequireRole("super_admin", "tenant_admin"), h.EmailTemplate.Update)
			emailTemplates.DELETE("/:id", middleware.ValidateUUID("id"), middleware.RequireRole("super_admin", "tenant_admin"), h.EmailTemplate.Delete)
		}

		// Email Account routes
		emailAccounts := protected.Group("/email-accounts")
		{
			emailAccounts.GET("", h.EmailAccount.List)
			emailAccounts.POST("/imap", middleware.RequireRole("super_admin", "tenant_admin"), h.EmailAccount.CreateIMAP)
			emailAccounts.GET("/oauth/:provider", h.EmailAccount.OAuthInitiate)
			emailAccounts.GET("/oauth/:provider/callback", h.EmailAccount.OAuthCallback)
			emailAccounts.GET("/:id", middleware.ValidateUUID("id"), h.EmailAccount.Get)
			emailAccounts.PATCH("/:id", middleware.ValidateUUID("id"), middleware.RequireRole("super_admin", "tenant_admin"), h.EmailAccount.Update)
			emailAccounts.DELETE("/:id", middleware.ValidateUUID("id"), middleware.RequireRole("super_admin", "tenant_admin"), h.EmailAccount.Delete)
			emailAccounts.POST("/:id/sync", middleware.ValidateUUID("id"), h.EmailAccount.Sync)
			emailAccounts.POST("/:id/test", middleware.ValidateUUID("id"), h.EmailAccount.TestConnection)
			emailAccounts.POST("/:id/disconnect", middleware.ValidateUUID("id"), h.EmailAccount.Disconnect)
			emailAccounts.POST("/:id/reconnect", middleware.ValidateUUID("id"), h.EmailAccount.Reconnect)
		}

		// Chase Log routes
		chaseLogs := protected.Group("/chase-logs")
		{
			chaseLogs.GET("", h.ChaseLog.List)
			chaseLogs.GET("/stats", h.ChaseLog.GetStats)
			chaseLogs.POST("", middleware.RequireRole("super_admin", "tenant_admin", "staff"), h.ChaseLog.Create)
			chaseLogs.GET("/:id", middleware.ValidateUUID("id"), h.ChaseLog.Get)
		}

		// Notification routes
		notifications := protected.Group("/notifications")
		{
			notifications.GET("", h.Notification.List)
			notifications.GET("/unread-count", h.Notification.GetUnreadCount)
			notifications.POST("", middleware.RequireRole("super_admin", "tenant_admin"), h.Notification.Create)
			notifications.POST("/read-all", h.Notification.MarkAllRead)
			notifications.POST("/dismiss-all", h.Notification.DismissAll)
			notifications.GET("/:id", middleware.ValidateUUID("id"), h.Notification.Get)
			notifications.PATCH("/:id/read", middleware.ValidateUUID("id"), h.Notification.MarkRead)
			notifications.DELETE("/:id", middleware.ValidateUUID("id"), h.Notification.Dismiss)
		}

		// Settings routes
		settings := protected.Group("/settings")
		{
			settings.GET("", h.Settings.Get)
			settings.PATCH("", middleware.RequireRole("super_admin", "tenant_admin"), h.Settings.Update)
			settings.GET("/branding", h.Settings.GetBranding)
			settings.PATCH("/branding", middleware.RequireRole("super_admin", "tenant_admin"), h.Settings.UpdateBranding)
		}

		// E-Sign routes
		esign := protected.Group("/e-sign")
		{
			esign.GET("", h.ESign.List)
			esign.POST("", middleware.RequireRole("super_admin", "tenant_admin", "staff"), h.ESign.Create)
			esign.GET("/:id", middleware.ValidateUUID("id"), h.ESign.Get)
			esign.POST("/:id/send", middleware.ValidateUUID("id"), h.ESign.Send)
			esign.DELETE("/:id", middleware.ValidateUUID("id"), h.ESign.Delete)
		}

		// Reminder routes
		reminders := protected.Group("/reminders")
		{
			reminders.GET("", h.Reminder.List)
			reminders.GET("/upcoming", h.Reminder.GetUpcoming)
			reminders.POST("", h.Reminder.Create)
			reminders.GET("/:id", middleware.ValidateUUID("id"), h.Reminder.Get)
			reminders.POST("/:id/complete", middleware.ValidateUUID("id"), h.Reminder.Complete)
			reminders.POST("/:id/dismiss", middleware.ValidateUUID("id"), h.Reminder.Dismiss)
			reminders.DELETE("/:id", middleware.ValidateUUID("id"), h.Reminder.Delete)
		}

		// Subscription routes (admin only)
		subscription := protected.Group("/subscription")
		{
			subscription.GET("", h.Subscription.Get)
			subscription.GET("/invoices", h.Subscription.ListInvoices)
			subscription.GET("/usage", h.Subscription.GetUsage)
			subscription.POST("/portal", middleware.RequireRole("super_admin", "tenant_admin"), h.Subscription.CreatePortalSession)
			subscription.POST("/checkout", middleware.RequireRole("super_admin", "tenant_admin"), h.Subscription.CreateCheckoutSession)
		}

		// Push Token routes
		pushTokens := protected.Group("/push-tokens")
		{
			pushTokens.GET("", h.PushToken.List)
			pushTokens.POST("", h.PushToken.Register)
			pushTokens.POST("/unregister", h.PushToken.UnregisterByToken)
			pushTokens.DELETE("/:id", middleware.ValidateUUID("id"), h.PushToken.Unregister)
		}

		// Client Portal routes (client role only)
		portal := protected.Group("/portal")
		portal.Use(middleware.RequireRole("client"))
		{
			portal.GET("/dashboard", h.Portal.Dashboard)
			portal.GET("/me", h.Portal.GetProfile)
			portal.PATCH("/me", h.Portal.UpdateProfile)
			portal.GET("/documents", h.Portal.ListDocuments)
			portal.GET("/services", h.Portal.ListServices)
			portal.GET("/deadlines", h.Portal.ListDeadlines)
			portal.POST("/password", app.RateLimiter.PasswordChangeLimit(), h.Portal.ChangePassword)
		}

		// WebSocket routes (protected)
		wsRoutes := protected.Group("/ws")
		{
			wsRoutes.GET("", h.WebSocket.ConnectAuthenticated)
			wsRoutes.GET("/stats", middleware.RequireRole("super_admin", "tenant_admin"), h.WebSocket.Stats)
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
// Fix #37: Added max size cap and periodic cleanup to prevent unbounded growth
type memoryRateLimiter struct {
	counts   map[string]int
	resets   map[string]time.Time
	mu       sync.Mutex
	maxSize  int
	cleanups int // Track cleanups for monitoring
}

const fallbackLimiterMaxSize = 10000 // Max unique IPs to track

var fallbackLimiter = &memoryRateLimiter{
	counts:  make(map[string]int),
	resets:  make(map[string]time.Time),
	maxSize: fallbackLimiterMaxSize,
}

func (m *memoryRateLimiter) check(ip string, limit int, window time.Duration) (allowed bool, count int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()

	// Fix #37: Cleanup expired entries if we're at capacity
	if len(m.counts) >= m.maxSize {
		m.cleanupExpired(now)
		m.cleanups++

		// If still at capacity after cleanup, reject new IPs (defensive)
		if len(m.counts) >= m.maxSize {
			_, exists := m.counts[ip]
			if !exists {
				// New IP when at capacity - rate limit it
				return false, limit + 1
			}
		}
	}

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

// cleanupExpired removes entries with expired windows (must be called with lock held)
func (m *memoryRateLimiter) cleanupExpired(now time.Time) {
	for ip, resetTime := range m.resets {
		if now.After(resetTime) {
			delete(m.counts, ip)
			delete(m.resets, ip)
		}
	}
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

// memStatsCache holds periodically sampled memory statistics to avoid STW pauses.
var memStatsCache struct {
	sync.RWMutex
	allocMB uint64
}

// healthHandler returns a handler for liveness checks.
// Fix #26: Added basic runtime checks to ensure process is healthy.
// Fix #27: Memory stats sampled every 30s to avoid STW pauses on frequent ALB probes.
func healthHandler() gin.HandlerFunc {
	startTime := time.Now()

	// Sample memory stats immediately on startup
	var initialStats runtime.MemStats
	runtime.ReadMemStats(&initialStats)
	memStatsCache.Lock()
	memStatsCache.allocMB = initialStats.Alloc / 1024 / 1024
	memStatsCache.Unlock()

	// Background goroutine samples memory stats every 30 seconds
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			var stats runtime.MemStats
			runtime.ReadMemStats(&stats)
			memStatsCache.Lock()
			memStatsCache.allocMB = stats.Alloc / 1024 / 1024
			memStatsCache.Unlock()
		}
	}()

	return func(c *gin.Context) {
		memStatsCache.RLock()
		allocMB := memStatsCache.allocMB
		memStatsCache.RUnlock()

		c.JSON(http.StatusOK, gin.H{
			"status":     "ok",
			"uptime":     time.Since(startTime).String(),
			"goroutines": runtime.NumGoroutine(),
			"memory_mb":  allocMB,
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
// Fix #32: Read METRICS_TOKEN once at startup instead of per-request.
func metricsAuthMiddleware(cfg config.AppConfig) gin.HandlerFunc {
	// Capture token at startup - not per request
	expectedToken := os.Getenv("METRICS_TOKEN")
	isDev := cfg.Env == "development"
	tokenConfigured := expectedToken != ""

	// Log warning once at startup if not configured in non-dev
	if !isDev && !tokenConfigured {
		log.Warn().Msg("METRICS_TOKEN not set, metrics endpoint will be blocked")
	}

	return func(c *gin.Context) {
		// In development, allow access without token
		if isDev && !tokenConfigured {
			c.Next()
			return
		}

		// Require token in production/staging
		if !tokenConfigured {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error":   "forbidden",
				"message": "Metrics endpoint not configured",
			})
			return
		}

		// Check X-Metrics-Token header using constant-time comparison
		// Fix #32: Prevent timing attacks on token comparison
		token := c.GetHeader("X-Metrics-Token")
		if subtle.ConstantTimeCompare([]byte(token), []byte(expectedToken)) != 1 {
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
