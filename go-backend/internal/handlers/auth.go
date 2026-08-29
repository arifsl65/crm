package handlers

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/rs/zerolog/log"

	"github.com/accountant-crm/go-backend/internal/auth"
	"github.com/accountant-crm/go-backend/internal/database"
	"github.com/accountant-crm/go-backend/internal/email"
	"github.com/accountant-crm/go-backend/internal/middleware"
)

type AuthHandler struct {
	db          *database.Pool
	jwt         *auth.JWTManager
	session     *auth.SessionManager
	email       *email.Client
	frontendURL string
	rateLimiter *middleware.AuthRateLimiter
}

func NewAuthHandler(db *database.Pool, jwt *auth.JWTManager, session *auth.SessionManager, emailClient *email.Client, frontendURL string, rateLimiter *middleware.AuthRateLimiter) *AuthHandler {
	return &AuthHandler{
		db:          db,
		jwt:         jwt,
		session:     session,
		email:       emailClient,
		frontendURL: frontendURL,
		rateLimiter: rateLimiter,
	}
}

type LoginRequest struct {
	Email        string `json:"email" binding:"required,email"`
	Password     string `json:"password" binding:"required,min=8"`
	TenantDomain string `json:"tenant_domain"` // Optional: for multi-tenant login resolution
}

type RegisterRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=8"`
	Name     string `json:"name" binding:"required"`
	TenantID string `json:"tenant_id" binding:"required,uuid"`
}

type AuthResponse struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
	User         UserInfo  `json:"user"`
}

type UserInfo struct {
	ID       string `json:"id"`
	Email    string `json:"email"`
	Name     string `json:"name"`
	Role     string `json:"role"`
	TenantID string `json:"tenant_id"`
}

type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "validation_error",
			"message": "Invalid request body",
		})
		return
	}

	ctx := c.Request.Context()
	ip := c.ClientIP()

	// Rate limit check: 5 attempts per IP+email, 15min window
	if h.rateLimiter != nil {
		allowed, count, ttl, err := h.rateLimiter.CheckLoginRate(ctx, ip, req.Email)
		if err != nil {
			log.Warn().Err(err).Msg("Rate limit check failed, allowing request")
		} else if !allowed {
			middleware.RateLimitExceededWithLog(c, "/auth/login", ip+":"+req.Email, count, ttl)
			return
		}
	}

	// Resolve tenant if domain provided
	var tenantID *uuid.UUID
	if req.TenantDomain != "" {
		tid, err := h.getTenantByDomain(ctx, req.TenantDomain)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error":   "invalid_credentials",
				"message": "Invalid email or password",
			})
			return
		}
		tenantID = &tid
	}

	user, err := h.getUserByEmail(ctx, req.Email, tenantID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error":   "invalid_credentials",
				"message": "Invalid email or password",
			})
			return
		}
		if err.Error() == "multiple_tenants" {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "tenant_required",
				"message": "Email exists in multiple tenants. Please specify tenant_domain.",
			})
			return
		}
		log.Error().Err(err).Str("email", req.Email).Msg("Failed to fetch user")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "internal_error",
			"message": "An error occurred",
		})
		return
	}

	if user.LockedUntil != nil && user.LockedUntil.After(time.Now()) {
		c.JSON(http.StatusForbidden, gin.H{
			"error":   "account_locked",
			"message": "Account is temporarily locked",
		})
		return
	}

	valid, err := auth.VerifyPassword(req.Password, user.Password)
	if err != nil || !valid {
		_ = h.incrementFailedAttempts(ctx, user.ID)
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":   "invalid_credentials",
			"message": "Invalid email or password",
		})
		return
	}

	_ = h.resetFailedAttempts(ctx, user.ID)
	_ = h.updateLastLogin(ctx, user.ID)

	// Clear rate limit on successful login
	if h.rateLimiter != nil {
		_ = h.rateLimiter.ClearLoginRate(ctx, ip, req.Email)
	}

	// Use uuid.Nil for super_admin users (NULL tenant_id)
	tokenTenantID := uuid.Nil
	tenantIDStr := ""
	if user.TenantID != nil {
		tokenTenantID = *user.TenantID
		tenantIDStr = user.TenantID.String()
	}

	tokenPair, err := h.jwt.GenerateTokenPair(user.ID, tokenTenantID, user.Role)
	if err != nil {
		log.Error().Err(err).Msg("Failed to generate token pair")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "internal_error",
			"message": "An error occurred",
		})
		return
	}

	// Store refresh token in database for revocation tracking
	ipAddress := c.ClientIP()
	userAgent := c.Request.UserAgent()
	_, err = h.session.StoreRefreshToken(ctx, user.ID, user.TenantID, tokenPair.RefreshToken, ipAddress, userAgent)
	if err != nil {
		log.Error().Err(err).Msg("Failed to store refresh token")
		// Continue anyway - token will still work, just won't be revocable
	}

	c.JSON(http.StatusOK, AuthResponse{
		AccessToken:  tokenPair.AccessToken,
		RefreshToken: tokenPair.RefreshToken,
		ExpiresAt:    tokenPair.ExpiresAt,
		User: UserInfo{
			ID:       user.ID.String(),
			Email:    user.Email,
			Name:     user.Name,
			Role:     user.Role,
			TenantID: tenantIDStr,
		},
	})
}

