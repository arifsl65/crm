package graphql

import (
	"context"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/extension"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"github.com/accountant-crm/go-backend/internal/ai"
	"github.com/accountant-crm/go-backend/internal/database"
	"github.com/accountant-crm/go-backend/internal/graphql/dataloader"
	"github.com/accountant-crm/go-backend/internal/graphql/security"
	"github.com/accountant-crm/go-backend/internal/middleware"
)

// HandlerConfig holds GraphQL handler configuration
type HandlerConfig struct {
	DB                  *database.Pool
	Redis               *redis.Client
	AIClient            *ai.Client
	EnablePlayground    bool
	EnableIntrospection bool
	PlaygroundEndpoint  string // GraphQL endpoint URL for playground (default: "/graphql")
}

// NewHandler creates a new GraphQL handler
func NewHandler(cfg HandlerConfig) *handler.Server {
	resolver := NewResolver(cfg.DB, cfg.Redis, cfg.AIClient)

	// Create executable schema from generated code
	es := NewExecutableSchema(Config{Resolvers: resolver})

	srv := handler.New(es)

	// Add transports
	srv.AddTransport(transport.Options{})
	srv.AddTransport(transport.GET{})
	srv.AddTransport(transport.POST{})

	// Add security extensions
	srv.Use(&security.DepthLimiter{})
	srv.Use(&security.TimeoutExtension{})
	srv.Use(extension.FixedComplexityLimit(security.MaxComplexity))

	// Add rate limiter if Redis is available
	if cfg.Redis != nil {
		srv.Use(security.NewRateLimiter(cfg.Redis))
	}

	// Introspection (disable in production)
	if cfg.EnableIntrospection {
		srv.Use(extension.Introspection{})
	}

	// Automatic persisted queries (production hardening)
	srv.Use(extension.AutomaticPersistedQuery{
		Cache: newAPQCache(cfg.Redis),
	})

	return srv
}

// GinHandler wraps the GraphQL handler for Gin
func GinHandler(cfg HandlerConfig) gin.HandlerFunc {
	srv := NewHandler(cfg)

	return func(c *gin.Context) {
		// Get auth info from JWT middleware
		tenantID, _ := middleware.GetTenantID(c)
		userID, _ := middleware.GetUserID(c)
		role, _ := middleware.GetRole(c)

		// Create context with auth info
		ctx := c.Request.Context()
		ctx = context.WithValue(ctx, middleware.AuthTenantID, tenantID)
		ctx = context.WithValue(ctx, middleware.AuthUserID, userID)
		ctx = context.WithValue(ctx, middleware.AuthRole, role)

		// Get TenantDB from gin context (set by TenantRLS middleware)
		// This carries the RLS context (tenant_id, role) for database queries
		tenantDB, hasTenantDB := middleware.GetTenantDB(c)
		if hasTenantDB && tenantDB != nil {
			// Add TenantDB to context for resolvers
			ctx = middleware.WithTenantDB(ctx, tenantDB)
			// Add DataLoaders with TenantDB (RLS-aware)
			ctx = context.WithValue(ctx, dataloader.LoadersKey, dataloader.NewLoaders(tenantDB))
		} else if tenantID != uuid.Nil {
			// Fallback: create TenantDB manually if middleware didn't set it
			// This shouldn't happen in normal flow but provides safety
			ctx = context.WithValue(ctx, dataloader.LoadersKey, dataloader.NewLoadersWithPool(cfg.DB, tenantID))
		}
		// Note: If no tenant (super_admin without tenant), no DataLoaders are created
		// Resolvers should handle this by checking if loaders exist

		// Add client IP for rate limiting
		ctx = security.WithClientIP(ctx, c.ClientIP())

		// Serve GraphQL
		srv.ServeHTTP(c.Writer, c.Request.WithContext(ctx))
	}
}

// PlaygroundHandler returns a GraphQL playground handler for development
func PlaygroundHandler(endpoint string) gin.HandlerFunc {
	h := playground.Handler("GraphQL Playground", endpoint)
	return func(c *gin.Context) {
		h.ServeHTTP(c.Writer, c.Request)
	}
}

// =============================================================================
// APQ Cache
// =============================================================================

type apqCache struct {
	redis *redis.Client
}

func newAPQCache(redis *redis.Client) *apqCache {
	return &apqCache{redis: redis}
}

func (c *apqCache) Get(ctx context.Context, key string) (interface{}, bool) {
	if c.redis == nil {
		return nil, false
	}
	val, err := c.redis.Get(ctx, "gql:apq:"+key).Result()
	if err != nil {
		return nil, false
	}
	return val, true
}

func (c *apqCache) Add(ctx context.Context, key string, value interface{}) {
	if c.redis == nil {
		return
	}
	// APQ queries are cached for 24 hours
	c.redis.Set(ctx, "gql:apq:"+key, value, 24*60*60*1e9)
}

// LoadersKey is exported for use in handler
var LoadersKey = dataloader.LoadersKey
