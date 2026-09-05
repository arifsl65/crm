// Package security provides GraphQL query security extensions
package security

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/99designs/gqlgen/graphql"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
	"github.com/vektah/gqlparser/v2/gqlerror"
)

// Rate limiting constants
const (
	RequestsPerMinute = 100
	RateLimitWindow   = time.Minute
)

// RateLimiter implements GraphQL-specific rate limiting
type RateLimiter struct {
	redis  *redis.Client
	prefix string
}

// NewRateLimiter creates a new GraphQL rate limiter
func NewRateLimiter(redisClient *redis.Client) *RateLimiter {
	return &RateLimiter{
		redis:  redisClient,
		prefix: "gql:ratelimit:",
	}
}

// ExtensionName returns the extension name
func (r *RateLimiter) ExtensionName() string {
	return "RateLimiter"
}

// Validate validates the schema
func (r *RateLimiter) Validate(schema graphql.ExecutableSchema) error {
	return nil
}

// InterceptOperation checks rate limits before executing
func (r *RateLimiter) InterceptOperation(ctx context.Context, next graphql.OperationHandler) graphql.ResponseHandler {
	if r.redis == nil {
		// No Redis configured, skip rate limiting
		return next(ctx)
	}

	// Get client IP from context (set by middleware)
	clientIP := getClientIP(ctx)
	if clientIP == "" {
		return next(ctx)
	}

	// Check request rate limit
	reqKey := fmt.Sprintf("%s%s:req", r.prefix, clientIP)
	reqCount, err := r.redis.Incr(ctx, reqKey).Result()
	if err != nil {
		log.Error().Err(err).Msg("Failed to check request rate limit")
		return next(ctx)
	}

	// Set expiry on first request
	if reqCount == 1 {
		r.redis.Expire(ctx, reqKey, RateLimitWindow)
	}

	if reqCount > RequestsPerMinute {
		log.Warn().
			Str("ip", clientIP).
			Int64("count", reqCount).
			Int("limit", RequestsPerMinute).
			Msg("GraphQL rate limit exceeded")

		return func(ctx context.Context) *graphql.Response {
			return &graphql.Response{
				Errors: gqlerror.List{{
					Message: "rate limit exceeded",
					Extensions: map[string]interface{}{
						"code":       "RATE_LIMIT_EXCEEDED",
						"limit":      RequestsPerMinute,
						"window":     RateLimitWindow.String(),
						"retryAfter": r.getRetryAfter(ctx, reqKey),
					},
				}},
			}
		}
	}

	// Execute the operation
	return next(ctx)
}

// getRetryAfter returns seconds until rate limit window resets
func (r *RateLimiter) getRetryAfter(ctx context.Context, key string) int {
	ttl, err := r.redis.TTL(ctx, key).Result()
	if err != nil || ttl < 0 {
		return 60 // Default to 60 seconds
	}
	return int(ttl.Seconds())
}

// ClientIPKey is the context key for client IP
type clientIPKeyType struct{}

var ClientIPKey = clientIPKeyType{}

// getClientIP extracts client IP from context
func getClientIP(ctx context.Context) string {
	if ip, ok := ctx.Value(ClientIPKey).(string); ok {
		return ip
	}
	return ""
}

// WithClientIP adds client IP to context
func WithClientIP(ctx context.Context, ip string) context.Context {
	return context.WithValue(ctx, ClientIPKey, ip)
}

// RateLimitMiddleware is HTTP middleware that adds client IP to context
func RateLimitMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Get client IP from X-Forwarded-For or RemoteAddr
		ip := r.Header.Get("X-Forwarded-For")
		if ip == "" {
			ip = r.Header.Get("X-Real-IP")
		}
		if ip == "" {
			ip = r.RemoteAddr
		}

		ctx := WithClientIP(r.Context(), ip)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
