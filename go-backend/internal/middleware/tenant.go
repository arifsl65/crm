// Package middleware provides HTTP middleware for the API.
package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

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

// GetTenantDB retrieves the TenantDB from the gin context.
func GetTenantDB(c *gin.Context) (*TenantDB, bool) {
	val, exists := c.Get(TenantDBKey)
	if !exists {
		return nil, false
	}
	db, ok := val.(*TenantDB)
	return db, ok
}
