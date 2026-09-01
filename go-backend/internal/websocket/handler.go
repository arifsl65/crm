// Package websocket provides HTTP handlers for WebSocket connections.
package websocket

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/rs/zerolog/log"

	"github.com/accountant-crm/go-backend/internal/auth"
	"github.com/accountant-crm/go-backend/internal/cache"
	"github.com/accountant-crm/go-backend/internal/middleware"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		// Origin check is handled by CORS middleware
		// In production, implement proper origin validation
		return true
	},
}

// Handler handles WebSocket connections.
type Handler struct {
	hub *Hub
	jwt *auth.JWTManager
	redis *cache.Client
}

// NewHandler creates a new WebSocket handler.
func NewHandler(hub *Hub, jwt *auth.JWTManager, redis *cache.Client) *Handler {
	return &Handler{
		hub:   hub,
		jwt:   jwt,
		redis: redis,
	}
}

// Connect handles WebSocket connection requests.
// GET /ws
//
// Query parameters:
//   - token: JWT access token for authentication
//
// The connection is authenticated via the token parameter since
// WebSocket connections can't use Authorization headers.
func (h *Handler) Connect(c *gin.Context) {
	// Get token from query parameter
	token := c.Query("token")
	if token == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "token_required"})
		return
	}

	// Validate JWT token
	claims, err := h.jwt.ValidateToken(token)
	if err != nil {
		log.Warn().Err(err).Msg("Invalid WebSocket token")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid_token"})
		return
	}

	// Check if token is blocked (JTI is stored in RegisteredClaims.ID)
	blocked, err := h.redis.IsTokenBlocked(c.Request.Context(), claims.ID)
	if err != nil {
		log.Error().Err(err).Msg("Failed to check token blocklist")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}
	if blocked {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "token_revoked"})
		return
	}

	// Upgrade to WebSocket
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Error().Err(err).Msg("Failed to upgrade WebSocket connection")
		return
	}

	// Register client with hub
	client := h.hub.RegisterClient(conn, claims.TenantID, claims.UserID)

	log.Info().
		Str("tenant_id", claims.TenantID.String()).
		Str("user_id", claims.UserID.String()).
		Str("remote_addr", c.ClientIP()).
		Msg("WebSocket client connected")

	// Start read and write pumps
	go client.WritePump()
	go client.ReadPump()
}

// ConnectAuthenticated handles WebSocket connections for already authenticated requests.
// This is used when the JWT middleware has already validated the token.
// GET /api/v1/ws
func (h *Handler) ConnectAuthenticated(c *gin.Context) {
	tenantID, ok := middleware.GetTenantID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not_authenticated"})
		return
	}

	userID, ok := middleware.GetUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not_authenticated"})
		return
	}

	// Upgrade to WebSocket
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Error().Err(err).Msg("Failed to upgrade WebSocket connection")
		return
	}

	// Register client with hub
	client := h.hub.RegisterClient(conn, tenantID, userID)

	log.Info().
		Str("tenant_id", tenantID.String()).
		Str("user_id", userID.String()).
		Str("remote_addr", c.ClientIP()).
		Msg("WebSocket client connected (authenticated)")

	// Start read and write pumps
	go client.WritePump()
	go client.ReadPump()
}

// Stats returns WebSocket hub statistics.
// GET /api/v1/ws/stats
func (h *Handler) Stats(c *gin.Context) {
	stats := h.hub.Stats()
	c.JSON(http.StatusOK, stats)
}