func (h *AuthHandler) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "validation_error",
			"message": "Invalid request body",
		})
		return
	}

	tenantID, _ := uuid.Parse(req.TenantID)
	ctx := c.Request.Context()

	exists, err := h.emailExists(ctx, req.Email)
	if err != nil {
		log.Error().Err(err).Msg("Failed to check email")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "internal_error",
			"message": "An error occurred",
		})
		return
	}
	if exists {
		c.JSON(http.StatusConflict, gin.H{
			"error":   "email_exists",
			"message": "Email already registered",
		})
		return
	}

	passwordHash, err := auth.HashPassword(req.Password)
	if err != nil {
		log.Error().Err(err).Msg("Failed to hash password")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "internal_error",
			"message": "An error occurred",
		})
		return
	}

	userID := uuid.New()
	err = h.createUser(ctx, userID, tenantID, req.Email, passwordHash, req.Name)
	if err != nil {
		log.Error().Err(err).Msg("Failed to create user")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "internal_error",
			"message": "An error occurred",
		})
		return
	}

	tokenPair, err := h.jwt.GenerateTokenPair(userID, tenantID, "staff")
	if err != nil {
		log.Error().Err(err).Msg("Failed to generate token pair")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "internal_error",
			"message": "An error occurred",
		})
		return
	}

	// Store refresh token in database
	ipAddress := c.ClientIP()
	userAgent := c.Request.UserAgent()
	_, err = h.session.StoreRefreshToken(ctx, userID, &tenantID, tokenPair.RefreshToken, ipAddress, userAgent)
	if err != nil {
		log.Error().Err(err).Msg("Failed to store refresh token")
		// Continue anyway
	}

	c.JSON(http.StatusCreated, AuthResponse{
		AccessToken:  tokenPair.AccessToken,
		RefreshToken: tokenPair.RefreshToken,
		ExpiresAt:    tokenPair.ExpiresAt,
		User: UserInfo{
			ID:       userID.String(),
			Email:    req.Email,
			Name:     req.Name,
			Role:     "staff",
			TenantID: tenantID.String(),
		},
	})
}

func (h *AuthHandler) Refresh(c *gin.Context) {
	var req RefreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "validation_error",
			"message": "Invalid request body",
		})
		return
	}

	ctx := c.Request.Context()

	// Validate refresh token from database (checks revocation, expiry, reuse)
	tokenRecord, err := h.session.ValidateRefreshToken(ctx, req.RefreshToken)
	if err == nil && h.rateLimiter != nil {
		// Rate limit check: 10 requests per token family, 1 hour window
		allowed, count, ttl, rlErr := h.rateLimiter.CheckRefreshRate(ctx, tokenRecord.Family.String())
		if rlErr != nil {
			log.Warn().Err(rlErr).Msg("Rate limit check failed, allowing request")
		} else if !allowed {
			middleware.RateLimitExceededWithLog(c, "/auth/refresh", tokenRecord.Family.String(), count, ttl)
			return
		}
	}
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":   "invalid_token",
			"message": "Invalid or expired refresh token",
		})
		return
	}

	// Parse the JWT to get claims (we know it's valid from DB check)
	claims, err := h.jwt.ValidateToken(req.RefreshToken)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":   "invalid_token",
			"message": "Invalid or expired refresh token",
		})
		return
	}

	// Generate new token pair
	tokenPair, err := h.jwt.GenerateTokenPair(claims.UserID, claims.TenantID, claims.Role)
	if err != nil {
		log.Error().Err(err).Msg("Failed to generate token pair")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "internal_error",
			"message": "An error occurred",
		})
		return
	}

	// Store new refresh token with rotation (marks old one as used)
	ipAddress := c.ClientIP()
	userAgent := c.Request.UserAgent()
	err = h.session.StoreRotatedRefreshToken(ctx, claims.UserID, tokenRecord.TenantID, tokenPair.RefreshToken, req.RefreshToken, ipAddress, userAgent, tokenRecord.Family)
	if err != nil {
		log.Error().Err(err).Msg("Failed to store rotated refresh token")
		// Continue anyway - token rotation not critical
	}

	c.JSON(http.StatusOK, gin.H{
		"access_token":  tokenPair.AccessToken,
		"refresh_token": tokenPair.RefreshToken,
		"expires_at":    tokenPair.ExpiresAt,
	})
}

type LogoutRequest struct {
	RefreshToken string `json:"refresh_token"`
	RevokeAll    bool   `json:"revoke_all"` // If true, revoke all sessions for user
}

func (h *AuthHandler) Logout(c *gin.Context) {
	var req LogoutRequest
	// Body is optional for logout
	_ = c.ShouldBindJSON(&req)

	ctx := c.Request.Context()
	userID := middleware.GetUserID(c)

	if req.RevokeAll {
		// Revoke all user sessions
		if err := h.session.RevokeAllUserTokens(ctx, userID); err != nil {
			log.Error().Err(err).Str("user_id", userID.String()).Msg("Failed to revoke all user tokens")
		}
		c.JSON(http.StatusOK, gin.H{
			"message": "Logged out from all devices",
		})
		return
	}

	if req.RefreshToken != "" {
		// Revoke specific refresh token
		if err := h.session.RevokeRefreshToken(ctx, req.RefreshToken); err != nil {
			log.Error().Err(err).Msg("Failed to revoke refresh token")
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Logged out successfully",
	})
}

type userRecord struct {
	ID                  uuid.UUID
	TenantID            *uuid.UUID // Nullable for super_admin users
	Email               string
	Password            string
	Name                string
	Role                string
	FailedLoginAttempts int
	LockedUntil         *time.Time
}

