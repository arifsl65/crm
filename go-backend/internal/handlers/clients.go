package handlers

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/rs/zerolog/log"

	"github.com/accountant-crm/go-backend/internal/audit"
	"github.com/accountant-crm/go-backend/internal/database"
	"github.com/accountant-crm/go-backend/internal/middleware"
)

type ClientHandler struct {
	db    *database.Pool
	audit *audit.Logger
}

func NewClientHandler(db *database.Pool, auditLogger *audit.Logger) *ClientHandler {
	return &ClientHandler{
		db:    db,
		audit: auditLogger,
	}
}

// Client represents a client record
type Client struct {
	ID                uuid.UUID  `json:"id"`
	TenantID          uuid.UUID  `json:"tenant_id"`
	UserID            *uuid.UUID `json:"user_id,omitempty"`
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
	VATNumber         *string    `json:"vat_number,omitempty"`
	VATQuarter        *string    `json:"vat_quarter,omitempty"`
	Status            string     `json:"status"`
	RiskScore         *int       `json:"risk_score,omitempty"`
	EmailStatus       string     `json:"email_status"`
	LastContactAt     *time.Time `json:"last_contact_at,omitempty"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

type CreateClientRequest struct {
	CompanyName       string  `json:"company_name" binding:"required"`
	ContactName       string  `json:"contact_name" binding:"required"`
	Email             string  `json:"email" binding:"required,email"`
	Phone             *string `json:"phone,omitempty"`
	Address           *string `json:"address,omitempty"`
	YearEnd           *string `json:"year_end,omitempty"`
	UTR               *string `json:"utr,omitempty"`
	CompanyNumber     *string `json:"company_number,omitempty"`
	CompanyType       *string `json:"company_type,omitempty"`
	IncorporationDate *string `json:"incorporation_date,omitempty"`
	VATNumber         *string `json:"vat_number,omitempty"`
	VATQuarter        *string `json:"vat_quarter,omitempty"`
}

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
	VATNumber         *string `json:"vat_number,omitempty"`
	VATQuarter        *string `json:"vat_quarter,omitempty"`
	Status            *string `json:"status,omitempty"`
}

// List returns all clients for the tenant (staff-scoped)
// GET /api/v1/clients
func (h *ClientHandler) List(c *gin.Context) {
	tenantID, _ := middleware.GetTenantID(c)
	userID, _ := middleware.GetUserID(c)
	role, _ := c.Get(middleware.AuthRole)

	// Get TenantDB for RLS-protected operations
	tenantDB, ok := middleware.GetTenantDB(c)
	if !ok {
		log.Error().Msg("TenantDB not found in context")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	// Pagination
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if limit > 100 {
		limit = 100
	}

	// Filters
	status := c.Query("status")
	search := c.Query("search")

	roleStr, _ := role.(string)
	isSuperAdmin := roleStr == "super_admin"

	var query strings.Builder
	var args []interface{}
	argNum := 1

	query.WriteString(`
		SELECT c.id, c.tenant_id, c.user_id, c.company_name, c.contact_name,
		       c.email, c.phone, c.address, c.year_end, c.utr, c.company_number,
		       c.company_type, c.incorporation_date, c.vat_number, c.vat_quarter,
		       c.status, c.risk_score, c.email_status, c.last_contact_at,
		       c.created_at, c.updated_at
		FROM clients c
	`)
	if isSuperAdmin {
		// Super admins can list clients across all tenants.
		query.WriteString(`WHERE 1=1`)
	} else {
		query.WriteString(`WHERE c.tenant_id = $`)
		query.WriteString(strconv.Itoa(argNum))
		args = append(args, tenantID)
		argNum++
	}

	// Staff scoping - only see assigned clients unless admin
	if roleStr == "staff" {
		query.WriteString(` AND c.id IN (SELECT client_id FROM staff_clients WHERE staff_id = $`)
		query.WriteString(strconv.Itoa(argNum))
		query.WriteString(`)`)
		args = append(args, userID)
		argNum++
	}

	// Status filter
	if status != "" {
		query.WriteString(` AND c.status = $`)
		query.WriteString(strconv.Itoa(argNum))
		args = append(args, status)
		argNum++
	}

	// Search filter
	if search != "" {
		query.WriteString(` AND (c.company_name ILIKE $`)
		query.WriteString(strconv.Itoa(argNum))
		query.WriteString(` OR c.contact_name ILIKE $`)
		query.WriteString(strconv.Itoa(argNum))
		query.WriteString(` OR c.email ILIKE $`)
		query.WriteString(strconv.Itoa(argNum))
		query.WriteString(`)`)
		args = append(args, "%"+search+"%")
		argNum++
	}

	query.WriteString(` ORDER BY c.company_name ASC LIMIT $`)
	query.WriteString(strconv.Itoa(argNum))
	args = append(args, limit)
	argNum++

	query.WriteString(` OFFSET $`)
	query.WriteString(strconv.Itoa(argNum))
	args = append(args, offset)

	clients := []Client{}
	err := tenantDB.Query(c, query.String(), args, func(rows pgx.Rows) error {
		var client Client
		var yearEnd, incDate *time.Time
		err := rows.Scan(
			&client.ID, &client.TenantID, &client.UserID, &client.CompanyName, &client.ContactName,
			&client.Email, &client.Phone, &client.Address, &yearEnd, &client.UTR, &client.CompanyNumber,
			&client.CompanyType, &incDate, &client.VATNumber, &client.VATQuarter,
			&client.Status, &client.RiskScore, &client.EmailStatus, &client.LastContactAt,
			&client.CreatedAt, &client.UpdatedAt,
		)
		if err != nil {
			return err
		}
		if yearEnd != nil {
			s := yearEnd.Format("2006-01-02")
			client.YearEnd = &s
		}
		if incDate != nil {
			s := incDate.Format("2006-01-02")
			client.IncorporationDate = &s
		}
		clients = append(clients, client)
		return nil
	})

	if err != nil {
		log.Error().Err(err).Msg("Failed to list clients")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"clients": clients,
		"limit":   limit,
		"offset":  offset,
	})
}

// ListSuppressed returns clients with suppressed email status (bounced, unsubscribed, complained)
// GET /api/v1/clients/suppressed
func (h *ClientHandler) ListSuppressed(c *gin.Context) {
	tenantID, _ := middleware.GetTenantID(c)

	tenantDB, ok := middleware.GetTenantDB(c)
	if !ok {
		log.Error().Msg("TenantDB not found in context")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	// Pagination
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if limit > 100 {
		limit = 100
	}

	// Filter by specific status if provided
	statusFilter := c.Query("status") // bounced, unsubscribed, complained

	var queryBuilder strings.Builder
	var args []interface{}
	argNum := 1

	queryBuilder.WriteString(`
		SELECT id, tenant_id, company_name, contact_name, email, phone,
		       status, email_status, email_status_at, created_at, updated_at
		FROM clients
		WHERE tenant_id = $1 AND deleted_at IS NULL
		  AND email_status IN ('bounced', 'unsubscribed', 'complained')
	`)
	args = append(args, tenantID)
	argNum++

	if statusFilter != "" {
		queryBuilder.WriteString(` AND email_status = $`)
		queryBuilder.WriteString(strconv.Itoa(argNum))
		args = append(args, statusFilter)
		argNum++
	}

	queryBuilder.WriteString(` ORDER BY email_status_at DESC NULLS LAST, company_name ASC LIMIT $`)
	queryBuilder.WriteString(strconv.Itoa(argNum))
	args = append(args, limit)
	argNum++

	queryBuilder.WriteString(` OFFSET $`)
	queryBuilder.WriteString(strconv.Itoa(argNum))
	args = append(args, offset)

	type SuppressedClient struct {
		ID            uuid.UUID  `json:"id"`
		TenantID      uuid.UUID  `json:"tenant_id"`
		CompanyName   string     `json:"company_name"`
		ContactName   string     `json:"contact_name"`
		Email         string     `json:"email"`
		Phone         *string    `json:"phone,omitempty"`
		Status        string     `json:"status"`
		EmailStatus   string     `json:"email_status"`
		EmailStatusAt *time.Time `json:"email_status_at,omitempty"`
		CreatedAt     time.Time  `json:"created_at"`
		UpdatedAt     time.Time  `json:"updated_at"`
	}

	var clients []SuppressedClient
	err := tenantDB.Query(c, queryBuilder.String(), args, func(rows pgx.Rows) error {
		var client SuppressedClient
		if err := rows.Scan(&client.ID, &client.TenantID, &client.CompanyName, &client.ContactName,
			&client.Email, &client.Phone, &client.Status, &client.EmailStatus,
			&client.EmailStatusAt, &client.CreatedAt, &client.UpdatedAt); err != nil {
			return err
		}
		clients = append(clients, client)
		return nil
	})

	if err != nil {
		log.Error().Err(err).Msg("Failed to list suppressed clients")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	if clients == nil {
		clients = []SuppressedClient{}
	}

	c.JSON(http.StatusOK, gin.H{
		"clients": clients,
		"limit":   limit,
		"offset":  offset,
	})
}

// Get returns a single client by ID
// GET /api/v1/clients/:id
func (h *ClientHandler) Get(c *gin.Context) {
	tenantID, _ := middleware.GetTenantID(c)
	role, _ := middleware.GetRole(c)

	// Get TenantDB for RLS-protected operations
	tenantDB, ok := middleware.GetTenantDB(c)
	if !ok {
		log.Error().Msg("TenantDB not found in context")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	clientID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_client_id"})
		return
	}

	var client Client
	var yearEnd, incDate *time.Time
	var sql string
	var args []interface{}
	if role == "super_admin" {
		sql = `
			SELECT id, tenant_id, user_id, company_name, contact_name,
			       email, phone, address, year_end, utr, company_number,
			       company_type, incorporation_date, vat_number, vat_quarter,
			       status, risk_score, email_status, last_contact_at,
			       created_at, updated_at
			FROM clients
			WHERE id = $1
		`
		args = []interface{}{clientID}
	} else {
		sql = `
			SELECT id, tenant_id, user_id, company_name, contact_name,
			       email, phone, address, year_end, utr, company_number,
			       company_type, incorporation_date, vat_number, vat_quarter,
			       status, risk_score, email_status, last_contact_at,
			       created_at, updated_at
			FROM clients
			WHERE id = $1 AND tenant_id = $2
		`
		args = []interface{}{clientID, tenantID}
	}
	err = tenantDB.QueryRowScan(c, []interface{}{
		&client.ID, &client.TenantID, &client.UserID, &client.CompanyName, &client.ContactName,
		&client.Email, &client.Phone, &client.Address, &yearEnd, &client.UTR, &client.CompanyNumber,
		&client.CompanyType, &incDate, &client.VATNumber, &client.VATQuarter,
		&client.Status, &client.RiskScore, &client.EmailStatus, &client.LastContactAt,
		&client.CreatedAt, &client.UpdatedAt,
	}, sql, args...)
	if err != nil {
		if err == pgx.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "client_not_found"})
			return
		}
		log.Error().Err(err).Msg("Failed to get client")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	if yearEnd != nil {
		s := yearEnd.Format("2006-01-02")
		client.YearEnd = &s
	}
	if incDate != nil {
		s := incDate.Format("2006-01-02")
		client.IncorporationDate = &s
	}

	c.JSON(http.StatusOK, client)
}

// Create creates a new client
// POST /api/v1/clients
func (h *ClientHandler) Create(c *gin.Context) {
	var req CreateClientRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "validation_error", "message": err.Error()})
		return
	}

	ctx := c.Request.Context()
	tenantID, _ := middleware.GetTenantID(c)
	userID, _ := middleware.GetUserID(c)

	// Get TenantDB for RLS-protected operations
	tenantDB, ok := middleware.GetTenantDB(c)
	if !ok {
		log.Error().Msg("TenantDB not found in context")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	clientID := uuid.New()

	// Parse dates if provided
	var yearEnd, incDate *time.Time
	if req.YearEnd != nil && *req.YearEnd != "" {
		t, err := time.Parse("2006-01-02", *req.YearEnd)
		if err == nil {
			yearEnd = &t
		}
	}
	if req.IncorporationDate != nil && *req.IncorporationDate != "" {
		t, err := time.Parse("2006-01-02", *req.IncorporationDate)
		if err == nil {
			incDate = &t
		}
	}

	// Use TenantDB.Transaction for RLS-protected insert
	err := tenantDB.Transaction(c, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO clients (
				id, tenant_id, company_name, contact_name, email, phone, address,
				year_end, utr, company_number, company_type, incorporation_date,
				vat_number, vat_quarter, status, email_status
			) VALUES (
				$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, 'active', 'active'
			)
		`, clientID, tenantID, req.CompanyName, req.ContactName, strings.ToLower(req.Email),
			req.Phone, req.Address, yearEnd, req.UTR, req.CompanyNumber, req.CompanyType,
			incDate, req.VATNumber, req.VATQuarter)
		return err
	})

	if err != nil {
		log.Error().Err(err).Msg("Failed to create client")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	// Audit log
	h.audit.LogEntity(ctx, audit.ActionClientCreate, &userID, &tenantID, "client", &clientID, c.ClientIP(), nil)

	c.JSON(http.StatusCreated, gin.H{
		"id":      clientID,
		"message": "Client created successfully",
	})
}

