package handlers

import (
	"crypto/rand"
	"encoding/hex"
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

type DocumentHandler struct {
	db    *database.Pool
	audit *audit.Logger
}

func NewDocumentHandler(db *database.Pool, auditLogger *audit.Logger) *DocumentHandler {
	return &DocumentHandler{
		db:    db,
		audit: auditLogger,
	}
}

// Document represents a document record
type Document struct {
	ID           uuid.UUID  `json:"id"`
	TenantID     uuid.UUID  `json:"tenant_id"`
	ClientID     *uuid.UUID `json:"client_id,omitempty"`
	ServiceID    *uuid.UUID `json:"service_id,omitempty"`
	UploadedBy   *uuid.UUID `json:"uploaded_by,omitempty"`
	TypeID       *uuid.UUID `json:"type_id,omitempty"`
	Name         string     `json:"name"`
	OriginalName string     `json:"original_name"`
	FilePath     *string    `json:"file_path,omitempty"`
	FileSize     *int       `json:"file_size,omitempty"`
	MimeType     *string    `json:"mime_type,omitempty"`
	Status       string     `json:"status"`
	Access       string     `json:"access"`
	Version      int        `json:"version"`
	ParentID     *uuid.UUID `json:"parent_id,omitempty"`
	RequestedAt  *time.Time `json:"requested_at,omitempty"`
	ExpiryDate   *string    `json:"expiry_date,omitempty"`
	RequestNote  *string    `json:"request_note,omitempty"`
	UploadNote   *string    `json:"upload_note,omitempty"`
	ReviewNote   *string    `json:"review_note,omitempty"`
	ReviewedBy   *uuid.UUID `json:"reviewed_by,omitempty"`
	ReviewedAt   *time.Time `json:"reviewed_at,omitempty"`
	ChaseCount   int        `json:"chase_count"`
	LastChasedAt *time.Time `json:"last_chased_at,omitempty"`
	AISummary    *string    `json:"ai_summary,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
	// Joined fields
	ClientName   *string `json:"client_name,omitempty"`
	TypeName     *string `json:"type_name,omitempty"`
	UploadedName *string `json:"uploaded_by_name,omitempty"`
}

type CreateDocumentRequest struct {
	ClientID    *string `json:"client_id,omitempty"`
	ServiceID   *string `json:"service_id,omitempty"`
	TypeID      *string `json:"type_id,omitempty"`
	Name        string  `json:"name" binding:"required"`
	ExpiryDate  *string `json:"expiry_date,omitempty"`
	RequestNote *string `json:"request_note,omitempty"`
	Status      *string `json:"status,omitempty"`
}

type UpdateDocumentRequest struct {
	Name        *string `json:"name,omitempty"`
	TypeID      *string `json:"type_id,omitempty"`
	ExpiryDate  *string `json:"expiry_date,omitempty"`
	RequestNote *string `json:"request_note,omitempty"`
	UploadNote  *string `json:"upload_note,omitempty"`
	Status      *string `json:"status,omitempty"`
}

// List returns all documents for the tenant (staff-scoped)
// GET /api/v1/documents
func (h *DocumentHandler) List(c *gin.Context) {
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
	clientID := c.Query("client_id")
	typeID := c.Query("type_id")
	search := c.Query("search")

	var query strings.Builder
	var args []interface{}
	argNum := 1

	// Don't fetch ai_extracted (TOAST performance)
	query.WriteString(`
		SELECT d.id, d.tenant_id, d.client_id, d.service_id, d.uploaded_by, d.type_id,
		       d.name, d.original_name, d.file_size, d.mime_type, d.status, d.access,
		       d.version, d.expiry_date, d.chase_count, d.last_chased_at, d.ai_summary,
		       d.created_at, d.updated_at,
		       c.company_name as client_name, dt.name as type_name, u.name as uploaded_by_name
		FROM documents d
		LEFT JOIN clients c ON d.client_id = c.id
		LEFT JOIN document_types dt ON d.type_id = dt.id
		LEFT JOIN users u ON d.uploaded_by = u.id
		WHERE d.tenant_id = $1 AND d.client_id IS NOT NULL
	`)
	args = append(args, tenantID)
	argNum++

	// Staff scoping - only see documents for assigned clients unless admin
	roleStr, _ := role.(string)
	if roleStr == "staff" {
		query.WriteString(` AND d.client_id IN (SELECT client_id FROM staff_clients WHERE staff_id = $`)
		query.WriteString(strconv.Itoa(argNum))
		query.WriteString(`)`)
		args = append(args, userID)
		argNum++
	}

	// Status filter
	if status != "" {
		query.WriteString(` AND d.status = $`)
		query.WriteString(strconv.Itoa(argNum))
		args = append(args, status)
		argNum++
	}

	// Client filter
	if clientID != "" {
		query.WriteString(` AND d.client_id = $`)
		query.WriteString(strconv.Itoa(argNum))
		args = append(args, clientID)
		argNum++
	}

	// Type filter
	if typeID != "" {
		query.WriteString(` AND d.type_id = $`)
		query.WriteString(strconv.Itoa(argNum))
		args = append(args, typeID)
		argNum++
	}

	// Search filter
	if search != "" {
		query.WriteString(` AND (d.name ILIKE $`)
		query.WriteString(strconv.Itoa(argNum))
		query.WriteString(` OR d.original_name ILIKE $`)
		query.WriteString(strconv.Itoa(argNum))
		query.WriteString(`)`)
		args = append(args, "%"+search+"%")
		argNum++
	}

	query.WriteString(` ORDER BY d.created_at DESC LIMIT $`)
	query.WriteString(strconv.Itoa(argNum))
	args = append(args, limit)
	argNum++

	query.WriteString(` OFFSET $`)
	query.WriteString(strconv.Itoa(argNum))
	args = append(args, offset)

	rows, err := h.db.Query(ctx, query.String(), args...)
	if err != nil {
		log.Error().Err(err).Msg("Failed to list documents")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}
	defer rows.Close()

	documents := []Document{}
	for rows.Next() {
		var doc Document
		var expiryDate *time.Time
		err := rows.Scan(
			&doc.ID, &doc.TenantID, &doc.ClientID, &doc.ServiceID, &doc.UploadedBy, &doc.TypeID,
			&doc.Name, &doc.OriginalName, &doc.FileSize, &doc.MimeType, &doc.Status, &doc.Access,
			&doc.Version, &expiryDate, &doc.ChaseCount, &doc.LastChasedAt, &doc.AISummary,
			&doc.CreatedAt, &doc.UpdatedAt, &doc.ClientName, &doc.TypeName, &doc.UploadedName,
		)
		if err != nil {
			log.Error().Err(err).Msg("Failed to scan document")
			continue
		}
		if expiryDate != nil {
			s := expiryDate.Format("2006-01-02")
			doc.ExpiryDate = &s
		}
		documents = append(documents, doc)
	}

	c.JSON(http.StatusOK, gin.H{
		"documents": documents,
		"limit":     limit,
		"offset":    offset,
	})
}

// Get returns a single document by ID
// GET /api/v1/documents/:id
func (h *DocumentHandler) Get(c *gin.Context) {
	ctx := c.Request.Context()
	tenantID, _ := middleware.GetTenantID(c)

	documentID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_document_id"})
		return
	}

	var doc Document
	var expiryDate *time.Time
	err = h.db.QueryRow(ctx, `
		SELECT d.id, d.tenant_id, d.client_id, d.service_id, d.uploaded_by, d.type_id,
		       d.name, d.original_name, d.file_path, d.file_size, d.mime_type, d.status, d.access,
		       d.version, d.parent_id, d.requested_at, d.expiry_date, d.request_note,
		       d.upload_note, d.review_note, d.reviewed_by, d.reviewed_at,
		       d.chase_count, d.last_chased_at, d.ai_summary, d.created_at, d.updated_at,
		       c.company_name as client_name, dt.name as type_name, u.name as uploaded_by_name
		FROM documents d
		LEFT JOIN clients c ON d.client_id = c.id
		LEFT JOIN document_types dt ON d.type_id = dt.id
		LEFT JOIN users u ON d.uploaded_by = u.id
		WHERE d.id = $1 AND d.tenant_id = $2
	`, documentID, tenantID).Scan(
		&doc.ID, &doc.TenantID, &doc.ClientID, &doc.ServiceID, &doc.UploadedBy, &doc.TypeID,
		&doc.Name, &doc.OriginalName, &doc.FilePath, &doc.FileSize, &doc.MimeType, &doc.Status, &doc.Access,
		&doc.Version, &doc.ParentID, &doc.RequestedAt, &expiryDate, &doc.RequestNote,
		&doc.UploadNote, &doc.ReviewNote, &doc.ReviewedBy, &doc.ReviewedAt,
		&doc.ChaseCount, &doc.LastChasedAt, &doc.AISummary, &doc.CreatedAt, &doc.UpdatedAt,
		&doc.ClientName, &doc.TypeName, &doc.UploadedName,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "document_not_found"})
			return
		}
		log.Error().Err(err).Msg("Failed to get document")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	if expiryDate != nil {
		s := expiryDate.Format("2006-01-02")
		doc.ExpiryDate = &s
	}

	c.JSON(http.StatusOK, doc)
}

// Create creates a new document (metadata only)
// POST /api/v1/documents
func (h *DocumentHandler) Create(c *gin.Context) {
	var req CreateDocumentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "validation_error", "message": err.Error()})
		return
	}

	ctx := c.Request.Context()
	tenantID, _ := middleware.GetTenantID(c)
	userID, _ := middleware.GetUserID(c)

	documentID := uuid.New()

	// Parse optional UUIDs
	var clientID, serviceID, typeID *uuid.UUID
	if req.ClientID != nil && *req.ClientID != "" {
		c, _ := uuid.Parse(*req.ClientID)
		clientID = &c
	}
	if req.ServiceID != nil && *req.ServiceID != "" {
		s, _ := uuid.Parse(*req.ServiceID)
		serviceID = &s
	}
	if req.TypeID != nil && *req.TypeID != "" {
		t, _ := uuid.Parse(*req.TypeID)
		typeID = &t
	}

	// Parse expiry date
	var expiryDate *time.Time
	if req.ExpiryDate != nil && *req.ExpiryDate != "" {
		t, err := time.Parse("2006-01-02", *req.ExpiryDate)
		if err == nil {
			expiryDate = &t
		}
	}

	status := "requested"
	if req.Status != nil {
		status = *req.Status
	}

	_, err := h.db.Exec(ctx, `
		INSERT INTO documents (
			id, tenant_id, client_id, service_id, uploaded_by, type_id,
			name, original_name, status, access, expiry_date, request_note, requested_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $7, $8, 'all_staff', $9, $10, NOW()
		)
	`, documentID, tenantID, clientID, serviceID, userID, typeID,
		req.Name, status, expiryDate, req.RequestNote)

	if err != nil {
		log.Error().Err(err).Msg("Failed to create document")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	// Audit log
	h.audit.LogEntity(ctx, audit.ActionDocumentCreate, &userID, &tenantID, "document", &documentID, c.ClientIP(), nil)

	c.JSON(http.StatusCreated, gin.H{
		"id":      documentID,
		"message": "Document created successfully",
	})
}

// Update updates an existing document
// PATCH /api/v1/documents/:id
func (h *DocumentHandler) Update(c *gin.Context) {
	documentID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_document_id"})
		return
	}

	var req UpdateDocumentRequest
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

	if req.Name != nil {
		setClauses = append(setClauses, "name = $"+strconv.Itoa(argNum))
		args = append(args, *req.Name)
		argNum++
	}
	if req.TypeID != nil {
		var typeID *uuid.UUID
		if *req.TypeID != "" {
			t, _ := uuid.Parse(*req.TypeID)
			typeID = &t
		}
		setClauses = append(setClauses, "type_id = $"+strconv.Itoa(argNum))
		args = append(args, typeID)
		argNum++
	}
	if req.ExpiryDate != nil {
		var expiryDate *time.Time
		if *req.ExpiryDate != "" {
			t, _ := time.Parse("2006-01-02", *req.ExpiryDate)
			expiryDate = &t
		}
		setClauses = append(setClauses, "expiry_date = $"+strconv.Itoa(argNum))
		args = append(args, expiryDate)
		argNum++
	}
	if req.RequestNote != nil {
		setClauses = append(setClauses, "request_note = $"+strconv.Itoa(argNum))
		args = append(args, *req.RequestNote)
		argNum++
	}
	if req.UploadNote != nil {
		setClauses = append(setClauses, "upload_note = $"+strconv.Itoa(argNum))
		args = append(args, *req.UploadNote)
		argNum++
	}
	if req.Status != nil {
		setClauses = append(setClauses, "status = $"+strconv.Itoa(argNum))
		args = append(args, *req.Status)
		argNum++
	}

	if len(setClauses) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no_fields_to_update"})
		return
	}

	setClauses = append(setClauses, "updated_at = NOW()")

	query := "UPDATE documents SET " + strings.Join(setClauses, ", ") +
		" WHERE id = $" + strconv.Itoa(argNum) + " AND tenant_id = $" + strconv.Itoa(argNum+1)
	args = append(args, documentID, tenantID)

	result, err := h.db.Exec(ctx, query, args...)
	if err != nil {
		log.Error().Err(err).Msg("Failed to update document")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	if result.RowsAffected() == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "document_not_found"})
		return
	}

	// Audit log
	h.audit.LogEntity(ctx, audit.ActionDocumentUpdate, &userID, &tenantID, "document", &documentID, c.ClientIP(), nil)

	c.JSON(http.StatusOK, gin.H{"message": "Document updated successfully"})
}

// Approve approves a document
// POST /api/v1/documents/:id/approve
func (h *DocumentHandler) Approve(c *gin.Context) {
	documentID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_document_id"})
		return
	}

	var req struct {
		Note string `json:"note,omitempty"`
	}
	_ = c.ShouldBindJSON(&req)

	ctx := c.Request.Context()
	tenantID, _ := middleware.GetTenantID(c)
	userID, _ := middleware.GetUserID(c)

	result, err := h.db.Exec(ctx, `
		UPDATE documents SET status = 'approved', review_note = $1,
		       reviewed_by = $2, reviewed_at = NOW(), updated_at = NOW()
		WHERE id = $3 AND tenant_id = $4
	`, req.Note, userID, documentID, tenantID)

	if err != nil {
		log.Error().Err(err).Msg("Failed to approve document")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	if result.RowsAffected() == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "document_not_found"})
		return
	}

	// Audit log
	h.audit.LogEntity(ctx, audit.ActionDocumentApprove, &userID, &tenantID, "document", &documentID, c.ClientIP(), nil)

	c.JSON(http.StatusOK, gin.H{"message": "Document approved successfully"})
}

// Reject rejects a document
// POST /api/v1/documents/:id/reject
func (h *DocumentHandler) Reject(c *gin.Context) {
	documentID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_document_id"})
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

	result, err := h.db.Exec(ctx, `
		UPDATE documents SET status = 'rejected', review_note = $1,
		       reviewed_by = $2, reviewed_at = NOW(), updated_at = NOW()
		WHERE id = $3 AND tenant_id = $4
	`, req.Note, userID, documentID, tenantID)

	if err != nil {
		log.Error().Err(err).Msg("Failed to reject document")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	if result.RowsAffected() == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "document_not_found"})
		return
	}

	// Audit log
	h.audit.LogEntity(ctx, audit.ActionDocumentReject, &userID, &tenantID, "document", &documentID, c.ClientIP(), nil)

	c.JSON(http.StatusOK, gin.H{"message": "Document rejected successfully"})
}

// GetVersions returns version history for a document
// GET /api/v1/documents/:id/versions
func (h *DocumentHandler) GetVersions(c *gin.Context) {
	documentID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_document_id"})
		return
	}

	ctx := c.Request.Context()
	tenantID, _ := middleware.GetTenantID(c)

	rows, err := h.db.Query(ctx, `
		WITH RECURSIVE version_chain AS (
			SELECT id, parent_id, version, name, file_size, created_at, uploaded_by
			FROM documents
			WHERE id = $1 AND tenant_id = $2
			UNION ALL
			SELECT d.id, d.parent_id, d.version, d.name, d.file_size, d.created_at, d.uploaded_by
			FROM documents d
			INNER JOIN version_chain vc ON d.id = vc.parent_id
			WHERE d.tenant_id = $2
		)
		SELECT vc.id, vc.version, vc.name, vc.file_size, vc.created_at, u.name as uploaded_by_name
		FROM version_chain vc
		LEFT JOIN users u ON vc.uploaded_by = u.id
		ORDER BY vc.version DESC
	`, documentID, tenantID)

	if err != nil {
		log.Error().Err(err).Msg("Failed to get document versions")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}
	defer rows.Close()

	var versions []map[string]interface{}
	for rows.Next() {
		var id uuid.UUID
		var version int
		var name string
		var fileSize *int
		var createdAt time.Time
		var uploadedByName *string

		if err := rows.Scan(&id, &version, &name, &fileSize, &createdAt, &uploadedByName); err != nil {
			continue
		}

		versions = append(versions, map[string]interface{}{
			"id":               id,
			"version":          version,
			"name":             name,
			"file_size":        fileSize,
			"created_at":       createdAt,
			"uploaded_by_name": uploadedByName,
		})
	}

	c.JSON(http.StatusOK, gin.H{"versions": versions})
}

// RestoreVersion restores a previous version
// POST /api/v1/documents/:id/versions/:versionId/restore
func (h *DocumentHandler) RestoreVersion(c *gin.Context) {
	documentID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_document_id"})
		return
	}

	versionID, err := uuid.Parse(c.Param("versionId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_version_id"})
		return
	}

	ctx := c.Request.Context()
	tenantID, _ := middleware.GetTenantID(c)
	userID, _ := middleware.GetUserID(c)

	// Get current document version
	var currentVersion int
	err = h.db.QueryRow(ctx, `
		SELECT version FROM documents WHERE id = $1 AND tenant_id = $2
	`, documentID, tenantID).Scan(&currentVersion)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "document_not_found"})
		return
	}

	// Get the version to restore
	var oldFilePath, oldMimeType *string
	var oldFileSize *int
	err = h.db.QueryRow(ctx, `
		SELECT file_path, file_size, mime_type FROM documents WHERE id = $1 AND tenant_id = $2
	`, versionID, tenantID).Scan(&oldFilePath, &oldFileSize, &oldMimeType)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "version_not_found"})
		return
	}

	// Create new version with restored content
	newID := uuid.New()
	_, err = h.db.Exec(ctx, `
		INSERT INTO documents (
			id, tenant_id, client_id, service_id, uploaded_by, type_id,
			name, original_name, file_path, file_size, mime_type, status, access,
			version, parent_id
		)
		SELECT $1, tenant_id, client_id, service_id, $2, type_id,
		       name, original_name, $3, $4, $5, 'uploaded', access,
		       $6, $7
		FROM documents WHERE id = $7 AND tenant_id = $8
	`, newID, userID, oldFilePath, oldFileSize, oldMimeType, currentVersion+1, documentID, tenantID)

	if err != nil {
		log.Error().Err(err).Msg("Failed to restore document version")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	// Audit log
	h.audit.LogEntity(ctx, audit.ActionDocumentUpdate, &userID, &tenantID, "document", &newID, c.ClientIP(), map[string]interface{}{
		"action":           "version_restored",
		"restored_from_id": versionID,
	})

	c.JSON(http.StatusOK, gin.H{
		"id":      newID,
		"message": "Version restored successfully",
	})
}

// BulkRequest requests multiple documents at once
// POST /api/v1/documents/bulk-request
func (h *DocumentHandler) BulkRequest(c *gin.Context) {
	var req struct {
		ClientID string `json:"client_id" binding:"required,uuid"`
		TypeIDs  []string `json:"type_ids" binding:"required"`
		Note     *string  `json:"note,omitempty"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "validation_error", "message": err.Error()})
		return
	}

	ctx := c.Request.Context()
	tenantID, _ := middleware.GetTenantID(c)
	userID, _ := middleware.GetUserID(c)

	clientID, _ := uuid.Parse(req.ClientID)
	var created []uuid.UUID

	for _, typeIDStr := range req.TypeIDs {
		typeID, err := uuid.Parse(typeIDStr)
		if err != nil {
			continue
		}

		// Get type name for document name
		var typeName string
		err = h.db.QueryRow(ctx, `SELECT name FROM document_types WHERE id = $1`, typeID).Scan(&typeName)
		if err != nil {
			continue
		}

		documentID := uuid.New()
		_, err = h.db.Exec(ctx, `
			INSERT INTO documents (
				id, tenant_id, client_id, uploaded_by, type_id,
				name, original_name, status, access, request_note, requested_at
			) VALUES (
				$1, $2, $3, $4, $5, $6, $6, 'requested', 'all_staff', $7, NOW()
			)
		`, documentID, tenantID, clientID, userID, typeID, typeName, req.Note)

		if err != nil {
			log.Error().Err(err).Str("type_id", typeIDStr).Msg("Failed to create document request")
			continue
		}

		created = append(created, documentID)
	}

	// Audit log
	h.audit.LogEntity(ctx, audit.ActionDocumentCreate, &userID, &tenantID, "document", nil, c.ClientIP(), map[string]interface{}{
		"bulk":       true,
		"count":      len(created),
		"client_id":  req.ClientID,
	})

	c.JSON(http.StatusCreated, gin.H{
		"created": created,
		"count":   len(created),
		"message": "Document requests created successfully",
	})
}

