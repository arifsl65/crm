// Package cache provides Redis client management.
package cache

import (
	"context"
	"crypto/tls"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"

	"github.com/accountant-crm/go-backend/internal/config"
)

// Client wraps a Redis client with health check capabilities.
type Client struct {
	*redis.Client
	cfg config.RedisConfig
}

// NewClient creates a new Redis client.
func NewClient(ctx context.Context, cfg config.RedisConfig) (*Client, error) {
	opts := &redis.Options{
		Addr:         cfg.Addr(),
		Password:     cfg.Password,
		DB:           cfg.DB,
		PoolSize:     cfg.PoolSize,
		MinIdleConns: 5,
		MaxRetries:   3,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
	}

	// Configure TLS if enabled
	if cfg.TLSEnabled {
		opts.TLSConfig = &tls.Config{
			MinVersion: tls.VersionTLS12,
		}
	}

	client := redis.NewClient(opts)

	// Verify connection
	if err := client.Ping(ctx).Err(); err != nil {
		client.Close()
		return nil, fmt.Errorf("failed to ping Redis: %w", err)
	}

	log.Info().
		Str("addr", cfg.Addr()).
		Int("db", cfg.DB).
		Bool("tls", cfg.TLSEnabled).
		Int("pool_size", cfg.PoolSize).
		Msg("Connected to Redis")

	return &Client{
		Client: client,
		cfg:    cfg,
	}, nil
}

// HealthCheck verifies the Redis connection is alive.
func (c *Client) HealthCheck(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := c.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("redis health check failed: %w", err)
	}

	return nil
}

// Close gracefully closes the Redis client.
func (c *Client) Close() error {
	log.Info().Msg("Closing Redis client")
	return c.Client.Close()
}

// GetFeatureFlag retrieves a feature flag value from Redis.
func (c *Client) GetFeatureFlag(ctx context.Context, flag string) (bool, error) {
	key := fmt.Sprintf("ai:%s:enabled", flag)
	val, err := c.Get(ctx, key).Result()
	if err == redis.Nil {
		return false, nil // Flag not set, default to disabled
	}
	if err != nil {
		return false, fmt.Errorf("failed to get feature flag %s: %w", flag, err)
	}
	return val == "true", nil
}

// SetFeatureFlag sets a feature flag value in Redis.
func (c *Client) SetFeatureFlag(ctx context.Context, flag string, enabled bool) error {
	key := fmt.Sprintf("ai:%s:enabled", flag)
	val := "false"
	if enabled {
		val = "true"
	}
	return c.Set(ctx, key, val, 0).Err()
}

// CacheGet retrieves a cached value with automatic JSON deserialization.
func (c *Client) CacheGet(ctx context.Context, key string) (string, error) {
	val, err := c.Get(ctx, key).Result()
	if err == redis.Nil {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("cache get failed: %w", err)
	}
	return val, nil
}

// CacheSet stores a value with optional TTL.
func (c *Client) CacheSet(ctx context.Context, key, value string, ttl time.Duration) error {
	return c.Set(ctx, key, value, ttl).Err()
}

// CacheDelete removes a key from cache.
func (c *Client) CacheDelete(ctx context.Context, key string) error {
	return c.Del(ctx, key).Err()
}

// Stats returns Redis connection pool statistics.
func (c *Client) Stats() *redis.PoolStats {
	return c.PoolStats()
}

// PoolMetrics returns a map of pool statistics suitable for monitoring.
// Fix #22: Expose pool metrics for CloudMonitor/Prometheus.
func (c *Client) PoolMetrics(ctx context.Context) map[string]interface{} {
	stats := c.PoolStats()
	return map[string]interface{}{
		"hits":        stats.Hits,
		"misses":      stats.Misses,
		"timeouts":    stats.Timeouts,
		"total_conns": stats.TotalConns,
		"idle_conns":  stats.IdleConns,
		"stale_conns": stats.StaleConns,
	}
}

// rateLimitScript is a Lua script that atomically increments and checks rate limits.
// Returns: [current_count, ttl_remaining]
// This eliminates the race condition between GET and INCR.
var rateLimitScript = redis.NewScript(`
local key = KEYS[1]
local limit = tonumber(ARGV[1])
local window = tonumber(ARGV[2])

local current = redis.call('INCR', key)
if current == 1 then
    redis.call('EXPIRE', key, window)
end

local ttl = redis.call('TTL', key)
return {current, ttl}
`)

// RateLimitResult contains the result of an atomic rate limit check.
type RateLimitResult struct {
	CurrentCount int
	TTLSeconds   int
	Allowed      bool
}

// RateLimitCheck atomically increments the counter and checks if the request is allowed.
// This is atomic - no race condition between check and increment.
func (c *Client) RateLimitCheck(ctx context.Context, key string, limit int, windowSeconds int) (*RateLimitResult, error) {
	result, err := rateLimitScript.Run(ctx, c.Client, []string{key}, limit, windowSeconds).Slice()
	if err != nil {
		return nil, fmt.Errorf("rate limit check failed: %w", err)
	}

	if len(result) != 2 {
		return nil, fmt.Errorf("unexpected rate limit result length: %d", len(result))
	}

	currentCount, ok := result[0].(int64)
	if !ok {
		return nil, fmt.Errorf("unexpected current count type")
	}

	ttl, ok := result[1].(int64)
	if !ok {
		return nil, fmt.Errorf("unexpected ttl type")
	}

	return &RateLimitResult{
		CurrentCount: int(currentCount),
		TTLSeconds:   int(ttl),
		Allowed:      int(currentCount) <= limit,
	}, nil
}