// Update updates an existing client
// PATCH /api/v1/clients/:id
func (h *ClientHandler) Update(c *gin.Context) {
	clientID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_client_id"})
		return
	}

	var req UpdateClientRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "validation_error", "message": err.Error()})
		return
	}

	ctx := c.Request.Context()
	tenantID, _ := middleware.GetTenantID(c)
	userID, _ := middleware.GetUserID(c)

	// Get TenantDB for RLS-protected operations
	tenantDB, ok := middleware.GetTenantDB(c)
	if !ok {
		log.Error().Msg("TenantDB not found in context")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	// Build dynamic update query
	var setClauses []string
	var args []interface{}
	argNum := 1

	if req.CompanyName != nil {
		setClauses = append(setClauses, "company_name = $"+strconv.Itoa(argNum))
		args = append(args, *req.CompanyName)
		argNum++
	}
	if req.ContactName != nil {
		setClauses = append(setClauses, "contact_name = $"+strconv.Itoa(argNum))
		args = append(args, *req.ContactName)
		argNum++
	}
	if req.Email != nil {
		setClauses = append(setClauses, "email = $"+strconv.Itoa(argNum))
		args = append(args, strings.ToLower(*req.Email))
		argNum++
	}
	if req.Phone != nil {
		setClauses = append(setClauses, "phone = $"+strconv.Itoa(argNum))
		args = append(args, *req.Phone)
		argNum++
	}
	if req.Address != nil {
		setClauses = append(setClauses, "address = $"+strconv.Itoa(argNum))
		args = append(args, *req.Address)
		argNum++
	}
	if req.Status != nil {
		setClauses = append(setClauses, "status = $"+strconv.Itoa(argNum))
		args = append(args, *req.Status)
		argNum++
	}
	if req.UTR != nil {
		setClauses = append(setClauses, "utr = $"+strconv.Itoa(argNum))
		args = append(args, *req.UTR)
		argNum++
	}
	if req.CompanyNumber != nil {
		setClauses = append(setClauses, "company_number = $"+strconv.Itoa(argNum))
		args = append(args, *req.CompanyNumber)
		argNum++
	}
	if req.VATNumber != nil {
		setClauses = append(setClauses, "vat_number = $"+strconv.Itoa(argNum))
		args = append(args, *req.VATNumber)
		argNum++
	}
	if req.VATQuarter != nil {
		setClauses = append(setClauses, "vat_quarter = $"+strconv.Itoa(argNum))
		args = append(args, *req.VATQuarter)
		argNum++
	}

	if len(setClauses) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no_fields_to_update"})
		return
	}

	setClauses = append(setClauses, "updated_at = NOW()")

	query := "UPDATE clients SET " + strings.Join(setClauses, ", ") +
		" WHERE id = $" + strconv.Itoa(argNum) + " AND tenant_id = $" + strconv.Itoa(argNum+1)
	args = append(args, clientID, tenantID)

	var rowsAffected int64
	err = tenantDB.Transaction(c, func(tx pgx.Tx) error {
		result, err := tx.Exec(ctx, query, args...)
		if err != nil {
			return err
		}
		rowsAffected = result.RowsAffected()
		return nil
	})

	if err != nil {
		log.Error().Err(err).Msg("Failed to update client")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	if rowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "client_not_found"})
		return
	}

	// Audit log
	h.audit.LogEntity(ctx, audit.ActionClientUpdate, &userID, &tenantID, "client", &clientID, c.ClientIP(), nil)

	c.JSON(http.StatusOK, gin.H{"message": "Client updated successfully"})
}