// BulkApprove approves multiple documents at once
// POST /api/v1/documents/bulk-approve
func (h *DocumentHandler) BulkApprove(c *gin.Context) {
	var req struct {
		DocumentIDs []string `json:"document_ids" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "validation_error", "message": err.Error()})
		return
	}

	ctx := c.Request.Context()
	tenantID, _ := middleware.GetTenantID(c)
	userID, _ := middleware.GetUserID(c)

	var documentIDs []uuid.UUID
	for _, id := range req.DocumentIDs {
		if uid, err := uuid.Parse(id); err == nil {
			documentIDs = append(documentIDs, uid)
		}
	}

	result, err := h.db.Exec(ctx, `
		UPDATE documents SET status = 'approved',
		       reviewed_by = $1, reviewed_at = NOW(), updated_at = NOW()
		WHERE id = ANY($2) AND tenant_id = $3 AND status = 'pending_review'
	`, userID, documentIDs, tenantID)

	if err != nil {
		log.Error().Err(err).Msg("Failed to bulk approve documents")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	// Audit log
	h.audit.LogEntity(ctx, audit.ActionDocumentApprove, &userID, &tenantID, "document", nil, c.ClientIP(), map[string]interface{}{
		"bulk":  true,
		"count": result.RowsAffected(),
	})

	c.JSON(http.StatusOK, gin.H{
		"approved": result.RowsAffected(),
		"message":  "Documents approved successfully",
	})
}

// ListFirm returns firm-level documents (no client_id)
// GET /api/v1/documents/firm
func (h *DocumentHandler) ListFirm(c *gin.Context) {
	ctx := c.Request.Context()
	tenantID, _ := middleware.GetTenantID(c)
	userID, _ := middleware.GetUserID(c)
	role, _ := c.Get(middleware.AuthRole)

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if limit > 100 {
		limit = 100
	}

	var query strings.Builder
	var args []interface{}
	argNum := 1

	query.WriteString(`
		SELECT d.id, d.name, d.original_name, d.file_size, d.mime_type, d.status, d.access,
		       d.created_at, d.updated_at, u.name as uploaded_by_name
		FROM documents d
		LEFT JOIN users u ON d.uploaded_by = u.id
		WHERE d.tenant_id = $1 AND d.client_id IS NULL
	`)
	args = append(args, tenantID)
	argNum++

	// Check access permissions for non-admin
	roleStr, _ := role.(string)
	if roleStr != "tenant_admin" && roleStr != "super_admin" {
		query.WriteString(` AND (d.access = 'all_staff' OR d.access = 'admin' AND EXISTS (
			SELECT 1 FROM document_access da WHERE da.document_id = d.id AND da.staff_id = $`)
		query.WriteString(strconv.Itoa(argNum))
		query.WriteString(`))`)
		args = append(args, userID)
		argNum++
	}

	query.WriteString(` ORDER BY d.created_at DESC LIMIT $`)
	query.WriteString(strconv.Itoa(argNum))
	args = append(args, limit)
	argNum++

	query.WriteString(` OFFSET $`)
	query.WriteString(strconv.Itoa(argNum))
	args = append(args, offset)

	rows, err := h.db.Query(ctx, query.String(), args...)
	if err != nil {
		log.Error().Err(err).Msg("Failed to list firm documents")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}
	defer rows.Close()

	var documents []map[string]interface{}
	for rows.Next() {
		var id uuid.UUID
		var name, originalName, status, access string
		var fileSize *int
		var mimeType, uploadedByName *string
		var createdAt, updatedAt time.Time

		if err := rows.Scan(&id, &name, &originalName, &fileSize, &mimeType, &status, &access,
			&createdAt, &updatedAt, &uploadedByName); err != nil {
			continue
		}

		documents = append(documents, map[string]interface{}{
			"id":               id,
			"name":             name,
			"original_name":    originalName,
			"file_size":        fileSize,
			"mime_type":        mimeType,
			"status":           status,
			"access":           access,
			"created_at":       createdAt,
			"updated_at":       updatedAt,
			"uploaded_by_name": uploadedByName,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"documents": documents,
		"limit":     limit,
		"offset":    offset,
	})
}

// GenerateQRToken generates a secure upload token for QR code
// POST /api/v1/documents/qr
func (h *DocumentHandler) GenerateQRToken(c *gin.Context) {
	var req struct {
		ClientID string   `json:"client_id" binding:"required,uuid"`
		TypeIDs  []string `json:"type_ids,omitempty"`
		Note     *string  `json:"note,omitempty"`
		ExpiresIn *int    `json:"expires_in,omitempty"` // minutes
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "validation_error", "message": err.Error()})
		return
	}

	ctx := c.Request.Context()
	tenantID, _ := middleware.GetTenantID(c)
	userID, _ := middleware.GetUserID(c)

	clientID, _ := uuid.Parse(req.ClientID)

	// Generate secure token
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}
	token := hex.EncodeToString(tokenBytes)

	// Default expiry: 24 hours
	expiresIn := 24 * 60 // minutes
	if req.ExpiresIn != nil && *req.ExpiresIn > 0 {
		expiresIn = *req.ExpiresIn
	}

	tokenID := uuid.New()
	_, err := h.db.Exec(ctx, `
		INSERT INTO upload_tokens (id, tenant_id, client_id, token, created_by, note, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, NOW() + $7 * INTERVAL '1 minute')
	`, tokenID, tenantID, clientID, token, userID, req.Note, expiresIn)

	if err != nil {
		log.Error().Err(err).Msg("Failed to create upload token")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"token":      token,
		"expires_in": expiresIn,
		"upload_url": "/api/v1/documents/qr/" + token + "/upload",
	})
}

// GetQRToken validates and returns QR token info (PUBLIC)
// GET /api/v1/documents/qr/:token
func (h *DocumentHandler) GetQRToken(c *gin.Context) {
	token := c.Param("token")

	ctx := c.Request.Context()

	var clientName string
	var expiresAt time.Time
	var note *string
	err := h.db.QueryRow(ctx, `
		SELECT c.company_name, ut.expires_at, ut.note
		FROM upload_tokens ut
		JOIN clients c ON ut.client_id = c.id
		WHERE ut.token = $1 AND ut.expires_at > NOW() AND ut.used_at IS NULL
	`, token).Scan(&clientName, &expiresAt, &note)

	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "invalid_or_expired_token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"valid":       true,
		"client_name": clientName,
		"expires_at":  expiresAt,
		"note":        note,
	})
}
