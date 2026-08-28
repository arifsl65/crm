package handlers

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/rs/zerolog/log"

	"github.com/accountant-crm/go-backend/internal/auth"
	"github.com/accountant-crm/go-backend/internal/database"
)

type AuthHandler struct {
	db  *database.Pool
	jwt *auth.JWTManager
}

func NewAuthHandler(db *database.Pool, jwt *auth.JWTManager) *AuthHandler {
	return &AuthHandler{db: db, jwt: jwt}
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

	tokenPair, err := h.jwt.GenerateTokenPair(user.ID, user.TenantID, user.Role)
	if err != nil {
		log.Error().Err(err).Msg("Failed to generate token pair")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "internal_error",
			"message": "An error occurred",
		})
		return
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
			TenantID: user.TenantID.String(),
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

	tokenPair, err := h.jwt.RefreshAccessToken(req.RefreshToken)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":   "invalid_token",
			"message": "Invalid or expired refresh token",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"access_token":  tokenPair.AccessToken,
		"refresh_token": tokenPair.RefreshToken,
		"expires_at":    tokenPair.ExpiresAt,
	})
}

func (h *AuthHandler) Logout(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"message": "Logged out successfully",
	})
}

type userRecord struct {
	ID                  uuid.UUID
	TenantID            uuid.UUID
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
	query := `
		UPDATE users
		SET failed_login_attempts = failed_login_attempts + 1,
		    locked_until = CASE WHEN failed_login_attempts >= 4 THEN NOW() + INTERVAL '15 minutes' ELSE locked_until END,
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
