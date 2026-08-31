package handlers

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/accountant-crm/go-backend/internal/database"
	"github.com/accountant-crm/go-backend/internal/middleware"
)

// TenantHandler handles tenant-related HTTP requests
type TenantHandler struct {
	db *database.Pool
}

// NewTenantHandler creates a new TenantHandler instance
func NewTenantHandler(db *database.Pool) *TenantHandler {
	return &TenantHandler{db: db}
}

// Tenant represents a tenant record
type Tenant struct {
	ID             string     `json:"id"`
	Name           string     `json:"name"`
	Domain         string     `json:"domain"`
	CustomDomain   *string    `json:"custom_domain,omitempty"`
	Plan           string     `json:"plan"`
	LogoURL        *string    `json:"logo_url,omitempty"`
	FaviconURL     *string    `json:"favicon_url,omitempty"`
	PrimaryColor   *string    `json:"primary_color,omitempty"`
	SecondaryColor *string    `json:"secondary_color,omitempty"`
	Timezone       string     `json:"timezone"`
	IsActive       bool       `json:"is_active"`
	Settings       string     `json:"settings"` // JSONB as string
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
	DeletedAt      *time.Time `json:"deleted_at,omitempty"`
}

// CreateTenantRequest represents the request body for creating a tenant
type CreateTenantRequest struct {
	Name           string  `json:"name" binding:"required,min=1,max=255"`
	Domain         string  `json:"domain" binding:"required,min=1,max=255"`
	CustomDomain   *string `json:"custom_domain,omitempty"`
	Plan           string  `json:"plan,omitempty"`
	LogoURL        *string `json:"logo_url,omitempty"`
	FaviconURL     *string `json:"favicon_url,omitempty"`
	PrimaryColor   *string `json:"primary_color,omitempty"`
	SecondaryColor *string `json:"secondary_color,omitempty"`
	Timezone       string  `json:"timezone,omitempty"`
}

// UpdateTenantRequest represents the request body for updating a tenant
type UpdateTenantRequest struct {
	Name           *string `json:"name,omitempty"`
	Domain         *string `json:"domain,omitempty"`
	CustomDomain   *string `json:"custom_domain,omitempty"`
	Plan           *string `json:"plan,omitempty"`
	LogoURL        *string `json:"logo_url,omitempty"`
	FaviconURL     *string `json:"favicon_url,omitempty"`
	PrimaryColor   *string `json:"primary_color,omitempty"`
	SecondaryColor *string `json:"secondary_color,omitempty"`
	Timezone       *string `json:"timezone,omitempty"`
	IsActive       *bool   `json:"is_active,omitempty"`
}

// List returns all tenants (super_admin only, others see only their tenant)
func (h *TenantHandler) List(c *gin.Context) {
	ctx := c.Request.Context()
	role, _ := middleware.GetRole(c)
	tenantID, _ := middleware.GetTenantID(c)

	tenants := []Tenant{}

	if role == "super_admin" {
		// Super admins can see all tenants; use SuperAdminTransaction so RLS
		// policies see app.role='super_admin' and allow cross-tenant reads.
		err := h.db.SuperAdminTransaction(ctx, func(tx pgx.Tx) error {
			rows, err := tx.Query(ctx, `
				SELECT id, name, domain, custom_domain, plan, logo_url, favicon_url,
				       primary_color, secondary_color, timezone, is_active,
				       COALESCE(settings::text, '{}'), created_at, updated_at
				FROM tenants
				WHERE deleted_at IS NULL
				ORDER BY created_at DESC
			`)
			if err != nil {
				return err
			}
			defer rows.Close()

			for rows.Next() {
				var t Tenant
				if err := rows.Scan(
					&t.ID, &t.Name, &t.Domain, &t.CustomDomain, &t.Plan,
					&t.LogoURL, &t.FaviconURL, &t.PrimaryColor, &t.SecondaryColor,
					&t.Timezone, &t.IsActive, &t.Settings, &t.CreatedAt, &t.UpdatedAt,
				); err != nil {
					return err
				}
				tenants = append(tenants, t)
			}
			return rows.Err()
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "database_error",
				"message": "Failed to fetch tenants",
			})
			return
		}
	} else {
		// Non-super_admins only see their own tenant; use TenantDB so RLS
		// policies see the correct app.tenant_id.
		tenantDB, ok := middleware.GetTenantDB(c)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "internal_error",
				"message": "Database context not available",
			})
			return
		}

		var t Tenant
		err := tenantDB.QueryRowScan(c, []interface{}{
			&t.ID, &t.Name, &t.Domain, &t.CustomDomain, &t.Plan,
			&t.LogoURL, &t.FaviconURL, &t.PrimaryColor, &t.SecondaryColor,
			&t.Timezone, &t.IsActive, &t.Settings, &t.CreatedAt, &t.UpdatedAt,
		}, `
			SELECT id, name, domain, custom_domain, plan, logo_url, favicon_url,
			       primary_color, secondary_color, timezone, is_active,
			       COALESCE(settings::text, '{}'), created_at, updated_at
			FROM tenants
			WHERE id = $1 AND deleted_at IS NULL
		`, tenantID)
		if err != nil {
			if err == pgx.ErrNoRows {
				c.JSON(http.StatusNotFound, gin.H{
					"error":   "not_found",
					"message": "Tenant not found",
				})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "database_error",
				"message": "Failed to fetch tenant",
			})
			return
		}
		tenants = append(tenants, t)
	}

	c.JSON(http.StatusOK, gin.H{
		"tenants": tenants,
		"count":   len(tenants),
	})
}

