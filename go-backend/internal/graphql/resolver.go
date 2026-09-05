// Package graphql implements the GraphQL API for Accountant CRM
package graphql

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/redis/go-redis/v9"

	"github.com/accountant-crm/go-backend/internal/ai"
	"github.com/accountant-crm/go-backend/internal/database"
	"github.com/accountant-crm/go-backend/internal/graphql/dataloader"
	"github.com/accountant-crm/go-backend/internal/middleware"
)

// This file will not be regenerated automatically.
//
// It serves as dependency injection for your app, add any dependencies you require here.

// Resolver is the root resolver with all dependencies
type Resolver struct {
	DB       *database.Pool // Raw pool (fallback for super_admin without tenant)
	Redis    *redis.Client
	AIClient *ai.Client
}

// NewResolver creates a new GraphQL resolver with dependencies
func NewResolver(db *database.Pool, redis *redis.Client, aiClient *ai.Client) *Resolver {
	return &Resolver{
		DB:       db,
		Redis:    redis,
		AIClient: aiClient,
	}
}

// =============================================================================
// Tenant-aware database access helpers
// =============================================================================

// GetTenantDB retrieves TenantDB from context for RLS-enforced queries.
// Returns nil if no TenantDB is available (e.g., super_admin without tenant).
func (r *Resolver) GetTenantDB(ctx context.Context) *middleware.TenantDB {
	return middleware.TenantDBFromContext(ctx)
}

// WithTransaction executes a function within an RLS-enforced transaction.
// Falls back to raw pool transaction if TenantDB is not available.
func (r *Resolver) WithTransaction(ctx context.Context, fn func(tx pgx.Tx) error) error {
	tenantDB := middleware.TenantDBFromContext(ctx)
	if tenantDB != nil {
		return tenantDB.TransactionCtx(ctx, fn)
	}
	// Fallback: use raw pool (no RLS enforcement)
	return r.DB.Transaction(ctx, fn)
}

// QueryRow executes a single-row query with RLS enforcement.
// dest should be a slice of pointers to scan into.
func (r *Resolver) QueryRow(ctx context.Context, dest []interface{}, sql string, args ...interface{}) error {
	tenantDB := middleware.TenantDBFromContext(ctx)
	if tenantDB != nil {
		return tenantDB.QueryRowCtx(ctx, dest, sql, args...)
	}
	// Fallback: direct query (no RLS)
	return r.DB.QueryRow(ctx, sql, args...).Scan(dest...)
}

// QueryDB executes a multi-row query with RLS enforcement.
// Note: Named QueryDB to avoid conflict with gqlgen's Query() QueryResolver interface.
func (r *Resolver) QueryDB(ctx context.Context, sql string, args []interface{}, scanFn func(pgx.Rows) error) error {
	tenantDB := middleware.TenantDBFromContext(ctx)
	if tenantDB != nil {
		return tenantDB.QueryCtx(ctx, sql, args, scanFn)
	}
	// Fallback: direct query (no RLS)
	rows, err := r.DB.Query(ctx, sql, args...)
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
}

// GetLoaders retrieves DataLoaders from context.
// Returns nil if no loaders available (caller should handle gracefully).
func (r *Resolver) GetLoaders(ctx context.Context) *dataloader.Loaders {
	return dataloader.For(ctx)
}