// Delete archives a client (soft delete)
// DELETE /api/v1/clients/:id
func (h *ClientHandler) Delete(c *gin.Context) {
	clientID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_client_id"})
		return
	}

	ctx := c.Request.Context()
	tenantID, _ := middleware.GetTenantID(c)
	userID, _ := middleware.GetUserID(c)
	role, _ := middleware.GetRole(c)

	// Get TenantDB for RLS-protected operations
	tenantDB, ok := middleware.GetTenantDB(c)
	if !ok {
		log.Error().Msg("TenantDB not found in context")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	var rowsAffected int64
	err = tenantDB.Transaction(c, func(tx pgx.Tx) error {
		var sql string
		var args []interface{}
		if role == "super_admin" {
			sql = `
				UPDATE clients SET status = 'archived', updated_at = NOW()
				WHERE id = $1
			`
			args = []interface{}{clientID}
		} else {
			sql = `
				UPDATE clients SET status = 'archived', updated_at = NOW()
				WHERE id = $1 AND tenant_id = $2
			`
			args = []interface{}{clientID, tenantID}
		}
		result, err := tx.Exec(ctx, sql, args...)
		if err != nil {
			return err
		}
		rowsAffected = result.RowsAffected()
		return nil
	})

	if err != nil {
		log.Error().Err(err).Msg("Failed to archive client")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	if rowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "client_not_found"})
		return
	}

	// Audit log
	h.audit.LogEntity(ctx, audit.ActionClientDelete, &userID, &tenantID, "client", &clientID, c.ClientIP(), nil)

	c.JSON(http.StatusOK, gin.H{"message": "Client archived successfully"})
}

