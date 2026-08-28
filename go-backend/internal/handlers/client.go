package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/accountant-crm/go-backend/internal/database"
	"github.com/accountant-crm/go-backend/internal/middleware"
)

// ClientHandler handles client-related HTTP requests
type ClientHandler struct {
	db *database.Pool
}

// NewClientHandler creates a new ClientHandler instance
func NewClientHandler(db *database.Pool) *ClientHandler {
	return &ClientHandler{db: db}
}

// Client represents a client record
type Client struct {
	ID                string     `json:"id"`
	TenantID          string     `json:"tenant_id"`
	UserID            *string    `json:"user_id,omitempty"`
	CompanyName       string     `json:"company_name"`
	ContactName       string     `json:"contact_name"`
	Email             string     `json:"email"`
	Phone             *string    `json:"phone,omitempty"`
	Address           *string    `json:"address,omitempty"`
	YearEnd           *string    `json:"year_end,omitempty"`
	UTR               *string    `json:"utr,omitempty"`
	CompanyNumber     *string    `json:"company_number,omitempty"`
	CompanyType       *string    `json:"company_type,omitempty"`
	IncorporationDate *string    `json:"incorporation_date,omitempty"`
	SICCodes          *string    `json:"sic_codes,omitempty"`
	VATNumber         *string    `json:"vat_number,omitempty"`
	VATQuarter        *string    `json:"vat_quarter,omitempty"`
	Status            string     `json:"status"`
	RiskScore         *int       `json:"risk_score,omitempty"`
	Tags              string     `json:"tags"`
	EmailStatus       string     `json:"email_status"`
	LastContactAt     *time.Time `json:"last_contact_at,omitempty"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

// CreateClientRequest represents the request body for creating a client
type CreateClientRequest struct {
	CompanyName       string  `json:"company_name" binding:"required,min=1,max=255"`
	ContactName       string  `json:"contact_name" binding:"required,min=1,max=255"`
	Email             string  `json:"email" binding:"required,email"`
	Phone             *string `json:"phone,omitempty"`
	Address           *string `json:"address,omitempty"`
	YearEnd           *string `json:"year_end,omitempty"`
	UTR               *string `json:"utr,omitempty"`
	CompanyNumber     *string `json:"company_number,omitempty"`
	CompanyType       *string `json:"company_type,omitempty"`
	IncorporationDate *string `json:"incorporation_date,omitempty"`
	SICCodes          *string `json:"sic_codes,omitempty"`
	VATNumber         *string `json:"vat_number,omitempty"`
	VATQuarter        *string `json:"vat_quarter,omitempty"`
	Tags              *string `json:"tags,omitempty"`
}

// UpdateClientRequest represents the request body for updating a client
type UpdateClientRequest struct {
	CompanyName       *string `json:"company_name,omitempty"`
	ContactName       *string `json:"contact_name,omitempty"`
	Email             *string `json:"email,omitempty"`
	Phone             *string `json:"phone,omitempty"`
	Address           *string `json:"address,omitempty"`
	YearEnd           *string `json:"year_end,omitempty"`
	UTR               *string `json:"utr,omitempty"`
	CompanyNumber     *string `json:"company_number,omitempty"`
	CompanyType       *string `json:"company_type,omitempty"`
	IncorporationDate *string `json:"incorporation_date,omitempty"`
	SICCodes          *string `json:"sic_codes,omitempty"`
	VATNumber         *string `json:"vat_number,omitempty"`
	VATQuarter        *string `json:"vat_quarter,omitempty"`
	Status            *string `json:"status,omitempty"`
	Tags              *string `json:"tags,omitempty"`
}

// List returns clients based on role permissions (tenant-scoped)
func (h *ClientHandler) List(c *gin.Context) {
	ctx := c.Request.Context()
	role, _ := middleware.GetRole(c)
	userTenantID, _ := middleware.GetTenantID(c)

	var clients []Client

	if role == "super_admin" {
		// Super admins can see all clients across all tenants
		rows, err := h.db.Query(ctx, `
			SELECT id, tenant_id, user_id, company_name, contact_name, email, phone,
			       address, year_end::text, utr, company_number, company_type,
			       incorporation_date::text, COALESCE(sic_codes::text, '[]'),
			       vat_number, vat_quarter, status, risk_score,
			       COALESCE(tags::text, '[]'), email_status,
			       last_contact_at, created_at, updated_at
			FROM clients
			WHERE anonymized_at IS NULL
			ORDER BY created_at DESC
		`)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "database_error",
				"message": "Failed to fetch clients",
			})
			return
		}
		defer rows.Close()

		for rows.Next() {
			var cl Client
			if err := rows.Scan(
				&cl.ID, &cl.TenantID, &cl.UserID, &cl.CompanyName, &cl.ContactName,
				&cl.Email, &cl.Phone, &cl.Address, &cl.YearEnd, &cl.UTR,
				&cl.CompanyNumber, &cl.CompanyType, &cl.IncorporationDate, &cl.SICCodes,
				&cl.VATNumber, &cl.VATQuarter, &cl.Status, &cl.RiskScore,
				&cl.Tags, &cl.EmailStatus, &cl.LastContactAt, &cl.CreatedAt, &cl.UpdatedAt,
			); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{
					"error":   "scan_error",
					"message": "Failed to parse client data",
				})
				return
			}
			clients = append(clients, cl)
		}
	} else {
		// Others only see clients in their tenant
		rows, err := h.db.Query(ctx, `
			SELECT id, tenant_id, user_id, company_name, contact_name, email, phone,
			       address, year_end::text, utr, company_number, company_type,
			       incorporation_date::text, COALESCE(sic_codes::text, '[]'),
			       vat_number, vat_quarter, status, risk_score,
			       COALESCE(tags::text, '[]'), email_status,
			       last_contact_at, created_at, updated_at
			FROM clients
			WHERE tenant_id = $1 AND anonymized_at IS NULL
			ORDER BY created_at DESC
		`, userTenantID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "database_error",
				"message": "Failed to fetch clients",
			})
			return
		}
		defer rows.Close()

		for rows.Next() {
			var cl Client
			if err := rows.Scan(
				&cl.ID, &cl.TenantID, &cl.UserID, &cl.CompanyName, &cl.ContactName,
				&cl.Email, &cl.Phone, &cl.Address, &cl.YearEnd, &cl.UTR,
				&cl.CompanyNumber, &cl.CompanyType, &cl.IncorporationDate, &cl.SICCodes,
				&cl.VATNumber, &cl.VATQuarter, &cl.Status, &cl.RiskScore,
				&cl.Tags, &cl.EmailStatus, &cl.LastContactAt, &cl.CreatedAt, &cl.UpdatedAt,
			); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{
					"error":   "scan_error",
					"message": "Failed to parse client data",
				})
				return
			}
			clients = append(clients, cl)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"clients": clients,
		"count":   len(clients),
	})
}

// Create creates a new client (tenant_admin or staff)
func (h *ClientHandler) Create(c *gin.Context) {
	ctx := c.Request.Context()
	role, _ := middleware.GetRole(c)
	userTenantID, _ := middleware.GetTenantID(c)

	// super_admin must specify tenant_id (not implemented yet - they'd need a tenant context)
	if role == "super_admin" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "tenant_required",
			"message": "Super admin must specify a tenant context to create clients",
		})
		return
	}

	var req CreateClientRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "validation_error",
			"message": err.Error(),
		})
		return
	}

	// Check if email already exists in this tenant
	var emailExists bool
	err := h.db.QueryRow(ctx, `
		SELECT EXISTS(SELECT 1 FROM clients WHERE email = $1 AND tenant_id = $2 AND anonymized_at IS NULL)
	`, req.Email, userTenantID).Scan(&emailExists)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "database_error",
			"message": "Failed to check email uniqueness",
		})
		return
	}
	if emailExists {
		c.JSON(http.StatusConflict, gin.H{
			"error":   "email_exists",
			"message": "A client with this email already exists in this tenant",
		})
		return
	}

	// Set defaults
	id := uuid.New()
	tags := "[]"
	if req.Tags != nil {
		tags = *req.Tags
	}

	_, err = h.db.Exec(ctx, `
		INSERT INTO clients (id, tenant_id, company_name, contact_name, email, phone,
		                     address, year_end, utr, company_number, company_type,
		                     incorporation_date, sic_codes, vat_number, vat_quarter,
		                     status, tags, email_status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8::date, $9, $10, $11, $12::date, $13::jsonb,
		        $14, $15, 'active', $16::jsonb, 'active', NOW(), NOW())
	`, id, userTenantID, req.CompanyName, req.ContactName, req.Email, req.Phone,
		req.Address, req.YearEnd, req.UTR, req.CompanyNumber, req.CompanyType,
		req.IncorporationDate, req.SICCodes, req.VATNumber, req.VATQuarter, tags)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "create_error",
			"message": "Failed to create client",
		})
		return
	}

	// Fetch the created client
	client, err := h.getClientByID(ctx, id, userTenantID, role)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "fetch_error",
			"message": "Client created but failed to fetch",
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Client created successfully",
		"client":  client,
	})
}

// Get retrieves a client by ID
func (h *ClientHandler) Get(c *gin.Context) {
	ctx := c.Request.Context()
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid_id",
			"message": "Invalid client ID format",
		})
		return
	}

	role, _ := middleware.GetRole(c)
	userTenantID, _ := middleware.GetTenantID(c)

	client, err := h.getClientByID(ctx, id, userTenantID, role)
	if err != nil {
		if err == pgx.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{
				"error":   "not_found",
				"message": "Client not found",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "database_error",
			"message": "Failed to fetch client",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"client": client,
	})
}

// Update updates a client
func (h *ClientHandler) Update(c *gin.Context) {
	ctx := c.Request.Context()
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid_id",
			"message": "Invalid client ID format",
		})
		return
	}

	role, _ := middleware.GetRole(c)
	userTenantID, _ := middleware.GetTenantID(c)

	// Check if client exists and user has access
	_, err = h.getClientByID(ctx, id, userTenantID, role)
	if err != nil {
		if err == pgx.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{
				"error":   "not_found",
				"message": "Client not found",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "database_error",
			"message": "Failed to fetch client",
		})
		return
	}

	var req UpdateClientRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "validation_error",
			"message": err.Error(),
		})
		return
	}

	// Validate status if provided
	if req.Status != nil {
		validStatuses := map[string]bool{"active": true, "inactive": true, "archived": true}
		if !validStatuses[*req.Status] {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "invalid_status",
				"message": "Status must be active, inactive, or archived",
			})
			return
		}
	}

	// Update client
	_, err = h.db.Exec(ctx, `
		UPDATE clients SET
			company_name = COALESCE($2, company_name),
			contact_name = COALESCE($3, contact_name),
			email = COALESCE($4, email),
			phone = COALESCE($5, phone),
			address = COALESCE($6, address),
			year_end = COALESCE($7::date, year_end),
			utr = COALESCE($8, utr),
			company_number = COALESCE($9, company_number),
			company_type = COALESCE($10, company_type),
			incorporation_date = COALESCE($11::date, incorporation_date),
			sic_codes = COALESCE($12::jsonb, sic_codes),
			vat_number = COALESCE($13, vat_number),
			vat_quarter = COALESCE($14, vat_quarter),
			status = COALESCE($15, status),
			tags = COALESCE($16::jsonb, tags),
			updated_at = NOW()
		WHERE id = $1 AND anonymized_at IS NULL
	`, id, req.CompanyName, req.ContactName, req.Email, req.Phone,
		req.Address, req.YearEnd, req.UTR, req.CompanyNumber, req.CompanyType,
		req.IncorporationDate, req.SICCodes, req.VATNumber, req.VATQuarter,
		req.Status, req.Tags)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "update_error",
			"message": "Failed to update client",
		})
		return
	}

	// Fetch the updated client
	client, err := h.getClientByID(ctx, id, userTenantID, role)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "fetch_error",
			"message": "Client updated but failed to fetch",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Client updated successfully",
		"client":  client,
	})
}

// Delete soft-deletes a client (marks as archived)
func (h *ClientHandler) Delete(c *gin.Context) {
	ctx := c.Request.Context()
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid_id",
			"message": "Invalid client ID format",
		})
		return
	}

	role, _ := middleware.GetRole(c)
	userTenantID, _ := middleware.GetTenantID(c)

	// Check if client exists and user has access
	_, err = h.getClientByID(ctx, id, userTenantID, role)
	if err != nil {
		if err == pgx.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{
				"error":   "not_found",
				"message": "Client not found",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "database_error",
			"message": "Failed to fetch client",
		})
		return
	}

	// Soft delete by setting status to archived
	_, err = h.db.Exec(ctx, `
		UPDATE clients SET status = 'archived', updated_at = NOW()
		WHERE id = $1 AND anonymized_at IS NULL
	`, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "delete_error",
			"message": "Failed to delete client",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Client deleted successfully",
	})
}

// getClientByID is a helper function to fetch a client by ID with tenant isolation
func (h *ClientHandler) getClientByID(ctx context.Context, id uuid.UUID, tenantID uuid.UUID, role string) (*Client, error) {
	var cl Client
	var query string

	if role == "super_admin" {
		// Super admins can access any client
		query = `
			SELECT id, tenant_id, user_id, company_name, contact_name, email, phone,
			       address, year_end::text, utr, company_number, company_type,
			       incorporation_date::text, COALESCE(sic_codes::text, '[]'),
			       vat_number, vat_quarter, status, risk_score,
			       COALESCE(tags::text, '[]'), email_status,
			       last_contact_at, created_at, updated_at
			FROM clients
			WHERE id = $1 AND anonymized_at IS NULL
		`
		err := h.db.QueryRow(ctx, query, id).Scan(
			&cl.ID, &cl.TenantID, &cl.UserID, &cl.CompanyName, &cl.ContactName,
			&cl.Email, &cl.Phone, &cl.Address, &cl.YearEnd, &cl.UTR,
			&cl.CompanyNumber, &cl.CompanyType, &cl.IncorporationDate, &cl.SICCodes,
			&cl.VATNumber, &cl.VATQuarter, &cl.Status, &cl.RiskScore,
			&cl.Tags, &cl.EmailStatus, &cl.LastContactAt, &cl.CreatedAt, &cl.UpdatedAt,
		)
		return &cl, err
	}

	// Regular users can only access clients in their tenant
	query = `
		SELECT id, tenant_id, user_id, company_name, contact_name, email, phone,
		       address, year_end::text, utr, company_number, company_type,
		       incorporation_date::text, COALESCE(sic_codes::text, '[]'),
		       vat_number, vat_quarter, status, risk_score,
		       COALESCE(tags::text, '[]'), email_status,
		       last_contact_at, created_at, updated_at
		FROM clients
		WHERE id = $1 AND tenant_id = $2 AND anonymized_at IS NULL
	`
	err := h.db.QueryRow(ctx, query, id, tenantID).Scan(
		&cl.ID, &cl.TenantID, &cl.UserID, &cl.CompanyName, &cl.ContactName,
		&cl.Email, &cl.Phone, &cl.Address, &cl.YearEnd, &cl.UTR,
		&cl.CompanyNumber, &cl.CompanyType, &cl.IncorporationDate, &cl.SICCodes,
		&cl.VATNumber, &cl.VATQuarter, &cl.Status, &cl.RiskScore,
		&cl.Tags, &cl.EmailStatus, &cl.LastContactAt, &cl.CreatedAt, &cl.UpdatedAt,
	)
	return &cl, err
}
