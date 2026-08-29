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
	ctx := c.Request.Context()
	tenantID, _ := middleware.GetTenantID(c)
	userID, _ := middleware.GetUserID(c)
	role, _ := c.Get(middleware.AuthRole)

	// Pagination
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if limit > 100 {
		limit = 100
	}

	// Filters
	status := c.Query("status")
	search := c.Query("search")

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
		WHERE c.tenant_id = $1
	`)
	args = append(args, tenantID)
	argNum++

	// Staff scoping - only see assigned clients unless admin
	roleStr, _ := role.(string)
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

	rows, err := h.db.Query(ctx, query.String(), args...)
	if err != nil {
		log.Error().Err(err).Msg("Failed to list clients")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}
	defer rows.Close()

	clients := []Client{}
	for rows.Next() {
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
			log.Error().Err(err).Msg("Failed to scan client")
			continue
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
	ctx := c.Request.Context()
	tenantID, _ := middleware.GetTenantID(c)

	clientID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_client_id"})
		return
	}

	var client Client
	var yearEnd, incDate *time.Time
	err = h.db.QueryRow(ctx, `
		SELECT id, tenant_id, user_id, company_name, contact_name,
		       email, phone, address, year_end, utr, company_number,
		       company_type, incorporation_date, vat_number, vat_quarter,
		       status, risk_score, email_status, last_contact_at,
		       created_at, updated_at
		FROM clients
		WHERE id = $1 AND tenant_id = $2
	`, clientID, tenantID).Scan(
		&client.ID, &client.TenantID, &client.UserID, &client.CompanyName, &client.ContactName,
		&client.Email, &client.Phone, &client.Address, &yearEnd, &client.UTR, &client.CompanyNumber,
		&client.CompanyType, &incDate, &client.VATNumber, &client.VATQuarter,
		&client.Status, &client.RiskScore, &client.EmailStatus, &client.LastContactAt,
		&client.CreatedAt, &client.UpdatedAt,
	)
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

	_, err := h.db.Exec(ctx, `
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

	result, err := h.db.Exec(ctx, query, args...)
	if err != nil {
		log.Error().Err(err).Msg("Failed to update client")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	if result.RowsAffected() == 0 {
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

	result, err := h.db.Exec(ctx, `
		UPDATE clients SET status = 'archived', updated_at = NOW()
		WHERE id = $1 AND tenant_id = $2
	`, clientID, tenantID)

	if err != nil {
		log.Error().Err(err).Msg("Failed to archive client")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	if result.RowsAffected() == 0 {
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

	result, err := h.db.Exec(ctx, `
		UPDATE clients SET status = 'active', updated_at = NOW()
		WHERE id = $1 AND tenant_id = $2 AND status = 'archived'
	`, clientID, tenantID)

	if err != nil {
		log.Error().Err(err).Msg("Failed to restore client")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	if result.RowsAffected() == 0 {
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

	ctx := c.Request.Context()
	tenantID, _ := middleware.GetTenantID(c)

	rows, err := h.db.Query(ctx, `
		SELECT id, name, original_name, file_size, mime_type, status, created_at
		FROM documents
		WHERE client_id = $1 AND tenant_id = $2
		ORDER BY created_at DESC
		LIMIT 100
	`, clientID, tenantID)

	if err != nil {
		log.Error().Err(err).Msg("Failed to get client documents")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}
	defer rows.Close()

	var documents []map[string]interface{}
	for rows.Next() {
		var id uuid.UUID
		var name, originalName, status string
		var fileSize *int
		var mimeType *string
		var createdAt time.Time

		if err := rows.Scan(&id, &name, &originalName, &fileSize, &mimeType, &status, &createdAt); err != nil {
			continue
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

	ctx := c.Request.Context()
	tenantID, _ := middleware.GetTenantID(c)

	rows, err := h.db.Query(ctx, `
		SELECT id, name, status, priority, deadline, docs_required, docs_received, created_at
		FROM services
		WHERE client_id = $1 AND tenant_id = $2
		ORDER BY deadline ASC NULLS LAST, created_at DESC
		LIMIT 100
	`, clientID, tenantID)

	if err != nil {
		log.Error().Err(err).Msg("Failed to get client services")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}
	defer rows.Close()

	var services []map[string]interface{}
	for rows.Next() {
		var id uuid.UUID
		var name, status, priority string
		var deadline *time.Time
		var docsRequired, docsReceived int
		var createdAt time.Time

		if err := rows.Scan(&id, &name, &status, &priority, &deadline, &docsRequired, &docsReceived, &createdAt); err != nil {
			continue
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
	}

	c.JSON(http.StatusOK, gin.H{"services": services})
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

	// If setting as primary, clear existing primary first
	if req.IsPrimary {
		_, _ = h.db.Exec(ctx, `
			UPDATE staff_clients SET is_primary = false
			WHERE client_id = $1 AND tenant_id = $2
		`, clientID, tenantID)
	}

	_, err = h.db.Exec(ctx, `
		INSERT INTO staff_clients (tenant_id, staff_id, client_id, is_primary)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (staff_id, client_id) DO UPDATE SET is_primary = $4
	`, tenantID, staffID, clientID, req.IsPrimary)

	if err != nil {
		log.Error().Err(err).Msg("Failed to assign staff to client")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Staff assigned successfully"})
}