// Restore restores a deleted/archived client
// POST /api/v1/clients/:id/restore
func (h *ClientHandler) Restore(c *gin.Context) {
	clientID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_client_id"})
		return
	}

	ctx := c.Request.Context()
	tenantID, _ := middleware.GetTenantID(c)

	// Get TenantDB for RLS-protected operations
	tenantDB, ok := middleware.GetTenantDB(c)
	if !ok {
		log.Error().Msg("TenantDB not found in context")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	var rowsAffected int64
	err = tenantDB.Transaction(c, func(tx pgx.Tx) error {
		result, err := tx.Exec(ctx, `
			UPDATE clients SET status = 'active', updated_at = NOW()
			WHERE id = $1 AND tenant_id = $2 AND status = 'archived'
		`, clientID, tenantID)
		if err != nil {
			return err
		}
		rowsAffected = result.RowsAffected()
		return nil
	})

	if err != nil {
		log.Error().Err(err).Msg("Failed to restore client")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	if rowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "client_not_found_or_not_archived"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Client restored successfully"})
}

// GetDocuments returns documents for a client
// GET /api/v1/clients/:id/documents
func (h *ClientHandler) GetDocuments(c *gin.Context) {
	clientID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_client_id"})
		return
	}

	tenantID, _ := middleware.GetTenantID(c)

	// Get TenantDB for RLS-protected operations
	tenantDB, ok := middleware.GetTenantDB(c)
	if !ok {
		log.Error().Msg("TenantDB not found in context")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	var documents []map[string]interface{}
	err = tenantDB.Query(c, `
		SELECT id, name, original_name, file_size, mime_type, status, created_at
		FROM documents
		WHERE client_id = $1 AND tenant_id = $2
		ORDER BY created_at DESC
		LIMIT 100
	`, []interface{}{clientID, tenantID}, func(rows pgx.Rows) error {
		var id uuid.UUID
		var name, originalName, status string
		var fileSize *int
		var mimeType *string
		var createdAt time.Time

		if err := rows.Scan(&id, &name, &originalName, &fileSize, &mimeType, &status, &createdAt); err != nil {
			return err
		}

		documents = append(documents, map[string]interface{}{
			"id":            id,
			"name":          name,
			"original_name": originalName,
			"file_size":     fileSize,
			"mime_type":     mimeType,
			"status":        status,
			"created_at":    createdAt,
		})
		return nil
	})

	if err != nil {
		log.Error().Err(err).Msg("Failed to get client documents")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"documents": documents})
}

