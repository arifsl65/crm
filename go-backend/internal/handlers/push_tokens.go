package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/rs/zerolog/log"

	"github.com/accountant-crm/go-backend/internal/database"
	"github.com/accountant-crm/go-backend/internal/middleware"
)

// PushTokenHandler handles push notification token operations.
type PushTokenHandler struct {
	db *database.Pool
}

// NewPushTokenHandler creates a new push token handler.
func NewPushTokenHandler(db *database.Pool) *PushTokenHandler {
	return &PushTokenHandler{db: db}
}

// PushToken represents a push notification token.
type PushToken struct {
	ID         uuid.UUID  `json:"id"`
	TenantID   uuid.UUID  `json:"tenant_id"`
	UserID     uuid.UUID  `json:"user_id"`
	Token      string     `json:"token"`
	Platform   string     `json:"platform"` // ios, android, web
	IsActive   bool       `json:"is_active"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
}

// RegisterTokenRequest represents the request to register a push token.
type RegisterTokenRequest struct {
	Token    string `json:"token" binding:"required"`
	Platform string `json:"platform" binding:"required,oneof=ios android web"`
}

// Register registers a new push token for the current user.
// POST /api/v1/push-tokens
func (h *PushTokenHandler) Register(c *gin.Context) {
	tenantID, _ := middleware.GetTenantID(c)
	userID, _ := middleware.GetUserID(c)

	tenantDB, ok := middleware.GetTenantDB(c)
	if !ok {
		log.Error().Msg("TenantDB not found in context")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	var req RegisterTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Check if token already exists
	var existingID uuid.UUID
	err := tenantDB.QueryRowScan(c, []interface{}{&existingID},
		`SELECT id FROM push_tokens WHERE token = $1 AND tenant_id = $2`,
		req.Token, tenantID)

	if err == nil {
		// Token exists, update it
		_, err = tenantDB.Exec(c, `
			UPDATE push_tokens
			SET user_id = $1, platform = $2, is_active = true, last_used_at = NOW()
			WHERE id = $3
		`, userID, req.Platform, existingID)

		if err != nil {
			log.Error().Err(err).Msg("Failed to update push token")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to register push token"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"message": "Push token updated",
			"id":      existingID,
		})
		return
	}

	if err != pgx.ErrNoRows {
		log.Error().Err(err).Msg("Failed to check existing push token")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to register push token"})
		return
	}

	// Create new token
	tokenID := uuid.New()
	now := time.Now()

	_, err = tenantDB.Exec(c, `
		INSERT INTO push_tokens (id, tenant_id, user_id, token, platform, is_active, created_at)
		VALUES ($1, $2, $3, $4, $5, true, $6)
	`, tokenID, tenantID, userID, req.Token, req.Platform, now)

	if err != nil {
		log.Error().Err(err).Msg("Failed to create push token")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to register push token"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Push token registered",
		"push_token": PushToken{
			ID:        tokenID,
			TenantID:  tenantID,
			UserID:    userID,
			Token:     req.Token,
			Platform:  req.Platform,
			IsActive:  true,
			CreatedAt: now,
		},
	})
}

// List returns all push tokens for the current user.
// GET /api/v1/push-tokens
func (h *PushTokenHandler) List(c *gin.Context) {
	tenantID, _ := middleware.GetTenantID(c)
	userID, _ := middleware.GetUserID(c)

	tenantDB, ok := middleware.GetTenantDB(c)
	if !ok {
		log.Error().Msg("TenantDB not found in context")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	var tokens []PushToken
	err := tenantDB.Query(c, `
		SELECT id, tenant_id, user_id, token, platform, is_active, last_used_at, created_at
		FROM push_tokens
		WHERE tenant_id = $1 AND user_id = $2
		ORDER BY created_at DESC
	`, []interface{}{tenantID, userID}, func(rows pgx.Rows) error {
		var t PushToken
		err := rows.Scan(
			&t.ID, &t.TenantID, &t.UserID, &t.Token, &t.Platform,
			&t.IsActive, &t.LastUsedAt, &t.CreatedAt,
		)
		if err != nil {
			return err
		}
		tokens = append(tokens, t)
		return nil
	})

	if err != nil {
		log.Error().Err(err).Msg("Failed to list push tokens")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch push tokens"})
		return
	}

	if tokens == nil {
		tokens = []PushToken{}
	}

	c.JSON(http.StatusOK, gin.H{
		"push_tokens": tokens,
		"count":       len(tokens),
	})
}

// Unregister deactivates a push token.
// DELETE /api/v1/push-tokens/:id
func (h *PushTokenHandler) Unregister(c *gin.Context) {
	tenantID, _ := middleware.GetTenantID(c)
	userID, _ := middleware.GetUserID(c)
	id := c.Param("id")

	tenantDB, ok := middleware.GetTenantDB(c)
	if !ok {
		log.Error().Msg("TenantDB not found in context")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	tokenID, err := uuid.Parse(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid token ID"})
		return
	}

	result, err := tenantDB.Exec(c, `
		UPDATE push_tokens SET is_active = false
		WHERE id = $1 AND tenant_id = $2 AND user_id = $3
	`, tokenID, tenantID, userID)

	if err != nil {
		log.Error().Err(err).Msg("Failed to unregister push token")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to unregister push token"})
		return
	}

	if result.RowsAffected() == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Push token not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Push token unregistered"})
}

// UnregisterByToken deactivates a push token by its token value.
// POST /api/v1/push-tokens/unregister
func (h *PushTokenHandler) UnregisterByToken(c *gin.Context) {
	tenantID, _ := middleware.GetTenantID(c)
	userID, _ := middleware.GetUserID(c)

	tenantDB, ok := middleware.GetTenantDB(c)
	if !ok {
		log.Error().Msg("TenantDB not found in context")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	var req struct {
		Token string `json:"token" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	result, err := tenantDB.Exec(c, `
		UPDATE push_tokens SET is_active = false
		WHERE token = $1 AND tenant_id = $2 AND user_id = $3
	`, req.Token, tenantID, userID)

	if err != nil {
		log.Error().Err(err).Msg("Failed to unregister push token")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to unregister push token"})
		return
	}

	if result.RowsAffected() == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Push token not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Push token unregistered"})
}
