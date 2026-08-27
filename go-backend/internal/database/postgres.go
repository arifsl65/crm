// Package database provides PostgreSQL connection management.
package database

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"

	"github.com/accountant-crm/go-backend/internal/config"
)

// Pool wraps a pgx connection pool with health check capabilities.
type Pool struct {
	*pgxpool.Pool
	cfg config.PostgresConfig
}

// NewPool creates a new PostgreSQL connection pool.
func NewPool(ctx context.Context, cfg config.PostgresConfig) (*Pool, error) {
	poolConfig, err := pgxpool.ParseConfig(cfg.URL())
	if err != nil {
		return nil, fmt.Errorf("failed to parse database config: %w", err)
	}

	// Configure pool settings
	poolConfig.MinConns = int32(cfg.PoolMin)
	poolConfig.MaxConns = int32(cfg.PoolMax)
	poolConfig.MaxConnLifetime = 1 * time.Hour
	poolConfig.MaxConnIdleTime = 30 * time.Minute
	poolConfig.HealthCheckPeriod = 1 * time.Minute

	// Create the pool
	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	// Verify connection
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	log.Info().
		Str("host", cfg.Host).
		Int("port", cfg.Port).
		Str("database", cfg.Database).
		Int("pool_min", cfg.PoolMin).
		Int("pool_max", cfg.PoolMax).
		Msg("Connected to PostgreSQL")

	return &Pool{
		Pool: pool,
		cfg:  cfg,
	}, nil
}

// HealthCheck verifies the database connection is alive.
func (p *Pool) HealthCheck(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var result int
	err := p.QueryRow(ctx, "SELECT 1").Scan(&result)
	if err != nil {
		return fmt.Errorf("database health check failed: %w", err)
	}

	if result != 1 {
		return fmt.Errorf("unexpected health check result: %d", result)
	}

	return nil
}

// Stats returns connection pool statistics.
func (p *Pool) Stats() *pgxpool.Stat {
	return p.Stat()
}

// PoolMetrics returns a map of pool statistics suitable for monitoring.
// Fix #22: Expose pool metrics for CloudMonitor/Prometheus.
func (p *Pool) PoolMetrics() map[string]interface{} {
	stat := p.Stat()
	return map[string]interface{}{
		"acquired_conns":          stat.AcquiredConns(),
		"constructing_conns":      stat.ConstructingConns(),
		"idle_conns":              stat.IdleConns(),
		"total_conns":             stat.TotalConns(),
		"max_conns":               stat.MaxConns(),
		"acquire_count":           stat.AcquireCount(),
		"acquire_duration_ns":     stat.AcquireDuration().Nanoseconds(),
		"empty_acquire_count":     stat.EmptyAcquireCount(),
		"canceled_acquire_count":  stat.CanceledAcquireCount(),
		"new_conns_count":         stat.NewConnsCount(),
		"max_lifetime_destroy":    stat.MaxLifetimeDestroyCount(),
		"max_idle_destroy":        stat.MaxIdleDestroyCount(),
	}
}

// Close gracefully closes the connection pool.
func (p *Pool) Close() {
	log.Info().Msg("Closing PostgreSQL connection pool")
	p.Pool.Close()
}

// Transaction executes a function within a database transaction.
func (p *Pool) Transaction(ctx context.Context, fn func(tx pgx.Tx) error) error {
	tx, err := p.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	defer func() {
		if r := recover(); r != nil {
			_ = tx.Rollback(ctx)
			panic(r) // re-throw panic after rollback
		}
	}()

	if err := fn(tx); err != nil {
		if rbErr := tx.Rollback(ctx); rbErr != nil {
			log.Error().Err(rbErr).Msg("Failed to rollback transaction")
		}
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}