// Create creates a new tenant (super_admin only)
func (h *TenantHandler) Create(c *gin.Context) {
	ctx := c.Request.Context()

	var req CreateTenantRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "validation_error",
			"message": err.Error(),
		})
		return
	}

	// Set defaults
	id := uuid.New()
	plan := req.Plan
	if plan == "" {
		plan = "starter"
	}
	timezone := req.Timezone
	if timezone == "" {
		timezone = "Europe/London"
	}

	var tenant *Tenant
	err := h.db.SuperAdminTransaction(ctx, func(tx pgx.Tx) error {
		// Check if domain already exists
		var exists bool
		err := tx.QueryRow(ctx, `
			SELECT EXISTS(SELECT 1 FROM tenants WHERE domain = $1 AND deleted_at IS NULL)
		`, req.Domain).Scan(&exists)
		if err != nil {
			return err
		}
		if exists {
			return fmt.Errorf("domain_exists")
		}

		_, err = tx.Exec(ctx, `
			INSERT INTO tenants (id, name, domain, custom_domain, plan, logo_url, favicon_url,
			                     primary_color, secondary_color, timezone, is_active, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, true, NOW(), NOW())
		`, id, req.Name, req.Domain, req.CustomDomain, plan,
			req.LogoURL, req.FaviconURL, req.PrimaryColor, req.SecondaryColor, timezone)
		if err != nil {
			return err
		}

		// Fetch the created tenant
		tenant, err = h.getTenantByIDTx(ctx, tx, id)
		return err
	})

	if err != nil {
		if err.Error() == "domain_exists" {
			c.JSON(http.StatusConflict, gin.H{
				"error":   "domain_exists",
				"message": "A tenant with this domain already exists",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "create_error",
			"message": "Failed to create tenant",
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Tenant created successfully",
		"tenant":  tenant,
	})
}

// Get retrieves a tenant by ID
func (h *TenantHandler) Get(c *gin.Context) {
	ctx := c.Request.Context()
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid_id",
			"message": "Invalid tenant ID format",
		})
		return
	}

	// Authorization check
	role, _ := middleware.GetRole(c)
	userTenantID, _ := middleware.GetTenantID(c)

	if role != "super_admin" && userTenantID != id {
		c.JSON(http.StatusForbidden, gin.H{
			"error":   "forbidden",
			"message": "You can only view your own tenant",
		})
		return
	}

	tenant, err := h.getTenantByID(ctx, id)
	if err != nil {
		if err == pgx.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{
				"error":   "not_found",
				"message": "Tenant not found",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "database_error",
			"message": "Failed to fetch tenant",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"tenant": tenant,
	})
}

