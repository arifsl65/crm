// Package middleware provides HTTP middleware for the API.
package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/rs/zerolog/log"

	"github.com/accountant-crm/go-backend/internal/database"
)

const (
	// TenantDBKey is the context key for tenant-scoped database access
	TenantDBKey = "tenant_db"
)

// TenantDB provides tenant-scoped database operations with RLS context.
type TenantDB struct {
	pool     *database.Pool
	tenantID string
	role     string
}

// TenantRLS creates a middleware that provides tenant-scoped database access.
// It extracts tenant_id and role from the JWT claims (set by JWTAuth middleware)
// and provides a TenantDB instance that automatically sets RLS context.
func TenantRLS(pool *database.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		var tenantID, role string

		// Get tenant_id from context (set by JWTAuth)
		if tid, exists := c.Get(AuthTenantID); exists {
			if id, ok := tid.(uuid.UUID); ok {
				tenantID = id.String()
			}
		}

		// Get role from context (set by JWTAuth)
		if r, exists := c.Get(AuthRole); exists {
			if roleStr, ok := r.(string); ok {
				role = roleStr
			}
		}

		// Create tenant-scoped DB accessor
		tenantDB := &TenantDB{
			pool:     pool,
			tenantID: tenantID,
			role:     role,
		}

		c.Set(TenantDBKey, tenantDB)
		c.Next()
	}
}

// Transaction executes a function within a transaction with RLS context set.
func (t *TenantDB) Transaction(c *gin.Context, fn func(tx pgx.Tx) error) error {
	return t.pool.TenantTransaction(c.Request.Context(), t.tenantID, t.role, fn)
}

// TenantID returns the current tenant ID.
func (t *TenantDB) TenantID() string {
	return t.tenantID
}

// Role returns the current user's role.
func (t *TenantDB) Role() string {
	return t.role
}

// Pool returns the underlying database pool for direct access when needed.
// Note: Direct pool access bypasses RLS - use Transaction() for RLS-protected queries.
func (t *TenantDB) Pool() *database.Pool {
	return t.pool
}

// QueryCollect executes a query with RLS context set and collects all rows.
// Use this for queries where you want RLS enforced. The callback processes each row.
func (t *TenantDB) QueryCollect(c *gin.Context, sql string, args []interface{}, scanFn func(pgx.Rows) error) error {
	ctx := c.Request.Context()

	return t.pool.TenantTransaction(ctx, t.tenantID, t.role, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, sql, args...)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			if err := scanFn(rows); err != nil {
				return err
			}
		}
		return rows.Err()
	})
}

// QueryRowScan executes a query that returns a single row and scans it into dest.
// This is the preferred method for single-row queries as it properly handles RLS context.
func (t *TenantDB) QueryRowScan(c *gin.Context, dest []interface{}, sql string, args ...interface{}) error {
	ctx := c.Request.Context()

	return t.pool.TenantTransaction(ctx, t.tenantID, t.role, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, sql, args...).Scan(dest...)
	})
}

// Query executes a query with RLS context and returns rows via a callback.
// The callback is called for each row and should scan the values.
// This is the preferred method for multi-row queries.
func (t *TenantDB) Query(c *gin.Context, sql string, args []interface{}, scanFn func(pgx.Rows) error) error {
	ctx := c.Request.Context()

	return t.pool.TenantTransaction(ctx, t.tenantID, t.role, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, sql, args...)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			if err := scanFn(rows); err != nil {
				return err
			}
		}
		return rows.Err()
	})
}

// QueryRow executes a query that returns a single row with RLS context set.
// DEPRECATED: Use QueryRowScan instead for proper RLS handling.
// Fix #11: WARNING - This method returns a Row AFTER the transaction commits,
// which means the underlying connection may be returned to the pool and reused.
// This can cause cross-tenant data leakage in high-concurrency scenarios.
// This method is kept only for backwards compatibility and will be removed.
func (t *TenantDB) QueryRow(c *gin.Context, sql string, args ...interface{}) pgx.Row {
	ctx := c.Request.Context()

	// Log deprecation warning to track usage and enforce migration
	log.Warn().
		Str("tenant_id", t.tenantID).
		Str("method", "TenantDB.QueryRow").
		Msg("DEPRECATED: QueryRow called - use QueryRowScan instead to prevent potential data leakage")

	// For QueryRow, we need to return a Row interface.
	// We'll use the pool directly but set RLS context in a transaction wrapper.
	// This is a limitation - QueryRow doesn't fit well with transaction-based RLS.
	// For safety, we'll use Transaction internally and return a custom Row.

	var result rlsRow
	err := t.pool.TenantTransaction(ctx, t.tenantID, t.role, func(tx pgx.Tx) error {
		row := tx.QueryRow(ctx, sql, args...)
		result.row = row
		return nil
	})
	if err != nil {
		result.err = err
	}
	return &result
}

// Exec executes a statement with RLS context set.
func (t *TenantDB) Exec(c *gin.Context, sql string, args ...interface{}) (pgconn.CommandTag, error) {
	ctx := c.Request.Context()
	var tag pgconn.CommandTag

	err := t.pool.TenantTransaction(ctx, t.tenantID, t.role, func(tx pgx.Tx) error {
		var err error
		tag, err = tx.Exec(ctx, sql, args...)
		return err
	})

	return tag, err
}

// rlsRow wraps a pgx.Row to handle RLS transaction errors
type rlsRow struct {
	row pgx.Row
	err error
}

func (r *rlsRow) Scan(dest ...interface{}) error {
	if r.err != nil {
		return r.err
	}
	return r.row.Scan(dest...)
}

// GetTenantDB retrieves the TenantDB from the gin context.
func GetTenantDB(c *gin.Context) (*TenantDB, bool) {
	val, exists := c.Get(TenantDBKey)
	if !exists {
		return nil, false
	}
	db, ok := val.(*TenantDB)
	return db, ok
}