// GetServices returns services for a client
// GET /api/v1/clients/:id/services
func (h *ClientHandler) GetServices(c *gin.Context) {
	clientID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_client_id"})
		return
	}

	tenantID, _ := middleware.GetTenantID(c)

	// Get TenantDB for RLS-protected operations
	tenantDB, ok := middleware.GetTenantDB(c)
	if !ok {
		log.Error().Msg("TenantDB not found in context")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	var services []map[string]interface{}
	err = tenantDB.Query(c, `
		SELECT id, name, status, priority, deadline, docs_required, docs_received, created_at
		FROM services
		WHERE client_id = $1 AND tenant_id = $2
		ORDER BY deadline ASC NULLS LAST, created_at DESC
		LIMIT 100
	`, []interface{}{clientID, tenantID}, func(rows pgx.Rows) error {
		var id uuid.UUID
		var name, status, priority string
		var deadline *time.Time
		var docsRequired, docsReceived int
		var createdAt time.Time

		if err := rows.Scan(&id, &name, &status, &priority, &deadline, &docsRequired, &docsReceived, &createdAt); err != nil {
			return err
		}

		services = append(services, map[string]interface{}{
			"id":            id,
			"name":          name,
			"status":        status,
			"priority":      priority,
			"deadline":      deadline,
			"docs_required": docsRequired,
			"docs_received": docsReceived,
			"created_at":    createdAt,
		})
		return nil
	})

	if err != nil {
		log.Error().Err(err).Msg("Failed to get client services")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"services": services})
}