func (h *AuthHandler) getUserByEmail(ctx context.Context, email string, tenantID *uuid.UUID) (*userRecord, error) {
	email = strings.ToLower(email)

	if tenantID != nil {
		// Tenant-scoped lookup
		query := `
			SELECT id, tenant_id, email, password, name, role,
			       failed_login_attempts, locked_until
			FROM users
			WHERE email = $1 AND tenant_id = $2 AND deleted_at IS NULL
		`
		var user userRecord
		err := h.db.QueryRow(ctx, query, email, *tenantID).Scan(
			&user.ID, &user.TenantID, &user.Email, &user.Password,
			&user.Name, &user.Role,
			&user.FailedLoginAttempts, &user.LockedUntil,
		)
		return &user, err
	}

	// No tenant specified - check how many tenants have this email
	countQuery := `SELECT COUNT(DISTINCT tenant_id) FROM users WHERE email = $1 AND deleted_at IS NULL`
	var count int
	if err := h.db.QueryRow(ctx, countQuery, email).Scan(&count); err != nil {
		return nil, err
	}

	if count > 1 {
		return nil, errors.New("multiple_tenants")
	}

	// Single tenant or super_admin - proceed with lookup
	query := `
		SELECT id, tenant_id, email, password, name, role,
		       failed_login_attempts, locked_until
		FROM users
		WHERE email = $1 AND deleted_at IS NULL
	`
	var user userRecord
	err := h.db.QueryRow(ctx, query, email).Scan(
		&user.ID, &user.TenantID, &user.Email, &user.Password,
		&user.Name, &user.Role,
		&user.FailedLoginAttempts, &user.LockedUntil,
	)
	return &user, err
}

