package middleware

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/accountant-crm/go-backend/internal/auth"
)

const (
	AuthUserID   = "auth_user_id"
	AuthTenantID = "auth_tenant_id"
	AuthRole     = "auth_role"
)

// TokenBlocklist allows revoked access tokens to be tracked.
type TokenBlocklist interface {
	BlockToken(ctx context.Context, jti string, ttl time.Duration) error
	IsTokenBlocked(ctx context.Context, jti string) (bool, error)
}

func JWTAuth(jwtManager *auth.JWTManager, blocklist TokenBlocklist) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error":   "missing_token",
				"message": "Authorization header required",
			})
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error":   "invalid_token",
				"message": "Invalid authorization header format",
			})
			return
		}

		claims, err := jwtManager.ValidateToken(parts[1])
		if err != nil {
			if err == auth.ErrExpiredToken {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
					"error":   "token_expired",
					"message": "Token has expired",
				})
				return
			}
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error":   "invalid_token",
				"message": "Invalid token",
			})
			return
		}

		if claims.Type != auth.AccessToken {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error":   "invalid_token",
				"message": "Invalid token type",
			})
			return
		}

		// Check token blocklist (e.g. after logout)
		// Fix #5: Fail closed - if blocklist check fails, reject the token
		if blocklist != nil && claims.ID != "" {
			blocked, err := blocklist.IsTokenBlocked(c.Request.Context(), claims.ID)
			if err != nil {
				log.Error().Err(err).Str("jti", claims.ID).Msg("Token blocklist check failed - failing closed")
				c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{
					"error":   "service_unavailable",
					"message": "Unable to validate token status. Please try again.",
				})
				return
			}
			if blocked {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
					"error":   "token_revoked",
					"message": "Token has been revoked",
				})
				return
			}
		}

		c.Set(AuthUserID, claims.UserID)
		c.Set(AuthTenantID, claims.TenantID)
		c.Set(AuthRole, claims.Role)

		c.Next()
	}
}

func RequireRole(roles ...string) gin.HandlerFunc {
	roleSet := make(map[string]bool)
	for _, r := range roles {
		roleSet[r] = true
	}

	return func(c *gin.Context) {
		role, exists := c.Get(AuthRole)
		if !exists {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error":   "unauthorized",
				"message": "Authentication required",
			})
			return
		}

		roleStr, ok := role.(string)
		if !ok || !roleSet[roleStr] {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error":   "forbidden",
				"message": "Insufficient permissions",
			})
			return
		}

		c.Next()
	}
}

func GetUserID(c *gin.Context) (uuid.UUID, bool) {
	val, exists := c.Get(AuthUserID)
	if !exists {
		return uuid.Nil, false
	}
	id, ok := val.(uuid.UUID)
	return id, ok
}

func GetTenantID(c *gin.Context) (uuid.UUID, bool) {
	val, exists := c.Get(AuthTenantID)
	if !exists {
		return uuid.Nil, false
	}
	id, ok := val.(uuid.UUID)
	return id, ok
}

func GetRole(c *gin.Context) (string, bool) {
	val, exists := c.Get(AuthRole)
	if !exists {
		return "", false
	}
	role, ok := val.(string)
	return role, ok
}