// ClientNote represents a note on a client
type ClientNote struct {
	ID        uuid.UUID `json:"id"`
	TenantID  uuid.UUID `json:"tenant_id"`
	ClientID  uuid.UUID `json:"client_id"`
	StaffID   uuid.UUID `json:"staff_id"`
	Note      string    `json:"note"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	// Joined fields
	StaffName *string `json:"staff_name,omitempty"`
}

// ListNotes returns all notes for a client
// GET /api/v1/clients/:id/notes
func (h *ClientHandler) ListNotes(c *gin.Context) {
	clientID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_client_id"})
		return
	}

	tenantID, _ := middleware.GetTenantID(c)

	tenantDB, ok := middleware.GetTenantDB(c)
	if !ok {
		log.Error().Msg("TenantDB not found in context")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	var notes []ClientNote
	err = tenantDB.Query(c, `
		SELECT cn.id, cn.tenant_id, cn.client_id, cn.staff_id, cn.note,
		       cn.created_at, cn.updated_at, u.name as staff_name
		FROM client_notes cn
		LEFT JOIN users u ON cn.staff_id = u.id
		WHERE cn.client_id = $1 AND cn.tenant_id = $2
		ORDER BY cn.created_at DESC
	`, []interface{}{clientID, tenantID}, func(rows pgx.Rows) error {
		var note ClientNote
		if err := rows.Scan(&note.ID, &note.TenantID, &note.ClientID, &note.StaffID,
			&note.Note, &note.CreatedAt, &note.UpdatedAt, &note.StaffName); err != nil {
			return err
		}
		notes = append(notes, note)
		return nil
	})

	if err != nil {
		log.Error().Err(err).Msg("Failed to list client notes")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	if notes == nil {
		notes = []ClientNote{}
	}

	c.JSON(http.StatusOK, gin.H{"notes": notes})
}

// CreateNote creates a new note for a client
// POST /api/v1/clients/:id/notes
func (h *ClientHandler) CreateNote(c *gin.Context) {
	clientID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_client_id"})
		return
	}

	var req struct {
		Note string `json:"note" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "validation_error", "message": err.Error()})
		return
	}

	ctx := c.Request.Context()
	tenantID, _ := middleware.GetTenantID(c)
	userID, _ := middleware.GetUserID(c)

	tenantDB, ok := middleware.GetTenantDB(c)
	if !ok {
		log.Error().Msg("TenantDB not found in context")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	noteID := uuid.New()
	err = tenantDB.Transaction(c, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO client_notes (id, tenant_id, client_id, staff_id, note)
			VALUES ($1, $2, $3, $4, $5)
		`, noteID, tenantID, clientID, userID, req.Note)
		return err
	})

	if err != nil {
		log.Error().Err(err).Msg("Failed to create client note")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	h.audit.LogEntity(ctx, audit.ActionClientUpdate, &userID, &tenantID, "client_note", &noteID, c.ClientIP(), nil)

	c.JSON(http.StatusCreated, gin.H{
		"id":      noteID,
		"message": "Note created successfully",
	})
}

// UpdateNote updates an existing note
// PATCH /api/v1/clients/:id/notes/:noteId
// Fix #42: Added ownership check - only note author or admins can update
func (h *ClientHandler) UpdateNote(c *gin.Context) {
	clientID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_client_id"})
		return
	}

	noteID, err := uuid.Parse(c.Param("noteId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_note_id"})
		return
	}

	var req struct {
		Note string `json:"note" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "validation_error", "message": err.Error()})
		return
	}

	ctx := c.Request.Context()
	tenantID, _ := middleware.GetTenantID(c)
	userID, _ := middleware.GetUserID(c)
	role, _ := middleware.GetRole(c)

	tenantDB, ok := middleware.GetTenantDB(c)
	if !ok {
		log.Error().Msg("TenantDB not found in context")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	// Fix #42: Check note ownership - only author or admins can update
	var noteAuthorID uuid.UUID
	err = tenantDB.Transaction(c, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `
			SELECT staff_id FROM client_notes
			WHERE id = $1 AND client_id = $2 AND tenant_id = $3
		`, noteID, clientID, tenantID).Scan(&noteAuthorID)
	})

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "note_not_found"})
			return
		}
		log.Error().Err(err).Msg("Failed to fetch note author")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	// Allow update if user is author, tenant_admin, or super_admin
	isAdmin := role == "tenant_admin" || role == "super_admin"
	if noteAuthorID != userID && !isAdmin {
		c.JSON(http.StatusForbidden, gin.H{
			"error":   "forbidden",
			"message": "You can only edit your own notes",
		})
		return
	}

	var rowsAffected int64
	err = tenantDB.Transaction(c, func(tx pgx.Tx) error {
		result, err := tx.Exec(ctx, `
			UPDATE client_notes SET note = $1, updated_at = NOW()
			WHERE id = $2 AND client_id = $3 AND tenant_id = $4
		`, req.Note, noteID, clientID, tenantID)
		if err != nil {
			return err
		}
		rowsAffected = result.RowsAffected()
		return nil
	})

	if err != nil {
		log.Error().Err(err).Msg("Failed to update client note")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	if rowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "note_not_found"})
		return
	}

	h.audit.LogEntity(ctx, audit.ActionClientUpdate, &userID, &tenantID, "client_note", &noteID, c.ClientIP(), nil)

	c.JSON(http.StatusOK, gin.H{"message": "Note updated successfully"})
}