// Update updates a tenant (super_admin or tenant_admin of that tenant)
func (h *TenantHandler) Update(c *gin.Context) {
	ctx := c.Request.Context()
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid_id",
			"message": "Invalid tenant ID format",
		})
		return
	}

	// Authorization check
	role, _ := middleware.GetRole(c)
	userTenantID, _ := middleware.GetTenantID(c)

	if role != "super_admin" && (role != "tenant_admin" || userTenantID != id) {
		c.JSON(http.StatusForbidden, gin.H{
			"error":   "forbidden",
			"message": "You don't have permission to update this tenant",
		})
		return
	}

	var req UpdateTenantRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "validation_error",
			"message": err.Error(),
		})
		return
	}

	tenantDB, ok := middleware.GetTenantDB(c)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "internal_error",
			"message": "Database context not available",
		})
		return
	}

	var tenant *Tenant
	err = tenantDB.Transaction(c, func(tx pgx.Tx) error {
		// Check if tenant exists
		var exists bool
		err := tx.QueryRow(ctx, `
			SELECT EXISTS(SELECT 1 FROM tenants WHERE id = $1 AND deleted_at IS NULL)
		`, id).Scan(&exists)
		if err != nil {
			return err
		}
		if !exists {
			return pgx.ErrNoRows
		}

		// Check domain uniqueness if being changed
		if req.Domain != nil {
			var domainExists bool
			err = tx.QueryRow(ctx, `
				SELECT EXISTS(SELECT 1 FROM tenants WHERE domain = $1 AND id != $2 AND deleted_at IS NULL)
			`, *req.Domain, id).Scan(&domainExists)
			if err != nil {
				return err
			}
			if domainExists {
				return fmt.Errorf("domain_exists")
			}
		}

		// Build dynamic update query
		_, err = tx.Exec(ctx, `
			UPDATE tenants SET
				name = COALESCE($2, name),
				domain = COALESCE($3, domain),
				custom_domain = COALESCE($4, custom_domain),
				plan = COALESCE($5, plan),
				logo_url = COALESCE($6, logo_url),
				favicon_url = COALESCE($7, favicon_url),
				primary_color = COALESCE($8, primary_color),
				secondary_color = COALESCE($9, secondary_color),
				timezone = COALESCE($10, timezone),
				is_active = COALESCE($11, is_active),
				updated_at = NOW()
			WHERE id = $1 AND deleted_at IS NULL
		`, id, req.Name, req.Domain, req.CustomDomain, req.Plan,
			req.LogoURL, req.FaviconURL, req.PrimaryColor, req.SecondaryColor,
			req.Timezone, req.IsActive)
		if err != nil {
			return err
		}

		// Fetch the updated tenant
		tenant, err = h.getTenantByIDTx(ctx, tx, id)
		return err
	})

	if err != nil {
		if err == pgx.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{
				"error":   "not_found",
				"message": "Tenant not found",
			})
			return
		}
		if err.Error() == "domain_exists" {
			c.JSON(http.StatusConflict, gin.H{
				"error":   "domain_exists",
				"message": "A tenant with this domain already exists",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "update_error",
			"message": "Failed to update tenant",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Tenant updated successfully",
		"tenant":  tenant,
	})
}

// Delete soft-deletes a tenant (super_admin only)
func (h *TenantHandler) Delete(c *gin.Context) {
	ctx := c.Request.Context()
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid_id",
			"message": "Invalid tenant ID format",
		})
		return
	}

	tenantDB, ok := middleware.GetTenantDB(c)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "internal_error",
			"message": "Database context not available",
		})
		return
	}

	err = tenantDB.Transaction(c, func(tx pgx.Tx) error {
		// Check if tenant exists
		var exists bool
		err := tx.QueryRow(ctx, `
			SELECT EXISTS(SELECT 1 FROM tenants WHERE id = $1 AND deleted_at IS NULL)
		`, id).Scan(&exists)
		if err != nil {
			return err
		}
		if !exists {
			return pgx.ErrNoRows
		}

		// Soft delete the tenant
		_, err = tx.Exec(ctx, `
			UPDATE tenants SET deleted_at = NOW(), is_active = false, updated_at = NOW()
			WHERE id = $1 AND deleted_at IS NULL
		`, id)
		return err
	})

	if err != nil {
		if err == pgx.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{
				"error":   "not_found",
				"message": "Tenant not found",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "delete_error",
			"message": "Failed to delete tenant",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Tenant deleted successfully",
	})
}

// getTenantByID is a helper function to fetch a tenant by ID
func (h *TenantHandler) getTenantByID(ctx context.Context, id uuid.UUID) (*Tenant, error) {
	var t *Tenant
	err := h.db.SuperAdminTransaction(ctx, func(tx pgx.Tx) error {
		var err error
		t, err = h.getTenantByIDTx(ctx, tx, id)
		return err
	})
	if err != nil {
		return nil, err
	}
	return t, nil
}

// getTenantByIDTx fetches a tenant by ID within an existing transaction.
func (h *TenantHandler) getTenantByIDTx(ctx context.Context, tx pgx.Tx, id uuid.UUID) (*Tenant, error) {
	var t Tenant
	err := tx.QueryRow(ctx, `
		SELECT id, name, domain, custom_domain, plan, logo_url, favicon_url,
		       primary_color, secondary_color, timezone, is_active,
		       COALESCE(settings::text, '{}'), created_at, updated_at
		FROM tenants
		WHERE id = $1 AND deleted_at IS NULL
	`, id).Scan(
		&t.ID, &t.Name, &t.Domain, &t.CustomDomain, &t.Plan,
		&t.LogoURL, &t.FaviconURL, &t.PrimaryColor, &t.SecondaryColor,
		&t.Timezone, &t.IsActive, &t.Settings, &t.CreatedAt, &t.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &t, nil
}
