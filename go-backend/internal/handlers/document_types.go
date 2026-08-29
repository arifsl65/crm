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

type DocumentTypeHandler struct {
	db    *database.Pool
	audit *audit.Logger
}

func NewDocumentTypeHandler(db *database.Pool, auditLogger *audit.Logger) *DocumentTypeHandler {
	return &DocumentTypeHandler{
		db:    db,
		audit: auditLogger,
	}
}

// DocumentType represents a document type template (e.g., Bank Statement, P60, Invoice)
type DocumentType struct {
	ID               uuid.UUID  `json:"id"`
	TenantID         uuid.UUID  `json:"tenant_id"`
	Name             string     `json:"name"`
	Category         string     `json:"category"`
	Description      *string    `json:"description,omitempty"`
	AllowedMimeTypes []string   `json:"allowed_mime_types,omitempty"`
	MaxFileSizeMB    *int       `json:"max_file_size_mb,omitempty"`
	RetentionDays    *int       `json:"retention_days,omitempty"`
	RequiresApproval bool       `json:"requires_approval"`
	ExpiryRequired   bool       `json:"expiry_required"`
	IsActive         bool       `json:"is_active"`
	SortOrder        int        `json:"sort_order"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
	// Computed fields
	DocumentCount    int        `json:"document_count,omitempty"`
}

type CreateDocumentTypeRequest struct {
	Name             string   `json:"name" binding:"required"`
	Category         string   `json:"category" binding:"required"`
	Description      *string  `json:"description,omitempty"`
	AllowedMimeTypes []string `json:"allowed_mime_types,omitempty"`
	MaxFileSizeMB    *int     `json:"max_file_size_mb,omitempty"`
	RetentionDays    *int     `json:"retention_days,omitempty"`
	RequiresApproval *bool    `json:"requires_approval,omitempty"`
	ExpiryRequired   *bool    `json:"expiry_required,omitempty"`
}

type UpdateDocumentTypeRequest struct {
	Name             *string  `json:"name,omitempty"`
	Category         *string  `json:"category,omitempty"`
	Description      *string  `json:"description,omitempty"`
	AllowedMimeTypes []string `json:"allowed_mime_types,omitempty"`
	MaxFileSizeMB    *int     `json:"max_file_size_mb,omitempty"`
	RetentionDays    *int     `json:"retention_days,omitempty"`
	RequiresApproval *bool    `json:"requires_approval,omitempty"`
	ExpiryRequired   *bool    `json:"expiry_required,omitempty"`
	IsActive         *bool    `json:"is_active,omitempty"`
	SortOrder        *int     `json:"sort_order,omitempty"`
}

// List returns all document types for the tenant
// GET /api/v1/document-types
func (h *DocumentTypeHandler) List(c *gin.Context) {
	ctx := c.Request.Context()
	tenantID, _ := middleware.GetTenantID(c)

	// Pagination
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if limit > 100 {
		limit = 100
	}

	// Filters
	category := c.Query("category")
	activeOnly := c.DefaultQuery("active", "true") == "true"
	search := c.Query("search")

	var query strings.Builder
	var args []interface{}
	argNum := 1

	query.WriteString(`
		SELECT dt.id, dt.tenant_id, dt.name, dt.category, dt.description,
		       dt.allowed_mime_types, dt.max_file_size_mb, dt.retention_days,
		       dt.requires_approval, dt.expiry_required, dt.is_active, dt.sort_order,
		       dt.created_at, dt.updated_at,
		       COALESCE((SELECT COUNT(*) FROM documents d WHERE d.type_id = dt.id), 0) as document_count
		FROM document_types dt
		WHERE dt.tenant_id = $1
	`)
	args = append(args, tenantID)
	argNum++

	if activeOnly {
		query.WriteString(` AND dt.is_active = true`)
	}

	if category != "" {
		query.WriteString(` AND dt.category = $`)
		query.WriteString(strconv.Itoa(argNum))
		args = append(args, category)
		argNum++
	}

	if search != "" {
		query.WriteString(` AND (dt.name ILIKE $`)
		query.WriteString(strconv.Itoa(argNum))
		query.WriteString(` OR dt.description ILIKE $`)
		query.WriteString(strconv.Itoa(argNum))
		query.WriteString(`)`)
		args = append(args, "%"+search+"%")
		argNum++
	}

	query.WriteString(` ORDER BY dt.sort_order ASC, dt.name ASC LIMIT $`)
	query.WriteString(strconv.Itoa(argNum))
	args = append(args, limit)
	argNum++

	query.WriteString(` OFFSET $`)
	query.WriteString(strconv.Itoa(argNum))
	args = append(args, offset)

	rows, err := h.db.Query(ctx, query.String(), args...)
	if err != nil {
		log.Error().Err(err).Msg("Failed to list document types")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch document types"})
		return
	}
	defer rows.Close()

	var documentTypes []DocumentType
	for rows.Next() {
		var dt DocumentType
		err := rows.Scan(
			&dt.ID, &dt.TenantID, &dt.Name, &dt.Category, &dt.Description,
			&dt.AllowedMimeTypes, &dt.MaxFileSizeMB, &dt.RetentionDays,
			&dt.RequiresApproval, &dt.ExpiryRequired, &dt.IsActive, &dt.SortOrder,
			&dt.CreatedAt, &dt.UpdatedAt, &dt.DocumentCount,
		)
		if err != nil {
			log.Error().Err(err).Msg("Failed to scan document type")
			continue
		}
		documentTypes = append(documentTypes, dt)
	}

	if documentTypes == nil {
		documentTypes = []DocumentType{}
	}

	c.JSON(http.StatusOK, gin.H{
		"document_types": documentTypes,
		"count":          len(documentTypes),
	})
}

// Get returns a single document type
// GET /api/v1/document-types/:id
func (h *DocumentTypeHandler) Get(c *gin.Context) {
	ctx := c.Request.Context()
	tenantID, _ := middleware.GetTenantID(c)
	id := c.Param("id")

	dtID, err := uuid.Parse(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid document type ID"})
		return
	}

	query := `
		SELECT dt.id, dt.tenant_id, dt.name, dt.category, dt.description,
		       dt.allowed_mime_types, dt.max_file_size_mb, dt.retention_days,
		       dt.requires_approval, dt.expiry_required, dt.is_active, dt.sort_order,
		       dt.created_at, dt.updated_at,
		       COALESCE((SELECT COUNT(*) FROM documents d WHERE d.type_id = dt.id), 0) as document_count
		FROM document_types dt
		WHERE dt.id = $1 AND dt.tenant_id = $2
	`

	var dt DocumentType
	err = h.db.QueryRow(ctx, query, dtID, tenantID).Scan(
		&dt.ID, &dt.TenantID, &dt.Name, &dt.Category, &dt.Description,
		&dt.AllowedMimeTypes, &dt.MaxFileSizeMB, &dt.RetentionDays,
		&dt.RequiresApproval, &dt.ExpiryRequired, &dt.IsActive, &dt.SortOrder,
		&dt.CreatedAt, &dt.UpdatedAt, &dt.DocumentCount,
	)

	if err == pgx.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "Document type not found"})
		return
	}
	if err != nil {
		log.Error().Err(err).Msg("Failed to get document type")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch document type"})
		return
	}

	c.JSON(http.StatusOK, dt)
}

// Create creates a new document type
// POST /api/v1/document-types
func (h *DocumentTypeHandler) Create(c *gin.Context) {
	ctx := c.Request.Context()
	tenantID, _ := middleware.GetTenantID(c)
	userID, _ := middleware.GetUserID(c)

	var req CreateDocumentTypeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Default values
	requiresApproval := true
	if req.RequiresApproval != nil {
		requiresApproval = *req.RequiresApproval
	}

	expiryRequired := false
	if req.ExpiryRequired != nil {
		expiryRequired = *req.ExpiryRequired
	}

	// Default mime types if not provided
	allowedMimeTypes := req.AllowedMimeTypes
	if allowedMimeTypes == nil {
		allowedMimeTypes = []string{"application/pdf", "image/jpeg", "image/png"}
	}

	// Get max sort order
	var maxOrder int
	err := h.db.QueryRow(ctx, `
		SELECT COALESCE(MAX(sort_order), 0) FROM document_types WHERE tenant_id = $1
	`, tenantID).Scan(&maxOrder)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get max sort order")
	}

	id := uuid.New()
	query := `
		INSERT INTO document_types (
			id, tenant_id, name, category, description, allowed_mime_types,
			max_file_size_mb, retention_days, requires_approval, expiry_required,
			is_active, sort_order, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, true, $11, NOW(), NOW())
		RETURNING id, created_at, updated_at
	`

	var dt DocumentType
	dt.ID = id
	dt.TenantID = tenantID
	dt.Name = req.Name
	dt.Category = req.Category
	dt.Description = req.Description
	dt.AllowedMimeTypes = allowedMimeTypes
	dt.MaxFileSizeMB = req.MaxFileSizeMB
	dt.RetentionDays = req.RetentionDays
	dt.RequiresApproval = requiresApproval
	dt.ExpiryRequired = expiryRequired
	dt.IsActive = true
	dt.SortOrder = maxOrder + 1

	err = h.db.QueryRow(ctx, query,
		id, tenantID, req.Name, req.Category, req.Description, allowedMimeTypes,
		req.MaxFileSizeMB, req.RetentionDays, requiresApproval, expiryRequired, maxOrder+1,
	).Scan(&dt.ID, &dt.CreatedAt, &dt.UpdatedAt)

	if err != nil {
		log.Error().Err(err).Msg("Failed to create document type")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create document type"})
		return
	}

	// Audit log
	h.audit.LogEntity(ctx, audit.ActionDocumentTypeCreate, &userID, &tenantID, "document_type", &dt.ID, c.ClientIP(), map[string]interface{}{
		"name":     req.Name,
		"category": req.Category,
	})

	c.JSON(http.StatusCreated, dt)
}

// Update updates a document type
// PATCH /api/v1/document-types/:id
func (h *DocumentTypeHandler) Update(c *gin.Context) {
	ctx := c.Request.Context()
	tenantID, _ := middleware.GetTenantID(c)
	userID, _ := middleware.GetUserID(c)
	id := c.Param("id")

	dtID, err := uuid.Parse(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid document type ID"})
		return
	}

	var req UpdateDocumentTypeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Build dynamic update query
	var updates []string
	var args []interface{}
	argNum := 1

	if req.Name != nil {
		updates = append(updates, "name = $"+strconv.Itoa(argNum))
		args = append(args, *req.Name)
		argNum++
	}
	if req.Category != nil {
		updates = append(updates, "category = $"+strconv.Itoa(argNum))
		args = append(args, *req.Category)
		argNum++
	}
	if req.Description != nil {
		updates = append(updates, "description = $"+strconv.Itoa(argNum))
		args = append(args, *req.Description)
		argNum++
	}
	if req.AllowedMimeTypes != nil {
		updates = append(updates, "allowed_mime_types = $"+strconv.Itoa(argNum))
		args = append(args, req.AllowedMimeTypes)
		argNum++
	}
	if req.MaxFileSizeMB != nil {
		updates = append(updates, "max_file_size_mb = $"+strconv.Itoa(argNum))
		args = append(args, *req.MaxFileSizeMB)
		argNum++
	}
	if req.RetentionDays != nil {
		updates = append(updates, "retention_days = $"+strconv.Itoa(argNum))
		args = append(args, *req.RetentionDays)
		argNum++
	}
	if req.RequiresApproval != nil {
		updates = append(updates, "requires_approval = $"+strconv.Itoa(argNum))
		args = append(args, *req.RequiresApproval)
		argNum++
	}
	if req.ExpiryRequired != nil {
		updates = append(updates, "expiry_required = $"+strconv.Itoa(argNum))
		args = append(args, *req.ExpiryRequired)
		argNum++
	}
	if req.IsActive != nil {
		updates = append(updates, "is_active = $"+strconv.Itoa(argNum))
		args = append(args, *req.IsActive)
		argNum++
	}
	if req.SortOrder != nil {
		updates = append(updates, "sort_order = $"+strconv.Itoa(argNum))
		args = append(args, *req.SortOrder)
		argNum++
	}

	if len(updates) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No fields to update"})
		return
	}

	updates = append(updates, "updated_at = NOW()")

	query := "UPDATE document_types SET " + strings.Join(updates, ", ") +
		" WHERE id = $" + strconv.Itoa(argNum) + " AND tenant_id = $" + strconv.Itoa(argNum+1) +
		" RETURNING id, tenant_id, name, category, description, allowed_mime_types, " +
		"max_file_size_mb, retention_days, requires_approval, expiry_required, " +
		"is_active, sort_order, created_at, updated_at"

	args = append(args, dtID, tenantID)

	var dt DocumentType
	err = h.db.QueryRow(ctx, query, args...).Scan(
		&dt.ID, &dt.TenantID, &dt.Name, &dt.Category, &dt.Description,
		&dt.AllowedMimeTypes, &dt.MaxFileSizeMB, &dt.RetentionDays,
		&dt.RequiresApproval, &dt.ExpiryRequired, &dt.IsActive, &dt.SortOrder,
		&dt.CreatedAt, &dt.UpdatedAt,
	)

	if err == pgx.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "Document type not found"})
		return
	}
	if err != nil {
		log.Error().Err(err).Msg("Failed to update document type")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update document type"})
		return
	}

	// Audit log
	h.audit.LogEntity(ctx, audit.ActionDocumentTypeUpdate, &userID, &tenantID, "document_type", &dtID, c.ClientIP(), nil)

	c.JSON(http.StatusOK, dt)
}

// Delete deletes a document type (soft delete by setting is_active = false)
// DELETE /api/v1/document-types/:id
func (h *DocumentTypeHandler) Delete(c *gin.Context) {
	ctx := c.Request.Context()
	tenantID, _ := middleware.GetTenantID(c)
	userID, _ := middleware.GetUserID(c)
	id := c.Param("id")

	dtID, err := uuid.Parse(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid document type ID"})
		return
	}

	// Check if any documents are using this type
	var documentCount int
	err = h.db.QueryRow(ctx, `
		SELECT COUNT(*) FROM documents WHERE type_id = $1 AND tenant_id = $2
	`, dtID, tenantID).Scan(&documentCount)
	if err != nil {
		log.Error().Err(err).Msg("Failed to check document type usage")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete document type"})
		return
	}

	if documentCount > 0 {
		// Soft delete - just deactivate
		_, err = h.db.Exec(ctx, `
			UPDATE document_types SET is_active = false, updated_at = NOW()
			WHERE id = $1 AND tenant_id = $2
		`, dtID, tenantID)
		if err != nil {
			log.Error().Err(err).Msg("Failed to deactivate document type")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete document type"})
			return
		}
	} else {
		// Hard delete - no documents using it
		result, err := h.db.Exec(ctx, `
			DELETE FROM document_types WHERE id = $1 AND tenant_id = $2
		`, dtID, tenantID)
		if err != nil {
			log.Error().Err(err).Msg("Failed to delete document type")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete document type"})
			return
		}
		if result.RowsAffected() == 0 {
			c.JSON(http.StatusNotFound, gin.H{"error": "Document type not found"})
			return
		}
	}

	// Audit log
	h.audit.LogEntity(ctx, audit.ActionDocumentTypeDelete, &userID, &tenantID, "document_type", &dtID, c.ClientIP(), map[string]interface{}{
		"soft_delete": documentCount > 0,
	})

	c.JSON(http.StatusOK, gin.H{"message": "Document type deleted"})
}

// GetCategories returns distinct categories for document types
// GET /api/v1/document-types/categories
func (h *DocumentTypeHandler) GetCategories(c *gin.Context) {
	ctx := c.Request.Context()
	tenantID, _ := middleware.GetTenantID(c)

	query := `
		SELECT DISTINCT category FROM document_types
		WHERE tenant_id = $1 AND is_active = true
		ORDER BY category ASC
	`

	rows, err := h.db.Query(ctx, query, tenantID)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get categories")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch categories"})
		return
	}
	defer rows.Close()

	var categories []string
	for rows.Next() {
		var cat string
		if err := rows.Scan(&cat); err != nil {
			continue
		}
		categories = append(categories, cat)
	}

	if categories == nil {
		categories = []string{}
	}

	c.JSON(http.StatusOK, gin.H{"categories": categories})
}