func (h *AuthHandler) emailExists(ctx context.Context, email string) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM users WHERE email = $1 AND deleted_at IS NULL)`
	var exists bool
	err := h.db.QueryRow(ctx, query, strings.ToLower(email)).Scan(&exists)
	return exists, err
}

func (h *AuthHandler) createUser(ctx context.Context, id, tenantID uuid.UUID, email, password, name string) error {
	query := `
		INSERT INTO users (id, tenant_id, email, password, name, role, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, 'staff', NOW(), NOW())
	`
	_, err := h.db.Exec(ctx, query, id, tenantID, strings.ToLower(email), password, name)
	return err
}

func (h *AuthHandler) incrementFailedAttempts(ctx context.Context, userID uuid.UUID) error {
	// Exponential backoff lockout per spec:
	// - After 5 failed attempts, start locking
	// - Lock duration starts at 15 min and doubles each subsequent failure
	// - 5 fails → 15 min, 6 fails → 30 min, 7 fails → 60 min, etc.
	// - Capped at 1440 minutes (24 hours)
	// Note: SQL uses OLD value in SET, so when current=4 (becomes 5), formula uses 4
	query := `
		UPDATE users
		SET failed_login_attempts = failed_login_attempts + 1,
		    locked_until = CASE
		        WHEN failed_login_attempts >= 4 THEN
		            NOW() + LEAST(
		                15 * POWER(2, GREATEST(failed_login_attempts - 4, 0))::integer,
		                1440
		            ) * INTERVAL '1 minute'
		        ELSE locked_until
		    END,
		    updated_at = NOW()
		WHERE id = $1
	`
	_, err := h.db.Exec(ctx, query, userID)
	return err
}

func (h *AuthHandler) resetFailedAttempts(ctx context.Context, userID uuid.UUID) error {
	query := `
		UPDATE users
		SET failed_login_attempts = 0, locked_until = NULL, updated_at = NOW()
		WHERE id = $1
	`
	_, err := h.db.Exec(ctx, query, userID)
	return err
}

func (h *AuthHandler) updateLastLogin(ctx context.Context, userID uuid.UUID) error {
	query := `UPDATE users SET last_login_at = NOW(), updated_at = NOW() WHERE id = $1`
	_, err := h.db.Exec(ctx, query, userID)
	return err
}

func (h *AuthHandler) getTenantByDomain(ctx context.Context, domain string) (uuid.UUID, error) {
	query := `
		SELECT id FROM tenants
		WHERE (domain = $1 OR custom_domain = $1) AND is_active = true AND deleted_at IS NULL
	`
	var tenantID uuid.UUID
	err := h.db.QueryRow(ctx, query, strings.ToLower(domain)).Scan(&tenantID)
	return tenantID, err
}

// ============================================================================
// Password Reset
// ============================================================================

type ForgotPasswordRequest struct {
	Email string `json:"email" binding:"required,email"`
}

type ResetPasswordRequest struct {
	Token       string `json:"token" binding:"required"`
	NewPassword string `json:"new_password" binding:"required,min=8"`
}

// ForgotPassword handles password reset requests.
// POST /api/v1/auth/reset-password
func (h *AuthHandler) ForgotPassword(c *gin.Context) {
	var req ForgotPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "validation_error",
			"message": "Invalid request body",
		})
		return
	}

	ctx := c.Request.Context()

	// Rate limit check: 3 requests per email, 1 hour window
	if h.rateLimiter != nil {
		allowed, count, ttl, err := h.rateLimiter.CheckResetPasswordRate(ctx, req.Email)
		if err != nil {
			log.Warn().Err(err).Msg("Rate limit check failed, allowing request")
		} else if !allowed {
			middleware.RateLimitExceededWithLog(c, "/auth/reset-password", req.Email, count, ttl)
			return
		}
	}

	// Always return success to prevent email enumeration
	defer func() {
		c.JSON(http.StatusOK, gin.H{
			"message": "If an account exists with this email, a reset link has been sent.",
		})
	}()

	// Find user by email
	user, err := h.getUserByEmailForReset(ctx, strings.ToLower(req.Email))
	if err != nil {
		log.Debug().Err(err).Str("email", req.Email).Msg("User not found for password reset")
		return
	}

	// Generate reset token (32 bytes = 64 hex chars)
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		log.Error().Err(err).Msg("Failed to generate reset token")
		return
	}
	resetToken := hex.EncodeToString(tokenBytes)

	// Store reset token with 1 hour expiry
	expiresAt := time.Now().Add(1 * time.Hour)
	err = h.storeResetToken(ctx, user.ID, resetToken, expiresAt)
	if err != nil {
		log.Error().Err(err).Msg("Failed to store reset token")
		return
	}

	// Send email
	if h.email != nil && h.email.IsConfigured() {
		resetURL := fmt.Sprintf("%s/reset-password?token=%s", h.frontendURL, resetToken)
		err = h.email.SendPasswordReset(user.Email, resetURL, user.Name)
		if err != nil {
			log.Error().Err(err).Str("email", user.Email).Msg("Failed to send password reset email")
		} else {
			log.Info().Str("email", user.Email).Msg("Password reset email sent")
		}
	} else {
		log.Warn().Msg("Email client not configured, skipping password reset email")
	}
}

// ResetPassword handles password reset confirmation.
// POST /api/v1/auth/reset-password/confirm
func (h *AuthHandler) ResetPassword(c *gin.Context) {
	var req ResetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "validation_error",
			"message": "Invalid request body",
		})
		return
	}

	ctx := c.Request.Context()

	// Validate reset token
	userID, err := h.validateResetToken(ctx, req.Token)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid_token",
			"message": "Invalid or expired reset token",
		})
		return
	}

	// Hash new password
	passwordHash, err := auth.HashPassword(req.NewPassword)
	if err != nil {
		log.Error().Err(err).Msg("Failed to hash password")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "internal_error",
			"message": "An error occurred",
		})
		return
	}

	// Update password and clear reset token
	err = h.updatePasswordAndClearToken(ctx, userID, passwordHash)
	if err != nil {
		log.Error().Err(err).Msg("Failed to update password")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "internal_error",
			"message": "An error occurred",
		})
		return
	}

	// Revoke all existing sessions for security
	_ = h.session.RevokeAllUserTokens(ctx, userID)

	log.Info().Str("user_id", userID.String()).Msg("Password reset completed")

	c.JSON(http.StatusOK, gin.H{
		"message": "Password has been reset successfully. Please log in with your new password.",
	})
}

type userRecordForReset struct {
	ID    uuid.UUID
	Email string
	Name  string
}

func (h *AuthHandler) getUserByEmailForReset(ctx context.Context, email string) (*userRecordForReset, error) {
	query := `SELECT id, email, name FROM users WHERE email = $1 AND deleted_at IS NULL`
	var user userRecordForReset
	err := h.db.QueryRow(ctx, query, email).Scan(&user.ID, &user.Email, &user.Name)
	return &user, err
}

func (h *AuthHandler) storeResetToken(ctx context.Context, userID uuid.UUID, token string, expiresAt time.Time) error {
	query := `UPDATE users SET reset_token = $1, reset_token_expires = $2, updated_at = NOW() WHERE id = $3`
	_, err := h.db.Exec(ctx, query, token, expiresAt, userID)
	return err
}

func (h *AuthHandler) validateResetToken(ctx context.Context, token string) (uuid.UUID, error) {
	query := `
		SELECT id FROM users
		WHERE reset_token = $1
		AND reset_token_expires > NOW()
		AND deleted_at IS NULL
	`
	var userID uuid.UUID
	err := h.db.QueryRow(ctx, query, token).Scan(&userID)
	if err != nil {
		return uuid.Nil, errors.New("invalid or expired token")
	}
	return userID, nil
}

func (h *AuthHandler) updatePasswordAndClearToken(ctx context.Context, userID uuid.UUID, passwordHash string) error {
	query := `
		UPDATE users
		SET password = $1, reset_token = NULL, reset_token_expires = NULL,
		    failed_login_attempts = 0, locked_until = NULL, updated_at = NOW()
		WHERE id = $2
	`
	_, err := h.db.Exec(ctx, query, passwordHash, userID)
	return err
}

// ============================================================================
// Current User Endpoints
// ============================================================================

// GetMe returns the current user's profile.
// GET /api/v1/auth/me
func (h *AuthHandler) GetMe(c *gin.Context) {
	userID := middleware.GetUserID(c)
	ctx := c.Request.Context()

	query := `
		SELECT id, tenant_id, email, name, role, last_login_at, created_at
		FROM users WHERE id = $1 AND deleted_at IS NULL
	`

	var user struct {
		ID          uuid.UUID
		TenantID    *uuid.UUID
		Email       string
		Name        string
		Role        string
		LastLoginAt *time.Time
		CreatedAt   time.Time
	}

	err := h.db.QueryRow(ctx, query, userID).Scan(
		&user.ID, &user.TenantID, &user.Email, &user.Name, &user.Role,
		&user.LastLoginAt, &user.CreatedAt,
	)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "not_found",
			"message": "User not found",
		})
		return
	}

	tenantIDStr := ""
	if user.TenantID != nil {
		tenantIDStr = user.TenantID.String()
	}

	c.JSON(http.StatusOK, gin.H{
		"id":            user.ID.String(),
		"tenant_id":     tenantIDStr,
		"email":         user.Email,
		"name":          user.Name,
		"role":          user.Role,
		"last_login_at": user.LastLoginAt,
		"created_at":    user.CreatedAt,
	})
}

// UpdateMe updates the current user's profile.
// PATCH /api/v1/auth/me
func (h *AuthHandler) UpdateMe(c *gin.Context) {
	userID := middleware.GetUserID(c)
	ctx := c.Request.Context()

	var req struct {
		Name string `json:"name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "validation_error",
			"message": "Invalid request body",
		})
		return
	}

	query := `UPDATE users SET name = $1, updated_at = NOW() WHERE id = $2`
	_, err := h.db.Exec(ctx, query, req.Name, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "internal_error",
			"message": "An error occurred",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Profile updated successfully",
	})
}