// DeleteNote deletes a note
// DELETE /api/v1/clients/:id/notes/:noteId
// Fix #42: Added ownership check - only note author or admins can delete
func (h *ClientHandler) DeleteNote(c *gin.Context) {
	clientID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_client_id"})
		return
	}

	noteID, err := uuid.Parse(c.Param("noteId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_note_id"})
		return
	}

	ctx := c.Request.Context()
	tenantID, _ := middleware.GetTenantID(c)
	userID, _ := middleware.GetUserID(c)
	role, _ := middleware.GetRole(c)

	tenantDB, ok := middleware.GetTenantDB(c)
	if !ok {
		log.Error().Msg("TenantDB not found in context")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	// Fix #42: Check note ownership - only author or admins can delete
	var noteAuthorID uuid.UUID
	err = tenantDB.Transaction(c, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `
			SELECT staff_id FROM client_notes
			WHERE id = $1 AND client_id = $2 AND tenant_id = $3
		`, noteID, clientID, tenantID).Scan(&noteAuthorID)
	})

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "note_not_found"})
			return
		}
		log.Error().Err(err).Msg("Failed to fetch note author")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	// Allow delete if user is author, tenant_admin, or super_admin
	isAdmin := role == "tenant_admin" || role == "super_admin"
	if noteAuthorID != userID && !isAdmin {
		c.JSON(http.StatusForbidden, gin.H{
			"error":   "forbidden",
			"message": "You can only delete your own notes",
		})
		return
	}

	var rowsAffected int64
	err = tenantDB.Transaction(c, func(tx pgx.Tx) error {
		result, err := tx.Exec(ctx, `
			DELETE FROM client_notes WHERE id = $1 AND client_id = $2 AND tenant_id = $3
		`, noteID, clientID, tenantID)
		if err != nil {
			return err
		}
		rowsAffected = result.RowsAffected()
		return nil
	})

	if err != nil {
		log.Error().Err(err).Msg("Failed to delete client note")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	if rowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "note_not_found"})
		return
	}

	h.audit.LogEntity(ctx, audit.ActionClientUpdate, &userID, &tenantID, "client_note", &noteID, c.ClientIP(), nil)

	c.JSON(http.StatusOK, gin.H{"message": "Note deleted successfully"})
}

