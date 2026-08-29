package middleware

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"

	"github.com/accountant-crm/go-backend/internal/cache"
)

// AuthRateLimiter provides endpoint-specific rate limiting for auth routes.
type AuthRateLimiter struct {
	redis *cache.Client
}

// NewAuthRateLimiter creates a new auth rate limiter.
func NewAuthRateLimiter(redis *cache.Client) *AuthRateLimiter {
	return &AuthRateLimiter{redis: redis}
}

// RateLimitConfig defines rate limiting parameters.
type RateLimitConfig struct {
	MaxAttempts int
	Window      time.Duration
}

// Default rate limit configurations per spec
var (
	// Login: 5 attempts per IP+email, 15min window
	LoginRateLimit = RateLimitConfig{MaxAttempts: 5, Window: 15 * time.Minute}

	// Password reset request: 3 per email, 1 hour
	ResetPasswordRateLimit = RateLimitConfig{MaxAttempts: 3, Window: time.Hour}

	// Magic link: 3 per email, 1 hour
	MagicLinkRateLimit = RateLimitConfig{MaxAttempts: 3, Window: time.Hour}

	// 2FA verify: 5 attempts per session, 15min
	TwoFAVerifyRateLimit = RateLimitConfig{MaxAttempts: 5, Window: 15 * time.Minute}

	// Backup codes verify: 5 attempts per IP+email, 15min
	BackupCodeRateLimit = RateLimitConfig{MaxAttempts: 5, Window: 15 * time.Minute}

	// Refresh: 10 per token family, 1 hour
	RefreshRateLimit = RateLimitConfig{MaxAttempts: 10, Window: time.Hour}
)

// checkRateLimit checks if the rate limit is exceeded for a given key.
// Returns (allowed, currentCount, ttlSeconds, error)
func (rl *AuthRateLimiter) checkRateLimit(ctx context.Context, key string, cfg RateLimitConfig) (bool, int, int, error) {
	result, err := rl.redis.RateLimitCheck(ctx, key, cfg.MaxAttempts, int(cfg.Window.Seconds()))
	if err != nil {
		return false, 0, 0, err
	}
	return result.Allowed, result.CurrentCount, result.TTLSeconds, nil
}

// LoginRateLimit middleware for /auth/login endpoint.
// Limits by IP + email combination.
func (rl *AuthRateLimiter) LoginRateLimit() gin.HandlerFunc {
	return func(c *gin.Context) {
		// We need to read the email from the request body
		// But we can't consume the body here. Instead, we'll check after binding.
		// This middleware just sets up the rate limiter for use in the handler.
		c.Set("authRateLimiter", rl)
		c.Next()
	}
}

// CheckLoginRate checks login rate limit. Called from handler after parsing email.
func (rl *AuthRateLimiter) CheckLoginRate(ctx context.Context, ip, email string) (bool, int, int, error) {
	key := fmt.Sprintf("ratelimit:login:%s:%s", ip, email)
	return rl.checkRateLimit(ctx, key, LoginRateLimit)
}

// IncrementLoginFailure increments the login failure count.
func (rl *AuthRateLimiter) IncrementLoginFailure(ctx context.Context, ip, email string) error {
	key := fmt.Sprintf("ratelimit:login:%s:%s", ip, email)
	_, err := rl.redis.RateLimitCheck(ctx, key, LoginRateLimit.MaxAttempts, int(LoginRateLimit.Window.Seconds()))
	return err
}

// ClearLoginRate clears the login rate limit on successful login.
func (rl *AuthRateLimiter) ClearLoginRate(ctx context.Context, ip, email string) error {
	key := fmt.Sprintf("ratelimit:login:%s:%s", ip, email)
	return rl.redis.CacheDelete(ctx, key)
}

// ResetPasswordRateLimit middleware for /auth/reset-password endpoint.
func (rl *AuthRateLimiter) ResetPasswordRateLimit() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set("authRateLimiter", rl)
		c.Next()
	}
}

// CheckResetPasswordRate checks reset password rate limit by email.
func (rl *AuthRateLimiter) CheckResetPasswordRate(ctx context.Context, email string) (bool, int, int, error) {
	key := fmt.Sprintf("ratelimit:reset-password:%s", email)
	return rl.checkRateLimit(ctx, key, ResetPasswordRateLimit)
}

// MagicLinkRateLimit middleware for /auth/magic-link endpoint.
func (rl *AuthRateLimiter) MagicLinkRateLimit() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set("authRateLimiter", rl)
		c.Next()
	}
}

// CheckMagicLinkRate checks magic link rate limit by email.
func (rl *AuthRateLimiter) CheckMagicLinkRate(ctx context.Context, email string) (bool, int, int, error) {
	key := fmt.Sprintf("ratelimit:magic-link:%s", email)
	return rl.checkRateLimit(ctx, key, MagicLinkRateLimit)
}

// RefreshRateLimit middleware for /auth/refresh endpoint.
func (rl *AuthRateLimiter) RefreshRateLimit() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set("authRateLimiter", rl)
		c.Next()
	}
}

// CheckRefreshRate checks refresh rate limit by token family.
func (rl *AuthRateLimiter) CheckRefreshRate(ctx context.Context, family string) (bool, int, int, error) {
	key := fmt.Sprintf("ratelimit:refresh:%s", family)
	return rl.checkRateLimit(ctx, key, RefreshRateLimit)
}

// TwoFARateLimit middleware for /auth/2fa/* endpoints.
func (rl *AuthRateLimiter) TwoFARateLimit() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set("authRateLimiter", rl)
		c.Next()
	}
}

// Check2FARate checks 2FA verification rate limit by session/user.
func (rl *AuthRateLimiter) Check2FARate(ctx context.Context, sessionID string) (bool, int, int, error) {
	key := fmt.Sprintf("ratelimit:2fa:%s", sessionID)
	return rl.checkRateLimit(ctx, key, TwoFAVerifyRateLimit)
}

// CheckBackupCodeRate checks backup code verification rate limit by IP+email.
func (rl *AuthRateLimiter) CheckBackupCodeRate(ctx context.Context, ip, email string) (bool, int, int, error) {
	key := fmt.Sprintf("ratelimit:backup-code:%s:%s", ip, email)
	return rl.checkRateLimit(ctx, key, BackupCodeRateLimit)
}

// GetRateLimiter retrieves the rate limiter from gin context.
func GetRateLimiter(c *gin.Context) *AuthRateLimiter {
	if rl, exists := c.Get("authRateLimiter"); exists {
		return rl.(*AuthRateLimiter)
	}
	return nil
}

// RateLimitExceeded sends a rate limit exceeded response.
func RateLimitExceeded(c *gin.Context, retryAfter int) {
	c.Header("Retry-After", fmt.Sprintf("%d", retryAfter))
	c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
		"error":       "rate_limit_exceeded",
		"message":     "Too many attempts. Please try again later.",
		"retry_after": retryAfter,
	})
}

// RateLimitExceededWithLog sends a rate limit exceeded response with logging.
func RateLimitExceededWithLog(c *gin.Context, endpoint string, identifier string, count int, retryAfter int) {
	log.Warn().
		Str("endpoint", endpoint).
		Str("identifier", identifier).
		Int("count", count).
		Int("retry_after", retryAfter).
		Msg("Rate limit exceeded")

	RateLimitExceeded(c, retryAfter)
}