// ChangePassword allows the current user to change their password.
// PATCH /api/v1/auth/password
func (h *AuthHandler) ChangePassword(c *gin.Context) {
	userID := middleware.GetUserID(c)
	ctx := c.Request.Context()

	var req struct {
		CurrentPassword string `json:"current_password" binding:"required"`
		NewPassword     string `json:"new_password" binding:"required,min=8"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "validation_error",
			"message": "Invalid request body",
		})
		return
	}

	// Get current password hash
	var currentHash string
	err := h.db.QueryRow(ctx, `SELECT password FROM users WHERE id = $1`, userID).Scan(&currentHash)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "internal_error",
			"message": "An error occurred",
		})
		return
	}

	// Verify current password
	valid, err := auth.VerifyPassword(req.CurrentPassword, currentHash)
	if err != nil || !valid {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":   "invalid_password",
			"message": "Current password is incorrect",
		})
		return
	}

	// Hash new password
	newHash, err := auth.HashPassword(req.NewPassword)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "internal_error",
			"message": "An error occurred",
		})
		return
	}

	// Update password
	_, err = h.db.Exec(ctx, `UPDATE users SET password = $1, updated_at = NOW() WHERE id = $2`, newHash, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "internal_error",
			"message": "An error occurred",
		})
		return
	}

	// Revoke all other sessions
	_ = h.session.RevokeAllUserTokens(ctx, userID)

	c.JSON(http.StatusOK, gin.H{
		"message": "Password changed successfully. All other sessions have been logged out.",
	})
}

// GetSessions returns the current user's active sessions.
// GET /api/v1/auth/sessions
func (h *AuthHandler) GetSessions(c *gin.Context) {
	userID := middleware.GetUserID(c)
	ctx := c.Request.Context()

	sessions, err := h.session.GetUserActiveSessions(ctx, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "internal_error",
			"message": "An error occurred",
		})
		return
	}

	type sessionResponse struct {
		ID        string    `json:"id"`
		IPAddress string    `json:"ip_address"`
		UserAgent string    `json:"user_agent"`
		CreatedAt time.Time `json:"created_at"`
		ExpiresAt time.Time `json:"expires_at"`
	}

	result := make([]sessionResponse, 0, len(sessions))
	for _, s := range sessions {
		result = append(result, sessionResponse{
			ID:        s.ID.String(),
			IPAddress: s.IPAddress,
			UserAgent: s.UserAgent,
			CreatedAt: s.CreatedAt,
			ExpiresAt: s.ExpiresAt,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"sessions": result,
	})
}

// ============================================================================
// Magic Link Authentication (Passwordless)
// ============================================================================

type MagicLinkRequest struct {
	Email string `json:"email" binding:"required,email"`
}

// SendMagicLink sends a magic link email for passwordless login.
// POST /api/v1/auth/magic-link
func (h *AuthHandler) SendMagicLink(c *gin.Context) {
	var req MagicLinkRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "validation_error",
			"message": "Invalid request body",
		})
		return
	}

	ctx := c.Request.Context()

	// Rate limit check: 3 requests per email, 1 hour window
	if h.rateLimiter != nil {
		allowed, count, ttl, err := h.rateLimiter.CheckMagicLinkRate(ctx, req.Email)
		if err != nil {
			log.Warn().Err(err).Msg("Rate limit check failed, allowing request")
		} else if !allowed {
			middleware.RateLimitExceededWithLog(c, "/auth/magic-link", req.Email, count, ttl)
			return
		}
	}

	// Always return success to prevent email enumeration
	defer func() {
		c.JSON(http.StatusOK, gin.H{
			"message": "If an account exists with this email, a magic link has been sent.",
		})
	}()

	// Find user by email
	user, err := h.getUserByEmailForReset(ctx, strings.ToLower(req.Email))
	if err != nil {
		log.Debug().Err(err).Str("email", req.Email).Msg("User not found for magic link")
		return
	}

	// Get user's tenant_id
	var tenantID uuid.UUID
	err = h.db.QueryRow(ctx, `SELECT tenant_id FROM users WHERE id = $1`, user.ID).Scan(&tenantID)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get user tenant")
		return
	}

	// Generate magic link token (32 bytes = 64 hex chars)
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		log.Error().Err(err).Msg("Failed to generate magic link token")
		return
	}
	token := hex.EncodeToString(tokenBytes)
	tokenHash := hashTokenString(token)

	// Store magic link token with 15 minute expiry
	expiresAt := time.Now().Add(15 * time.Minute)
	ipAddress := c.ClientIP()
	err = h.storeMagicLinkToken(ctx, tenantID, user.ID, tokenHash, expiresAt, ipAddress)
	if err != nil {
		log.Error().Err(err).Msg("Failed to store magic link token")
		return
	}

	// Send email
	if h.email != nil && h.email.IsConfigured() {
		loginURL := fmt.Sprintf("%s/magic-login?token=%s", h.frontendURL, token)
		err = h.email.SendMagicLink(user.Email, loginURL, user.Name)
		if err != nil {
			log.Error().Err(err).Str("email", user.Email).Msg("Failed to send magic link email")
		} else {
			log.Info().Str("email", user.Email).Msg("Magic link email sent")
		}
	} else {
		log.Warn().Msg("Email client not configured, skipping magic link email")
	}
}

// VerifyMagicLink verifies a magic link token and logs the user in.
// GET /api/v1/auth/magic-link?token=xxx
func (h *AuthHandler) VerifyMagicLink(c *gin.Context) {
	token := c.Query("token")
	if token == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "validation_error",
			"message": "Token is required",
		})
		return
	}

	ctx := c.Request.Context()
	tokenHash := hashTokenString(token)

	// Validate magic link token
	userID, tenantID, err := h.validateMagicLinkToken(ctx, tokenHash)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":   "invalid_token",
			"message": "Invalid or expired magic link",
		})
		return
	}

	// Mark token as used
	err = h.markMagicLinkTokenUsed(ctx, tokenHash)
	if err != nil {
		log.Error().Err(err).Msg("Failed to mark magic link token as used")
	}

	// Get user info for response
	var user userRecord
	err = h.db.QueryRow(ctx, `
		SELECT id, tenant_id, email, name, role FROM users WHERE id = $1 AND deleted_at IS NULL
	`, userID).Scan(&user.ID, &user.TenantID, &user.Email, &user.Name, &user.Role)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "internal_error",
			"message": "An error occurred",
		})
		return
	}

	// Update last login
	_ = h.updateLastLogin(ctx, userID)

	// Generate token pair
	tokenPair, err := h.jwt.GenerateTokenPair(userID, tenantID, user.Role)
	if err != nil {
		log.Error().Err(err).Msg("Failed to generate token pair")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "internal_error",
			"message": "An error occurred",
		})
		return
	}

	// Store refresh token
	ipAddress := c.ClientIP()
	userAgent := c.Request.UserAgent()
	_, err = h.session.StoreRefreshToken(ctx, userID, &tenantID, tokenPair.RefreshToken, ipAddress, userAgent)
	if err != nil {
		log.Error().Err(err).Msg("Failed to store refresh token")
	}

	tenantIDStr := ""
	if user.TenantID != nil {
		tenantIDStr = user.TenantID.String()
	}

	c.JSON(http.StatusOK, AuthResponse{
		AccessToken:  tokenPair.AccessToken,
		RefreshToken: tokenPair.RefreshToken,
		ExpiresAt:    tokenPair.ExpiresAt,
		User: UserInfo{
			ID:       user.ID.String(),
			Email:    user.Email,
			Name:     user.Name,
			Role:     user.Role,
			TenantID: tenantIDStr,
		},
	})
}

// hashTokenString creates a SHA256 hash of a token string.
func hashTokenString(token string) string {
	hash := fmt.Sprintf("%x", sha256.Sum256([]byte(token)))
	return hash
}

func (h *AuthHandler) storeMagicLinkToken(ctx context.Context, tenantID, userID uuid.UUID, tokenHash string, expiresAt time.Time, ipAddress string) error {
	query := `
		INSERT INTO magic_link_tokens (id, tenant_id, user_id, token_hash, expires_at, ip_address)
		VALUES ($1, $2, $3, $4, $5, $6)
	`
	_, err := h.db.Exec(ctx, query, uuid.New(), tenantID, userID, tokenHash, expiresAt, ipAddress)
	return err
}

func (h *AuthHandler) validateMagicLinkToken(ctx context.Context, tokenHash string) (uuid.UUID, uuid.UUID, error) {
	query := `
		SELECT user_id, tenant_id FROM magic_link_tokens
		WHERE token_hash = $1
		AND expires_at > NOW()
		AND used_at IS NULL
	`
	var userID, tenantID uuid.UUID
	err := h.db.QueryRow(ctx, query, tokenHash).Scan(&userID, &tenantID)
	if err != nil {
		return uuid.Nil, uuid.Nil, errors.New("invalid or expired token")
	}
	return userID, tenantID, nil
}

func (h *AuthHandler) markMagicLinkTokenUsed(ctx context.Context, tokenHash string) error {
	query := `UPDATE magic_link_tokens SET used_at = NOW() WHERE token_hash = $1`
	_, err := h.db.Exec(ctx, query, tokenHash)
	return err
}

// ============================================================================
// Two-Factor Authentication (2FA/TOTP)
// ============================================================================

type TwoFASetupResponse struct {
	Secret    string `json:"secret"`
	QRCodeURL string `json:"qr_code_url"`
	Issuer    string `json:"issuer"`
}

// Setup2FA initializes 2FA for the current user.
// POST /api/v1/auth/2fa/setup
func (h *AuthHandler) Setup2FA(c *gin.Context) {
	userID := middleware.GetUserID(c)
	ctx := c.Request.Context()

	// Check if 2FA is already enabled
	var existingSecret *string
	err := h.db.QueryRow(ctx, `SELECT totp_secret FROM users WHERE id = $1`, userID).Scan(&existingSecret)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "internal_error",
			"message": "An error occurred",
		})
		return
	}

	if existingSecret != nil && *existingSecret != "" {
		c.JSON(http.StatusConflict, gin.H{
			"error":   "2fa_already_enabled",
			"message": "2FA is already enabled for this account",
		})
		return
	}

	// Get user email for TOTP account name
	var email string
	err = h.db.QueryRow(ctx, `SELECT email FROM users WHERE id = $1`, userID).Scan(&email)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "internal_error",
			"message": "An error occurred",
		})
		return
	}

	// Generate a random TOTP secret (20 bytes = 160 bits, base32 encoded)
	secretBytes := make([]byte, 20)
	if _, err := rand.Read(secretBytes); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "internal_error",
			"message": "An error occurred",
		})
		return
	}
	secret := base32Encode(secretBytes)

	// Store the secret temporarily (user must verify before it's active)
	// We'll store it with a "pending_" prefix until verified
	pendingSecret := "pending_" + secret
	_, err = h.db.Exec(ctx, `UPDATE users SET totp_secret = $1, updated_at = NOW() WHERE id = $2`, pendingSecret, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "internal_error",
			"message": "An error occurred",
		})
		return
	}

	// Generate QR code URL (otpauth format)
	issuer := "AccountantCRM"
	qrCodeURL := fmt.Sprintf("otpauth://totp/%s:%s?secret=%s&issuer=%s&algorithm=SHA1&digits=6&period=30",
		issuer, email, secret, issuer)

	c.JSON(http.StatusOK, TwoFASetupResponse{
		Secret:    secret,
		QRCodeURL: qrCodeURL,
		Issuer:    issuer,
	})
}

type Verify2FARequest struct {
	Code string `json:"code" binding:"required,len=6"`
}

// Verify2FA verifies a TOTP code and activates 2FA if in setup mode.
// POST /api/v1/auth/2fa/verify
func (h *AuthHandler) Verify2FA(c *gin.Context) {
	var req Verify2FARequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "validation_error",
			"message": "Invalid request body. Code must be 6 digits.",
		})
		return
	}

	userID := middleware.GetUserID(c)
	ctx := c.Request.Context()

	// Rate limit check
	if h.rateLimiter != nil {
		allowed, count, ttl, err := h.rateLimiter.Check2FARate(ctx, userID.String())
		if err != nil {
			log.Warn().Err(err).Msg("Rate limit check failed, allowing request")
		} else if !allowed {
			middleware.RateLimitExceededWithLog(c, "/auth/2fa/verify", userID.String(), count, ttl)
			return
		}
	}

	// Get user's TOTP secret
	var totpSecret *string
	err := h.db.QueryRow(ctx, `SELECT totp_secret FROM users WHERE id = $1`, userID).Scan(&totpSecret)
	if err != nil || totpSecret == nil || *totpSecret == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "2fa_not_setup",
			"message": "2FA is not set up for this account",
		})
		return
	}

	// Check if this is a pending setup (secret starts with "pending_")
	isPending := strings.HasPrefix(*totpSecret, "pending_")
	secret := strings.TrimPrefix(*totpSecret, "pending_")

	// Verify the TOTP code
	valid := verifyTOTP(secret, req.Code)
	if !valid {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":   "invalid_code",
			"message": "Invalid verification code",
		})
		return
	}

	// If pending, activate 2FA
	if isPending {
		_, err = h.db.Exec(ctx, `UPDATE users SET totp_secret = $1, updated_at = NOW() WHERE id = $2`, secret, userID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "internal_error",
				"message": "An error occurred",
			})
			return
		}

		// Generate backup codes
		backupCodes, err := h.generateBackupCodes(ctx, userID)
		if err != nil {
			log.Error().Err(err).Msg("Failed to generate backup codes")
		}

		c.JSON(http.StatusOK, gin.H{
			"message":      "2FA has been enabled successfully",
			"backup_codes": backupCodes,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Code verified successfully",
	})
}

// Disable2FA disables 2FA for a user (admin only or self).
// DELETE /api/v1/auth/2fa
func (h *AuthHandler) Disable2FA(c *gin.Context) {
	userID := middleware.GetUserID(c)
	ctx := c.Request.Context()

	// Clear TOTP secret and backup codes
	_, err := h.db.Exec(ctx, `UPDATE users SET totp_secret = NULL, updated_at = NOW() WHERE id = $1`, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "internal_error",
			"message": "An error occurred",
		})
		return
	}

	// Delete all backup codes
	_, err = h.db.Exec(ctx, `DELETE FROM totp_backup_codes WHERE user_id = $1`, userID)
	if err != nil {
		log.Error().Err(err).Msg("Failed to delete backup codes")
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "2FA has been disabled",
	})
}

// GenerateBackupCodes generates new backup codes for the user.
// POST /api/v1/auth/2fa/backup-codes
func (h *AuthHandler) GenerateBackupCodes(c *gin.Context) {
	userID := middleware.GetUserID(c)
	ctx := c.Request.Context()

	// Check if 2FA is enabled
	var totpSecret *string
	err := h.db.QueryRow(ctx, `SELECT totp_secret FROM users WHERE id = $1`, userID).Scan(&totpSecret)
	if err != nil || totpSecret == nil || *totpSecret == "" || strings.HasPrefix(*totpSecret, "pending_") {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "2fa_not_enabled",
			"message": "2FA must be enabled to generate backup codes",
		})
		return
	}

	// Delete existing backup codes
	_, err = h.db.Exec(ctx, `DELETE FROM totp_backup_codes WHERE user_id = $1`, userID)
	if err != nil {
		log.Error().Err(err).Msg("Failed to delete existing backup codes")
	}

	// Generate new backup codes
	codes, err := h.generateBackupCodes(ctx, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "internal_error",
			"message": "An error occurred",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"backup_codes": codes,
		"message":      "New backup codes generated. Previous codes are now invalid.",
	})
}

type VerifyBackupCodeRequest struct {
	Email      string `json:"email" binding:"required,email"`
	Password   string `json:"password" binding:"required"`
	BackupCode string `json:"backup_code" binding:"required"`
}

// VerifyBackupCode uses a backup code to bypass 2FA.
// POST /api/v1/auth/2fa/backup-codes/verify
func (h *AuthHandler) VerifyBackupCode(c *gin.Context) {
	var req VerifyBackupCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "validation_error",
			"message": "Invalid request body",
		})
		return
	}

	ctx := c.Request.Context()
	ip := c.ClientIP()

	// Rate limit check
	if h.rateLimiter != nil {
		allowed, count, ttl, err := h.rateLimiter.CheckBackupCodeRate(ctx, ip, req.Email)
		if err != nil {
			log.Warn().Err(err).Msg("Rate limit check failed, allowing request")
		} else if !allowed {
			middleware.RateLimitExceededWithLog(c, "/auth/2fa/backup-codes/verify", ip+":"+req.Email, count, ttl)
			return
		}
	}

	// Find user and verify password
	user, err := h.getUserByEmail(ctx, req.Email, nil)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":   "invalid_credentials",
			"message": "Invalid email, password, or backup code",
		})
		return
	}

	valid, err := auth.VerifyPassword(req.Password, user.Password)
	if err != nil || !valid {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":   "invalid_credentials",
			"message": "Invalid email, password, or backup code",
		})
		return
	}

	// Verify backup code
	codeHash := hashTokenString(normalizeBackupCode(req.BackupCode))
	var codeID uuid.UUID
	err = h.db.QueryRow(ctx, `
		SELECT id FROM totp_backup_codes
		WHERE user_id = $1 AND code_hash = $2 AND used_at IS NULL
	`, user.ID, codeHash).Scan(&codeID)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":   "invalid_credentials",
			"message": "Invalid email, password, or backup code",
		})
		return
	}

	// Mark code as used
	_, err = h.db.Exec(ctx, `UPDATE totp_backup_codes SET used_at = NOW() WHERE id = $1`, codeID)
	if err != nil {
		log.Error().Err(err).Msg("Failed to mark backup code as used")
	}

	// Count remaining codes
	var remainingCodes int
	h.db.QueryRow(ctx, `SELECT COUNT(*) FROM totp_backup_codes WHERE user_id = $1 AND used_at IS NULL`, user.ID).Scan(&remainingCodes)

	// Generate tokens
	tokenTenantID := uuid.Nil
	tenantIDStr := ""
	if user.TenantID != nil {
		tokenTenantID = *user.TenantID
		tenantIDStr = user.TenantID.String()
	}

	tokenPair, err := h.jwt.GenerateTokenPair(user.ID, tokenTenantID, user.Role)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "internal_error",
			"message": "An error occurred",
		})
		return
	}

	// Store refresh token
	userAgent := c.Request.UserAgent()
	_, err = h.session.StoreRefreshToken(ctx, user.ID, user.TenantID, tokenPair.RefreshToken, ip, userAgent)
	if err != nil {
		log.Error().Err(err).Msg("Failed to store refresh token")
	}

	c.JSON(http.StatusOK, gin.H{
		"access_token":    tokenPair.AccessToken,
		"refresh_token":   tokenPair.RefreshToken,
		"expires_at":      tokenPair.ExpiresAt,
		"remaining_codes": remainingCodes,
		"user": UserInfo{
			ID:       user.ID.String(),
			Email:    user.Email,
			Name:     user.Name,
			Role:     user.Role,
			TenantID: tenantIDStr,
		},
	})
}

// generateBackupCodes creates 10 new backup codes for a user.
func (h *AuthHandler) generateBackupCodes(ctx context.Context, userID uuid.UUID) ([]string, error) {
	codes := make([]string, 10)

	for i := 0; i < 10; i++ {
		// Generate 8-character alphanumeric code (format: XXXX-XXXX)
		codeBytes := make([]byte, 4)
		if _, err := rand.Read(codeBytes); err != nil {
			return nil, err
		}
		code := fmt.Sprintf("%s-%s",
			strings.ToUpper(hex.EncodeToString(codeBytes[:2])),
			strings.ToUpper(hex.EncodeToString(codeBytes[2:])))
		codes[i] = code

		// Store hashed code
		codeHash := hashTokenString(normalizeBackupCode(code))
		_, err := h.db.Exec(ctx, `
			INSERT INTO totp_backup_codes (id, user_id, code_hash)
			VALUES ($1, $2, $3)
		`, uuid.New(), userID, codeHash)
		if err != nil {
			return nil, err
		}
	}

	return codes, nil
}

// normalizeBackupCode removes dashes and converts to uppercase.
func normalizeBackupCode(code string) string {
	return strings.ToUpper(strings.ReplaceAll(code, "-", ""))
}

// base32Encode encodes bytes to base32 without padding.
func base32Encode(data []byte) string {
	const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZ234567"
	result := make([]byte, (len(data)*8+4)/5)

	buffer := 0
	bitsInBuffer := 0
	idx := 0

	for _, b := range data {
		buffer = (buffer << 8) | int(b)
		bitsInBuffer += 8
		for bitsInBuffer >= 5 {
			bitsInBuffer -= 5
			result[idx] = alphabet[(buffer>>bitsInBuffer)&0x1F]
			idx++
		}
	}

	if bitsInBuffer > 0 {
		result[idx] = alphabet[(buffer<<(5-bitsInBuffer))&0x1F]
	}

	return string(result)
}

// verifyTOTP verifies a TOTP code against a secret.
// Uses SHA1, 6 digits, 30-second period (standard TOTP).
func verifyTOTP(secret, code string) bool {
	// Allow for time drift: check current, previous, and next period
	currentTime := time.Now().Unix()
	for _, offset := range []int64{-30, 0, 30} {
		timestamp := currentTime + offset
		period := timestamp / 30
		expectedCode := generateTOTP(secret, period)
		if expectedCode == code {
			return true
		}
	}
	return false
}

// generateTOTP generates a TOTP code for a given time period.
func generateTOTP(secret string, period int64) string {
	// Decode base32 secret
	secretBytes := base32Decode(secret)
	if secretBytes == nil {
		return ""
	}

	// Convert period to bytes (big-endian)
	counterBytes := make([]byte, 8)
	for i := 7; i >= 0; i-- {
		counterBytes[i] = byte(period & 0xff)
		period >>= 8
	}

	// HMAC-SHA1
	h := hmacSHA1(secretBytes, counterBytes)

	// Dynamic truncation
	offset := h[len(h)-1] & 0x0f
	code := int32(h[offset]&0x7f)<<24 |
		int32(h[offset+1])<<16 |
		int32(h[offset+2])<<8 |
		int32(h[offset+3])

	// 6 digits
	code = code % 1000000
	return fmt.Sprintf("%06d", code)
}

// base32Decode decodes a base32 string.
func base32Decode(s string) []byte {
	const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZ234567"

	result := make([]byte, len(s)*5/8)
	buffer := 0
	bitsInBuffer := 0
	idx := 0

	for _, c := range strings.ToUpper(s) {
		val := strings.IndexByte(alphabet, byte(c))
		if val == -1 {
			continue // Skip invalid characters
		}
		buffer = (buffer << 5) | val
		bitsInBuffer += 5
		if bitsInBuffer >= 8 {
			bitsInBuffer -= 8
			result[idx] = byte(buffer >> bitsInBuffer)
			idx++
		}
	}

	return result[:idx]
}

// hmacSHA1 computes HMAC-SHA1 using the standard crypto library.
func hmacSHA1(key, data []byte) []byte {
	h := hmac.New(sha1.New, key)
	h.Write(data)
	return h.Sum(nil)
}