// ClientEmail represents an email associated with a client
type ClientEmail struct {
	ID        uuid.UUID  `json:"id"`
	TenantID  uuid.UUID  `json:"tenant_id"`
	ClientID  *uuid.UUID `json:"client_id,omitempty"`
	StaffID   *uuid.UUID `json:"staff_id,omitempty"`
	Direction string     `json:"direction"`
	ToEmail   string     `json:"to_email"`
	FromEmail string     `json:"from_email"`
	Subject   string     `json:"subject"`
	Status    string     `json:"status"`
	IsRead    bool       `json:"is_read"`
	SentAt    *time.Time `json:"sent_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
	// Joined fields
	StaffName *string `json:"staff_name,omitempty"`
}

// GetEmails returns email history for a client
// GET /api/v1/clients/:id/emails
func (h *ClientHandler) GetEmails(c *gin.Context) {
	clientID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_client_id"})
		return
	}

	tenantID, _ := middleware.GetTenantID(c)

	tenantDB, ok := middleware.GetTenantDB(c)
	if !ok {
		log.Error().Msg("TenantDB not found in context")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	// Pagination
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if limit > 100 {
		limit = 100
	}

	var emails []ClientEmail
	err = tenantDB.Query(c, `
		SELECT e.id, e.tenant_id, e.client_id, e.staff_id, e.direction,
		       e.to_email, e.from_email, e.subject, e.status, e.is_read,
		       e.sent_at, e.created_at, u.name as staff_name
		FROM emails e
		LEFT JOIN users u ON e.staff_id = u.id
		WHERE e.client_id = $1 AND e.tenant_id = $2
		ORDER BY e.created_at DESC
		LIMIT $3 OFFSET $4
	`, []interface{}{clientID, tenantID, limit, offset}, func(rows pgx.Rows) error {
		var email ClientEmail
		if err := rows.Scan(&email.ID, &email.TenantID, &email.ClientID, &email.StaffID,
			&email.Direction, &email.ToEmail, &email.FromEmail, &email.Subject,
			&email.Status, &email.IsRead, &email.SentAt, &email.CreatedAt, &email.StaffName); err != nil {
			return err
		}
		emails = append(emails, email)
		return nil
	})

	if err != nil {
		log.Error().Err(err).Msg("Failed to get client emails")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	if emails == nil {
		emails = []ClientEmail{}
	}

	c.JSON(http.StatusOK, gin.H{
		"emails": emails,
		"limit":  limit,
		"offset": offset,
	})
}

// AssignStaff assigns a staff member to a client
// POST /api/v1/clients/:id/assign
func (h *ClientHandler) AssignStaff(c *gin.Context) {
	clientID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_client_id"})
		return
	}

	var req struct {
		StaffID   string `json:"staff_id" binding:"required,uuid"`
		IsPrimary bool   `json:"is_primary"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "validation_error", "message": err.Error()})
		return
	}

	staffID, _ := uuid.Parse(req.StaffID)
	ctx := c.Request.Context()
	tenantID, _ := middleware.GetTenantID(c)

	// Get TenantDB for RLS-protected operations
	tenantDB, ok := middleware.GetTenantDB(c)
	if !ok {
		log.Error().Msg("TenantDB not found in context")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	err = tenantDB.Transaction(c, func(tx pgx.Tx) error {
		// If setting as primary, clear existing primary first
		if req.IsPrimary {
			// Fix #44: Capture error instead of ignoring it
			_, err := tx.Exec(ctx, `
				UPDATE staff_clients SET is_primary = false
				WHERE client_id = $1 AND tenant_id = $2
			`, clientID, tenantID)
			if err != nil {
				return err
			}
		}

		_, err := tx.Exec(ctx, `
			INSERT INTO staff_clients (tenant_id, staff_id, client_id, is_primary)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT (staff_id, client_id) DO UPDATE SET is_primary = $4
		`, tenantID, staffID, clientID, req.IsPrimary)
		return err
	})

	if err != nil {
		log.Error().Err(err).Msg("Failed to assign staff to client")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Staff assigned successfully"})
}

// BulkReassign reassigns multiple clients to a different staff member
// POST /api/v1/clients/bulk-reassign
func (h *ClientHandler) BulkReassign(c *gin.Context) {
	var req struct {
		ClientIDs   []string `json:"client_ids" binding:"required"`
		NewStaffID  string   `json:"new_staff_id" binding:"required,uuid"`
		SetPrimary  bool     `json:"set_primary"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "validation_error", "message": err.Error()})
		return
	}

	ctx := c.Request.Context()
	tenantID, _ := middleware.GetTenantID(c)
	userID, _ := middleware.GetUserID(c)

	tenantDB, ok := middleware.GetTenantDB(c)
	if !ok {
		log.Error().Msg("TenantDB not found in context")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	newStaffID, _ := uuid.Parse(req.NewStaffID)

	// Parse all client IDs
	var clientIDs []uuid.UUID
	for _, idStr := range req.ClientIDs {
		if id, err := uuid.Parse(idStr); err == nil {
			clientIDs = append(clientIDs, id)
		}
	}

	if len(clientIDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no_valid_client_ids"})
		return
	}

	// Micro-batching: process 10 clients at a time to protect Neon 10-conn limit
	batchSize := 10
	var successCount int64
	var failedIDs []string

	for i := 0; i < len(clientIDs); i += batchSize {
		end := i + batchSize
		if end > len(clientIDs) {
			end = len(clientIDs)
		}
		batch := clientIDs[i:end]

		err := tenantDB.Transaction(c, func(tx pgx.Tx) error {
			for _, clientID := range batch {
				// Clear existing primary if setting new as primary
				if req.SetPrimary {
					// Fix #44: Capture error instead of ignoring it
					_, err := tx.Exec(ctx, `
						UPDATE staff_clients SET is_primary = false
						WHERE client_id = $1 AND tenant_id = $2
					`, clientID, tenantID)
					if err != nil {
						return err
					}
				}

				// Assign new staff
				result, err := tx.Exec(ctx, `
					INSERT INTO staff_clients (tenant_id, staff_id, client_id, is_primary)
					VALUES ($1, $2, $3, $4)
					ON CONFLICT (staff_id, client_id) DO UPDATE SET is_primary = $4
				`, tenantID, newStaffID, clientID, req.SetPrimary)
				if err != nil {
					return err
				}
				successCount += result.RowsAffected()
			}
			return nil
		})

		if err != nil {
			log.Error().Err(err).Int("batch_start", i).Msg("Failed to reassign batch")
			for _, id := range batch {
				failedIDs = append(failedIDs, id.String())
			}
		}
	}

	// Audit log
	h.audit.LogEntity(ctx, audit.ActionClientUpdate, &userID, &tenantID, "client", nil, c.ClientIP(), map[string]interface{}{
		"bulk_reassign": true,
		"count":         successCount,
		"new_staff_id":  newStaffID,
	})

	response := gin.H{
		"message":       "Bulk reassignment completed",
		"success_count": successCount,
		"total":         len(clientIDs),
	}
	if len(failedIDs) > 0 {
		response["failed_ids"] = failedIDs
	}

	c.JSON(http.StatusOK, response)
}
